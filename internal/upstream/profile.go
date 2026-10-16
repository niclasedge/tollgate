package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"time"
)

// Profile holds only fixed plan identifiers of the OAuth profile. Names, UUIDs
// and e-mail addresses of the response are discarded.
type Profile struct {
	OrganizationType   string
	RateLimitTier      string
	SubscriptionStatus string
}

var profileIdentifier = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// ReadProfile reads the subscription plan without a model call. Tokens stay in RAM.
func ReadProfile(ctx context.Context, token string) (Profile, error) {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return readProfile(ctx, token, client)
}

func readProfile(ctx context.Context, token string, client *http.Client) (Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/api/oauth/profile", nil)
	if err != nil {
		return Profile{}, errors.New("Profil-Anfrage ungültig")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-cli/2.1.220 (external, cli)")
	res, err := client.Do(req)
	if err != nil {
		return Profile{}, errors.New("Profil nicht erreichbar")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Profile{}, &UsageError{Status: res.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return Profile{}, errors.New("Profil-Antwort ungültig")
	}
	var data struct {
		Organization *struct {
			OrganizationType   string `json:"organization_type"`
			RateLimitTier      string `json:"rate_limit_tier"`
			SubscriptionStatus string `json:"subscription_status"`
		} `json:"organization"`
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if d.Decode(&data) != nil {
		return Profile{}, errors.New("Profil-Antwort ungültig")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return Profile{}, errors.New("Profil-Antwort ungültig")
	}
	if data.Organization == nil {
		return Profile{}, nil
	}
	keep := func(s string) string {
		if profileIdentifier.MatchString(s) {
			return s
		}
		return ""
	}
	o := data.Organization
	return Profile{OrganizationType: keep(o.OrganizationType), RateLimitTier: keep(o.RateLimitTier), SubscriptionStatus: keep(o.SubscriptionStatus)}, nil
}
