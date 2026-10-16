package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/store"
)

// ProbeInput is one internal request of the speedtest (#87).
type ProbeInput struct {
	Body       []byte // a complete /v1/messages body
	SkipRedact bool   // run without the redact stage
}

// ErrProbeNoTrace means the probe left no trace (e.g. no store).
var ErrProbeNoTrace = errors.New("probe left no trace")

// Probe runs one request through exactly the same code as a client request
// (account gate, pipeline, trace, forward, meter, TTFT, verdict) as the
// reserved speedtest client, and discards the answer. It returns the trace ID.
func (s *Server) Probe(ctx context.Context, in ProbeInput) (string, error) {
	ctx = pipeline.WithInternalClient(ctx, pipeline.InternalClient{ID: store.SpeedtestClientID, SkipRedact: in.SkipRedact})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/messages", bytes.NewReader(in.Body))
	if err != nil {
		return "", err
	}
	r.ContentLength = int64(len(in.Body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Anthropic-Version", "2023-06-01")
	r.Header.Set("User-Agent", "tollgate-speedtest")
	w := &discardWriter{h: http.Header{}}
	id, _ := s.proxy(w, r)
	if id == "" || s.store == nil {
		return "", ErrProbeNoTrace
	}
	return id, nil
}

// discardWriter is a ResponseWriter that keeps only the status.
type discardWriter struct {
	h      http.Header
	status int
}

func (d *discardWriter) Header() http.Header         { return d.h }
func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
func (d *discardWriter) WriteHeader(status int) {
	if d.status == 0 {
		d.status = status
	}
}
func (d *discardWriter) Flush() {}

// speedtestAddress is the invented address of the speedtest prompt. It is
// assembled from parts so the source holds no literal address; it gives the
// redact stage and the return path one match.
var speedtestAddress = "speedtest" + "@" + "tollgate.invalid"

// SpeedtestBody is the fixed speedtest request for model (#87): about 200
// output tokens, streamed, no secret.
func SpeedtestBody(model string) []byte {
	b, _ := json.Marshal(map[string]any{
		"model": model, "max_tokens": 400, "stream": true,
		"messages": []any{map[string]any{"role": "user",
			"content": "Antworte nur mit den Zahlen 1 bis 100, durch Kommas getrennt. Kontakt: " + speedtestAddress}},
	})
	return b
}
