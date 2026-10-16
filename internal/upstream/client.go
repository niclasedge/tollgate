package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const healthTimeout = 5 * time.Second

// Client is a minimal HTTP client for the local CLIProxyAPI process.
// It refuses any base URL that does not point at the loopback interface.
type Client struct {
	base string
	http *http.Client
}

// New validates baseURL (http, loopback host, no path or userinfo) and
// returns a Client.
func New(baseURL string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.New("upstream: invalid base URL")
	}
	if u.Scheme != "http" || u.Host == "" {
		return nil, errors.New("upstream: base URL must be http://<loopback-host>:<port>")
	}
	if u.User != nil {
		return nil, errors.New("upstream: base URL must not contain credentials")
	}
	if p := strings.Trim(u.Path, "/"); p != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("upstream: base URL must not contain a path or query")
	}
	if !isLoopback(u.Hostname()) {
		return nil, fmt.Errorf("upstream: host %q is not loopback; CLIProxyAPI is localhost-only", u.Hostname())
	}
	return &Client{
		base: "http://" + u.Host,
		http: &http.Client{
			Timeout: healthTimeout,
			// A redirect would leave localhost; treat it as a response.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Health calls GET /healthz and succeeds only on 200 with {"status":"ok"}.
// It sends no credentials; the endpoint needs none. Errors never include the
// response body.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("upstream health: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("upstream health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream health: unexpected status %d", resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body); err != nil {
		return errors.New("upstream health: response is not valid JSON")
	}
	if body.Status != "ok" {
		return fmt.Errorf("upstream health: status %q", body.Status)
	}
	return nil
}

// Credential is the part of a CLIProxyAPI credential entry that tollgate
// reads. Only these fields are decoded, so tokens and file contents that the
// management API might return never reach tollgate.
type Credential struct {
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`
	Email          string    `json:"email"`
	Status         string    `json:"status"`
	Disabled       bool      `json:"disabled"`
	NextRetryAfter time.Time `json:"-"` // zero unless rate limited
}

const maxCredentialsBody = 1 << 20

// Credentials calls GET /v8/management/credentials with the management key
// as bearer token and returns the credential entries. Errors never include the
// key or the response body.
func (c *Client) Credentials(ctx context.Context, managementKey string) ([]Credential, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v8/management/credentials", nil)
	if err != nil {
		return nil, fmt.Errorf("upstream credentials: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+managementKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream credentials: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream credentials: unexpected status %d", resp.StatusCode)
	}
	var body struct {
		Files []struct {
			Credential
			// Decoded leniently: a malformed value only loses the cooldown.
			NextRetryAfter json.RawMessage `json:"next_retry_after"`
		} `json:"files"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCredentialsBody)).Decode(&body); err != nil {
		return nil, errors.New("upstream credentials: response is not valid JSON")
	}
	out := make([]Credential, 0, len(body.Files))
	for _, f := range body.Files {
		c := f.Credential
		_ = json.Unmarshal(f.NextRetryAfter, &c.NextRetryAfter)
		out = append(out, c)
	}
	return out, nil
}
