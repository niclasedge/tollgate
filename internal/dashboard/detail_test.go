package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/auth"
	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/redact"
	"github.com/niclasedge/tollgate/internal/server"
	"github.com/niclasedge/tollgate/internal/store"
)

func TestDetailAndRedactionRoutesNeedAdminToken(t *testing.T) {
	e := newEnv(t, nil)
	clientKey := addClient(t, e.st, "c1")
	for _, route := range []string{"/api/traces/x", "/api/redactions"} {
		for name, tok := range map[string]string{"no token": "", "client key": clientKey} {
			if code, _ := get(t, e.srv.URL+route, tok); code != 401 {
				t.Errorf("%s %s = %d", route, name, code)
			}
		}
	}
}

// configVersion returns the ID of a stored configuration version, creating
// one on first use.
func configVersion(t *testing.T, st *store.Store) int64 {
	t.Helper()
	ctx := context.Background()
	if vs, err := st.ListConfigVersions(ctx, 1); err == nil && len(vs) > 0 {
		return vs[0].ID
	}
	v, err := st.CreateConfigVersion(ctx, store.NewConfigVersion{Author: "admin", Content: []byte("version: 1\n")})
	if err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func recordWithMatches(t *testing.T, st *store.Store, id, client string, at time.Time, n int) {
	t.Helper()
	cfg := configVersion(t, st)
	tr := store.Trace{ID: id, ClientID: client, StartedAt: at, Endpoint: "messages", Model: "claude-opus-5-5",
		ConfigVersionID: cfg, EstimatedInputTokens: 100,
		Stages: []store.Stage{
			{Name: "intake", DurationMicros: 40, TokensIn: 120, TokensOut: 120, HashIn: hexOf('a'), HashOut: hexOf('a')},
			{Name: "redact", DurationMicros: 250, TokensIn: 120, TokensOut: 118, HashIn: hexOf('a'), HashOut: hexOf('b'),
				ConfigVersionID: cfg, Diff: "messages[0].content: mail <redacted:email_1> bitte\n"},
		}}
	for i := 1; i <= n; i++ {
		tr.Stages[1].Matches = append(tr.Stages[1].Matches, store.Match{Offset: 10 * i, Length: 18, Type: "email", Rule: "email",
			Placeholder: fmt.Sprintf("<redacted:email_%d>", i), Mask: fmt.Sprintf("x•••%d", i)})
	}
	if err := st.RecordTrace(context.Background(), tr); err != nil {
		t.Fatal(err)
	}
}

func TestTraceDetail(t *testing.T) {
	e := newEnv(t, nil)
	addClient(t, e.st, "c1")
	recordWithMatches(t, e.st, "tr-1", "c1", midnight, 2)
	if err := e.st.FinishTrace(context.Background(), "tr-1", store.TraceResult{Status: 200, DurationMicros: 9000,
		Usage: &store.Usage{Input: 10, Output: 20}}); err != nil {
		t.Fatal(err)
	}

	var d TraceDetail
	getJSON(t, e.srv.URL+"/api/traces/tr-1", &d)
	if d.Trace.ID != "tr-1" || d.Trace.ClientName != "name-c1" || d.Trace.Status != 200 || d.Trace.Usage == nil {
		t.Fatalf("trace = %+v", d.Trace)
	}
	if len(d.Stages) != 2 || d.Stages[0].Name != "intake" || d.Stages[1].Name != "redact" {
		t.Fatalf("stages = %+v", d.Stages)
	}
	r := d.Stages[1]
	if r.DurationUs != 250 || r.TokensIn != 120 || r.TokensOut != 118 || r.HashIn != hexOf('a') || r.HashOut != hexOf('b') ||
		r.ConfigVersion != configVersion(t, e.st) || !strings.Contains(r.Diff, "<redacted:email_1>") {
		t.Fatalf("redact stage = %+v", r)
	}
	if len(r.Matches) != 2 || r.Matches[1] != (Match{Offset: 20, Length: 18, Type: "email", Rule: "email", Placeholder: "<redacted:email_2>", Mask: "x•••2"}) {
		t.Fatalf("matches = %+v", r.Matches)
	}
	if d.Stages[0].Matches == nil {
		t.Fatal("matches must be [] not null")
	}

	if code, _ := get(t, e.srv.URL+"/api/traces/gibtsnicht", adminToken); code != http.StatusNotFound {
		t.Fatalf("unknown = %d", code)
	}
}

func TestRecentRedactionsEndpoint(t *testing.T) {
	e := newEnv(t, nil)
	addClient(t, e.st, "c1")
	addClient(t, e.st, "c2")
	recordWithMatches(t, e.st, "tr-1", "c1", midnight, 4)
	recordWithMatches(t, e.st, "tr-2", "c2", midnight.Add(time.Minute), 8)

	var l RedactionList
	getJSON(t, e.srv.URL+"/api/redactions", &l)
	if len(l.Redactions) != 10 {
		t.Fatalf("default limit: %d", len(l.Redactions))
	}
	first, last := l.Redactions[0], l.Redactions[9]
	if first.TraceID != "tr-2" || first.ClientName != "name-c2" || first.Placeholder != "<redacted:email_1>" || first.Type != "email" ||
		first.Rule != "email" || first.Mask != "x•••1" || first.Stage != "redact" || first.At != "2025-10-10T00:01:00.000Z" {
		t.Fatalf("first = %+v", first)
	}
	if last.TraceID != "tr-1" || last.Placeholder != "<redacted:email_2>" {
		t.Fatalf("last = %+v", last)
	}
	getJSON(t, e.srv.URL+"/api/redactions?limit=3", &l)
	if len(l.Redactions) != 3 {
		t.Fatalf("limit 3: %d", len(l.Redactions))
	}
	for _, q := range []string{"0", "51", "x"} {
		if code, _ := get(t, e.srv.URL+"/api/redactions?limit="+q, adminToken); code != http.StatusBadRequest {
			t.Errorf("limit=%s = %d", q, code)
		}
	}

	empty := newEnv(t, nil)
	_, body := get(t, empty.srv.URL+"/api/redactions", adminToken)
	if strings.TrimSpace(string(body)) != `{"redactions":[]}` {
		t.Fatalf("empty = %s", body)
	}
}

// A real request with secrets through intake and the redact stage: detail and
// redaction endpoints show placeholders and types, never the values.
func TestNoPlaintextInDetailAndRedactions(t *testing.T) {
	e := newEnv(t, nil)
	key := addClient(t, e.st, "c1")
	const (
		mail   = "alice.leak@muster-firma.de"
		apiKey = "sk-ant-api03-LEAKCANARY0123456789abcdefghijklmnopqrstuv"
	)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), mail) {
			t.Error("upstream saw the plaintext")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"usage":{"input_tokens":10,"output_tokens":5}}`)
	}))
	t.Cleanup(up.Close)
	srv, err := server.New(server.Options{
		Admin: e.admin,
		Runner: pipeline.NewRunner([]pipeline.Stage{&pipeline.Intake{Keys: auth.NewVerifier(e.st)},
			redact.NewStage(redact.Builtin())}),
		Store: e.st, UpstreamURL: up.URL, UpstreamKey: cpaKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(srv.Handler())
	t.Cleanup(proxy.Close)
	body := `{"model":"claude-opus-5-5","messages":[{"role":"user","content":"mail ` + mail + ` key ` + apiKey + `"}]}`
	req, _ := http.NewRequest(http.MethodPost, proxy.URL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	var l RedactionList
	_, raw := get(t, e.srv.URL+"/api/redactions", adminToken)
	if err := json.Unmarshal(raw, &l); err != nil || len(l.Redactions) < 2 {
		t.Fatalf("redactions: %v %s", err, raw)
	}
	types := map[string]string{}
	for _, r := range l.Redactions {
		types[r.Type] = r.Placeholder
	}
	if types["email"] != "<redacted:email_1>" || types["api_key"] != "<redacted:api_key_1>" {
		t.Fatalf("types = %v", types)
	}
	_, detail := get(t, e.srv.URL+"/api/traces/"+l.Redactions[0].TraceID, adminToken)
	for _, out := range [][]byte{raw, detail} {
		for _, secret := range []string{mail, apiKey, "LEAKCANARY", "alice.leak"} {
			if strings.Contains(string(out), secret) {
				t.Fatalf("plaintext %q in %s", secret, out)
			}
		}
	}
	var d TraceDetail
	if err := json.Unmarshal(detail, &d); err != nil || len(d.Stages) != 2 || len(d.Stages[1].Matches) != 2 ||
		d.Stages[1].Matches[0].Placeholder != "<redacted:email_1>" || !strings.Contains(d.Stages[1].Diff, "<redacted:email_1>") {
		t.Fatalf("detail: %v %s", err, detail)
	}
	// Masks (issue #112): present, the same in detail and list, never the value.
	for i, m := range d.Stages[1].Matches {
		if m.Mask == "" || !strings.Contains(m.Mask, "•") || m.Mask != redact.Mask(m.Rule, m.Type, []string{mail, apiKey}[i]) {
			t.Errorf("detail match %d mask = %q", i, m.Mask)
		}
	}
	for _, r := range l.Redactions {
		if r.Mask == "" {
			t.Errorf("redaction %s without mask", r.Placeholder)
		}
	}
}
