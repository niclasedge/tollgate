package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/niclasedge/tollgate/internal/meter"
)

// UsageError carries only a status, never an upstream body or token.
type UsageError struct{ Status int }

func (e *UsageError) Error() string { return "Usage nicht abrufbar" }

// ReadUsage reads subscription limits without a model call. Tokens stay in RAM.
func ReadUsage(ctx context.Context, token string) (meter.Limits, error) {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return readUsage(ctx, token, client)
}
func readUsage(ctx context.Context, token string, client *http.Client) (meter.Limits, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return meter.Limits{}, errors.New("Usage-Anfrage ungültig")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-cli/2.1.220 (external, cli)")
	res, err := client.Do(req)
	if err != nil {
		return meter.Limits{}, errors.New("Usage nicht erreichbar")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return meter.Limits{}, &UsageError{Status: res.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return meter.Limits{}, errors.New("Usage-Antwort ungültig")
	}
	type window struct {
		Utilization *float64 `json:"utilization"`
		Reset       string   `json:"resets_at"`
	}
	var data struct {
		FiveHour *window `json:"five_hour"`
		SevenDay *window `json:"seven_day"`
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if d.Decode(&data) != nil {
		return meter.Limits{}, errors.New("Usage-Antwort ungültig")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return meter.Limits{}, errors.New("Usage-Antwort ungültig")
	}
	convert := func(w *window) meter.Window {
		if w == nil {
			return meter.Window{}
		}
		out := meter.Window{}
		if reset, err := time.Parse(time.RFC3339, w.Reset); err == nil {
			reset = reset.UTC()
			out.Reset = &reset
		}
		if w.Utilization != nil && !math.IsNaN(*w.Utilization) && !math.IsInf(*w.Utilization, 0) && *w.Utilization >= 0 && *w.Utilization <= 1000 {
			u := *w.Utilization / 100
			out.Utilization = &u
			out.Status = "allowed"
			if u >= 1 {
				out.Status = "rejected"
			} else if u >= 0.8 {
				out.Status = "allowed_warning"
			}
		}
		return out
	}
	return meter.Limits{FiveHour: convert(data.FiveHour), SevenDay: convert(data.SevenDay), ObservedAt: time.Now().UTC()}, nil
}
