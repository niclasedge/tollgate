package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/config"
	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/perf"
	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/redact"
	"github.com/niclasedge/tollgate/internal/store"
	"github.com/niclasedge/tollgate/internal/upstream"
)

// DefaultMaxBodyBytes caps a request body (Claude Code sends whole histories).
const DefaultMaxBodyBytes = 64 << 20

const (
	finishTimeout = 5 * time.Second
	// bodyReadTimeout bounds silence while reading the request body: slow
	// links may take longer in total as long as bytes keep coming (#130).
	bodyReadTimeout = 30 * time.Second
	// responseHeaderTimeout bounds the wait for upstream response headers. A
	// non-streaming request only gets headers once the answer is complete, so
	// this matches the 10-minute default of the Anthropic SDKs.
	responseHeaderTimeout = 10 * time.Minute
	// maxUsageBody caps how much of a non-streaming response is kept to read
	// its usage; a larger answer is still passed through, just not metered.
	maxUsageBody = 8 << 20
)

// Store is the part of the SQLite store the proxy writes traces to. Only
// hashes and metadata are passed (ADR-0009).
type Store interface {
	RecordTrace(ctx context.Context, t store.Trace) error
	FinishTrace(ctx context.Context, id string, r store.TraceResult) error
}

// PerfStore is the optional part of the store for the "slow" verdict (#87).
// A Store that implements it gets a verdict for every finished request.
type PerfStore interface {
	PerfSamples(ctx context.Context, model string, before time.Time, metric string) ([]perf.Sample, error)
	PutPerf(ctx context.Context, traceID string, ps []store.Perf) error
}

// TPSStore is the optional part of the store for the tokens/s verdict (#116).
type TPSStore interface {
	TPSSamples(ctx context.Context, model string, before time.Time) ([]perf.TPSSample, error)
	PutTPS(ctx context.Context, traceID string, t store.TPS) error
}

// Options configures a Server.
type Options struct {
	AccountGate  *accounts.Gate
	AccountReady func(context.Context) error
	// AccountID is resolved once while AccountGate protects the request.
	AccountID func(context.Context) (string, error)
	Runner    *pipeline.Runner
	// Store is optional; without it no traces are written.
	Store Store
	// UpstreamURL is the CLIProxyAPI base URL (loopback only).
	UpstreamURL string
	// UpstreamKey is the tollgate -> CLIProxyAPI key. It is never logged.
	UpstreamKey  string
	MaxBodyBytes int64
	Logger       *slog.Logger
	// Transport overrides the upstream transport (tests).
	Transport http.RoundTripper
	// Admin serves "/api" and below (optional). It brings its own admin-token
	// check; client keys are never accepted there (ADR-0009).
	Admin http.Handler
	// TraceObserver is optional and called synchronously with a trace right
	// after it was recorded (Status 0) and again after it was finished (Status,
	// Usage, ErrorKind set). It must not block: the proxy waits for it.
	TraceObserver func(store.Trace)
	// ResponseObserver is optional and called with the headers of every
	// upstream response before its body is copied (e.g. to read the
	// subscription rate limits). It must not block or keep the header.
	ResponseObserver        func(http.Header)
	AccountResponseObserver func(string, http.Header)
	// Frontend serves GET/HEAD of "/", "/index.html", "/favicon.svg" and "/assets/" (optional).
	Frontend http.Handler
}

// Server forwards requests to CLIProxyAPI after running the pipeline.
type Server struct {
	accountGate   *accounts.Gate
	accountReady  func(context.Context) error
	accountID     func(context.Context) (string, error)
	admin         http.Handler
	frontend      http.Handler
	observe       func(store.Trace)
	onResp        func(http.Header)
	onAccountResp func(string, http.Header)
	runner        *pipeline.Runner
	store         Store
	base          string
	key           string
	maxBody       int64
	log           *slog.Logger
	client        *http.Client
}

// New validates the options and returns a Server.
func New(o Options) (*Server, error) {
	if o.Runner == nil {
		return nil, errors.New("server: runner is required")
	}
	if o.UpstreamKey == "" {
		return nil, errors.New("server: upstream key is required")
	}
	if _, err := upstream.New(o.UpstreamURL); err != nil { // loopback-only check
		return nil, err
	}
	tr := o.Transport
	if tr == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		// We request identity encoding ourselves; never let Go add gzip and
		// transparently decode, the stream must pass through byte for byte.
		t.DisableCompression = true
		t.ResponseHeaderTimeout = responseHeaderTimeout
		tr = t
	}
	s := &Server{accountGate: o.AccountGate, accountReady: o.AccountReady, accountID: o.AccountID,
		admin: o.Admin, frontend: o.Frontend, observe: o.TraceObserver, onResp: o.ResponseObserver, onAccountResp: o.AccountResponseObserver,
		runner: o.Runner, store: o.Store,
		base: "http://" + strings.TrimPrefix(strings.TrimRight(o.UpstreamURL, "/"), "http://"),
		key:  o.UpstreamKey, maxBody: o.MaxBodyBytes, log: o.Logger,
		client: &http.Client{
			Transport: tr,
			// A redirect would leave localhost; hand it to the client as is.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	if s.maxBody <= 0 {
		s.maxBody = DefaultMaxBodyBytes
	}
	if s.log == nil {
		s.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return s, nil
}

// Handler returns the root handler: admin API, frontend and proxy.
func (s *Server) Handler() http.Handler { return s }

// Listen checks the binding rules (config.CheckBinding) and only then opens
// the listener. published is the host address Docker publishes addr on, or "".
func Listen(addr, published string, devAllowAny bool) (net.Listener, error) {
	if err := config.CheckBinding(addr, published, devAllowAny); err != nil {
		return nil, err
	}
	return net.Listen("tcp", addr)
}

// Serve serves on ln until ctx is done, then shuts down gracefully (running
// streams get a short grace period).
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	hs := &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// No WriteTimeout: SSE responses run for minutes.
		ErrorLog: slog.NewLogLogger(s.log.Handler(), slog.LevelWarn),
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
			return
		}
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if hs.Shutdown(sctx) != nil {
			_ = hs.Close()
		}
	}()
	err := hs.Serve(ln)
	close(done)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// ServeHTTP routes by exact path rules instead of an http.ServeMux, which would
// redirect unclean paths and so change what reaches the upstream. Only the
// admin API prefix and the frontend files are local; every other request,
// including unknown endpoints, goes through the proxy.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	switch {
	case s.admin != nil && (p == "/api" || strings.HasPrefix(p, "/api/")):
		s.admin.ServeHTTP(w, r)
	case s.frontend != nil && (r.Method == http.MethodGet || r.Method == http.MethodHead) &&
		(p == "/" || p == "/index.html" || p == "/favicon.svg" || strings.HasPrefix(p, "/assets/")):
		s.frontend.ServeHTTP(w, r)
	default:
		s.serveProxy(w, r)
	}
}

func (s *Server) serveProxy(w http.ResponseWriter, r *http.Request) {
	// A shallow copy: the server's own request must keep its body type, or it
	// drains up to 256 KiB before an early answer such as 413.
	pr := r.WithContext(r.Context())
	pr.Body = newIdleBody(w, r, bodyReadTimeout, s.log)
	if _, abort := s.proxy(w, pr); abort {
		// Headers are already sent. Returning normally would end the response
		// cleanly and a truncated stream would look complete; abort the
		// connection instead so the client sees the failure.
		panic(http.ErrAbortHandler)
	}
}

// proxy handles one proxied request. It returns the trace ID ("" if none was
// assigned) and whether the response stream was aborted.
func (s *Server) proxy(w http.ResponseWriter, r *http.Request) (traceID string, abort bool) {
	start := time.Now()
	// The client key is checked from the headers before the body is read, so
	// an unauthenticated client cannot make tollgate buffer up to maxBody
	// (#52). Such a request leaves no trace, as before.
	if err := s.runner.Authenticate(r.Context(), r.Header); err != nil {
		writeStageError(w, err)
		return "", false
	}
	if s.accountReady != nil {
		if err := s.accountReady(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"Account-Verwaltung startet; bitte erneut versuchen"}}`))
			return "", false
		}
	}
	if s.accountGate != nil {
		ctx, done, err := s.accountGate.WaitBegin(r.Context())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"Wartende Anfrage wurde abgebrochen"}}`))
			return "", false
		}
		defer done()
		// A forced account switch (#142) cancels this context.
		r = r.WithContext(ctx)
	}
	accountID := ""
	if s.accountID != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		var err error
		accountID, err = s.accountID(ctx)
		cancel()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"Aktiver Account nicht verfügbar; bitte erneut versuchen"}}`))
			return "", false
		}
	}
	if r.ContentLength > s.maxBody {
		writeStageError(w, tooLargeError())
		return "", false
	}
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(bodyReadTimeout))
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.maxBody))
	_ = rc.SetReadDeadline(time.Time{})
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeStageError(w, tooLargeError())
		} else {
			writeStageError(w, &pipeline.StageError{Stage: "server", Status: http.StatusBadRequest,
				Type: pipeline.ErrTypeInvalidRequest, Message: "could not read request body"})
		}
		return "", false
	}

	req, tr, perr := s.runner.Run(r.Context(), pipeline.Input{
		Method: r.Method, Path: r.URL.Path, Header: r.Header, Body: body,
	})
	traced := s.recordTrace(r.Context(), req, tr, accountID)
	if perr != nil {
		var se *pipeline.StageError
		kind := "pipeline"
		if errors.As(perr, &se) && se.Stage != "" {
			kind = "stage_" + se.Stage
		}
		status, _ := pipeline.ErrorResponse(perr)
		writeStageError(w, perr)
		s.finishTrace(traced, req, start, forwardResult{status: status, kind: kind})
		s.log.Warn("request aborted by pipeline", "trace_id", req.TraceID, "status", status, "error_kind", kind)
		return req.TraceID, false
	}

	s.log.Debug("forwarding request", "trace_id", req.TraceID, "endpoint", string(req.Endpoint),
		"model", logModel(req.Model), "estimated_input_tokens", estimate(tr))
	res := s.forward(w, r, req, accountID, start)
	s.logUsage(req, res.status, res.kind, res.usage)
	s.finishTrace(traced, req, start, res)
	return req.TraceID, res.kind == "stream_aborted"
}

// forwardResult is what forward reports about one request: the status sent
// to the client, an error kind ("" on success), the upstream usage of
// Messages requests (nil if none was reported) and the measurements of #87.
type forwardResult struct {
	status       int
	kind         string
	usage        *store.Usage
	ttft         *int64 // µs since start; nil when unknown
	genStart     *int64 // µs until message_start was flushed; streams only
	returnMicros *int64 // nil when no return path ran
	replacements *int64
	fiveHour     *int64 // 5-hour utilization in permille (#134)
	fiveHourAt   *int64 // 5-hour reset, unix seconds
	sevenDay     *int64 // 7-day utilization in permille (#136)
	sevenDayAt   *int64 // 7-day reset, unix seconds
}

// forward is stage 4.
func (s *Server) forward(w http.ResponseWriter, r *http.Request, req *pipeline.Request, accountID string, start time.Time) forwardResult {
	target := s.base + r.URL.EscapedPath()
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	var rd io.Reader = http.NoBody
	if len(req.Body) > 0 {
		rd = bytes.NewReader(req.Body)
	}
	out, err := http.NewRequestWithContext(r.Context(), r.Method, target, rd)
	if err != nil {
		writeStageError(w, upstreamError())
		return forwardResult{status: http.StatusBadGateway, kind: "upstream_request"}
	}
	out.Header = outgoingHeader(req.Header, s.key)

	resp, err := s.client.Do(out)
	if err != nil {
		if forcedSwitch(r) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"Anfrage für Account-Wechsel abgebrochen; bitte erneut versuchen"}}`))
			return forwardResult{status: http.StatusServiceUnavailable, kind: "account_switch"}
		}
		if r.Context().Err() != nil {
			return forwardResult{status: 499, kind: "client_canceled"} // client went away; nobody to answer
		}
		s.log.Warn("upstream request failed", "trace_id", req.TraceID, "error_type", errType(err))
		writeStageError(w, upstreamError())
		return forwardResult{status: http.StatusBadGateway, kind: "upstream_unreachable"}
	}
	defer resp.Body.Close()
	metered := req.Endpoint == pipeline.EndpointMessages
	var res forwardResult
	if metered && resp.StatusCode < 400 {
		// JSON answers: the headers come with the finished answer (#87).
		v := time.Since(start).Microseconds()
		res.ttft = &v
	}

	if resp.StatusCode == http.StatusUnauthorized {
		// tollgate already authenticated the client, so an upstream 401 means
		// the tollgate -> CLIProxyAPI key or the account token is wrong. Passing
		// it through would make the client blame its own key.
		s.log.Warn("upstream rejected tollgate credentials", "trace_id", req.TraceID, "upstream_status", resp.StatusCode)
		writeStageError(w, &pipeline.StageError{Stage: "forward", Status: http.StatusBadGateway, Type: pipeline.ErrTypeAPI,
			Message: "upstream authentication failed"})
		return forwardResult{status: http.StatusBadGateway, kind: "upstream_auth"}
	}
	if metered && resp.StatusCode == http.StatusForbidden && s.handleAccountDenied(w, resp, req, accountID) {
		return forwardResult{status: http.StatusServiceUnavailable, kind: "account_not_allowed"}
	}

	// Stage 5 (return path, #14) for non-streaming answers: the body is
	// rewritten before the headers go out because Content-Length changes.
	if len(req.Placeholders) > 0 && strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		raw, err := io.ReadAll(io.LimitReader(resp.Body, maxUsageBody+1))
		if err != nil {
			s.log.Warn("upstream body read failed", "trace_id", req.TraceID, "error_type", errType(err))
			writeStageError(w, &pipeline.StageError{Stage: "forward", Status: http.StatusBadGateway, Type: pipeline.ErrTypeAPI,
				Message: "upstream response could not be read"})
			return forwardResult{status: http.StatusBadGateway, kind: "upstream_read"}
		}
		if len(raw) > maxUsageBody {
			// Too large to rewrite in memory: pass through with placeholders.
			s.log.Warn("response too large to rehydrate", "trace_id", req.TraceID)
			resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), resp.Body))
		} else {
			t0 := time.Now()
			body, n := redact.RehydrateBodyCount(raw, req.Placeholders)
			us, rn := time.Since(t0).Microseconds(), int64(n)
			res.returnMicros, res.replacements = &us, &rn
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		}
	}
	if metered {
		res.fiveHour, res.fiveHourAt, res.sevenDay, res.sevenDayAt = windowMarks(resp.Header)
	}
	if s.onAccountResp != nil {
		s.onAccountResp(accountID, resp.Header)
	}
	if s.onResp != nil {
		s.onResp(resp.Header)
	}
	copyResponseHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	rc := http.NewResponseController(w)
	flush := func() { _ = rc.Flush() }

	// Stage 3 (meter, after the response): usage is read from the bytes as
	// they pass; the response itself is never changed.
	var copyErr error
	var usage func() *store.Usage
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	switch {
	case strings.HasPrefix(ct, "text/event-stream"):
		var observe func([]byte)
		streamFlush := flush
		if metered {
			ms := &meter.Stream{}
			observe, usage = ms.Observe, ms.Usage
			// TTFT ends when the first content_block_delta is written and
			// flushed (#87). Events are flushed one by one, so the flush after
			// the marked event is the one that delivers it. The generation
			// start, the base of tokens/s (#116), is the same for
			// message_start: hidden thinking runs before the first delta.
			res.ttft = nil
			failed := resp.StatusCode >= 400
			marked, done := false, failed
			startMarked, startDone := false, failed
			observe = func(ev []byte) {
				if !startDone && !startMarked && bytes.Contains(ev, []byte(`"message_start"`)) {
					startMarked = true
				}
				if !done && !marked && bytes.Contains(ev, []byte(`"content_block_delta"`)) {
					marked = true
				}
				ms.Observe(ev)
			}
			streamFlush = func() {
				flush()
				if startMarked && !startDone {
					startDone = true
					v := time.Since(start).Microseconds()
					res.genStart = &v
				}
				if marked && !done {
					done = true
					v := time.Since(start).Microseconds()
					res.ttft = &v
				}
			}
		}
		// Stage 5 (return path, #14): placeholders are translated back; the
		// meter still observes the upstream events.
		var st redact.ReturnStats
		_, copyErr = redact.CopyRehydratedStats(w, streamFlush, resp.Body, req.Placeholders, observe, &st)
		if len(req.Placeholders) > 0 {
			us, rn := st.Duration.Microseconds(), int64(st.Replacements)
			res.returnMicros, res.replacements = &us, &rn
		}
	case metered && strings.HasPrefix(ct, "application/json"):
		buf := &capBuffer{max: maxUsageBody}
		_, copyErr = io.Copy(flushWriter{io.MultiWriter(w, buf), flush}, resp.Body)
		usage = func() *store.Usage {
			if buf.over {
				return nil
			}
			return meter.FromJSON(buf.b)
		}
	default:
		_, copyErr = io.Copy(flushWriter{w, flush}, resp.Body)
	}
	if usage != nil {
		res.usage = usage()
	}
	res.status = resp.StatusCode
	if copyErr != nil {
		s.log.Warn("response stream aborted", "trace_id", req.TraceID, "error_type", errType(copyErr))
		res.kind, res.ttft, res.genStart = "stream_aborted", nil, nil
		return res
	}
	flush()
	if resp.StatusCode >= 400 {
		res.ttft, res.genStart = nil, nil
	}
	return res
}

// capBuffer keeps up to max bytes and then stops keeping (over = true). It
// never fails, so it cannot break the copy to the client.
type capBuffer struct {
	b    []byte
	max  int
	over bool
}

func (c *capBuffer) Write(p []byte) (int, error) {
	if !c.over {
		if len(c.b)+len(p) > c.max {
			c.over, c.b = true, nil
		} else {
			c.b = append(c.b, p...)
		}
	}
	return len(p), nil
}

// estimate is the token estimate of the final body (last stage).
func estimate(tr *pipeline.Trace) int {
	if tr == nil || len(tr.Stages) == 0 {
		return 0
	}
	return tr.Stages[len(tr.Stages)-1].TokensAfter
}

// logModel bounds the client-supplied model name before it is logged.
func logModel(m string) string {
	const max = 64
	if len(m) > max {
		return m[:max]
	}
	return m
}

// logUsage writes one line per metered request with the reported usage and
// the list-price cost. It carries counts only, never content.
func (s *Server) logUsage(req *pipeline.Request, status int, kind string, u *store.Usage) {
	if u == nil {
		return
	}
	attrs := []any{"trace_id", req.TraceID, "model", logModel(req.Model), "status", status,
		"input_tokens", u.Input, "output_tokens", u.Output,
		"cache_read_tokens", u.CacheRead, "cache_creation_tokens", u.CacheCreation}
	if kind != "" {
		attrs = append(attrs, "error_kind", kind)
	}
	if c, ok := meter.Cost(req.Model, *u); ok {
		attrs = append(attrs, "cost_nano_usd", c)
	}
	s.log.Info("request metered", attrs...)
}

type flushWriter struct {
	w     io.Writer
	flush func()
}

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	f.flush()
	return n, err
}

// forcedSwitch reports whether a forced account switch aborted r (#142).
func forcedSwitch(r *http.Request) bool {
	return errors.Is(context.Cause(r.Context()), accounts.ErrForcedSwitch)
}

func errType(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "unexpected_eof"
	}
	return "io"
}

func tooLargeError() *pipeline.StageError {
	return &pipeline.StageError{Stage: "server", Status: http.StatusRequestEntityTooLarge,
		Type: pipeline.ErrTypeInvalidRequest, Message: "request body too large"}
}

func upstreamError() *pipeline.StageError {
	return &pipeline.StageError{Stage: "forward", Status: http.StatusBadGateway, Type: pipeline.ErrTypeAPI,
		Message: "upstream unavailable"}
}

func writeStageError(w http.ResponseWriter, err error) {
	status, body := pipeline.ErrorResponse(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// hopByHop are the headers of RFC 9110 section 7.6.1 plus those named in
// Connection. They are never forwarded in either direction.
var hopByHop = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

func isHop(h http.Header, name string) bool {
	for _, k := range hopByHop {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	for _, v := range h.Values("Connection") {
		for tok := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), name) {
				return true
			}
		}
	}
	return false
}

// outgoingHeader applies the header rules of ADR-0001, point 5: everything is
// passed on except hop-by-hop headers, Content-Length (recomputed) and the
// client credentials, which are replaced by the tollgate -> CLIProxyAPI key.
// Accept-Encoding is forced to identity so the stream stays inspectable.
func outgoingHeader(in http.Header, upstreamKey string) http.Header {
	out := make(http.Header, len(in)+2)
	for k, vs := range in {
		switch {
		case isHop(in, k),
			strings.EqualFold(k, "Authorization"), strings.EqualFold(k, "X-Api-Key"),
			strings.EqualFold(k, "Content-Length"), strings.EqualFold(k, "Accept-Encoding"),
			strings.EqualFold(k, "Host"):
			continue
		}
		out[k] = append([]string(nil), vs...)
	}
	if _, ok := out["User-Agent"]; !ok {
		out["User-Agent"] = []string{""} // suppress Go's default agent
	}
	out.Set("Authorization", "Bearer "+upstreamKey)
	out.Set("Accept-Encoding", "identity")
	return out
}

func copyResponseHeader(dst, src http.Header) {
	for k, vs := range src {
		if isHop(src, k) {
			continue
		}
		dst[k] = append([]string(nil), vs...)
	}
}

// recordTrace writes the trace before forwarding so FinishTrace has a row. A
// request without a client (failed authentication) leaves no trace. Store
// errors are logged and never block traffic.
func (s *Server) recordTrace(ctx context.Context, req *pipeline.Request, tr *pipeline.Trace, accountID string) *store.Trace {
	if s.store == nil || req.ClientID == "" {
		return nil
	}
	st := store.Trace{
		ID: req.TraceID, ClientID: req.ClientID, AccountID: accountID, ConfigVersionID: req.ConfigVersion,
		StartedAt: tr.StartedAt, Endpoint: string(req.Endpoint), Model: req.Model, Stream: req.Stream,
	}
	if req.SessionKey != "" {
		st.SessionKeyHash, _ = hex.DecodeString(req.SessionKey)
	}
	for _, sg := range tr.Stages {
		var matches []store.Match
		for _, m := range sg.Matches {
			matches = append(matches, store.Match{Offset: m.Offset, Length: m.Length, Type: m.Type, Rule: m.Rule, Placeholder: m.Placeholder, Mask: m.Mask})
		}
		st.Stages = append(st.Stages, store.Stage{
			Name: sg.Name, DurationMicros: sg.Duration.Microseconds(),
			BytesIn: int64(sg.BytesBefore), BytesOut: int64(sg.BytesAfter),
			TokensIn: int64(sg.TokensBefore), TokensOut: int64(sg.TokensAfter),
			HashIn: sg.InHash, HashOut: sg.OutHash, ConfigVersionID: sg.ConfigVersion,
			Diff: sg.Diff, Matches: matches,
		})
	}
	if n := len(st.Stages); n > 0 {
		st.EstimatedInputTokens = st.Stages[n-1].TokensOut
	}
	if err := s.store.RecordTrace(ctx, st); err != nil {
		s.log.Error("record trace failed", "trace_id", req.TraceID, "error", err.Error())
		return nil
	}
	st = st.Normalized() // what the store holds
	if s.observe != nil {
		s.observe(st)
	}
	return &st
}

func (s *Server) finishTrace(traced *store.Trace, req *pipeline.Request, start time.Time, fr forwardResult) {
	if traced == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), finishTimeout)
	defer cancel()
	res := store.TraceResult{
		Status: fr.status, DurationMicros: time.Since(start).Microseconds(), ErrorKind: fr.kind, Usage: fr.usage,
		TTFTMicros: fr.ttft, GenStartMicros: fr.genStart, ReturnMicros: fr.returnMicros, ReturnReplacements: fr.replacements,
		FiveHourPermille: fr.fiveHour, FiveHourReset: fr.fiveHourAt, SevenDayPermille: fr.sevenDay, SevenDayReset: fr.sevenDayAt,
	}
	for _, p := range []**int64{&res.TTFTMicros, &res.GenStartMicros} {
		if *p != nil && **p > res.DurationMicros {
			v := res.DurationMicros
			*p = &v
		}
	}
	finished := true
	if err := s.store.FinishTrace(ctx, req.TraceID, res); err != nil {
		finished = false
		s.log.Error("finish trace failed", "trace_id", req.TraceID, "error", err.Error())
		// Still tell the observer: otherwise a live view shows the request as
		// running forever.
	}
	done := *traced
	done.Status, done.DurationMicros, done.Usage, done.ErrorKind = res.Status, res.DurationMicros, res.Usage, res.ErrorKind
	done.TTFTMicros, done.GenStartMicros = res.TTFTMicros, res.GenStartMicros
	done.ReturnMicros, done.ReturnReplacements = res.ReturnMicros, res.ReturnReplacements
	if finished && judgeable(done) {
		done.Perf = s.judge(ctx, done)
		done.TPS = s.judgeTPS(ctx, done)
	}
	if s.observe != nil {
		s.observe(done)
	}
}

// judgeable reports whether a finished trace gets verdicts: successful
// messages requests with usage, never the speedtest.
func judgeable(t store.Trace) bool {
	return t.Endpoint == string(pipeline.EndpointMessages) && t.ClientID != store.SpeedtestClientID &&
		t.Status >= 200 && t.Status <= 299 && t.ErrorKind == "" && t.Usage != nil
}

// judgeTPS computes and stores the tokens/s verdict (#116) after the verdict
// of #87. Like judge, it never blocks the proxy.
func (s *Server) judgeTPS(ctx context.Context, t store.Trace) *store.TPS {
	ts, ok := s.store.(TPSStore)
	if !ok || t.GenStartMicros == nil {
		return nil
	}
	v, ok := perf.TPSMilli(t.Usage.Output, t.DurationMicros, *t.GenStartMicros, true, t.Stream)
	if !ok {
		return nil
	}
	samples, err := ts.TPSSamples(ctx, t.Model, t.StartedAt)
	if err != nil {
		s.log.Error("tps samples failed", "trace_id", t.ID, "error", err.Error())
		return nil
	}
	in := t.Usage.Input + t.Usage.CacheRead + t.Usage.CacheCreation
	out := store.TPSFromJudgement(perf.JudgeTPS(samples, in, t.Usage.Output, v))
	if err := ts.PutTPS(ctx, t.ID, out); err != nil {
		s.log.Error("put tps failed", "trace_id", t.ID, "error", err.Error())
		return nil
	}
	return &out
}

// judge computes and stores the verdict of a finished trace (#87). Errors are
// logged and give no verdict; they never block the proxy.
func (s *Server) judge(ctx context.Context, t store.Trace) []store.Perf {
	ps, ok := s.store.(PerfStore)
	if !ok {
		return nil
	}
	in := t.Usage.Input + t.Usage.CacheRead + t.Usage.CacheCreation
	var out []store.Perf
	if t.TTFTMicros != nil {
		samples, err := ps.PerfSamples(ctx, t.Model, t.StartedAt, perf.MetricTTFT)
		if err != nil {
			s.log.Error("perf samples failed", "trace_id", t.ID, "error", err.Error())
			return nil
		}
		out = append(out, store.PerfFromJudgement(perf.JudgeTTFT(samples, in, *t.TTFTMicros)))
	}
	samples, err := ps.PerfSamples(ctx, t.Model, t.StartedAt, perf.MetricDuration)
	if err != nil {
		s.log.Error("perf samples failed", "trace_id", t.ID, "error", err.Error())
		return nil
	}
	out = append(out, store.PerfFromJudgement(perf.JudgeDuration(samples, in, t.Usage.Output, t.DurationMicros)))
	if err := ps.PutPerf(ctx, t.ID, out); err != nil {
		s.log.Error("put perf failed", "trace_id", t.ID, "error", err.Error())
		return nil
	}
	// The store orders verdicts by metric; match it so events and reads agree.
	slices.SortFunc(out, func(a, b store.Perf) int { return strings.Compare(a.Metric, b.Metric) })
	return out
}
