package pipeline

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

// Endpoint classifies a request path.
type Endpoint string

const (
	// EndpointMessages is POST /v1/messages.
	EndpointMessages Endpoint = "messages"
	// EndpointCountTokens is POST /v1/messages/count_tokens.
	EndpointCountTokens Endpoint = "count_tokens"
	// EndpointPassthrough is everything else; it is forwarded unchanged.
	EndpointPassthrough Endpoint = "passthrough"
)

// SessionSource says where the session id of a request came from.
type SessionSource string

const (
	SessionNone         SessionSource = "none"
	SessionFromHeader   SessionSource = "header"
	SessionFromMetadata SessionSource = "metadata"
)

// Input is what the caller (the proxy, #6) hands to the runner.
type Input struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// Request is the per-request context shared by all stages.
//
// Body is owned by the runner: stages must treat it as read-only and return
// replacements instead. Offsets in replacements refer to the Body the stage
// received, so a stage rescans Body itself and never caches offsets.
type Request struct {
	TraceID string
	Method  string
	Path    string
	// Header is a clone of the incoming headers; stages must not mutate it.
	Header       http.Header
	Body         []byte
	OriginalBody []byte
	Endpoint     Endpoint

	// Set by Intake.
	ClientID string
	Model    string
	Stream   bool
	// SessionKey is hex(SHA-256(client_id || 0x00 || session_id)), or empty
	// when the request carries no session ("client without session", ADR-0006).
	SessionKey    string
	SessionSource SessionSource
	// ConfigVersion is the pinned configuration version (0 = none pinned).
	ConfigVersion int64

	// Placeholders maps each placeholder the redact stage put into Body to
	// its plaintext (ADR-0006). It lives only in RAM for the lifetime of the
	// request, is used by the return path (#14) and must never be logged,
	// stored or serialized.
	Placeholders map[string]string `json:"-"`

	report stageReport // reset by the runner before every stage
}

// Match is one redaction hit in a stage report: the byte range of the
// placeholder in the stage's output body, its type and the detector. It never
// carries the replaced value.
type Match struct {
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	Type   string `json:"type"`
	Rule   string `json:"rule"`
	// Placeholder is the placeholder text, e.g. "<redacted:email_1>".
	Placeholder string `json:"placeholder,omitempty"`
	// Mask is the display mask of the replaced value (issue #112), e.g.
	// "n•••@f•••.de". It is computed by the redact stage and never holds
	// more than the mask rules allow.
	Mask string `json:"mask,omitempty"`
}

type stageReport struct {
	diff    string
	matches []Match
}

// Report attaches a diff and matches to the trace entry of the running stage.
// Both must be built from redacted data only. A later call replaces an
// earlier one; a failing stage's report is dropped.
func (r *Request) Report(diff string, matches []Match) {
	r.report = stageReport{diff: diff, matches: matches}
}

// Stage is one pipeline step. Process returns the replacements against
// req.Body; it must be deterministic (ADR-0004 checklist) and must not put
// plaintext secrets into the error it returns.
type Stage interface {
	Name() string
	Process(ctx context.Context, req *Request) ([]jsonsplice.Replacement, error)
}

// StageTrace is the record one stage leaves in the trace. It holds only
// hashes, counts and the replacement list (new, already redacted values) -
// never the replaced original text.
type StageTrace struct {
	Name          string                   `json:"name"`
	Duration      time.Duration            `json:"duration_ns"`
	TokensBefore  int                      `json:"tokens_before"`
	TokensAfter   int                      `json:"tokens_after"`
	InHash        string                   `json:"in_hash"`
	OutHash       string                   `json:"out_hash"`
	ConfigVersion int64                    `json:"config_version"`
	BytesBefore   int                      `json:"bytes_before"`
	BytesAfter    int                      `json:"bytes_after"`
	Replacements  []jsonsplice.Replacement `json:"replacements,omitempty"`
	Diff          string                   `json:"diff,omitempty"`
	Matches       []Match                  `json:"matches,omitempty"`
	Failed        bool                     `json:"failed,omitempty"`
}

// Trace is the complete record of one request.
type Trace struct {
	TraceID       string       `json:"trace_id"`
	StartedAt     time.Time    `json:"started_at"`
	Method        string       `json:"method"`
	Path          string       `json:"path"`
	ClientID      string       `json:"client_id,omitempty"`
	SessionKey    string       `json:"session_key,omitempty"`
	ConfigVersion int64        `json:"config_version"`
	Stages        []StageTrace `json:"stages"`
}

// Estimator estimates the token count of a body for the trace.
type Estimator func(body []byte) int

// EstimateTokens is the default estimator: ceil(bytes/4) over the raw JSON
// body. JSON structure inflates the byte count while the tokenizer of
// Claude 4.7+ yields about 30% more tokens per text; both roughly cancel. The
// estimate is a cheap, deterministic hint for the trace, not a bill: the real
// count is the upstream usage stored after the response (#7).
func EstimateTokens(body []byte) int { return (len(body) + 3) / 4 }

// NewTraceID returns 16 random bytes as 32 hex characters. Trace IDs live in
// traces only and never reach the forwarded body.
func NewTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("pipeline: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// Option configures a Runner.
type Option func(*Runner)

// WithClock replaces time.Now (durations and StartedAt only).
func WithClock(now func() time.Time) Option { return func(r *Runner) { r.now = now } }

// WithIDGenerator replaces NewTraceID.
func WithIDGenerator(gen func() string) Option { return func(r *Runner) { r.newID = gen } }

// WithEstimator replaces EstimateTokens.
func WithEstimator(e Estimator) Option { return func(r *Runner) { r.estimate = e } }

// Runner executes stages in order. It is safe for concurrent use as long as
// the stages are.
type Runner struct {
	stages   []Stage
	now      func() time.Time
	newID    func() string
	estimate Estimator
}

// NewRunner builds a runner for the given stages (stage 0 first).
func NewRunner(stages []Stage, opts ...Option) *Runner {
	r := &Runner{stages: stages, now: time.Now, newID: NewTraceID, estimate: EstimateTokens}
	for _, o := range opts {
		o(r)
	}
	return r
}

func hashHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Run assigns a trace ID, runs all stages and applies their replacements.
// It always returns a non-nil Request and Trace. On error the request must
// not be forwarded (ADR-0004, point 7); the error is a *StageError whose
// ErrorResponse is the Anthropic-format answer for the client.
func (r *Runner) Run(ctx context.Context, in Input) (*Request, *Trace, error) {
	req := &Request{
		TraceID:       r.newID(),
		Method:        in.Method,
		Path:          in.Path,
		Header:        in.Header.Clone(),
		Body:          in.Body,
		OriginalBody:  in.Body,
		Endpoint:      EndpointPassthrough,
		SessionSource: SessionNone,
	}
	if req.Header == nil {
		req.Header = http.Header{}
	}
	trace := &Trace{TraceID: req.TraceID, StartedAt: r.now(), Method: in.Method, Path: in.Path}

	hash, tokens := hashHex(req.Body), r.estimate(req.Body)
	internal, _ := internalClient(ctx)
	for _, st := range r.stages {
		if internal.SkipRedact && st.Name() == SkippableStage {
			continue
		}
		start := r.now()
		req.report = stageReport{}
		before := len(req.Body)
		reps, err := st.Process(ctx, req)
		rec := StageTrace{Name: st.Name(), Duration: r.now().Sub(start), TokensBefore: tokens, InHash: hash, BytesBefore: before, BytesAfter: before}
		if err == nil {
			var body []byte
			if err = onlyTargets(req.Body, reps); err == nil {
				body, err = jsonsplice.Apply(req.Body, reps)
			}
			if err != nil {
				err = &StageError{Stage: st.Name(), Status: http.StatusInternalServerError, Type: ErrTypeAPI,
					Message: "internal error in stage " + st.Name(), Cause: err}
			} else {
				req.Body = body
				rec.BytesAfter = len(body)
				rec.Replacements = reps
				rec.Diff, rec.Matches = req.report.diff, req.report.matches
			}
		}
		r.fill(trace, req)
		if err != nil {
			rec.Failed = true
			rec.TokensAfter, rec.OutHash, rec.ConfigVersion = tokens, hash, req.ConfigVersion
			trace.Stages = append(trace.Stages, rec)
			return req, trace, asStageError(st.Name(), err)
		}
		if len(reps) > 0 {
			hash, tokens = hashHex(req.Body), r.estimate(req.Body)
		}
		rec.TokensAfter, rec.OutHash, rec.ConfigVersion = tokens, hash, req.ConfigVersion
		trace.Stages = append(trace.Stages, rec)
	}
	return req, trace, nil
}

// Authenticator is implemented by a stage that can check the client from the
// headers alone (Intake).
type Authenticator interface {
	Authenticate(ctx context.Context, h http.Header) (clientID string, err error)
}

// Authenticate runs the header-only client check of the first stage that
// implements Authenticator, so a caller can reject a request before reading
// its body. Without such a stage it returns nil. The error is a *StageError.
func (r *Runner) Authenticate(ctx context.Context, h http.Header) error {
	for _, st := range r.stages {
		if a, ok := st.(Authenticator); ok {
			if _, err := a.Authenticate(ctx, h); err != nil {
				return asStageError(st.Name(), err)
			}
			return nil
		}
	}
	return nil
}

func (r *Runner) fill(t *Trace, req *Request) {
	t.ClientID, t.SessionKey, t.ConfigVersion = req.ClientID, req.SessionKey, req.ConfigVersion
}

// onlyTargets rejects replacements that are not exactly one of the strings Scan
// reports, so no stage can touch thinking blocks, signatures, member names or
// a range inside a string, even with stale offsets.
func onlyTargets(body []byte, reps []jsonsplice.Replacement) error {
	if len(reps) == 0 {
		return nil
	}
	doc, err := jsonsplice.Scan(body)
	if err != nil {
		return err
	}
	allowed := make(map[[2]int]bool, len(doc.Targets))
	for _, t := range doc.Targets {
		allowed[[2]int{t.Start, t.End}] = true
	}
	for i, r := range reps {
		if !allowed[[2]int{r.Start, r.End}] {
			return fmt.Errorf("pipeline: replacement %d: range [%d,%d) is not a rewritable target", i, r.Start, r.End)
		}
	}
	return nil
}
