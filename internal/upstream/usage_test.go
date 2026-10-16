package upstream

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type usageTransport func(*http.Request) (*http.Response, error)

func (f usageTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestUsageReadUsesFixedEndpointAndPercentageScale(t *testing.T) {
	client := &http.Client{Transport: usageTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.String() != "https://api.anthropic.com/api/oauth/usage" || r.Header.Get("Authorization") != "Bearer memory-only-token" {
			t.Fatal("unexpected request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"seven_day":{"utilization":80,"resets_at":"2026-10-08T00:00:00Z"},"five_hour":{"utilization":10,"resets_at":"2026-10-04T16:00:00Z"},"secret":"must-not-escape"}`)), Header: make(http.Header)}, nil
	})}
	l, err := readUsage(context.Background(), "memory-only-token", client)
	if err != nil || l.SevenDay.Utilization == nil || *l.SevenDay.Utilization != 0.8 || l.SevenDay.Reset == nil {
		t.Fatal(l, err)
	}
}
func TestUsageReadRejectsBodiesAndLeavesMissingValuesUnknown(t *testing.T) {
	for _, tc := range []struct {
		status    int
		body      string
		wantErr   bool
		wantValue bool
	}{{403, `{"token":"secret"}`, true, false}, {302, `redirect-secret`, true, false}, {200, `{"seven_day":{"utilization":-1,"resets_at":"bad"}}`, false, false}, {200, `{"five_hour":null,"seven_day":null}`, false, false}, {200, `{"seven_day":{"utilization":0}}`, false, true}, {200, `{} {}`, true, false}, {200, strings.Repeat("x", 65537), true, false}} {
		client := &http.Client{Transport: usageTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
		})}
		l, err := readUsage(context.Background(), "secret", client)
		if (err != nil) != tc.wantErr {
			t.Fatal(tc.status, err)
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Fatal("leaked body")
		}
		if (l.SevenDay.Utilization != nil) != tc.wantValue {
			t.Fatal("invalid/missing utilization treated as zero")
		}
	}
}
