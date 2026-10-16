package upstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The sample must use the exact Sonnet ID requested by the user and registered
// by CPA. An unregistered shorthand cannot verify an account.
func TestAccountSampleUsesRegisteredModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
		}
		if json.NewDecoder(r.Body).Decode(&b) != nil || b.Model != "claude-sonnet-5-5" {
			w.WriteHeader(400)
			return
		}
		if b.MaxTokens > 16 {
			t.Error("sample must remain small")
		}
		w.Write([]byte(`{"type":"message","model":"claude-sonnet-5-5","content":[{"type":"text","text":"OK"}]}`))
	}))
	defer srv.Close()
	up, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	result, err := up.TestAccount(context.Background(), "key")
	if err != nil || !result.OK {
		t.Fatal("sample did not test the account", result, err)
	}
}

func TestAccountSampleClassifiesOrganizationPolicyWithoutLeakingBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":{"type":"permission_error","message":"OAuth authentication is currently not allowed for this organization."},"token":"private-token-must-not-be-returned"}`))
	}))
	defer srv.Close()
	up, _ := New(srv.URL)
	result, err := up.TestAccount(context.Background(), "key")
	if err != nil || result.OK || result.Status != 403 || result.FailureKind != "oauth_not_allowed" {
		t.Fatal(result, err)
	}
}
