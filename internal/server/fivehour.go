package server

import (
	"math"
	"net/http"
	"time"

	"github.com/niclasedge/tollgate/internal/meter"
)

// windowMarks reads the window state of a response for the trace: the
// 5-hour (#134) and 7-day (#136) utilization in permille and their resets as
// unix seconds. A window stays nil unless the response carries both values.
func windowMarks(h http.Header) (fiveP, fiveR, sevenP, sevenR *int64) {
	l, ok := meter.ParseLimits(h, time.Time{})
	if !ok {
		return nil, nil, nil, nil
	}
	fiveP, fiveR = mark(l.FiveHour)
	sevenP, sevenR = mark(l.SevenDay)
	return fiveP, fiveR, sevenP, sevenR
}

func mark(w meter.Window) (permille, reset *int64) {
	if w.Utilization == nil || w.Reset == nil {
		return nil, nil
	}
	p, r := int64(math.Round(*w.Utilization*1000)), w.Reset.Unix()
	return &p, &r
}
