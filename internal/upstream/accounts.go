package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/niclasedge/tollgate/internal/meter"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// credentialCall never returns CPA error bodies (they may contain secrets).
func (c *Client) credentialCall(ctx context.Context, key, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+"/v8/management/credentials"+path, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("Management-Anfrage ungültig")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("CLIProxyAPI Management nicht erreichbar")
	}
	defer res.Body.Close()
	if res.StatusCode == 404 && method == http.MethodDelete {
		return nil, nil
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Management-Anfrage fehlgeschlagen (HTTP %d)", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, errors.New("Management-Antwort ungültig oder zu groß")
	}
	return data, nil
}
func (c *Client) DownloadCredential(ctx context.Context, key, name string) ([]byte, error) {
	return c.credentialCall(ctx, key, http.MethodGet, "/download?name="+url.QueryEscape(name), nil)
}
func (c *Client) UploadCredential(ctx context.Context, key, name string, data []byte) error {
	_, err := c.credentialCall(ctx, key, http.MethodPost, "?name="+url.QueryEscape(name), data)
	return err
}
func (c *Client) DeleteCredential(ctx context.Context, key, name string) error {
	_, err := c.credentialCall(ctx, key, http.MethodDelete, "?name="+url.QueryEscape(name), nil)
	return err
}

type AccountTest struct {
	OK          bool
	Status      int
	Model       string
	Limits      *meter.Limits
	FailureKind string
}

// TestAccount performs a small real call. Only metadata leaves the method;
// the generated text and arbitrary error bodies are deliberately discarded.
func (c *Client) TestAccount(ctx context.Context, key string) (AccountTest, error) {
	out := AccountTest{Model: "claude-sonnet-5-5"}
	body := []byte(`{"model":"claude-sonnet-5-5","max_tokens":16,"messages":[{"role":"user","content":"Reply with OK."}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return out, errors.New("Testanfrage ungültig")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-cli/2.1.220 (external, cli)")
	client := *c.http
	client.Timeout = 60 * time.Second
	res, err := client.Do(req)
	if err != nil {
		return out, errors.New("Beispielaufruf nicht erreichbar oder Zeitlimit überschritten")
	}
	defer res.Body.Close()
	out.Status = res.StatusCode
	if l, ok := meter.ParseLimits(res.Header, time.Now()); ok {
		out.Limits = &l
	}
	if res.StatusCode != 200 {
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if res.StatusCode == 403 && json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&failure) == nil && strings.Contains(failure.Error.Message, "OAuth authentication is currently not allowed for this organization") {
			out.FailureKind = "oauth_not_allowed"
		}
		return out, nil
	}
	var result struct {
		Type    string            `json:"type"`
		Model   string            `json:"model"`
		Content []json.RawMessage `json:"content"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result) != nil || result.Type != "message" || len(result.Content) == 0 {
		return out, errors.New("Beispielaufruf lieferte keine gültige Messages-Antwort")
	}
	out.OK = true
	if result.Model != "" {
		out.Model = result.Model
	}
	return out, nil
}

// PauseCredential is the ADR-0016 barrier. A missing endpoint fails closed.
func (c *Client) PauseCredential(ctx context.Context, key, name string, paused bool) error {
	body, _ := json.Marshal(struct {
		Name   string `json:"name"`
		Paused bool   `json:"paused"`
	}{name, paused})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v8/management/credentials/refresh-pause", bytes.NewReader(body))
	if err != nil {
		return errors.New("Refresh-Barriere nicht verfügbar")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := *c.http
	client.Timeout = 45 * time.Second
	res, err := client.Do(req)
	if err != nil {
		return errors.New("Refresh-Barriere nicht erreichbar oder Zeitlimit überschritten")
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode != 200 {
		return fmt.Errorf("Refresh-Barriere fehlgeschlagen (HTTP %d); CPA-Image neu bauen", res.StatusCode)
	}
	return nil
}
