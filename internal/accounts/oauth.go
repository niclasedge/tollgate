package accounts

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// These protocol values are verified against the pinned CPA source (ADR-0015).
const oauthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
const oauthRedirect = "http://localhost:54545/callback"

type pendingLogin struct {
	verifier string
	expires  time.Time
}
type OAuth struct {
	mu       sync.Mutex
	pending  map[string]pendingLogin
	http     *http.Client
	tokenURL string
	now      func() time.Time
}

func NewOAuth() *OAuth {
	return &OAuth{pending: map[string]pendingLogin{}, http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, tokenURL: "https://platform.claude.com/v1/oauth/token", now: time.Now}
}
func randomText() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("Zufallsquelle nicht verfügbar")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func (o *OAuth) Start() (Login, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.now()
	for id, p := range o.pending {
		if !now.Before(p.expires) {
			delete(o.pending, id)
		}
	}
	if len(o.pending) >= 10 {
		return Login{}, errors.New("Zu viele offene Logins; bitte in zehn Minuten erneut versuchen")
	}
	id, err := randomText()
	if err != nil {
		return Login{}, err
	}
	verifier, err := randomText()
	if err != nil {
		return Login{}, err
	}
	challenge := sha256.Sum256([]byte(verifier))
	exp := now.Add(10 * time.Minute)
	q := url.Values{"code": {"true"}, "client_id": {oauthClientID}, "response_type": {"code"}, "redirect_uri": {oauthRedirect}, "scope": {"user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}, "state": {id}}
	o.pending[id] = pendingLogin{verifier, exp}
	return Login{ID: id, URL: "https://claude.ai/oauth/authorize?" + q.Encode(), ExpiresAt: exp.UTC().Format(time.RFC3339)}, nil
}

type exchangeRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	ClientID     string `json:"client_id"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"`
}

func decodeJSON(r io.Reader, v any) error { return json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(v) }
func (o *OAuth) Complete(ctx context.Context, id, callback string) ([]byte, error) {
	u, err := url.Parse(strings.TrimSpace(callback))
	if err != nil || u.Scheme != "http" || u.Host != "localhost:54545" || u.Path != "/callback" || u.User != nil {
		return nil, errors.New("Bitte die vollständige Callback-URL (http://localhost:54545/callback?…) einfügen")
	}
	code, state := u.Query().Get("code"), u.Query().Get("state")
	parts := strings.SplitN(code, "#", 2)
	code = parts[0]
	if len(parts) == 2 {
		if state != "" && state != parts[1] {
			return nil, errors.New("Login-State stimmt nicht überein")
		}
		state = parts[1]
	}
	if state != id || code == "" || len(code) > 4096 || u.Query().Get("error") != "" {
		return nil, errors.New("Login-State stimmt nicht überein oder Freigabe fehlt")
	}
	o.mu.Lock()
	p, ok := o.pending[id]
	if ok {
		delete(o.pending, id)
	}
	o.mu.Unlock()
	if !ok || !o.now().Before(p.expires) {
		return nil, errors.New("Login abgelaufen oder bereits verwendet; bitte neu starten")
	}
	body, _ := json.Marshal(exchangeRequest{"authorization_code", code, oauthRedirect, oauthClientID, p.verifier, id})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.tokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("Login-Anfrage fehlgeschlagen")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "axios/1.15.2")
	res, err := o.http.Do(req)
	if err != nil {
		return nil, errors.New("Login-Dienst nicht erreichbar; bitte Login neu starten")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Login abgelehnt (HTTP %d); bitte neu starten", res.StatusCode)
	}
	var t struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Account      struct {
			Email string `json:"email_address"`
			UUID  string `json:"uuid"`
		} `json:"account"`
		Organization struct {
			UUID string `json:"uuid"`
		} `json:"organization"`
	}
	if decodeJSON(res.Body, &t) != nil || t.AccessToken == "" || t.RefreshToken == "" || t.Account.Email == "" || t.ExpiresIn <= 0 {
		return nil, errors.New("Login-Dienst lieferte keine vollständigen Account-Daten")
	}
	deviceBytes := make([]byte, 32)
	if _, err := rand.Read(deviceBytes); err != nil {
		return nil, errors.New("Geräteidentität konnte nicht erzeugt werden")
	}
	device := hex.EncodeToString(deviceBytes)
	now := o.now().UTC()
	return json.Marshal(map[string]any{"type": "claude", "email": t.Account.Email, "access_token": t.AccessToken, "refresh_token": t.RefreshToken, "account_uuid": t.Account.UUID, "organization_uuid": t.Organization.UUID, "last_refresh": now.Format(time.RFC3339), "expired": now.Add(time.Duration(t.ExpiresIn) * time.Second).Format(time.RFC3339), "claude_device_ids": []string{device}})
}
