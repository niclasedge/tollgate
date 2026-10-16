package dashboard

import (
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/store"
)

// minWeekUse is the smallest weekly use (fraction) an account needs to count:
// below it the whole-percent rounding of the limit dominates.
const minWeekUse = 0.25

// capacityLookback is how far back the rows behind the capacity estimate go:
// one 7-day window plus a day of slack.
const capacityLookback = 8 * 24 * time.Hour

// weekMark is the weekly use of an account as last observed.
type weekMark struct {
	account                string
	used                   float64 // fraction of the weekly limit
	start, observed, reset time.Time
}

// weekMarks reads the weekly use from the account snapshots, ordered by
// account.
func weekMarks(usage map[string]accounts.UsageSnapshot) []weekMark {
	var out []weekMark
	for id, u := range usage {
		if u.SevenDay.Utilization == nil {
			continue
		}
		w := u.SevenDay
		reset, err := time.Parse(time.RFC3339, w.Reset)
		if err != nil {
			continue
		}
		at := w.ObservedAt
		if at == "" {
			at = u.ObservedAt
		}
		observed, err := time.Parse(time.RFC3339, at)
		if err != nil {
			continue
		}
		out = append(out, weekMark{account: id, used: *w.Utilization / 100, start: reset.Add(-7 * 24 * time.Hour), observed: observed, reset: reset})
	}
	slices.SortFunc(out, func(a, b weekMark) int { return strings.Compare(a.account, b.account) })
	return out
}

// cost is the list price of a row in nano-USD; 0 for unknown models.
func cost(r store.FillRow) int64 {
	c, _ := meter.Cost(r.Model, r.Usage)
	return c
}

// capacity estimates how far the subscriptions reach (#136). The price of one
// full 5-hour window comes from the windows that rose by at least 30 points;
// a weekly limit holds what an account spent since its weekly window began,
// in such windows, scaled to its weekly use. fills_per_day counts the rows
// from since on. rows must be ordered by start; marks by account.
func capacity(rows []store.FillRow, since, now time.Time, marks []weekMark, accountCount int) Capacity {
	out := Capacity{Accounts: int64(accountCount)}
	var first, last time.Time
	seen := false
	for _, r := range rows {
		if r.Permille == nil || r.Reset == nil || r.StartedAt.Before(since) {
			continue
		}
		if !seen {
			first, seen = r.StartedAt, true
		}
		last = r.StartedAt
	}
	order, wins := fillWindows(rows)
	var total, spent, rise int64
	for _, k := range order {
		w := wins[k]
		if !w.first.StartedAt.Before(since) {
			total += w.rise()
		}
		if w.rise() < minFillRise {
			continue
		}
		for _, r := range rows {
			if r.AccountID == k.account && r.StartedAt.After(w.first.StartedAt) && !r.StartedAt.After(w.peak.StartedAt) {
				spent += cost(r)
			}
		}
		rise += w.rise()
		out.SampleWindows++
	}
	hours := last.Sub(first).Hours()
	out.MeasuredHours = math.Round(hours*10) / 10
	if hours >= 1 {
		v := math.Round(float64(total)/1000/(hours/24)*100) / 100
		out.FillsPerDay = &v
	}
	if spent <= 0 || len(rows) == 0 {
		return out
	}
	window := spent * 1000 / rise
	out.WindowCostNanoUSD = &window
	var windows, used float64
	for _, m := range marks {
		// The rows must cover the whole weekly window up to the observation.
		if m.used < minWeekUse || rows[0].StartedAt.After(m.start) || !m.observed.After(m.start) {
			continue
		}
		var c int64
		for _, r := range rows {
			if r.AccountID == m.account && !r.StartedAt.Before(m.start) && !r.StartedAt.After(m.observed) {
				c += cost(r)
			}
		}
		windows += float64(c) / float64(window)
		used += m.used
		out.SampleAccounts++
	}
	if used <= 0 {
		return out
	}
	wpw := math.Round(windows/used*10) / 10
	out.WindowsPerWeek = &wpw
	// Room left in the weekly windows still running.
	var free float64
	known := false
	for _, m := range marks {
		if m.reset.After(now) {
			free += max(0, 1-m.used) * wpw
			known = true
		}
	}
	if known {
		v := math.Round(free*10) / 10
		out.FreeThisWeek = &v
	}
	return out
}

// cachedUsage returns the last known usage per account and the number of
// accounts; empty without account management. A failing account is left out.
func (h *Handler) cachedUsage(r *http.Request) (map[string]accounts.UsageSnapshot, int) {
	if h.accounts == nil {
		return nil, 0
	}
	list, err := h.accounts.List(r.Context())
	if err != nil {
		h.log.WarnContext(r.Context(), "capacity: accounts unavailable", "error", err)
		return nil, 0
	}
	out := map[string]accounts.UsageSnapshot{}
	for _, a := range list {
		if u, err := h.accounts.CachedUsage(r.Context(), a.ID); err == nil {
			out[a.ID] = u
		}
	}
	return out, len(list)
}
