package upstream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func profileClient(t *testing.T, status int, body string) *http.Client {
	t.Helper()
	return &http.Client{Transport: usageTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.String() != "https://api.anthropic.com/api/oauth/profile" || r.Header.Get("Authorization") != "Bearer memory-only-token" || r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
			t.Fatal("unexpected request")
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
}

func TestProfileReadKeepsOnlyFixedIdentifiers(t *testing.T) {
	body := `{"account":{"uuid":"acc-uuid-secret","email":"person@example.com","display_name":"Erika Muster"},"organization":{"uuid":"org-uuid-secret","name":"Muster GmbH","organization_type":"claude_max","rate_limit_tier":"default_claude_max_20x","subscription_status":"active","billing_type":"stripe_subscription"}}`
	p, err := readProfile(context.Background(), "memory-only-token", profileClient(t, 200, body))
	if err != nil {
		t.Fatal(err)
	}
	if p.OrganizationType != "claude_max" || p.RateLimitTier != "default_claude_max_20x" || p.SubscriptionStatus != "active" {
		t.Fatalf("%+v", p)
	}
}

func TestProfileReadRejectsUnexpectedValuesAndNeverLeaks(t *testing.T) {
	long := strings.Repeat("a", 65)
	for _, tc := range []struct {
		name, body string
		status     int
		wantErr    bool
	}{
		{"spaces and capitals", `{"organization":{"organization_type":"Muster GmbH","rate_limit_tier":"Default","subscription_status":"act ive"}}`, 200, false},
		{"too long", `{"organization":{"organization_type":"` + long + `"}}`, 200, false},
		{"wrong type", `{"organization":{"organization_type":42}}`, 200, true},
		{"missing organization", `{"account":{"email":"person@example.com"}}`, 200, false},
		{"trailing data", `{} {}`, 200, true},
		{"too large", strings.Repeat("x", 65537), 200, true},
		{"unauthorized", `{"error":"secret-body"}`, 401, true},
	} {
		p, err := readProfile(context.Background(), "memory-only-token", profileClient(t, tc.status, tc.body))
		if (err != nil) != tc.wantErr {
			t.Fatal(tc.name, err)
		}
		if p != (Profile{}) {
			t.Fatalf("%s: unexpected values %+v", tc.name, p)
		}
		if err != nil && (strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "memory-only-token")) {
			t.Fatal(tc.name, "leaked body or token")
		}
	}
	_, err := readProfile(context.Background(), "memory-only-token", profileClient(t, 401, `{}`))
	var status *UsageError
	if !errors.As(err, &status) || status.Status != 401 {
		t.Fatal("401 must keep its status for the token state", err)
	}
}
