package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/store"
	"github.com/niclasedge/tollgate/internal/upstream"
)

const (
	defaultTraceLimit = 50
	maxTraceLimit     = 200
	defaultCostDays   = 30
	maxCostDays       = 366
	// maxHourlyCostDays is the longest stretch with hourly buckets: the last
	// days of a longer period. Eight days always cover a running weekly
	// window, whose start lies at most seven days back (#125).
	maxHourlyCostDays = 8
	// DefaultHeartbeat is the interval of the SSE keep-alive comment.
	DefaultHeartbeat = 15 * time.Second
)

// Store is the part of the SQLite store the dashboard reads.
type Store interface {
	ClientLister
	DetailStore
	ListTracesWithClient(ctx context.Context, f store.ListTracesFilter) ([]store.TraceListItem, error)
	CostByDay(ctx context.Context, since time.Time) ([]store.CostAggregate, error)
	CostByHour(ctx context.Context, since time.Time) ([]store.HourAggregate, error)
	CountTraces(ctx context.Context, from, to time.Time) (int64, error)
}

// Options configures Handler.
type Options struct {
	Accounts *accounts.Manager
	OAuth    *accounts.OAuth
	Store    Store
	Broker   *Broker
	// UpstreamURL is the CLIProxyAPI base URL (loopback only).
	UpstreamURL string
	// ManagementKey is the CLIProxyAPI management key; empty = not configured.
	// It is never logged or returned.
	ManagementKey string
	// Version is reported as the tollgate version; default: build info.
	Version string
	// Now and Heartbeat are test hooks (defaults: time.Now, DefaultHeartbeat).
	Now       func() time.Time
	Heartbeat time.Duration
	Logger    *slog.Logger
	// Done, when closed, ends all SSE streams (server shutdown); optional.
	Done <-chan struct{}
	// Limits returns the latest subscription rate limits (optional).
	Limits func() (meter.Limits, bool)
	// Probe runs one speedtest request (#87); nil disables the speedtest.
	Probe Prober
	// RedactEnabled tells the speedtest whether the redact stage is on.
	RedactEnabled bool
}

// Handler serves the dashboard routes.
type Handler struct {
	accounts  *accounts.Manager
	oauth     *accounts.OAuth
	st        Store
	broker    *Broker
	up        *upstream.Client
	mgmtKey   string
	version   string
	now       func() time.Time
	heartbeat time.Duration
	log       *slog.Logger
	done      <-chan struct{}
	limits    func() (meter.Limits, bool)
	probe     Prober
	// redactEnabled is whether the redact stage runs (speedtest variant).
	redactEnabled bool
	speedLock     speedtestLock
}

// New validates the options and returns a Handler.
func New(o Options) (*Handler, error) {
	if o.Store == nil || o.Broker == nil {
		return nil, errors.New("dashboard: store and broker are required")
	}
	up, err := upstream.New(o.UpstreamURL)
	if err != nil {
		return nil, err
	}
	h := &Handler{accounts: o.Accounts, oauth: o.OAuth, st: o.Store, broker: o.Broker, up: up, mgmtKey: o.ManagementKey, version: o.Version,
		now: o.Now, heartbeat: o.Heartbeat, log: o.Logger, done: o.Done, limits: o.Limits,
		probe: o.Probe, redactEnabled: o.RedactEnabled}
	if h.oauth == nil {
		h.oauth = accounts.NewOAuth()
	}
	if h.version == "" {
		h.version = buildVersion()
	}
	if h.now == nil {
		h.now = time.Now
	}
	if h.heartbeat <= 0 {
		h.heartbeat = DefaultHeartbeat
	}
	if h.log == nil {
		h.log = slog.New(slog.DiscardHandler)
	}
	return h, nil
}

// Register adds the dashboard routes to mux. The caller must put
// auth.RequireAdmin in front of mux; the routes check no token themselves.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/traces", h.traces)
	mux.HandleFunc("GET /api/traces/events", h.events)
	mux.HandleFunc("GET /api/traces/{id}", h.traceDetail)
	mux.HandleFunc("GET /api/redactions", h.redactions)
	mux.HandleFunc("GET /api/costs", h.costs)
	mux.HandleFunc("GET /api/status", h.status)
	mux.HandleFunc("GET /api/calls", h.calls)
	h.perfRoutes(mux)
	h.accountRoutes(mux)
}

// buildVersion is the module version from the build info, or "dev".
func buildVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, errorBody{Error: msg})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.log.ErrorContext(r.Context(), "dashboard api failure", "op", op, "err", err)
	writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal error"})
}

// intParam parses an optional integer query parameter within [lo, hi].
func intParam(r *http.Request, name string, def, lo, hi int) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("%s must be an integer from %d to %d", name, lo, hi)
	}
	return n, nil
}

func (h *Handler) traces(w http.ResponseWriter, r *http.Request) {
	limit, err := intParam(r, "limit", defaultTraceLimit, 1, maxTraceLimit)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	f := store.ListTracesFilter{Limit: limit + 1} // one extra row tells whether another page follows
	q := r.URL.Query()
	if v := q.Get("before"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			badRequest(w, "before must be an RFC 3339 timestamp")
			return
		}
		f.Before, f.BeforeID = t, q.Get("before_id")
	} else if q.Get("before_id") != "" {
		badRequest(w, "before_id needs before")
		return
	}
	items, err := h.st.ListTracesWithClient(r.Context(), f)
	if err != nil {
		h.fail(w, r, "list traces", err)
		return
	}
	out := TraceList{Traces: make([]TraceSummary, 0, min(len(items), limit))}
	if len(items) > limit {
		items = items[:limit]
		last := items[limit-1]
		out.NextBefore, out.NextBeforeID = formatTime(last.StartedAt), last.ID
	}
	for _, it := range items {
		sm := summary(it.Trace, it.ClientName, it.SavedTokens)
		setPerf(&sm, it.Trace, it.StageMicros)
		out.Traces = append(out.Traces, sm)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) costs(w http.ResponseWriter, r *http.Request) {
	days, err := intParam(r, "days", defaultCostDays, 1, maxCostDays)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	today := h.now().UTC().Truncate(24 * time.Hour)
	from := today.AddDate(0, 0, -(days - 1))
	aggs, err := h.st.CostByDay(r.Context(), from)
	if err != nil {
		h.fail(w, r, "cost report", err)
		return
	}
	out := CostReport{From: from.Format(time.DateOnly), To: today.Format(time.DateOnly), Rows: make([]CostRow, 0, len(aggs)), Hours: []CostHour{}}
	for _, a := range aggs {
		row := CostRow{
			Day: a.Day, ClientID: a.ClientID, AccountID: a.AccountID, ClientName: a.ClientName, Model: a.Model, Requests: a.Requests,
			Usage: toUsage(a.Usage), EstimatedInputTokens: a.Estimated, SavedTokens: a.SavedTokens,
		}
		// Prices are linear, so the cost of the summed usage equals the sum of
		// the per-request costs.
		if c, ok := meter.Cost(a.Model, a.Usage); ok {
			row.CostNanoUSD = &c
		}
		if c, ok := meter.CacheSavings(a.Model, a.Usage); ok {
			row.CacheSavingsNanoUSD = &c
		}
		out.Rows = append(out.Rows, row)
	}
	hours, err := h.st.CostByHour(r.Context(), today.AddDate(0, 0, -(min(days, maxHourlyCostDays)-1)))
	if err != nil {
		h.fail(w, r, "cost report hours", err)
		return
	}
	for _, a := range hours {
		hr := CostHour{Hour: a.Hour, AccountID: a.AccountID, Model: a.Model, Requests: a.Requests, Usage: toUsage(a.Usage)}
		if c, ok := meter.Cost(a.Model, a.Usage); ok {
			hr.CostNanoUSD = &c
		}
		out.Hours = append(out.Hours, hr)
	}
	writeJSON(w, http.StatusOK, out)
}
