package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

func runIntake(t *testing.T, in Input) (*Request, *Trace, error) {
	t.Helper()
	return NewRunner([]Stage{newIntake()}).Run(context.Background(), in)
}

func TestIntakeAuth(t *testing.T) {
	body := `{"messages":[]}`
	cases := []struct {
		name   string
		header func() http.Header
		client string
		status int
		typ    string
	}{
		{"bearer", authHeader, "client-a", 0, ""},
		{"bearer lowercase scheme", func() http.Header { h := http.Header{}; h.Set("Authorization", "bearer "+testKey); return h }, "client-a", 0, ""},
		{"x-api-key", func() http.Header { h := http.Header{}; h.Set("X-Api-Key", "tg_other"); return h }, "client-b", 0, ""},
		{"missing", func() http.Header { return http.Header{} }, "", 401, "authentication_error"},
		{"wrong key", func() http.Header { h := http.Header{}; h.Set("Authorization", "Bearer nope"); return h }, "", 401, "authentication_error"},
		{"wrong scheme", func() http.Header { h := http.Header{}; h.Set("Authorization", "Basic "+testKey); return h }, "", 401, "authentication_error"},
		{"other scheme falls back to x-api-key", func() http.Header {
			h := http.Header{}
			h.Set("Authorization", "Basic dXNlcjpwYXNz")
			h.Set("X-Api-Key", "tg_other")
			return h
		}, "client-b", 0, ""},
		{"empty bearer", func() http.Header { h := http.Header{}; h.Set("Authorization", "Bearer "); return h }, "", 401, "authentication_error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, trace, err := runIntake(t, Input{Method: "POST", Path: "/v1/messages", Header: c.header(), Body: []byte(body)})
			if trace == nil || trace.TraceID == "" {
				t.Fatal("every request needs a trace id, even rejected ones")
			}
			if c.status == 0 {
				if err != nil || req.ClientID != c.client || trace.ClientID != c.client {
					t.Fatalf("err=%v client=%q", err, req.ClientID)
				}
				return
			}
			var se *StageError
			if !errors.As(err, &se) || se.Status != c.status || se.Type != c.typ {
				t.Fatalf("err = %#v", err)
			}
			if len(trace.Stages) != 1 || !trace.Stages[0].Failed {
				t.Errorf("trace = %+v", trace.Stages)
			}
		})
	}
}

func TestIntakeVerifierFailureIs500(t *testing.T) {
	r := NewRunner([]Stage{&Intake{Keys: brokenKeys{}}})
	_, _, err := r.Run(context.Background(), input("/v1/messages", `{"messages":[]}`))
	var se *StageError
	if !errors.As(err, &se) || se.Status != 500 || se.Type != "api_error" {
		t.Fatalf("err = %#v", err)
	}
	if got := err.Error(); strings.Contains(got, "secret-dsn") {
		t.Errorf("error leaks cause: %q", got)
	}
}

func TestIntakeParsesMessagesEndpoints(t *testing.T) {
	body := `{"model":"claude-x","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	for path, ep := range map[string]Endpoint{
		"/v1/messages":              EndpointMessages,
		"/v1/messages/count_tokens": EndpointCountTokens,
	} {
		req, _, err := runIntake(t, input(path, body))
		if err != nil {
			t.Fatal(err)
		}
		if req.Endpoint != ep || req.Model != "claude-x" || !req.Stream {
			t.Errorf("%s: endpoint=%v model=%q stream=%v", path, req.Endpoint, req.Model, req.Stream)
		}
	}
}

func TestIntakeRejectsInvalidJSONWith400(t *testing.T) {
	for _, body := range []string{``, `not json`, `{"messages":[`, `{"messages":[]} trailing`, `[]`} {
		_, trace, err := runIntake(t, input("/v1/messages", body))
		var se *StageError
		if !errors.As(err, &se) || se.Status != 400 || se.Type != "invalid_request_error" {
			t.Fatalf("body %q: err = %#v", body, err)
		}
		if !trace.Stages[0].Failed {
			t.Errorf("body %q: stage not marked failed", body)
		}
		status, resp := ErrorResponse(err)
		if status != 400 || string(resp) != `{"type":"error","error":{"type":"invalid_request_error","message":"request body is not valid JSON"}}` {
			t.Errorf("response = %d %s", status, resp)
		}
	}
}

func TestIntakePassesOtherEndpointsUnparsed(t *testing.T) {
	for _, in := range []Input{
		{Method: "GET", Path: "/v1/models", Header: authHeader()},
		{Method: "POST", Path: "/v1/messages/batches", Header: authHeader(), Body: []byte("not json")},
		{Method: "GET", Path: "/v1/messages", Header: authHeader()},
	} {
		req, _, err := runIntake(t, in)
		if err != nil || req.Endpoint != EndpointPassthrough {
			t.Fatalf("%s %s: err=%v endpoint=%v", in.Method, in.Path, err, req.Endpoint)
		}
	}
	// Auth still applies to pass-through endpoints.
	_, _, err := runIntake(t, Input{Method: "GET", Path: "/v1/models", Header: http.Header{}})
	if err == nil {
		t.Fatal("pass-through must still require a client key")
	}
}

func TestSessionKeyDerivation(t *testing.T) {
	const sid = "11111111-2222-3333-4444-555555555555"
	cases := []struct {
		name    string
		header  string
		body    string
		wantSID string
		src     SessionSource
	}{
		{"header", sid, `{"messages":[]}`, sid, SessionFromHeader},
		{"header wins over metadata", sid, `{"metadata":{"user_id":"{\"session_id\":\"other\"}"},"messages":[]}`, sid, SessionFromHeader},
		{"json user_id", "", `{"metadata":{"user_id":"{\"device_id\":\"d\",\"session_id\":\"` + sid + `\"}"},"messages":[]}`, sid, SessionFromMetadata},
		{"legacy user_id", "", `{"metadata":{"user_id":"user_abc_account_def_session_` + sid + `"},"messages":[]}`, sid, SessionFromMetadata},
		{"none", "", `{"messages":[]}`, "", SessionNone},
		{"unparsable user_id", "", `{"metadata":{"user_id":"whatever"},"messages":[]}`, "", SessionNone},
		{"empty session in json", "", `{"metadata":{"user_id":"{\"session_id\":\"\"}"},"messages":[]}`, "", SessionNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := input("/v1/messages", c.body)
			if c.header != "" {
				in.Header.Set("X-Claude-Code-Session-Id", c.header)
			}
			req, trace, err := runIntake(t, in)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if c.wantSID != "" {
				want = sum([]byte("client-a\x00" + c.wantSID))
			}
			if req.SessionKey != want || req.SessionSource != c.src || trace.SessionKey != want {
				t.Fatalf("key=%q source=%v, want %q %v", req.SessionKey, req.SessionSource, want, c.src)
			}
		})
	}
}

func TestSessionKeyIsPerClient(t *testing.T) {
	mk := func(key string) string {
		h := http.Header{}
		h.Set("Authorization", "Bearer "+key)
		h.Set("X-Claude-Code-Session-Id", "same-session")
		req, _, err := runIntake(t, Input{Method: "POST", Path: "/v1/messages", Header: h, Body: []byte(`{"messages":[]}`)})
		if err != nil {
			t.Fatal(err)
		}
		return req.SessionKey
	}
	if a, b := mk(testKey), mk("tg_other"); a == b || a == "" || b == "" {
		t.Fatalf("session keys must differ per client: %q %q", a, b)
	}
}

// TestSessionKeyFromRecordedFixture runs a sanitized, real Claude Code 2.1.289
// request (testdata/fixtures) through stage 0.
func TestSessionKeyFromRecordedFixture(t *testing.T) {
	const sid = "00000000-0000-4000-8000-000000000001"
	raw, err := os.ReadFile("../../testdata/fixtures/claude-code-2.1.289-messages.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Method string            `json:"method"`
		Path   string            `json:"path"`
		Header map[string]string `json:"header"`
		Body   json.RawMessage   `json:"body"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		dropHeader bool
		src        SessionSource
	}{
		{"header present", false, SessionFromHeader},
		{"header removed falls back to metadata.user_id", true, SessionFromMetadata},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input(fx.Path, string(fx.Body))
			in.Method = fx.Method
			for k, v := range fx.Header {
				in.Header.Set(k, v)
			}
			if tc.dropHeader {
				in.Header.Del("X-Claude-Code-Session-Id")
			}
			req, _, err := runIntake(t, in)
			if err != nil {
				t.Fatal(err)
			}
			if want := sum([]byte("client-a\x00" + sid)); req.SessionKey != want || req.SessionSource != tc.src {
				t.Fatalf("key=%q source=%v, want %q %v", req.SessionKey, req.SessionSource, want, tc.src)
			}
			if req.Model != "claude-opus-5-5" || !req.Stream {
				t.Fatalf("model=%q stream=%v", req.Model, req.Stream)
			}
		})
	}
}
