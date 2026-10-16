package alert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "fake-slack-token-for-tests"

func TestSlackSend(t *testing.T) {
	var got struct{ Channel, Text string }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+testToken ||
			!strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("request %s auth=%v ct=%q", r.Method, r.Header.Get("Authorization") != "", r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	s := Slack{URL: srv.URL, Client: srv.Client()}
	if err := s.Send(context.Background(), testToken, "C123", "Hallo"); err != nil {
		t.Fatal(err)
	}
	if got.Channel != "C123" || got.Text != "Hallo" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestSlackErrorsHideToken(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   string
	}{
		{200, `{"ok":false,"error":"channel_not_found"}`, "channel_not_found"},
		{200, `{"ok":false,"error":"` + testToken + `"}`, "slack: request failed"},
		{429, `{"ok":false,"error":"ratelimited"}`, "HTTP 429"},
		{200, `kein json`, "slack: invalid response"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		err := Slack{URL: srv.URL, Client: srv.Client()}.Send(context.Background(), testToken, "C1", "x")
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), testToken) {
			t.Errorf("%d %s: err = %v, want %q without token", tc.status, tc.body, err, tc.want)
		}
	}
}
