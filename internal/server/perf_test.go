package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/perf"
	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/redact"
	"github.com/niclasedge/tollgate/internal/store"
)

// sseAnswer streams one answer: message_start, an optional pause, the first
// content_block_delta, another pause, then usage and stop.
func sseAnswer(before, after time.Duration, output int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(s string) { _, _ = io.WriteString(w, s); w.(http.Flusher).Flush() }
		send("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":100}}}\n\n")
		send("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		time.Sleep(before)
		send("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"1, 2\"}}\n\n")
		time.Sleep(after)
		send("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		send(fmt.Sprintf("event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":%d}}\n\n", output))
		send("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}
}

const streamBody = `{"model":"claude-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`

func TestTTFTOfStreamEndsAtFirstContentDelta(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, sseAnswer(300*time.Millisecond, 100*time.Millisecond, 200))
	ts := newProxy(t, up, proxyOpts{store: st})
	if resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", streamBody, auth(nil)); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	tr := waitFinished(t, st)
	if tr.TTFTMicros == nil || *tr.TTFTMicros < 300_000 || *tr.TTFTMicros > tr.DurationMicros-100_000 {
		t.Fatalf("ttft %v, duration %d", tr.TTFTMicros, tr.DurationMicros)
	}
	if tr.ReturnMicros != nil || tr.ReturnReplacements != nil {
		t.Fatalf("no return path ran, got %v %v", tr.ReturnMicros, tr.ReturnReplacements)
	}
}

// With hidden thinking the first content delta only comes after all thinking
// tokens. Tokens/s starts at message_start, so it stays plausible (#116).
func TestGenerationStartIsMessageStart(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, sseAnswer(400*time.Millisecond, 0, 200))
	ts := newProxy(t, up, proxyOpts{store: st})
	doReq(t, http.MethodPost, ts.URL+"/v1/messages", streamBody, auth(nil))
	tr := waitFinished(t, st)
	if tr.GenStartMicros == nil || tr.TTFTMicros == nil || *tr.GenStartMicros > *tr.TTFTMicros-300_000 {
		t.Fatalf("gen start %v, ttft %v", tr.GenStartMicros, tr.TTFTMicros)
	}
	tps, ok := perf.OutputTPS(tr.Usage.Output, tr.DurationMicros, *tr.GenStartMicros, true, tr.Stream)
	if !ok || tps > 600 {
		t.Fatalf("tps %v %v (200 tokens in about 0.4 s)", tps, ok)
	}
}

func TestTTFTOfJSONIsHeaderTime(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"usage":{"input_tokens":1,"output_tokens":2}}`)
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	tr := waitFinished(t, st)
	if tr.TTFTMicros == nil || *tr.TTFTMicros < 200_000 || *tr.TTFTMicros > tr.DurationMicros {
		t.Fatalf("ttft %v, duration %d", tr.TTFTMicros, tr.DurationMicros)
	}
}

func TestErrorStatusHasNoTTFTAndNoVerdict(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"type":"error"}`)
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	tr := waitFinished(t, st)
	if tr.Status != 429 || tr.TTFTMicros != nil || tr.GenStartMicros != nil || tr.Perf != nil || tr.TPS != nil {
		t.Fatalf("trace %+v", tr)
	}
}

// After 20 fast streams a stream with a long generation is judged slow (#87)
// and slower in tokens/s (#116); the finish event already carries both.
func TestSlowRequestIsJudgedAfterTwentyFast(t *testing.T) {
	st := openStore(t)
	var slow sync.Mutex
	slowNext := false
	fast, long := sseAnswer(0, 20*time.Millisecond, 300), sseAnswer(0, 1300*time.Millisecond, 300)
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		slow.Lock()
		s := slowNext
		slow.Unlock()
		if s {
			long(w, r)
			return
		}
		fast(w, r)
	})
	var mu sync.Mutex
	var events []store.Trace
	ts := newProxy(t, up, proxyOpts{store: st, observe: func(tr store.Trace) {
		mu.Lock()
		events = append(events, tr)
		mu.Unlock()
	}})
	for i := 0; i < perf.MinSamples; i++ {
		doReq(t, http.MethodPost, ts.URL+"/v1/messages", streamBody, auth(nil))
		time.Sleep(3 * time.Millisecond) // distinct started_at
	}
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(events) == 2*perf.MinSamples })
	slow.Lock()
	slowNext = true
	slow.Unlock()
	doReq(t, http.MethodPost, ts.URL+"/v1/messages", streamBody, auth(nil))
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(events) == 2*perf.MinSamples+2 })
	mu.Lock()
	first, last := events[1], events[len(events)-1]
	mu.Unlock()
	if len(first.Perf) != 2 || first.Perf[0].Verdict != "unbekannt" || first.TPS == nil || first.TPS.Verdict != "unbekannt" {
		t.Fatalf("first verdict %+v %+v", first.Perf, first.TPS)
	}
	var dur store.Perf
	for _, p := range last.Perf {
		if p.Metric == perf.MetricDuration {
			dur = p
		}
	}
	if dur.Verdict != "langsam" || dur.Samples != perf.MinSamples || dur.Class != "i0o1" {
		t.Fatalf("duration verdict %+v", last.Perf)
	}
	if last.TPS == nil || last.TPS.Verdict != "langsamer" || last.TPS.Samples != perf.MinSamples {
		t.Fatalf("tps verdict %+v", last.TPS)
	}
	stored, err := st.GetTrace(context.Background(), last.ID)
	if err != nil || len(stored.Perf) != 2 || stored.TPS == nil || stored.TPS.Verdict != "langsamer" {
		t.Fatalf("stored %+v %v", stored, err)
	}
}

type failingPerf struct{ *store.Store }

func (failingPerf) PerfSamples(context.Context, string, time.Time, string) ([]perf.Sample, error) {
	return nil, errors.New("disk full")
}

func (failingPerf) TPSSamples(context.Context, string, time.Time) ([]perf.TPSSample, error) {
	return nil, errors.New("disk full")
}

func TestVerdictFailureDoesNotBlock(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, sseAnswer(0, 0, 300))
	done := make(chan store.Trace, 2)
	ts := newProxy(t, up, proxyOpts{store: failingPerf{st}, observe: func(tr store.Trace) {
		if tr.Status != 0 {
			done <- tr
		}
	}})
	if resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", streamBody, auth(nil)); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	select {
	case tr := <-done:
		if tr.Perf != nil || tr.TPS != nil || tr.TTFTMicros == nil {
			t.Fatalf("event %+v", tr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no finish event")
	}
}

func TestStageBytesAndReturnStatsAreRecorded(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"+
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"a <redacted:email_1> b <redacted:email_1>\"}}\n\n"+
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		w.(http.Flusher).Flush() // chunked like a real stream
	})
	ts := newProxy(t, up, proxyOpts{store: st}, redact.NewStage(redact.Builtin()))
	body := `{"model":"claude-test","stream":true,"messages":[{"role":"user","content":"mail ` + speedtestAddress + `"}]}`
	if resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", body, auth(nil)); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	tr := waitFinished(t, st)
	if tr.ReturnReplacements == nil || *tr.ReturnReplacements != 2 || tr.ReturnMicros == nil {
		t.Fatalf("return stats %v %v", tr.ReturnMicros, tr.ReturnReplacements)
	}
	rd := tr.Stages[1]
	if rd.Name != "redact" || rd.BytesIn != int64(len(body)) || rd.BytesOut == rd.BytesIn {
		t.Fatalf("redact stage bytes %+v", rd)
	}
}

func newProbeServer(t *testing.T, st *store.Store, up *mockUpstream, logs io.Writer) *Server {
	t.Helper()
	srv, err := New(Options{
		Runner: pipeline.NewRunner([]pipeline.Stage{&pipeline.Intake{Keys: fakeKeys{clientKey: "client-1"}},
			redact.NewStage(redact.Builtin())}),
		Store: st, UpstreamURL: up.URL, UpstreamKey: cpaKey,
		Logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestProbeRunsWithAndWithoutRedact(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, sseAnswer(0, 0, 200))
	logs := &syncBuffer{}
	srv := newProbeServer(t, st, up, logs)
	ctx := context.Background()
	for _, skip := range []bool{true, false} {
		id, err := srv.Probe(ctx, ProbeInput{Body: SpeedtestBody("claude-test"), SkipRedact: skip})
		if err != nil {
			t.Fatal(err)
		}
		tr, err := st.GetTrace(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		hasRedact := false
		for _, sg := range tr.Stages {
			hasRedact = hasRedact || sg.Name == "redact"
		}
		if tr.ClientID != store.SpeedtestClientID || tr.Status != 200 || tr.TTFTMicros == nil || hasRedact == skip {
			t.Fatalf("skip=%v: %+v", skip, tr)
		}
		if tr.Perf != nil || tr.TPS != nil {
			t.Fatalf("speedtest must not be judged: %+v", tr)
		}
		if !skip && len(tr.Stages[1].Matches) != 1 {
			t.Fatalf("speedtest address not detected: %+v", tr.Stages[1])
		}
	}
	seen := up.seen()
	if len(seen) != 2 || !bytes.Contains(seen[0].Body, []byte(speedtestAddress)) || bytes.Contains(seen[1].Body, []byte(speedtestAddress)) {
		t.Fatal("upstream bodies: first must be plain, second redacted")
	}
	if ua := seen[0].Header.Get("User-Agent"); ua != "tollgate-speedtest" {
		t.Fatalf("user agent %q", ua)
	}
	// Leak test: the prompt text and the address are in neither DB nor log.
	list, _ := st.ListTracesWithClient(ctx, store.ListTracesFilter{})
	dump := fmt.Sprintf("%+v", list) + logs.String()
	for _, bad := range []string{speedtestAddress, "Antworte nur"} {
		if strings.Contains(dump, bad) {
			t.Fatalf("leak of %q", bad)
		}
	}
}

func TestSpeedtestClientIsNotReachableOverHTTP(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{store: st})
	resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, map[string]string{
		"Authorization": "Bearer " + store.SpeedtestClientID, "X-Internal-Client": store.SpeedtestClientID})
	if resp.StatusCode != http.StatusUnauthorized || len(up.seen()) != 0 {
		t.Fatalf("status %d, upstream calls %d", resp.StatusCode, len(up.seen()))
	}
}
