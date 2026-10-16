package dashboard

import (
	"math"
	"net/http"
	"time"
)

const (
	// callWindow is the length of one window of GET /api/calls (#132).
	callWindow = 5 * time.Minute
	// callWindows is how many windows before the current one form the average.
	callWindows = 10
)

// calls serves GET /api/calls: the requests of the last five minutes against
// the average of the ten five-minute windows before. Windows roll from now;
// empty windows count as zero, so the average is the sum over all ten.
func (h *Handler) calls(w http.ResponseWriter, r *http.Request) {
	now := h.now()
	start := now.Add(-callWindow)
	current, err := h.st.CountTraces(r.Context(), start, now)
	if err != nil {
		h.fail(w, r, "count calls", err)
		return
	}
	previous, err := h.st.CountTraces(r.Context(), start.Add(-callWindows*callWindow), start)
	if err != nil {
		h.fail(w, r, "count calls", err)
		return
	}
	out := CallRate{
		WindowSeconds: int64(callWindow / time.Second), Windows: callWindows,
		Current: current, PreviousTotal: previous,
		Average: math.Round(float64(previous)*10/callWindows) / 10,
		Trend:   "equal",
	}
	// Compare in integers so the verdict does not depend on rounding.
	switch c := current * callWindows; {
	case c > previous:
		out.Trend = "above"
	case c < previous:
		out.Trend = "below"
	}
	writeJSON(w, http.StatusOK, out)
}
