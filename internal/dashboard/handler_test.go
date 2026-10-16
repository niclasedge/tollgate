package dashboard

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/auth"
	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/server"
	"github.com/niclasedge/tollgate/internal/store"
)

const (
	adminToken = "test-admin-token-0123456789abcdef"
	mgmtKey    = "test-management-key-0123456789abcdef"
	cpaKey     = "cpa-secret-key-zyxwvutsrqponmlk"
)

// 2025-10-10T00:00:00Z.
var midnight = time.UnixMilli(1_760_054_400_000).UTC()

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "t.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// addClient creates a client with a fresh key and returns the key.
func addClient(t *testing.T, st *store.Store, id string) string {
	t.Helper()
	key, err := auth.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateClient(context.Background(), store.Client{
		ID: id, Name: "name-" + id, KeyHash: auth.HashKey(key), KeyHint: auth.KeyHint(key), Status: store.ClientActive,
	}); err != nil {
		t.Fatal(err)
	}
	return key
}

func hexOf(c byte) string { return strings.Repeat(string(c), 64) }

type rec struct {
	id, client, model string
	at                time.Time
	usage             *store.Usage // nil = unfinished
	saved             bool         // add stages saving 10 tokens
}

func record(t *testing.T, st *store.Store, r rec) {
	t.Helper()
	ctx := context.Background()
	tr := store.Trace{
		ID: r.id, ClientID: r.client, StartedAt: r.at, Endpoint: "messages", Model: r.model,
		Stream: true, EstimatedInputTokens: 100,
	}
	if r.saved {
		tr.Stages = []store.Stage{{Name: "intake", TokensIn: 120, TokensOut: 110, HashIn: hexOf('a'), HashOut: hexOf('b')},
			{Name: "redact", TokensIn: 110, TokensOut: 110, HashIn: hexOf('b'), HashOut: hexOf('b')}}
	}
	if err := st.RecordTrace(ctx, tr); err != nil {
		t.Fatal(err)
	}
	if r.usage != nil {
		if err := st.FinishTrace(ctx, r.id, store.TraceResult{Status: 200, DurationMicros: 2500, Usage: r.usage}); err != nil {
			t.Fatal(err)
		}
	}
}

type env struct {
	st     *store.Store
	broker *Broker
	srv    *httptest.Server
	admin  http.Handler
}

func newEnv(t *testing.T, mod func(*Options)) *env {
	t.Helper()
	e := &env{st: openStore(t), broker: NewBroker()}
	o := Options{
		Store: e.st, Broker: e.broker, UpstreamURL: "http://127.0.0.1:1", Version: "v-test",
		Now: func() time.Time { return midnight.Add(5 * time.Hour) },
	}
	if mod != nil {
		mod(&o)
	}
	h, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	e.admin = auth.RequireAdmin(adminToken, mux)
	e.srv = httptest.NewServer(e.admin)
	t.Cleanup(e.srv.Close)
	return e
}

func get(t *testing.T, url, bearer string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func getJSON(t *testing.T, url string, out any) {
	t.Helper()
	code, body := get(t, url, adminToken)
	if code != 200 {
		t.Fatalf("GET %s = %d %s", url, code, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("GET %s: %v: %s", url, err, body)
	}
}

func TestRoutesNeedAdminToken(t *testing.T) {
	e := newEnv(t, nil)
	clientKey := addClient(t, e.st, "c1")
	routes := []string{"/api/traces", "/api/traces/events", "/api/costs", "/api/status"}
	for _, route := range routes {
		for name, tok := range map[string]string{"no token": "", "client key": clientKey, "wrong token": "nope"} {
			t.Run(route+" "+name, func(t *testing.T) {
				if code, body := get(t, e.srv.URL+route, tok); code != 401 || strings.Contains(string(body), clientKey) {
					t.Fatalf("status %d: %s", code, body)
				}
			})
		}
	}
	for _, route := range routes[:1] {
		if code, _ := get(t, e.srv.URL+route, adminToken); code != 200 {
			t.Fatalf("admin %s = %d", route, code)
		}
	}
}

func TestTracesPagination(t *testing.T) {
	e := newEnv(t, nil)
	addClient(t, e.st, "c1")
	addClient(t, e.st, "c2")
	u := &store.Usage{Input: 1000, Output: 2000, CacheRead: 3000, CacheCreation: 4000}
	// Three traces share one millisecond: the cursor must not lose or repeat them.
	record(t, e.st, rec{"t1", "c1", "claude-opus-5-5", midnight, u, true})
	record(t, e.st, rec{"t2", "c2", "claude-opus-5-5", midnight, u, false})
	record(t, e.st, rec{"t3", "c1", "unknown-model", midnight, u, false})
	record(t, e.st, rec{"t4", "c1", "claude-opus-5-5", midnight.Add(time.Second), nil, false}) // running
	record(t, e.st, rec{"t5", "c2", "claude-opus-5-5", midnight.Add(-time.Second), u, false})

	var seen []string
	byID := map[string]TraceSummary{}
	url := e.srv.URL + "/api/traces?limit=2"
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("pagination does not end")
		}
		var l TraceList
		getJSON(t, url, &l)
		for _, tr := range l.Traces {
			seen = append(seen, tr.ID)
			byID[tr.ID] = tr
		}
		if l.NextBefore == "" {
			if l.NextBeforeID != "" || len(l.Traces) > 2 {
				t.Fatalf("last page: %+v", l)
			}
			break
		}
		url = e.srv.URL + "/api/traces?limit=2&before=" + l.NextBefore + "&before_id=" + l.NextBeforeID
	}
	if want := "t4 t3 t2 t1 t5"; strings.Join(seen, " ") != want {
		t.Fatalf("order %v, want %s", seen, want)
	}

	t1 := byID["t1"]
	if t1.ClientName != "name-c1" || t1.ClientID != "c1" || t1.Model != "claude-opus-5-5" || t1.Status != 200 ||
		t1.DurationMs != 2 || t1.SavedTokens != 10 || t1.EstimatedInputTokens != 100 || t1.StartedAt != "2025-10-10T00:00:00.000Z" {
		t.Errorf("t1 = %+v", t1)
	}
	if t1.Usage == nil || *t1.Usage != (Usage{Input: 1000, Output: 2000, CacheRead: 3000, CacheCreation: 4000}) {
		t.Errorf("t1 usage = %+v", t1.Usage)
	}
	// opus 5.5: 1000*4000 + 2000*20000 + 3000*200 + 4000*5000
	if t1.CostNanoUSD == nil || *t1.CostNanoUSD != 4_000_000+40_000_000+600_000+20_000_000 {
		t.Errorf("t1 cost = %v", t1.CostNanoUSD)
	}
	if byID["t3"].CostNanoUSD != nil || byID["t3"].Usage == nil {
		t.Errorf("unknown model: cost must be null, usage kept: %+v", byID["t3"])
	}
	if t4 := byID["t4"]; t4.Status != 0 || t4.Usage != nil || t4.CostNanoUSD != nil {
		t.Errorf("running trace: %+v", t4)
	}
}

func TestTracesParameters(t *testing.T) {
	e := newEnv(t, nil)
	addClient(t, e.st, "c1")
	for _, tt := range []struct {
		query string
		code  int
	}{
		{"", 200}, {"?limit=1", 200}, {"?limit=200", 200},
		{"?limit=0", 400}, {"?limit=201", 400}, {"?limit=x", 400}, {"?limit=-1", 400},
		{"?before=yesterday", 400}, {"?before_id=abc", 400},
		{"?before=2025-10-10T00:00:00.000Z", 200}, {"?before=2025-10-10T00:00:00.000Z&before_id=abc", 200},
	} {
		t.Run(tt.query, func(t *testing.T) {
			if code, body := get(t, e.srv.URL+"/api/traces"+tt.query, adminToken); code != tt.code {
				t.Fatalf("status %d, want %d: %s", code, tt.code, body)
			}
		})
	}
	// An empty list is [] and not null.
	if _, body := get(t, e.srv.URL+"/api/traces", adminToken); !strings.Contains(string(body), `"traces":[]`) {
		t.Fatalf("body %s", body)
	}
}

func TestCosts(t *testing.T) {
	e := newEnv(t, nil)
	addClient(t, e.st, "c1")
	u := &store.Usage{Input: 1000, Output: 1000, CacheRead: 1000, CacheCreation: 1000}
	record(t, e.st, rec{"a", "c1", "claude-opus-5-5", midnight.Add(time.Hour), u, true})
	record(t, e.st, rec{"b", "c1", "claude-opus-5-5", midnight.Add(2 * time.Hour), u, true})
	record(t, e.st, rec{"c", "c1", "unknown-model", midnight.Add(time.Hour), u, false})
	record(t, e.st, rec{"d", "c1", "claude-opus-5-5", midnight.Add(-5 * 24 * time.Hour), u, false}) // 5 days back
	record(t, e.st, rec{"e", "c1", "claude-opus-5-5", midnight, nil, false})                        // unfinished

	var r CostReport
	getJSON(t, e.srv.URL+"/api/costs?days=2", &r)
	// Now is 2025-10-10T05:00Z: two days = 09 and 10.
	if r.From != "2025-10-09" || r.To != "2025-10-10" || len(r.Rows) != 2 {
		t.Fatalf("report %+v", r)
	}
	opus, unk := r.Rows[0], r.Rows[1] // same day and client: ordered by model
	if opus.Model != "claude-opus-5-5" || opus.Requests != 2 || opus.Day != "2025-10-10" || opus.ClientName != "name-c1" ||
		opus.Usage != (Usage{Input: 2000, Output: 2000, CacheRead: 2000, CacheCreation: 2000}) ||
		opus.EstimatedInputTokens != 200 || opus.SavedTokens != 20 {
		t.Errorf("opus row %+v", opus)
	}
	// 2000*(4000+20000+200+5000)
	if opus.CostNanoUSD == nil || *opus.CostNanoUSD != 2000*(4000+20000+200+5000) {
		t.Errorf("opus cost %v", opus.CostNanoUSD)
	}
	if unk.Model != "unknown-model" || unk.CostNanoUSD != nil {
		t.Errorf("unknown row %+v", unk)
	}

	// The default is 30 days and includes the old trace; the raw JSON shows null.
	var def CostReport
	getJSON(t, e.srv.URL+"/api/costs", &def)
	if def.From != "2025-09-11" || len(def.Rows) != 3 {
		t.Fatalf("default report %+v", def)
	}
	_, body := get(t, e.srv.URL+"/api/costs?days=1", adminToken)
	if !strings.Contains(string(body), `"cost_nano_usd":null`) {
		t.Errorf("no null cost in %s", body)
	}
	for _, q := range []string{"?days=0", "?days=367", "?days=abc"} {
		if code, _ := get(t, e.srv.URL+"/api/costs"+q, adminToken); code != 400 {
			t.Errorf("costs%s = %d, want 400", q, code)
		}
	}
	if code, _ := get(t, e.srv.URL+"/api/costs?days=366", adminToken); code != 200 {
		t.Errorf("days=366 = %d", code)
	}
}

// mockCPA mimics CLIProxyAPI: /healthz and the management credentials.
type mockCPA struct {
	creds      string // body of the credentials response
	credsCode  int
	healthCode int
	mu         sync.Mutex
	gotAuth    string
}

func (m *mockCPA) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz":
		w.WriteHeader(m.healthCode)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	case "/v8/management/credentials":
		m.mu.Lock()
		m.gotAuth = r.Header.Get("Authorization")
		m.mu.Unlock()
		w.WriteHeader(m.credsCode)
		_, _ = io.WriteString(w, m.creds)
	default:
		http.NotFound(w, r)
	}
}

func cred(name, extra string) string {
	return `{"id":"x","name":"` + name + `","provider":"claude","type":"claude","email":"me@example.com",` +
		`"status":"active","disabled":false,"access_token":"SECRET-TOKEN","path":"/data/auth/` + name + `","id_token":{"x":"SECRET-CLAIM"}` + extra + `}`
}

func TestStatus(t *testing.T) {
	future := midnight.Add(5*time.Hour + 30*time.Minute).Format(time.RFC3339Nano)
	tests := []struct {
		name      string
		cpa       *mockCPA
		noServer  bool
		key       string
		wantOK    bool
		wantMsg   string
		wantNames []string
		cooldown  string
	}{
		{name: "one account", cpa: &mockCPA{healthCode: 200, credsCode: 200, creds: `{"observed_at":"x","files":[` + cred("a.json", "") + `]}`},
			key: mgmtKey, wantOK: true, wantNames: []string{"a.json"}},
		{name: "cooldown", cpa: &mockCPA{healthCode: 200, credsCode: 200, creds: `{"files":[` + cred("a.json", `,"next_retry_after":"`+future+`"`) + `]}`},
			key: mgmtKey, wantOK: true, wantNames: []string{"a.json"}, cooldown: "2025-10-10T05:30:00.000Z"},
		{name: "malformed cooldown tolerated", cpa: &mockCPA{healthCode: 200, credsCode: 200, creds: `{"files":[` + cred("a.json", `,"next_retry_after":"soon"`) + `]}`},
			key: mgmtKey, wantOK: true, wantNames: []string{"a.json"}},
		{name: "two accounts warn", cpa: &mockCPA{healthCode: 200, credsCode: 200, creds: `{"files":[` + cred("a.json", "") + `,` + cred("b.json", "") + `]}`},
			key: mgmtKey, wantOK: true, wantMsg: "Warnung: 2 Accounts", wantNames: []string{"a.json", "b.json"}},
		{name: "no account", cpa: &mockCPA{healthCode: 200, credsCode: 200, creds: `{"files":[]}`},
			key: mgmtKey, wantOK: true, wantMsg: "Kein Account"},
		{name: "no key", cpa: &mockCPA{healthCode: 200},
			key: "", wantOK: true, wantMsg: "Accounts nicht verfügbar"},
		{name: "key rejected", cpa: &mockCPA{healthCode: 200, credsCode: 401, creds: `{"error":"x"}`},
			key: mgmtKey, wantOK: true, wantMsg: "Accounts nicht verfügbar"},
		{name: "unhealthy", cpa: &mockCPA{healthCode: 503},
			key: mgmtKey, wantOK: false, wantMsg: "nicht erreichbar"},
		{name: "unreachable", noServer: true,
			key: mgmtKey, wantOK: false, wantMsg: "nicht erreichbar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://127.0.0.1:1"
			if !tt.noServer {
				cpa := httptest.NewServer(tt.cpa)
				t.Cleanup(cpa.Close)
				url = cpa.URL
			}
			e := newEnv(t, func(o *Options) { o.UpstreamURL, o.ManagementKey = url, tt.key })
			code, body := get(t, e.srv.URL+"/api/status", adminToken)
			if code != 200 {
				t.Fatalf("status %d: %s", code, body)
			}
			for _, secret := range []string{"SECRET-TOKEN", "SECRET-CLAIM", mgmtKey} {
				if strings.Contains(string(body), secret) {
					t.Fatalf("%q leaked: %s", secret, body)
				}
			}
			var s Status
			if err := json.Unmarshal(body, &s); err != nil {
				t.Fatal(err)
			}
			if !s.Tollgate.OK || s.Tollgate.Version != "v-test" {
				t.Errorf("tollgate %+v", s.Tollgate)
			}
			if s.CLIProxyAPI.OK != tt.wantOK || !strings.Contains(s.CLIProxyAPI.Message, tt.wantMsg) ||
				(tt.wantMsg == "" && s.CLIProxyAPI.Message != "") {
				t.Errorf("cliproxyapi %+v, want ok=%v message %q", s.CLIProxyAPI, tt.wantOK, tt.wantMsg)
			}
			var names []string
			for _, a := range s.Accounts {
				names = append(names, a.Name)
			}
			if strings.Join(names, ",") != strings.Join(tt.wantNames, ",") {
				t.Errorf("accounts %v, want %v", names, tt.wantNames)
			}
			if s.Accounts == nil {
				t.Error("accounts must be [] not null")
			}
			if len(s.Accounts) > 0 {
				a := s.Accounts[0]
				if a.Provider != "claude" || a.Email != "me@example.com" || a.Status != "active" || a.Disabled || a.CooldownUntil != tt.cooldown {
					t.Errorf("account %+v", a)
				}
			}
			if tt.key != "" && !tt.noServer && tt.cpa.healthCode == 200 {
				tt.cpa.mu.Lock()
				defer tt.cpa.mu.Unlock()
				if tt.cpa.gotAuth != "Bearer "+tt.key {
					t.Errorf("management Authorization = %q", tt.cpa.gotAuth)
				}
			}
		})
	}
}

func TestStatusUpstreamTimesOutAfterThreeSeconds(t *testing.T) {
	hang := make(chan struct{})
	cpa := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-hang }))
	t.Cleanup(func() { close(hang); cpa.Close() })
	e := newEnv(t, func(o *Options) { o.UpstreamURL, o.ManagementKey = cpa.URL, mgmtKey })
	start := time.Now()
	var s Status
	getJSON(t, e.srv.URL+"/api/status", &s)
	if d := time.Since(start); d > 4*time.Second || s.CLIProxyAPI.OK || !s.Tollgate.OK {
		t.Fatalf("took %v, status %+v", d, s)
	}
}

func TestNewRejectsNonLoopbackUpstream(t *testing.T) {
	if _, err := New(Options{Store: openStore(t), Broker: NewBroker(), UpstreamURL: "http://10.0.0.5:8317"}); err == nil {
		t.Fatal("non-loopback upstream accepted")
	}
	if _, err := New(Options{UpstreamURL: "http://127.0.0.1:8317"}); err == nil {
		t.Fatal("missing store accepted")
	}
}

func TestBrokerNeverBlocksAndDropsForSlowSubscribers(t *testing.T) {
	b := NewBroker()
	idle, cancelIdle := b.Subscribe()
	fast, cancelFast := b.Subscribe()
	defer cancelFast()
	var got atomic.Int64
	go func() {
		for range fast {
			got.Add(1)
		}
	}()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10_000; i++ {
			b.Publish(TraceEvent{})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked on a subscriber that does not read")
	}
	if len(idle) != subscriberBuffer {
		t.Errorf("idle subscriber holds %d events, want %d (rest dropped)", len(idle), subscriberBuffer)
	}
	cancelIdle()
	if b.Subscribers() != 1 {
		t.Errorf("subscribers = %d", b.Subscribers())
	}
}

// proxyEnv is a dashboard plus a real proxy server sharing one store and one
// broker, with a mock upstream that answers with usage.
type proxyEnv struct {
	*env
	proxy     *httptest.Server
	clientKey string
	upstream  atomic.Int64 // requests that reached the mock upstream
}

func newProxyEnv(t *testing.T) *proxyEnv {
	t.Helper()
	e := newEnv(t, nil)
	key := addClient(t, e.st, "c1")
	p := &proxyEnv{env: e, clientKey: key}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.upstream.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"usage":{"input_tokens":1000,"output_tokens":2000}}`)
	}))
	t.Cleanup(up.Close)
	pub := NewPublisher(e.broker, e.st)
	t.Cleanup(pub.Close)
	srv, err := server.New(server.Options{
		Admin:         e.admin,
		Runner:        pipeline.NewRunner([]pipeline.Stage{&pipeline.Intake{Keys: auth.NewVerifier(e.st)}}),
		Store:         e.st,
		UpstreamURL:   up.URL,
		UpstreamKey:   cpaKey,
		TraceObserver: pub.Observe,
	})
	if err != nil {
		t.Fatal(err)
	}
	p.proxy = httptest.NewServer(srv.Handler())
	t.Cleanup(p.proxy.Close)
	return p
}

const msgBody = `{"model":"claude-opus-5-5","stream":false,"messages":[{"role":"user","content":"hello"}]}`

func (p *proxyEnv) send(t *testing.T) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, p.proxy.URL+"/v1/messages", strings.NewReader(msgBody))
	req.Header.Set("Authorization", "Bearer "+p.clientKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("proxied request = %d", resp.StatusCode)
	}
}

// openEvents opens the SSE stream and returns a line reader and a closer.
func openEvents(t *testing.T, url string) (*bufio.Scanner, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/traces/events", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		cancel()
		t.Fatalf("events: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	t.Cleanup(func() { cancel(); resp.Body.Close() })
	return bufio.NewScanner(resp.Body), cancel
}

// nextEvent reads the next "trace" event, skipping comments.
func nextEvent(t *testing.T, sc *bufio.Scanner) TraceEvent {
	t.Helper()
	timer := time.AfterFunc(5*time.Second, func() { t.Error("timeout waiting for an SSE event") })
	defer timer.Stop()
	sawEvent := false
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "event: trace":
			sawEvent = true
		case strings.HasPrefix(line, "data: "):
			if !sawEvent {
				t.Fatalf("data without event name: %s", line)
			}
			var ev TraceEvent
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
				t.Fatal(err)
			}
			return ev
		}
	}
	t.Fatalf("stream ended: %v", sc.Err())
	return TraceEvent{}
}

func waitSubscribers(t *testing.T, b *Broker, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for b.Subscribers() != n {
		if time.Now().After(deadline) {
			t.Fatalf("subscribers = %d, want %d", b.Subscribers(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestEventsDeliverStartAndFinishOfProxiedRequest(t *testing.T) {
	p := newProxyEnv(t)
	sc, cancel := openEvents(t, p.srv.URL)
	waitSubscribers(t, p.broker, 1)

	p.send(t)
	start, done := nextEvent(t, sc), nextEvent(t, sc)
	if start.Trace.Status != 0 || start.Trace.Usage != nil || start.Trace.ClientName != "name-c1" ||
		start.Trace.Model != "claude-opus-5-5" || start.Trace.Endpoint != "messages" {
		t.Errorf("start event %+v", start.Trace)
	}
	if done.Trace.ID != start.Trace.ID || done.Trace.Status != 200 || done.Trace.Usage == nil ||
		done.Trace.Usage.Input != 1000 || done.Trace.Usage.Output != 2000 {
		t.Errorf("finish event %+v", done.Trace)
	}
	// opus 5.5: 1000*4000 + 2000*20000
	if done.Trace.CostNanoUSD == nil || *done.Trace.CostNanoUSD != 44_000_000 {
		t.Errorf("finish cost %v", done.Trace.CostNanoUSD)
	}
	// The event equals the list entry apart from the duration measured later.
	var l TraceList
	getJSON(t, p.srv.URL+"/api/traces", &l)
	if len(l.Traces) != 1 || l.Traces[0].ID != start.Trace.ID || l.Traces[0].Status != 200 {
		t.Errorf("list %+v", l)
	}

	cancel()
	waitSubscribers(t, p.broker, 0) // the handler ends with the client
}

func TestEventsHeartbeat(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.Heartbeat = 20 * time.Millisecond })
	sc, _ := openEvents(t, e.srv.URL)
	timer := time.AfterFunc(5*time.Second, func() { t.Error("no heartbeat") })
	defer timer.Stop()
	for sc.Scan() {
		if sc.Text() == ": heartbeat" {
			return
		}
	}
	t.Fatalf("stream ended: %v", sc.Err())
}

func TestHungSubscriberDoesNotDelayRequests(t *testing.T) {
	p := newProxyEnv(t)
	// A subscriber that never reads, at the HTTP level and at the broker level.
	_, _ = openEvents(t, p.srv.URL)
	_, unsubscribe := p.broker.Subscribe()
	defer unsubscribe()
	waitSubscribers(t, p.broker, 2)

	start := time.Now()
	for i := 0; i < 150; i++ { // 300 events: far more than the buffer holds
		p.send(t)
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Fatalf("150 requests took %v with a hung subscriber", d)
	}
}

func TestAPIBehindServerNeedsAdminToken(t *testing.T) {
	p := newProxyEnv(t)
	for _, route := range []string{"/api/traces", "/api/costs", "/api/status"} {
		for name, tok := range map[string]string{"no token": "", "client key": p.clientKey} {
			if code, _ := get(t, p.proxy.URL+route, tok); code != 401 {
				t.Errorf("%s with %s = %d, want 401", route, name, code)
			}
		}
	}
	if n := p.upstream.Load(); n != 0 {
		t.Fatalf("upstream got %d requests from rejected /api calls", n)
	}
	for _, route := range []string{"/api/costs", "/api/status"} {
		if code, body := get(t, p.proxy.URL+route, adminToken); code != 200 {
			t.Errorf("admin %s = %d %s", route, code, body)
		}
	}
	if n := p.upstream.Load(); n != 0 {
		t.Fatalf("upstream got %d requests from admin /api calls", n)
	}
}

func TestEventsEndOnShutdown(t *testing.T) {
	done := make(chan struct{})
	e := newEnv(t, func(o *Options) { o.Done = done })
	sc, _ := openEvents(t, e.srv.URL)
	waitSubscribers(t, e.broker, 1)
	close(done)
	finished := make(chan struct{})
	go func() {
		for sc.Scan() {
		}
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not end on shutdown")
	}
}

func TestPublisherResolvesNamesAndKeepsOrder(t *testing.T) {
	st := openStore(t)
	addClient(t, st, "c1")
	b := NewBroker()
	p := NewPublisher(b, st)
	defer p.Close()
	tr := store.Trace{ID: "x", ClientID: "c1", StartedAt: midnight, Endpoint: "messages", Model: "m",
		Stages: []store.Stage{{Name: "a", TokensIn: 50, TokensOut: 40}}}
	p.Observe(tr) // nobody listens: dropped
	ch, cancel := b.Subscribe()
	defer cancel()
	p.Observe(tr)
	done := tr
	done.Status = 200
	p.Observe(done)
	for i, wantStatus := range []int{0, 200} {
		select {
		case ev := <-ch:
			if ev.Trace.ClientName != "name-c1" || ev.Trace.SavedTokens != 10 || ev.Trace.Status != wantStatus {
				t.Fatalf("event %d: %+v", i, ev.Trace)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no event %d", i)
		}
	}
	select {
	case ev := <-ch:
		t.Fatalf("event from before subscription: %+v", ev.Trace)
	case <-time.After(50 * time.Millisecond):
	}
}

// slowClients blocks ListClients until released (a hung or failing store).
type slowClients struct{ release chan struct{} }

func (s slowClients) ListClients(ctx context.Context) ([]store.Client, error) {
	select {
	case <-s.release:
	case <-ctx.Done():
	}
	return nil, errors.New("store down")
}

func TestSlowClientLookupDoesNotDelayObserve(t *testing.T) {
	b := NewBroker()
	_, cancel := b.Subscribe()
	defer cancel()
	sc := slowClients{release: make(chan struct{})}
	p := NewPublisher(b, sc)
	defer func() { close(sc.release); p.Close() }()
	start := time.Now()
	for i := 0; i < 5000; i++ { // far more than the queue holds
		p.Observe(store.Trace{ID: "x", ClientID: "c1", StartedAt: midnight})
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("Observe blocked for %v behind a hung ListClients", d)
	}
}

func TestStatusLimits(t *testing.T) {
	cpa := httptest.NewServer(&mockCPA{healthCode: 503})
	t.Cleanup(cpa.Close)

	e := newEnv(t, func(o *Options) { o.UpstreamURL = cpa.URL })
	_, body := get(t, e.srv.URL+"/api/status", adminToken)
	if !strings.Contains(string(body), `"limits":null`) {
		t.Fatalf("limits without a source must be null: %s", body)
	}

	u := 0.42
	reset := midnight.Add(2 * time.Hour)
	l := meter.Limits{Status: "allowed", FiveHour: meter.Window{Utilization: &u, Reset: &reset, Status: "allowed"}, ObservedAt: midnight}
	calls := 0
	e = newEnv(t, func(o *Options) {
		o.UpstreamURL = cpa.URL
		o.Limits = func() (meter.Limits, bool) { calls++; return l, calls > 1 }
	})
	_, body = get(t, e.srv.URL+"/api/status", adminToken)
	if !strings.Contains(string(body), `"limits":null`) {
		t.Fatalf("limits before the first observation must be null: %s", body)
	}
	_, body = get(t, e.srv.URL+"/api/status", adminToken) // also while CLIProxyAPI is down
	var s Status
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatal(err)
	}
	want := RateLimits{Status: "allowed", ObservedAt: "2025-10-10T00:00:00.000Z",
		FiveHour: LimitWindow{Utilization: &u, Reset: "2025-10-10T02:00:00.000Z", Status: "allowed"}}
	if s.Limits == nil || s.Limits.Status != want.Status || s.Limits.ObservedAt != want.ObservedAt ||
		s.Limits.FiveHour.Reset != want.FiveHour.Reset || *s.Limits.FiveHour.Utilization != 0.42 ||
		s.Limits.SevenDay.Utilization != nil || s.Limits.SevenDay.Reset != "" {
		t.Fatalf("limits %+v", s.Limits)
	}
}

func TestCostsHoursAndCacheSavings(t *testing.T) {
	e := newEnv(t, nil) // now = 2025-10-10T05:00Z
	addClient(t, e.st, "c1")
	u := &store.Usage{Input: 10, Output: 10, CacheRead: 1000, CacheCreation: 10}
	at := func(r rec, account string) {
		t.Helper()
		ctx := context.Background()
		if err := e.st.RecordTrace(ctx, store.Trace{ID: r.id, ClientID: r.client, StartedAt: r.at, Endpoint: "messages", Model: r.model, AccountID: account}); err != nil {
			t.Fatal(err)
		}
		if err := e.st.FinishTrace(ctx, r.id, store.TraceResult{Status: 200, Usage: r.usage}); err != nil {
			t.Fatal(err)
		}
	}
	at(rec{id: "a", client: "c1", model: "claude-opus-5-5", at: midnight.Add(time.Hour + 10*time.Minute), usage: u}, "acc-a")
	at(rec{id: "b", client: "c1", model: "claude-opus-5-5", at: midnight.Add(time.Hour + 50*time.Minute), usage: u}, "acc-a")
	at(rec{id: "c", client: "c1", model: "claude-opus-5-5", at: midnight.Add(2*time.Hour + 5*time.Minute), usage: u}, "acc-b")
	at(rec{id: "d", client: "c1", model: "unknown-model", at: midnight.Add(2 * time.Hour), usage: u}, "acc-b")
	at(rec{id: "e", client: "c1", model: "claude-opus-5-5", at: midnight.Add(-7 * 24 * time.Hour), usage: u}, "acc-a") // 8 days incl. today

	var r CostReport
	getJSON(t, e.srv.URL+"/api/costs?days=1", &r)
	opus := int64(1000 * (4000 - 200))
	opusCost := int64(10*4000 + 10*20000 + 1000*200 + 10*5000)
	ptr := func(v int64) *int64 { return &v }
	wantHours := []CostHour{
		{Hour: "2025-10-10T01:00:00Z", AccountID: "acc-a", Model: "claude-opus-5-5", Requests: 2,
			Usage: Usage{Input: 20, Output: 20, CacheRead: 2000, CacheCreation: 20}, CostNanoUSD: ptr(2 * opusCost)},
		{Hour: "2025-10-10T02:00:00Z", AccountID: "acc-b", Model: "claude-opus-5-5", Requests: 1,
			Usage: Usage{Input: 10, Output: 10, CacheRead: 1000, CacheCreation: 10}, CostNanoUSD: ptr(opusCost)},
		{Hour: "2025-10-10T02:00:00Z", AccountID: "acc-b", Model: "unknown-model", Requests: 1,
			Usage: Usage{Input: 10, Output: 10, CacheRead: 1000, CacheCreation: 10}},
	}
	if len(r.Hours) != len(wantHours) {
		t.Fatalf("hours %+v", r.Hours)
	}
	for i, w := range wantHours {
		g := r.Hours[i]
		if g.Hour != w.Hour || g.AccountID != w.AccountID || g.Model != w.Model || g.Requests != w.Requests || g.Usage != w.Usage ||
			(g.CostNanoUSD == nil) != (w.CostNanoUSD == nil) || (g.CostNanoUSD != nil && *g.CostNanoUSD != *w.CostNanoUSD) {
			t.Errorf("hour %d = %+v (cost %v), want %+v", i, g, g.CostNanoUSD, w)
		}
	}
	// Cache savings per row; unknown price -> null.
	for _, row := range r.Rows {
		switch row.Model {
		case "claude-opus-5-5":
			if row.CacheSavingsNanoUSD == nil || *row.CacheSavingsNanoUSD != int64(row.Usage.CacheRead/1000)*opus {
				t.Errorf("savings %+v: %v", row, row.CacheSavingsNanoUSD)
			}
		default:
			if row.CacheSavingsNanoUSD != nil || row.CostNanoUSD != nil {
				t.Errorf("unknown row %+v", row)
			}
		}
	}
	_, body := get(t, e.srv.URL+"/api/costs?days=1", adminToken)
	if !strings.Contains(string(body), `"cache_savings_nano_usd":null`) {
		t.Errorf("no null savings in %s", body)
	}

	// Seven days include the same hours; the eight-day-old trace is outside.
	var week CostReport
	getJSON(t, e.srv.URL+"/api/costs?days=7", &week)
	if len(week.Hours) != 3 || week.From != "2025-10-04" {
		t.Errorf("7 days: from %s, hours %+v", week.From, week.Hours)
	}
	// Eight days: rows and hours include the old trace, enough for a full
	// weekly window (#125). Longer periods keep hours for eight days.
	var long CostReport
	getJSON(t, e.srv.URL+"/api/costs?days=8", &long)
	if len(long.Rows) != 4 || long.Rows[0].Day != "2025-10-03" {
		t.Errorf("8 days rows %+v", long.Rows)
	}
	if len(long.Hours) != 4 || long.Hours[0].Hour != "2025-10-03T00:00:00Z" || long.Hours[1].Hour != wantHours[0].Hour {
		t.Errorf("8 days hours %+v", long.Hours)
	}
	var longer CostReport
	getJSON(t, e.srv.URL+"/api/costs?days=14", &longer)
	if len(longer.Hours) != 4 {
		t.Errorf("14 days hours %+v", longer.Hours)
	}
}
