package accounts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOAuthStatePKCEAndReplay(t *testing.T) {
	var verifier string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("exchange shape")
		}
		var body exchangeRequest
		if err := decodeJSON(r.Body, &body); err != nil {
			t.Error(err)
		}
		verifier = body.CodeVerifier
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"secret-access","refresh_token":"secret-refresh","expires_in":3600,"account":{"email_address":"b@example.com","uuid":"acct-b"},"organization":{"uuid":"org-b"}}`))
	}))
	defer srv.Close()
	o := NewOAuth()
	o.tokenURL = srv.URL
	login, err := o.Start()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(login.URL)
	state := u.Query().Get("state")
	if state == "" || u.Query().Get("code_challenge") == "" || u.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("PKCE missing")
	}
	if _, err := o.Complete(context.Background(), login.ID, "http://localhost:54545/callback?code=x&state=wrong"); err == nil {
		t.Fatal("wrong state")
	}
	token, err := o.Complete(context.Background(), login.ID, "http://localhost:54545/callback?code=x&state="+state)
	if err != nil {
		t.Fatal(err)
	}
	if verifier == "" || !strings.Contains(string(token), "b@example.com") {
		t.Fatal("exchange failed")
	}
	if _, err := o.Complete(context.Background(), login.ID, "http://localhost:54545/callback?code=x&state="+state); err == nil {
		t.Fatal("replay accepted")
	}
}
func TestOAuthExpiryAndErrorBodyNeverLeaked(t *testing.T) {
	o := NewOAuth()
	now := time.Now()
	o.now = func() time.Time { return now }
	l, _ := o.Start()
	now = now.Add(11 * time.Minute)
	if _, err := o.Complete(context.Background(), l.ID, "http://localhost:54545/callback?code=x&state="+l.ID); err == nil {
		t.Fatal("expired login")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte("secret-code-and-token"))
	}))
	defer srv.Close()
	o.tokenURL = srv.URL
	l, _ = o.Start()
	_, err := o.Complete(context.Background(), l.ID, "http://localhost:54545/callback?code=x&state="+l.ID)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("upstream body leaked", err)
	}
}
