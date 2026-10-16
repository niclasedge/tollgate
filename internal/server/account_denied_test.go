package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

const oauthDenied = `{"type":"error","error":{"type":"permission_error","message":"OAuth authentication is currently not allowed for this organization.","details":{"error_code":"oauth_not_allowed_for_organization"}},"request_id":"req_x"}`

func TestUpstreamOAuthDeniedBecomesRetryable(t *testing.T) {
	st := openStore(t)
	var logs bytes.Buffer
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, oauthDenied)
	})
	ts := newProxy(t, up, proxyOpts{store: st, logger: slog.New(slog.NewTextHandler(&logs, nil))})
	resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages?beta=true", msgBody, auth(nil))
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), `"api_error"`) {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
	if resp.Header.Get("Retry-After") != "15" || resp.Header.Get("X-Should-Retry") != "true" {
		t.Fatalf("retry headers %v", resp.Header)
	}
	if strings.Contains(string(body), "req_x") {
		t.Fatalf("upstream body leaked: %q", body)
	}
	if tr := waitFinished(t, st); tr.Status != 503 || tr.ErrorKind != "account_not_allowed" {
		t.Fatalf("trace %+v", tr)
	}
	if l := logs.String(); !strings.Contains(l, "upstream account not allowed") || !strings.Contains(l, "oauth_not_allowed_for_organization") {
		t.Fatalf("log %q", l)
	}
}

func TestOtherUpstream403PassesThrough(t *testing.T) {
	for name, b := range map[string]string{
		"permission without code": `{"type":"error","error":{"type":"permission_error","message":"no access to model"}}`,
		"other code":              `{"type":"error","error":{"type":"permission_error","message":"x","details":{"error_code":"model_not_allowed"}}}`,
		"not json":                `forbidden`,
	} {
		t.Run(name, func(t *testing.T) {
			up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, b)
			})
			ts := newProxy(t, up, proxyOpts{})
			resp, body := doReq(t, http.MethodPost, ts.URL+"/v1/messages", msgBody, auth(nil))
			if resp.StatusCode != http.StatusForbidden || string(body) != b {
				t.Fatalf("status %d body %q", resp.StatusCode, body)
			}
		})
	}
}

func TestOAuthDeniedOnOtherEndpointPassesThrough(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, oauthDenied)
	})
	ts := newProxy(t, up, proxyOpts{})
	resp, body := doReq(t, http.MethodGet, ts.URL+"/v1/models", "", auth(nil))
	if resp.StatusCode != http.StatusForbidden || string(body) != oauthDenied {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
}
