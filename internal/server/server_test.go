package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/store"
)

const (
	clientKey = "tg_client-key-0123456789abcdef"
	cpaKey    = "cpa-secret-key-zyxwvutsrqponmlk"
)

type fakeKeys map[string]string

func (f fakeKeys) Verify(_ context.Context, key string) (string, error) {
	if id, ok := f[key]; ok {
		return id, nil
	}
	return "", pipeline.ErrInvalidKey
}

type funcStage struct {
	name string
	fn   func(context.Context, *pipeline.Request) ([]jsonsplice.Replacement, error)
}

func (s funcStage) Name() string { return s.name }
func (s funcStage) Process(ctx context.Context, r *pipeline.Request) ([]jsonsplice.Replacement, error) {
	return s.fn(ctx, r)
}

func replaceStage(name, from, to string) pipeline.Stage {
	return funcStage{name, func(_ context.Context, r *pipeline.Request) ([]jsonsplice.Replacement, error) {
		doc, err := jsonsplice.Scan(r.Body)
		if err != nil {
			return nil, err
		}
		var reps []jsonsplice.Replacement
		for _, t := range doc.Targets {
			if t.Value == from {
				reps = append(reps, jsonsplice.Replacement{Start: t.Start, End: t.End, New: to})
			}
		}
		return reps, nil
	}}
}

// recorded is what the mock upstream saw.
type recorded struct {
	Method string
	URI    string
	Header http.Header
	Host   string
	Body   []byte
}

type mockUpstream struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []recorded
}

func newUpstream(t *testing.T, h http.HandlerFunc) *mockUpstream {
	t.Helper()
	m := &mockUpstream{}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		m.reqs = append(m.reqs, recorded{r.Method, r.RequestURI, r.Header.Clone(), r.Host, body})
		m.mu.Unlock()
		if h != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			h(w, r)
		}
	}))
	t.Cleanup(m.Close)
	return m
}

func (m *mockUpstream) seen() []recorded {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]recorded(nil), m.reqs...)
}

type proxyOpts struct {
	store    Store
	maxBody  int64
	upstream string // overrides the mock URL
	logger   *slog.Logger
	observe  func(store.Trace)
	onResp   func(http.Header)
}

func newProxy(t *testing.T, up *mockUpstream, o proxyOpts, stages ...pipeline.Stage) *httptest.Server {
	t.Helper()
	all := append([]pipeline.Stage{&pipeline.Intake{Keys: fakeKeys{clientKey: "client-1"}}}, stages...)
	url := o.upstream
	if url == "" {
		url = up.URL
	}
	logger := o.logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	srv, err := New(Options{
		Runner:           pipeline.NewRunner(all),
		Store:            o.store,
		UpstreamURL:      url,
		UpstreamKey:      cpaKey,
		MaxBodyBytes:     o.maxBody,
		Logger:           logger,
		TraceObserver:    o.observe,
		ResponseObserver: o.onResp,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

var rawClient = &http.Client{
	Transport:     &http.Transport{DisableCompression: true},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Timeout:       10 * time.Second,
}

func doReq(t *testing.T, method, url, body string, hdr map[string]string) (*http.Response, []byte) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := rawClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, b
}

func auth(extra map[string]string) map[string]string {
	h := map[string]string{"Authorization": "Bearer " + clientKey}
	for k, v := range extra {
		h[k] = v
	}
	return h
}

func okJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"ok":true}`)
}

const msgBody = `{"model":"claude-test","stream":false,"messages":[{"role":"user","content":"hello SECRET world"}]}`

func TestForwardsPipelineOutputWithHeaderRules(t *testing.T) {
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{}, replaceStage("redact", "hello SECRET world", "hello <redacted:x_1> world"))

	hdr := auth(map[string]string{
		"Anthropic-Version":        "2023-06-01",
		"Anthropic-Beta":           "claude-code-20250219,oauth-2025-04-20",
		"User-Agent":               "claude-cli/2.1.300 (external, cli)",
		"X-App":                    "cli",
		"X-Claude-Code-Session-Id": "sess-1",
		"X-Stainless-Lang":         "js",
		"X-Api-Key":                "tg_should-be-dropped",
		"Accept-Encoding":          "gzip, br",
		"Content-Type":             "application/json",
		"Keep-Alive":               "timeout=5",
		"Proxy-Authorization":      "Basic abc",
		"Connection":               "X-Hop",
		"X-Hop":                    "1",
	})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages?beta=true", msgBody, hdr)
	if resp.StatusCode != 200 || string(body) != `{"ok":true}` {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
	got := up.seen()
	if len(got) != 1 {
		t.Fatalf("upstream calls: %d", len(got))
	}
	r := got[0]
	if r.Method != http.MethodPost || r.URI != "/v1/messages?beta=true" {
		t.Fatalf("method/uri: %s %s", r.Method, r.URI)
	}
	wantBody := strings.Replace(msgBody, "hello SECRET world", "hello <redacted:x_1> world", 1)
	if string(r.Body) != wantBody {
		t.Fatalf("upstream body %q, want %q", r.Body, wantBody)
	}
	for k, v := range map[string]string{
		"Anthropic-Version":        "2023-06-01",
		"Anthropic-Beta":           "claude-code-20250219,oauth-2025-04-20",
		"User-Agent":               "claude-cli/2.1.300 (external, cli)",
		"X-App":                    "cli",
		"X-Claude-Code-Session-Id": "sess-1",
		"X-Stainless-Lang":         "js",
		"Content-Type":             "application/json",
		"Authorization":            "Bearer " + cpaKey,
		"Accept-Encoding":          "identity",
	} {
		if r.Header.Get(k) != v {
			t.Errorf("header %s = %q, want %q", k, r.Header.Get(k), v)
		}
	}
	if r.Header.Get("X-Api-Key") != "" {
		t.Error("x-api-key must be dropped")
	}
	for _, k := range []string{"Keep-Alive", "Proxy-Authorization", "X-Hop"} {
		if r.Header.Get(k) != "" {
			t.Errorf("hop-by-hop header %s reached upstream", k)
		}
	}
	for k, vs := range r.Header {
		for _, v := range vs {
			if strings.Contains(v, clientKey) || strings.Contains(v, "tg_should-be-dropped") {
				t.Errorf("client key leaked in header %s", k)
			}
		}
	}
	if r.Header.Get("Content-Length") != fmt.Sprint(len(wantBody)) {
		t.Errorf("content-length %q, want %d", r.Header.Get("Content-Length"), len(wantBody))
	}
}

func TestXAPIKeyClientIsAuthenticatedAndReplaced(t *testing.T) {
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{})
	resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, map[string]string{"X-Api-Key": clientKey})
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	r := up.seen()[0]
	if r.Header.Get("X-Api-Key") != "" || r.Header.Get("Authorization") != "Bearer "+cpaKey {
		t.Fatalf("credentials not replaced: %v", r.Header)
	}
}

func TestMissingUserAgentIsNotInvented(t *testing.T) {
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{})
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+clientKey)
	req.Header["User-Agent"] = []string{""}
	resp, err := rawClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if ua := up.seen()[0].Header.Get("User-Agent"); strings.Contains(ua, "Go-http-client") {
		t.Fatalf("proxy invented a User-Agent: %q", ua)
	}
}

func TestPassthroughEndpointsUnchanged(t *testing.T) {
	tests := []struct {
		name, method, path, body string
	}{
		{"count_tokens", http.MethodPost, "/v1/messages/count_tokens", `{"model":"m","messages":[{"role":"user","content":"x"}]}`},
		{"model list", http.MethodGet, "/v1/models?limit=5&after_id=abc", ""},
		{"model detail", http.MethodGet, "/v1/models/claude-test", ""},
		{"unknown endpoint with non-JSON body", http.MethodPost, "/v1/unknown", "not json at all {"},
		{"escaped path", http.MethodGet, "/v1/models/a%2Fb", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Request-Id", "req_123")
				w.Header().Set("Anthropic-Ratelimit-Requests-Remaining", "42")
				w.WriteHeader(http.StatusTeapot)
				_, _ = io.WriteString(w, `{"data":[1,2,3]}`)
			})
			ts := newProxy(t, up, proxyOpts{})
			resp, body := doReq(t, tt.method, ts.URL+tt.path, tt.body, auth(map[string]string{"Anthropic-Version": "2023-06-01"}))
			if resp.StatusCode != http.StatusTeapot || string(body) != `{"data":[1,2,3]}` {
				t.Fatalf("response not passed through: %d %q", resp.StatusCode, body)
			}
			if resp.Header.Get("Request-Id") != "req_123" || resp.Header.Get("Anthropic-Ratelimit-Requests-Remaining") != "42" {
				t.Fatalf("response headers lost: %v", resp.Header)
			}
			r := up.seen()[0]
			if r.Method != tt.method || r.URI != tt.path || string(r.Body) != tt.body {
				t.Fatalf("upstream saw %s %s %q", r.Method, r.URI, r.Body)
			}
			if r.Header.Get("Anthropic-Version") != "2023-06-01" || r.Header.Get("Authorization") != "Bearer "+cpaKey {
				t.Fatalf("header rules not applied to passthrough: %v", r.Header)
			}
		})
	}
}

func TestUpstreamRedirectIsNotFollowed(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.invalid/x", http.StatusFound)
	})
	ts := newProxy(t, up, proxyOpts{})
	resp, _ := doReq(t, http.MethodGet, ts.URL+"/v1/models", "", auth(nil))
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "http://example.invalid/x" {
		t.Fatalf("redirect must be passed to the client: %d %v", resp.StatusCode, resp.Header)
	}
}

func TestSSEIsStreamedPerEventAcrossChunkBoundaries(t *testing.T) {
	release := make(chan struct{})
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fl := w.(http.Flusher)
		write := func(s string) { _, _ = io.WriteString(w, s); fl.Flush() }
		write("event: message_start\ndata: {\"a\":1}\n\n")
		write("event: content_block_delta\nda") // event split across chunks
		<-release
		write("ta: {\"b\":2}\n\n")
		write("event: message_stop\ndata: {}\n\n")
	})
	ts := newProxy(t, up, proxyOpts{})

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/messages", strings.NewReader(`{"model":"m","stream":true,"messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+clientKey)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	br := bufio.NewReader(resp.Body)
	first := make(chan string, 1)
	go func() {
		var sb strings.Builder
		for {
			line, err := br.ReadString('\n')
			sb.WriteString(line)
			if err != nil || line == "\n" {
				break
			}
		}
		first <- sb.String()
	}()
	select {
	case ev := <-first:
		if ev != "event: message_start\ndata: {\"a\":1}\n\n" {
			t.Fatalf("first event %q", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first event not delivered before upstream finished: response is buffered")
	}
	close(release)
	rest, err := io.ReadAll(br)
	if err != nil {
		t.Fatal(err)
	}
	want := "event: content_block_delta\ndata: {\"b\":2}\n\nevent: message_stop\ndata: {}\n\n"
	if string(rest) != want {
		t.Fatalf("rest %q, want %q", rest, want)
	}
}

func TestAuthFailuresNeverReachUpstream(t *testing.T) {
	tests := []struct {
		name string
		hdr  map[string]string
	}{
		{"no key", nil},
		{"unknown key", map[string]string{"Authorization": "Bearer tg_nope"}},
		{"unknown x-api-key", map[string]string{"X-Api-Key": "tg_nope"}},
		{"wrong scheme", map[string]string{"Authorization": "Basic " + clientKey}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up := newUpstream(t, okJSON)
			ts := newProxy(t, up, proxyOpts{})
			for _, path := range []string{"/v1/messages", "/v1/models"} {
				resp, body := doReq(t, http.MethodPost, ts.URL+path, msgBody, tt.hdr)
				if resp.StatusCode != 401 {
					t.Fatalf("%s: status %d", path, resp.StatusCode)
				}
				var e struct {
					Type  string `json:"type"`
					Error struct{ Type, Message string }
				}
				if err := json.Unmarshal(body, &e); err != nil || e.Type != "error" || e.Error.Type != "authentication_error" {
					t.Fatalf("not an Anthropic error body: %q", body)
				}
				if resp.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("content type %q", resp.Header.Get("Content-Type"))
				}
			}
			if n := len(up.seen()); n != 0 {
				t.Fatalf("upstream was called %d times", n)
			}
		})
	}
}

func TestStageFailureAbortsWithoutForwardingOrLeaking(t *testing.T) {
	up := newUpstream(t, okJSON)
	boom := funcStage{"boom", func(context.Context, *pipeline.Request) ([]jsonsplice.Replacement, error) {
		return nil, errors.New("internal detail: password=hunter2")
	}}
	ts := newProxy(t, up, proxyOpts{}, boom)
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != 500 || strings.Contains(string(body), "hunter2") || !strings.Contains(string(body), `"api_error"`) {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
	if len(up.seen()) != 0 {
		t.Fatal("request was forwarded despite a pipeline error")
	}
}

func TestInvalidJSONIsRejectedWithoutForwarding(t *testing.T) {
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", `{"model":`, auth(nil))
	if resp.StatusCode != 400 || !strings.Contains(string(body), "invalid_request_error") {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
	if len(up.seen()) != 0 {
		t.Fatal("invalid body was forwarded")
	}
}

func TestOversizedBodyIsRejected(t *testing.T) {
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{maxBody: 64})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody+strings.Repeat(" ", 100), auth(nil))
	if resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), "invalid_request_error") {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
	if len(up.seen()) != 0 {
		t.Fatal("oversized body was forwarded")
	}
}

func TestUnreachableUpstreamGives502WithoutDetails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := "http://" + ln.Addr().String()
	ln.Close()
	ts := newProxy(t, nil, proxyOpts{upstream: dead})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), `"api_error"`) {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "127.0.0.1") || strings.Contains(string(body), cpaKey) {
		t.Fatalf("upstream details leaked: %q", body)
	}
}

func TestNewRejectsBadOptions(t *testing.T) {
	r := pipeline.NewRunner(nil)
	for name, o := range map[string]Options{
		"non-loopback upstream": {Runner: r, UpstreamURL: "http://100.64.0.1:8317", UpstreamKey: cpaKey},
		"missing key":           {Runner: r, UpstreamURL: "http://127.0.0.1:8317"},
		"missing runner":        {UpstreamURL: "http://127.0.0.1:8317", UpstreamKey: cpaKey},
	} {
		if _, err := New(o); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestListenEnforcesBindingRules(t *testing.T) {
	for _, tt := range []struct {
		addr string
		pub  string
		dev  bool
		ok   bool
	}{
		{"0.0.0.0:0", "", false, false},
		{"0.0.0.0:0", "", true, false},
		{"[::]:0", "", true, false},
		{"127.0.0.1:0", "", false, false},
		{"127.0.0.1:0", "", true, true},
		// With published, loopback is refused even with the dev flag.
		{"127.0.0.1:0", "100.64.0.1:8443", true, false},
	} {
		ln, err := Listen(tt.addr, tt.pub, tt.dev)
		if (err == nil) != tt.ok {
			t.Errorf("Listen(%q, %q, dev=%v): err=%v, want ok=%v", tt.addr, tt.pub, tt.dev, err, tt.ok)
		}
		if ln != nil {
			ln.Close()
		}
	}
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "t.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	hash := bytes.Repeat([]byte{7}, 32)
	if _, err := st.CreateClient(context.Background(), store.Client{ID: "client-1", Name: "c", KeyHash: hash, KeyHint: "tg_...cdef", Status: store.ClientActive}); err != nil {
		t.Fatal(err)
	}
	return st
}

// waitFor polls cond for up to 5 s. The finish event follows the verdict
// (#87), which runs after the trace is finished in the store.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitFinished(t *testing.T, st *store.Store) store.Trace {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		list, err := st.ListTraces(context.Background(), store.ListTracesFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) == 1 && list[0].Status != 0 {
			tr, err := st.GetTrace(context.Background(), list[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			return tr
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("trace was not recorded and finished")
	return store.Trace{}
}

func TestTraceIsRecordedWithMetadataOnly(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{store: st}, replaceStage("redact", "hello SECRET world", "hello <redacted:x_1> world"))
	body := `{"model":"claude-test","stream":true,"messages":[{"role":"user","content":"hello SECRET world"}]}`
	resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", body, auth(map[string]string{"X-Claude-Code-Session-Id": "sess-1"}))
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	tr := waitFinished(t, st)
	if tr.ClientID != "client-1" || tr.Endpoint != "messages" || tr.Model != "claude-test" || !tr.Stream ||
		tr.Status != 200 || len(tr.ID) != 32 || len(tr.SessionKeyHash) != 32 || tr.ErrorKind != "" {
		t.Fatalf("unexpected trace: %+v", tr)
	}
	if len(tr.Stages) != 2 || tr.Stages[0].Name != "intake" || tr.Stages[1].Name != "redact" {
		t.Fatalf("stages: %+v", tr.Stages)
	}
	if tr.Stages[0].HashOut != tr.Stages[1].HashIn || tr.Stages[1].HashIn == tr.Stages[1].HashOut {
		t.Fatalf("stage hashes inconsistent: %+v", tr.Stages)
	}
	dump := fmt.Sprintf("%+v", tr)
	for _, bad := range []string{clientKey, cpaKey, "SECRET", "hello"} {
		if strings.Contains(dump, bad) {
			t.Fatalf("trace contains %q: %s", bad, dump)
		}
	}
}

func TestFailedStageTraceHasErrorKindAndNoUpstreamCall(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, okJSON)
	boom := funcStage{"boom", func(context.Context, *pipeline.Request) ([]jsonsplice.Replacement, error) {
		return nil, errors.New("x")
	}}
	ts := newProxy(t, up, proxyOpts{store: st}, boom)
	resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != 500 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	tr := waitFinished(t, st)
	if tr.Status != 500 || tr.ErrorKind != "stage_boom" || len(up.seen()) != 0 {
		t.Fatalf("trace %+v, upstream calls %d", tr, len(up.seen()))
	}
}

func TestUnauthenticatedRequestLeavesNoTrace(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{store: st})
	doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, nil)
	list, err := st.ListTraces(context.Background(), store.ListTracesFilter{})
	if err != nil || len(list) != 0 {
		t.Fatalf("traces: %v, %v", list, err)
	}
}

func TestUpstreamErrorStatusIsTraced(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"type":"error"}`)
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != 429 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if tr := waitFinished(t, st); tr.Status != 429 {
		t.Fatalf("trace status %d", tr.Status)
	}
}

func TestAbortedUpstreamStreamAbortsClientConnection(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message_start\ndata: {}\n\n")
		w.(http.Flusher).Flush()
		conn, _, err := http.NewResponseController(w).Hijack()
		if err == nil {
			_ = conn.Close() // stream breaks before message_stop
		}
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/messages", strings.NewReader(msgBody))
	req.Header.Set("Authorization", "Bearer "+clientKey)
	resp, err := rawClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err == nil {
		t.Fatalf("truncated stream ended cleanly: %q", b)
	}
	if !strings.Contains(string(b), "message_start") {
		t.Fatalf("first event missing: %q", b)
	}
	if tr := waitFinished(t, st); tr.ErrorKind != "stream_aborted" {
		t.Fatalf("trace %+v", tr)
	}
}

func TestUpstream401BecomesBadGateway(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Internal", "cpa-detail")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid api key cpa-internal"}`)
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), `"api_error"`) {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "cpa-internal") || resp.Header.Get("X-Internal") != "" {
		t.Fatalf("upstream details leaked: %q %v", body, resp.Header)
	}
	if tr := waitFinished(t, st); tr.Status != 502 || tr.ErrorKind != "upstream_auth" {
		t.Fatalf("trace %+v", tr)
	}
}

func TestResponseHopByHopHeadersAreRemoved(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Connection", "X-Hop")
		w.Header().Set("X-Hop", "1")
		w.Header().Set("Keep-Alive", "timeout=5")
		w.Header().Set("Request-Id", "req_1")
		okJSON(w, nil)
	})
	ts := newProxy(t, up, proxyOpts{})
	resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.Header.Get("X-Hop") != "" || resp.Header.Get("Keep-Alive") != "" {
		t.Fatalf("hop-by-hop header passed: %v", resp.Header)
	}
	if resp.Header.Get("Request-Id") != "req_1" {
		t.Fatalf("end-to-end header lost: %v", resp.Header)
	}
}

func TestClientCancelCancelsUpstream(t *testing.T) {
	st := openStore(t)
	upstreamDone := make(chan struct{})
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(upstreamDone)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/v1/messages", strings.NewReader(msgBody))
	req.Header.Set("Authorization", "Bearer "+clientKey)
	go func() {
		for len(up.seen()) == 0 {
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()
	if _, err := rawClient.Do(req); err == nil {
		t.Fatal("expected canceled request")
	}
	select {
	case <-upstreamDone:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream request was not canceled")
	}
	if tr := waitFinished(t, st); tr.Status != 499 || tr.ErrorKind != "client_canceled" {
		t.Fatalf("trace %+v", tr)
	}
}

func TestDeclaredOversizedBodyIsRejectedBeforeReading(t *testing.T) {
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{maxBody: 64})
	pr, pw := io.Pipe()
	defer pw.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/v1/messages", pr)
	req.ContentLength = 1 << 20 // declared, never sent
	req.Header.Set("Authorization", "Bearer "+clientKey)
	resp, err := rawClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge || len(up.seen()) != 0 {
		t.Fatalf("status %d, upstream calls %d", resp.StatusCode, len(up.seen()))
	}
}

func TestRoutesAdminFrontendAndProxy(t *testing.T) {
	up := newUpstream(t, okJSON)
	tag := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, name) })
	}
	srv, err := New(Options{
		Runner:      pipeline.NewRunner([]pipeline.Stage{&pipeline.Intake{Keys: fakeKeys{clientKey: "client-1"}}}),
		UpstreamURL: up.URL, UpstreamKey: cpaKey,
		Admin: tag("admin"), Frontend: tag("frontend"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	for _, tt := range []struct{ method, path, want string }{
		{http.MethodGet, "/api/clients", "admin"},
		{http.MethodPost, "/api/clients/abc/block", "admin"},
		{http.MethodGet, "/", "frontend"},
		{http.MethodHead, "/", ""},
		{http.MethodGet, "/index.html", "frontend"},
		{http.MethodGet, "/assets/app-1.js", "frontend"},
		{http.MethodPost, "/", "proxy"},
		{http.MethodGet, "/api", "admin"},
		{http.MethodGet, "/api/", "admin"},
		{http.MethodGet, "/api/traces", "admin"},
		{http.MethodGet, "/api/clientsX", "admin"},
		{http.MethodGet, "/api/other", "admin"},
		{http.MethodGet, "/apix", "proxy"},
		{http.MethodGet, "/api2/traces", "proxy"},
		{http.MethodGet, "/v1/models", "proxy"},
		{http.MethodGet, "/favicon.ico", "proxy"},
		// Path tricks: the decoded path decides; anything not matching exactly
		// goes to the proxy, which demands a client key.
		{http.MethodPost, "/api/clients%2Fx/block", "admin"},
		{http.MethodGet, "/api/clients/../clients", "admin"},
		{http.MethodOptions, "/api/clients", "admin"},
		{http.MethodGet, "/API/clients", "proxy"},
		{http.MethodGet, "//api/clients", "proxy"},
		{http.MethodGet, "/api/./clients", "admin"}, // the admin mux decides; auth runs first
		{http.MethodOptions, "/", "proxy"},
	} {
		before := len(up.seen())
		resp, body := doReq(t, tt.method, ts.URL+tt.path, "", auth(nil))
		got := string(body)
		if len(up.seen()) > before {
			got = "proxy"
		}
		if tt.method == http.MethodHead {
			if resp.StatusCode != 200 || len(up.seen()) > before {
				t.Errorf("HEAD %s: status %d, proxied %v", tt.path, resp.StatusCode, len(up.seen()) > before)
			}
			continue
		}
		if got != tt.want {
			t.Errorf("%s %s routed to %q, want %q", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestWithoutFrontendRootGoesToProxy(t *testing.T) {
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{})
	doReq(t, http.MethodGet, ts.URL+"/", "", auth(nil))
	if len(up.seen()) != 1 {
		t.Fatalf("upstream calls %d", len(up.seen()))
	}
}

func TestUsageIsStoredForNonStreamingResponse(t *testing.T) {
	st := openStore(t)
	const answer = `{"id":"msg_1","type":"message","content":[{"type":"text","text":"hi"}],` +
		`"usage":{"input_tokens":11,"output_tokens":22,"cache_read_input_tokens":33,"cache_creation_input_tokens":44}}`
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, answer)
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != 200 || string(body) != answer {
		t.Fatalf("status %d, body changed: %s", resp.StatusCode, body)
	}
	tr := waitFinished(t, st)
	want := store.Usage{Input: 11, Output: 22, CacheRead: 33, CacheCreation: 44}
	if tr.Usage == nil || *tr.Usage != want {
		t.Fatalf("usage %+v, want %+v", tr.Usage, want)
	}
	if tr.EstimatedInputTokens <= 0 {
		t.Fatalf("estimate missing: %d", tr.EstimatedInputTokens)
	}
}

func TestUsageIsStoredForStreamingResponse(t *testing.T) {
	st := openStore(t)
	events := []string{
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":7,\"cache_read_input_tokens\":70,\"cache_creation_input_tokens\":3,\"output_tokens\":1}}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n",
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":42}}\n\n",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
	}
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range events {
			// Split every event across two writes to cross chunk boundaries.
			_, _ = io.WriteString(w, ev[:len(ev)/2])
			w.(http.Flusher).Flush()
			_, _ = io.WriteString(w, ev[len(ev)/2:])
			w.(http.Flusher).Flush()
		}
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	body := `{"model":"claude-opus-5-5","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	resp, got := doReq(t, http.MethodPost, ts.URL+"/v1/messages", body, auth(nil))
	if resp.StatusCode != 200 || string(got) != strings.Join(events, "") {
		t.Fatalf("status %d, stream changed: %q", resp.StatusCode, got)
	}
	tr := waitFinished(t, st)
	want := store.Usage{Input: 7, Output: 42, CacheRead: 70, CacheCreation: 3}
	if tr.Usage == nil || *tr.Usage != want {
		t.Fatalf("usage %+v, want %+v", tr.Usage, want)
	}
}

func TestCountTokensIsNotMetered(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"input_tokens":5,"usage":{"input_tokens":5}}`)
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages/count_tokens", msgBody, auth(nil))
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if tr := waitFinished(t, st); tr.Usage != nil {
		t.Fatalf("count_tokens metered: %+v", tr.Usage)
	}
}

func TestOversizedJSONResponseIsPassedThroughUnmetered(t *testing.T) {
	st := openStore(t)
	big := `{"usage":{"input_tokens":1,"output_tokens":1},"pad":"` + strings.Repeat("x", maxUsageBody) + `"}`
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, big)
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != 200 || len(body) != len(big) {
		t.Fatalf("status %d, len %d", resp.StatusCode, len(body))
	}
	if tr := waitFinished(t, st); tr.Usage != nil {
		t.Fatalf("oversized response metered: %+v", tr.Usage)
	}
}

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestMeteredRequestIsLoggedWithNumbersOnly(t *testing.T) {
	st := openStore(t)
	logs := &syncBuffer{}
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ANSWERTEXT"}],"usage":{"input_tokens":1000000,"output_tokens":2}}`)
	})
	ts := newProxy(t, up, proxyOpts{store: st, logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))})
	body := `{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hello SECRET world"}]}`
	if resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", body, auth(nil)); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	waitFinished(t, st)
	out := logs.String()
	for _, want := range []string{
		`msg="forwarding request"`, "estimated_input_tokens=",
		`msg="request metered"`, "model=claude-opus-5-5", "status=200", "input_tokens=1000000", "output_tokens=2",
		"cache_read_tokens=0", "cache_creation_tokens=0", "cost_nano_usd=4000040000",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"SECRET", "hello", "ANSWERTEXT", clientKey, cpaKey} {
		if strings.Contains(out, bad) {
			t.Fatalf("log contains %q:\n%s", bad, out)
		}
	}
}

func TestAbortedStreamKeepsPartialUsage(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":9,\"output_tokens\":1}}}\n\n")
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler) // cut the connection mid-stream
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/messages", strings.NewReader(msgBody))
	req.Header.Set("Authorization", "Bearer "+clientKey)
	if resp, err := rawClient.Do(req); err == nil {
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
	}
	tr := waitFinished(t, st)
	want := store.Usage{Input: 9, Output: 1}
	if tr.ErrorKind != "stream_aborted" || tr.Usage == nil || *tr.Usage != want {
		t.Fatalf("trace %+v usage %+v", tr, tr.Usage)
	}
}

func TestUpstreamErrorBodyPassesUnchangedWithoutUsage(t *testing.T) {
	st := openStore(t)
	const errBody = `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(529)
		_, _ = io.WriteString(w, errBody)
	})
	ts := newProxy(t, up, proxyOpts{store: st})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != 529 || string(body) != errBody {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
	if tr := waitFinished(t, st); tr.Status != 529 || tr.Usage != nil {
		t.Fatalf("trace %+v usage %+v", tr, tr.Usage)
	}
}

func TestUnauthenticatedRequestIsRejectedWithoutReadingBody(t *testing.T) {
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{})
	// Raw connection: announce a 1 MB body but send only a few bytes. If the
	// server read the body before the key check, no answer would come.
	conn, err := net.Dial("tcp", ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = io.WriteString(conn, "POST /v1/messages HTTP/1.1\r\nHost: x\r\nAuthorization: Bearer tg_nope\r\n"+
		"Content-Type: application/json\r\nContent-Length: 1000000\r\n\r\n{\"model\":")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("no early answer: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || len(up.seen()) != 0 {
		t.Fatalf("status %d, upstream calls %d", resp.StatusCode, len(up.seen()))
	}
}

type failingFinish struct{ *store.Store }

func (failingFinish) FinishTrace(context.Context, string, store.TraceResult) error {
	return errors.New("disk full")
}

func TestTraceObserverSeesFinishEvenWhenStoreFails(t *testing.T) {
	st := openStore(t)
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"usage":{"input_tokens":1,"output_tokens":2}}`)
	})
	var mu sync.Mutex
	var got []store.Trace
	ts := newProxy(t, up, proxyOpts{store: failingFinish{st}, observe: func(tr store.Trace) {
		mu.Lock()
		got = append(got, tr)
		mu.Unlock()
	}})
	if resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil)); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("observed %d traces, want 2", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if got[1].Status != 200 || got[1].Usage == nil || got[1].Usage.Output != 2 {
		t.Fatalf("finish event: %+v", got[1])
	}
}

func TestTraceObserverSeesStartAndFinish(t *testing.T) {
	st := openStore(t)
	const answer = `{"usage":{"input_tokens":11,"output_tokens":22}}`
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, answer)
	})
	var mu sync.Mutex
	var got []store.Trace
	ts := newProxy(t, up, proxyOpts{store: st, observe: func(tr store.Trace) {
		mu.Lock()
		got = append(got, tr)
		mu.Unlock()
	}})
	if resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil)); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	waitFinished(t, st)
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) >= 2 })
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("observed %d traces, want 2", len(got))
	}
	start, done := got[0], got[1]
	if start.Status != 0 || start.Usage != nil || start.ClientID != "client-1" {
		t.Errorf("start event: %+v", start)
	}
	if done.ID != start.ID || done.Status != 200 || done.Usage == nil || done.Usage.Input != 11 || done.Usage.Output != 22 {
		t.Errorf("finish event: %+v", done)
	}
}

func TestResponseObserverSeesUpstreamHeaders(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/messages" {
			w.Header().Set("Anthropic-Ratelimit-Unified-5h-Utilization", "0.42")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	})
	var mu sync.Mutex
	var got []string
	ts := newProxy(t, up, proxyOpts{onResp: func(h http.Header) {
		mu.Lock()
		got = append(got, h.Get("Anthropic-Ratelimit-Unified-5h-Utilization"))
		mu.Unlock()
	}})
	resp, _ := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != 200 || resp.Header.Get("Anthropic-Ratelimit-Unified-5h-Utilization") != "0.42" {
		t.Fatalf("status %d, header passed to client: %q", resp.StatusCode, resp.Header.Get("Anthropic-Ratelimit-Unified-5h-Utilization"))
	}
	if resp, _ := doReq(t, http.MethodGet, ts.URL+"/v1/models", "", auth(nil)); resp.StatusCode != 200 {
		t.Fatalf("models status %d", resp.StatusCode)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(got, ",") != "0.42," {
		t.Fatalf("observed %q, want one call per upstream response", got)
	}
}
