package dashboard

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/niclasedge/tollgate/internal/store"
)

// minFillRise is the smallest rise (permille) that makes a window count: a
// short rise extrapolates noise to an absurd time to 100 %.
const minFillRise = 300

// FillStore is the optional part of the store behind fastest_fill (#134).
type FillStore interface {
	FillRows(ctx context.Context, since time.Time) ([]store.FillRow, error)
}

type windowKey struct {
	account string
	reset   int64
}

// fillWindow is one 5-hour window of an account: its first row with 5-hour
// values and the first row at the highest value.
type fillWindow struct{ first, peak store.FillRow }

// rise is the observed 5-hour rise in permille.
func (w *fillWindow) rise() int64 { return *w.peak.Permille - *w.first.Permille }

// fillWindows groups the rows with 5-hour values by account and reset, in
// the order of their first row.
func fillWindows(rows []store.FillRow) ([]windowKey, map[windowKey]*fillWindow) {
	var order []windowKey
	wins := map[windowKey]*fillWindow{}
	for _, r := range rows {
		if r.Permille == nil || r.Reset == nil {
			continue
		}
		k := windowKey{r.AccountID, *r.Reset}
		w, ok := wins[k]
		if !ok {
			wins[k] = &fillWindow{first: r, peak: r}
			order = append(order, k)
			continue
		}
		if *r.Permille > *w.peak.Permille {
			w.peak = r
		}
	}
	return order, wins
}

// fastestFill returns the 5-hour window (account + reset) with the shortest
// extrapolated time to 100 %, or nil. rows must be ordered by start; the
// result depends on the rows only.
func fastestFill(rows []store.FillRow) *WindowFill {
	order, wins := fillWindows(rows)
	var best *WindowFill
	var bestReset int64
	for _, k := range order {
		w := wins[k]
		rise := w.rise()
		observed := w.peak.StartedAt.Sub(w.first.StartedAt).Microseconds()
		if rise < minFillRise || observed <= 0 {
			continue
		}
		full := observed * 1000 / rise
		if best != nil && (full > best.FullUs || full == best.FullUs &&
			(k.reset > bestReset || k.reset == bestReset && strings.Compare(k.account, best.AccountID) >= 0)) {
			continue
		}
		var n int64
		for _, r := range rows {
			if r.AccountID == k.account && !r.StartedAt.Before(w.first.StartedAt) && !r.StartedAt.After(w.peak.StartedAt) {
				n++
			}
		}
		best, bestReset = &WindowFill{
			AccountID: k.account, Reset: time.Unix(k.reset, 0).UTC().Format(time.RFC3339),
			FromPermille: *w.first.Permille, ToPermille: *w.peak.Permille,
			ObservedUs: observed, FullUs: full, Requests: n,
			PerMinute: math.Round(float64(n)*600e6/float64(observed)) / 10,
		}, k.reset
	}
	return best
}
