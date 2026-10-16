package meter

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Header names of the unified subscription rate limits. Anthropic sends them
// with every response to an OAuth (subscription) request; CLIProxyAPI reads
// the same headers (internal/runtime/executor/helps/claude_ratelimit.go) and
// passes them through.
const (
	hdrUnified       = "Anthropic-Ratelimit-Unified-"
	maxLimitTextLen  = 64
	maxUtilization   = 10.0 // above 1 means over the limit; caps garbage values
	minResetUnixSecs = 1    // 0 and negative values are not a reset time
)

// Window is one rate-limit window (5 hours or 7 days). Nil fields are
// unknown: the header was absent or not parseable.
type Window struct {
	Utilization *float64 // share of the window used, 0..1 (above 1: over the limit)
	Reset       *time.Time
	Status      string // allowed, allowed_warning, rejected or "" if absent
}

// Limits is the subscription rate-limit state of one response.
type Limits struct {
	Status              string
	Reset               *time.Time
	RepresentativeClaim string
	FiveHour            Window
	SevenDay            Window
	ObservedAt          time.Time
}

// ParseLimits reads the unified rate-limit headers. ok is false if none of
// them is present. Values that do not parse stay unknown instead of 0.
func ParseLimits(h http.Header, now time.Time) (Limits, bool) {
	found := false
	get := func(name string) string {
		v := strings.TrimSpace(h.Get(hdrUnified + name))
		if v != "" {
			found = true
		}
		return v
	}
	text := func(name string) string {
		v := strings.ToLower(get(name))
		if len(v) > maxLimitTextLen {
			v = v[:maxLimitTextLen]
		}
		return strings.ToValidUTF8(v, "")
	}
	l := Limits{
		Status:              text("Status"),
		Reset:               parseReset(get("Reset")),
		RepresentativeClaim: text("Representative-Claim"),
		FiveHour: Window{
			Utilization: parseUtilization(get("5h-Utilization")),
			Reset:       parseReset(get("5h-Reset")),
			Status:      text("5h-Status"),
		},
		SevenDay: Window{
			Utilization: parseUtilization(get("7d-Utilization")),
			Reset:       parseReset(get("7d-Reset")),
			Status:      text("7d-Status"),
		},
		ObservedAt: now.UTC(),
	}
	return l, found
}

func parseUtilization(raw string) *float64 {
	if raw == "" {
		return nil
	}
	u, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(u) || math.IsInf(u, 0) || u < 0 || u > maxUtilization {
		return nil
	}
	return &u
}

// parseReset accepts Unix seconds (as Anthropic sends them) or RFC 3339.
func parseReset(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	if sec, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if sec < minResetUnixSecs || sec > 1<<40 {
			return nil
		}
		t := time.Unix(sec, 0).UTC()
		return &t
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		t = t.UTC()
		return &t
	}
	return nil
}

// LimitTracker holds the latest Limits seen. It is safe for concurrent use.
type LimitTracker struct {
	now func() time.Time

	mu     sync.Mutex
	latest Limits
	ok     bool
}

// NewLimitTracker returns a tracker; now defaults to time.Now.
func NewLimitTracker(now func() time.Time) *LimitTracker {
	if now == nil {
		now = time.Now
	}
	return &LimitTracker{now: now}
}

// Observe takes the limits of one upstream response. A response without
// unified headers (e.g. count_tokens) keeps the previous state.
func (t *LimitTracker) Observe(h http.Header) {
	l, ok := ParseLimits(h, t.now())
	if !ok {
		return
	}
	t.mu.Lock()
	t.latest, t.ok = l, true
	t.mu.Unlock()
}

// Latest returns the latest limits; ok is false before the first observation.
// Pointer fields of the result are never mutated by the tracker.
func (t *LimitTracker) Latest() (Limits, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.latest, t.ok
}

// Reset drops observations from the previous account after an explicit switch.
func (t *LimitTracker) Reset() { t.mu.Lock(); defer t.mu.Unlock(); t.latest = Limits{}; t.ok = false }
