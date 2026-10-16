package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/store"
)

func i64(v int64) *int64 { return &v }

// perfRec records a finished streamed messages trace with TTFT.
func perfRec(t *testing.T, st *store.Store, id, client, model string, at time.Time, dur, ttft, output, cacheRead int64) {
	t.Helper()
	ctx := context.Background()
	tr := store.Trace{ID: id, ClientID: client, StartedAt: at, Endpoint: "messages", Model: model, Stream: true,
		Stages: []store.Stage{{Name: "intake", DurationMicros: 100, HashIn: hexOf('a'), HashOut: hexOf('a')}}}
	if err := st.RecordTrace(ctx, tr); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishTrace(ctx, id, store.TraceResult{Status: 200, DurationMicros: dur, TTFTMicros: i64(ttft), GenStartMicros: i64(ttft),
		Usage: &store.Usage{Input: 1000 - cacheRead, Output: output, CacheRead: cacheRead}}); err != nil {
		t.Fatal(err)
	}
}

func post(t *testing.T, url, body string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestPerformanceReport(t *testing.T) {
	e := newEnv(t, nil)
	addClient(t, e.st, "c1")
	at := midnight.Add(time.Hour)
	// Model A: 3 requests with 40, 50, 60 t/s (200 output tokens after 1 s TTFT).
	perfRec(t, e.st, "a1", "c1", "model-a", at, 1_000_000+5_000_000, 1_000_000, 200, 950)
	perfRec(t, e.st, "a2", "c1", "model-a", at.Add(time.Second), 1_000_000+4_000_000, 1_000_000, 200, 950)
	perfRec(t, e.st, "a3", "c1", "model-a", at.Add(2*time.Second), 1_000_000+3_333_334, 1_000_000, 200, 0)
	perfRec(t, e.st, "b1", "c1", "model-b", at, 2_000_000, 500_000, 10, 0)
	perfRec(t, e.st, "s1", store.SpeedtestClientID, "model-a", at.Add(3*time.Second), 9_000_000, 2_000_000, 200, 0)
	// Older than days=1: not in the report.
	perfRec(t, e.st, "old", "c1", "model-b", midnight.Add(-time.Hour), 2_000_000, 500_000, 10, 0)

	var rep PerfReport
	getJSON(t, e.srv.URL+"/api/performance?days=1", &rep)
	if len(rep.Models) != 2 || rep.Models[0].Model != "model-a" || rep.Models[0].Requests != 3 || rep.Models[1].Requests != 1 {
		t.Fatalf("models %+v", rep.Models)
	}
	a := rep.Models[0]
	if a.TTFT == nil || a.TTFT.MedianUs != 1_000_000 || a.Duration.MinUs != 4_333_334 || a.Duration.MaxUs != 6_000_000 {
		t.Fatalf("dist %+v %+v", a.TTFT, a.Duration)
	}
	if a.OutputTPS == nil || a.OutputTPS.Median != 50 || a.OutputTPS.Min != 40 || a.OutputTPS.Max != 60 {
		t.Fatalf("tps %+v", a.OutputTPS)
	}
	if len(a.Days) != 1 || a.Days[0].TPSMedianMilli == nil || *a.Days[0].TPSMedianMilli != 50_000 ||
		*a.Days[0].CacheSharePermilleMedian != 950 {
		t.Fatalf("days %+v", a.Days)
	}
	// Hourly curve: only hours with judgeable tokens/s; model B has 10 tokens.
	if len(a.Hours) != 1 || a.Hours[0] != (HourPerf{Hour: "2025-10-10T01:00:00Z", Samples: 3, TPSMedianMilli: 50_000}) {
		t.Fatalf("hours %+v", a.Hours)
	}
	if b := rep.Models[1]; b.Hours == nil || len(b.Hours) != 0 {
		t.Fatalf("model b hours %+v", b.Hours)
	}
	if len(a.Influence.Cells) != 12 || a.Influence.Cells[0].Samples != 1 || a.Influence.Cells[2].Samples != 2 || a.Influence.Cells[2].TTFTMedianUs != nil {
		t.Fatalf("influence %+v", a.Influence)
	}
	if a.Tollgate == nil || a.Tollgate.MedianUs != 100 {
		t.Fatalf("tollgate %+v", a.Tollgate)
	}
	if rep.Redactor != nil {
		t.Fatalf("no redact stage, want null: %+v", rep.Redactor)
	}
	if len(rep.Speedtests) != 1 || rep.Speedtests[0].TraceID != "s1" || rep.Speedtests[0].UsualDurationMedian == nil ||
		*rep.Speedtests[0].UsualDurationMedian != 5_000_000 {
		t.Fatalf("speedtests %+v", rep.Speedtests)
	}
	if code, _ := get(t, e.srv.URL+"/api/performance?days=91", adminToken); code != 400 {
		t.Fatalf("days=91: %d", code)
	}
	if code, _ := get(t, e.srv.URL+"/api/performance", ""); code != 401 {
		t.Fatalf("without token: %d", code)
	}
}

func TestTraceSummaryPerfFields(t *testing.T) {
	e := newEnv(t, nil)
	addClient(t, e.st, "c1")
	perfRec(t, e.st, "a1", "c1", "model-a", midnight, 5_000_000, 1_000_000, 200, 0)
	record(t, e.st, rec{id: "old", client: "c1", model: "m", at: midnight.Add(time.Second), usage: &store.Usage{Output: 1}})
	ctx := context.Background()
	med, p90 := int64(4_200_000), int64(6_100_000)
	if err := e.st.PutPerf(ctx, "a1", []store.Perf{{Metric: "duration", Class: "i0o0", Samples: 37, MedianMicros: &med, P90Micros: &p90, ValueMicros: 5_000_000, Verdict: "langsam"}}); err != nil {
		t.Fatal(err)
	}
	m := int64(62_000)
	if err := e.st.PutTPS(ctx, "a1", store.TPS{Class: "*", Samples: 87, TPSMilli: 50_000, MedianMilli: &m, P10Milli: &m, P90Milli: &m, DeviationPermille: i64(-193), Verdict: "üblich"}); err != nil {
		t.Fatal(err)
	}
	var l TraceList
	getJSON(t, e.srv.URL+"/api/traces", &l)
	if len(l.Traces) != 2 {
		t.Fatalf("traces %+v", l.Traces)
	}
	old, a := l.Traces[0], l.Traces[1]
	if old.TTFTMs != nil || old.OutputTPS != nil || old.Perf != nil || old.TPS != nil {
		t.Fatalf("old trace %+v", old)
	}
	if a.TTFTMs == nil || *a.TTFTMs != 1000 || a.OutputTPS == nil || *a.OutputTPS != 50 || a.TollgateMs == nil || *a.TollgateMs != 0.1 {
		t.Fatalf("values %+v", a)
	}
	if a.Perf == nil || !a.Perf.Slow || a.Perf.Duration == nil || *a.Perf.Duration.MedianMs != 4200 || a.Perf.TTFT != nil {
		t.Fatalf("perf %+v", a.Perf)
	}
	if a.TPS == nil || a.TPS.Verdict != "üblich" || *a.TPS.Median != 62 || *a.TPS.DeviationPermille != -193 {
		t.Fatalf("tps %+v", a.TPS)
	}
}

func TestSpeedtestEndpoint(t *testing.T) {
	var mu sync.Mutex
	var calls []bool
	release := make(chan struct{})
	block := false
	var st *store.Store
	e := newEnv(t, func(o *Options) {
		st = o.Store.(*store.Store)
		o.RedactEnabled = true
		o.Probe = func(ctx context.Context, model string, skip bool) (string, error) {
			mu.Lock()
			calls = append(calls, skip)
			n, b := len(calls), block
			mu.Unlock()
			if b {
				<-release
			}
			id := "p" + string(rune('0'+n))
			tr := store.Trace{ID: id, ClientID: store.SpeedtestClientID, StartedAt: midnight.Add(time.Duration(n) * time.Second),
				Endpoint: "messages", Model: model, Stream: true}
			if !skip {
				tr.Stages = []store.Stage{{Name: "redact", HashIn: hexOf('a'), HashOut: hexOf('b')}}
			}
			if err := st.RecordTrace(ctx, tr); err != nil {
				return "", err
			}
			return id, st.FinishTrace(ctx, id, store.TraceResult{Status: 200, DurationMicros: 5_000_000, TTFTMicros: i64(1_000_000), GenStartMicros: i64(1_000_000), Usage: &store.Usage{Output: 200}})
		}
	})
	if code, body := post(t, e.srv.URL+"/api/speedtest", `{"models":["claude-opus-5-5"]}`); code != 400 || !strings.Contains(string(body), "Abo-Kontingent") || len(calls) != 0 {
		t.Fatalf("without confirm: %d %s, calls %d", code, body, len(calls))
	}
	for _, bad := range []string{`{"models":[],"confirm":true}`, `{"models":["a","b","c","d","e"],"confirm":true}`, `{"models":["a b"],"confirm":true}`, `{"models":["a"],"confirm":true,"x":1}`} {
		if code, _ := post(t, e.srv.URL+"/api/speedtest", bad); code != 400 {
			t.Errorf("%s: %d", bad, code)
		}
	}
	code, body := post(t, e.srv.URL+"/api/speedtest", `{"models":["claude-opus-5-5"],"confirm":true}`)
	var res SpeedtestResponse
	if code != 200 || json.Unmarshal(body, &res) != nil || len(res.Results) != 2 {
		t.Fatalf("run: %d %s", code, body)
	}
	if r := res.Results[0]; r.Redact || r.TraceID != "p1" || *r.TTFTUs != 1_000_000 || *r.OutputTPS != 50 || r.Error != "" {
		t.Fatalf("without redact %+v", r)
	}
	if r := res.Results[1]; !r.Redact || r.TraceID != "p2" || r.Error != "" {
		t.Fatalf("with redact %+v", r)
	}
	if len(calls) != 2 || !calls[0] || calls[1] {
		t.Fatalf("calls %v (skip first, then redact)", calls)
	}
	// Parallel: the second call gets 409 while the first runs.
	mu.Lock()
	block = true
	mu.Unlock()
	done := make(chan int)
	go func() { c, _ := post(t, e.srv.URL+"/api/speedtest", `{"models":["m2"],"confirm":true}`); done <- c }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(calls)
		mu.Unlock()
		if n == 3 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if code, _ := post(t, e.srv.URL+"/api/speedtest", `{"models":["m3"],"confirm":true}`); code != 409 {
		t.Fatalf("parallel: %d", code)
	}
	close(release)
	if c := <-done; c != 200 {
		t.Fatalf("first: %d", c)
	}
}

func TestPerformanceRedactorCost(t *testing.T) {
	e := newEnv(t, nil)
	addClient(t, e.st, "c1")
	ctx := context.Background()
	for i, ret := range []int64{300, 500, 700} {
		id := "r" + string(rune('1'+i))
		tr := store.Trace{ID: id, ClientID: "c1", StartedAt: midnight.Add(time.Duration(i) * time.Second), Endpoint: "messages", Model: "m", Stream: true,
			Stages: []store.Stage{{Name: "redact", DurationMicros: 200 * int64(i+1), BytesIn: 1000, BytesOut: 1010, TokensIn: 250, TokensOut: 253,
				HashIn: hexOf('a'), HashOut: hexOf('b'), Matches: []store.Match{{Offset: 1, Length: 20, Type: "email", Rule: "email", Placeholder: "<redacted:email_1>"}}}}}
		if err := e.st.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
		if err := e.st.FinishTrace(ctx, id, store.TraceResult{Status: 200, DurationMicros: 10_000, ReturnMicros: i64(ret), ReturnReplacements: i64(2),
			Usage: &store.Usage{Input: 10, Output: 10}}); err != nil {
			t.Fatal(err)
		}
	}
	var rep PerfReport
	getJSON(t, e.srv.URL+"/api/performance?days=1", &rep)
	r := rep.Redactor
	if r == nil || r.Requests != 3 || r.Stage.MedianUs != 400 || r.Return.MaxUs != 700 || r.MatchesTotal != 3 || r.ReturnReplacementsTotal != 6 ||
		r.TokensAddedTotal != 9 || r.TokensAddedMedian != 3 || r.BytesInMedian != 1000 || r.ShareMedianPermille != 90 {
		t.Fatalf("redactor %+v", r)
	}
}
