package dashboard

import (
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/store"
)

func TestFastestFill(t *testing.T) {
	base := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	p := func(v int64) *int64 { return &v }
	at := func(min float64) time.Time { return base.Add(time.Duration(min * float64(time.Minute))) }
	row := func(acc string, min float64, permille, reset int64) store.FillRow {
		return store.FillRow{AccountID: acc, StartedAt: at(min), Permille: p(permille), Reset: p(reset)}
	}
	// full: 0 -> 1000 permille in 100 min, 250 requests (first and peak included).
	var full []store.FillRow
	for i := range 250 {
		full = append(full, row("a", float64(i)*100/249, int64(i)*1000/249, 1000))
	}

	cases := map[string]struct {
		rows []store.FillRow
		want *WindowFill
	}{
		"full window": {rows: full, want: &WindowFill{AccountID: "a", Reset: "1970-01-01T00:16:40Z",
			FromPermille: 0, ToPermille: 1000, ObservedUs: 100 * 60e6, FullUs: 100 * 60e6, Requests: 250, PerMinute: 2.5}},
		"partial window extrapolates": {rows: []store.FillRow{
			row("a", 0, 200, 1000),
			{AccountID: "a", StartedAt: at(30)}, // request without headers still counts
			row("a", 60, 700, 1000),
			row("a", 90, 650, 1000), // below the peak: ignored
		}, want: &WindowFill{AccountID: "a", Reset: "1970-01-01T00:16:40Z",
			FromPermille: 200, ToPermille: 700, ObservedUs: 60 * 60e6, FullUs: 120 * 60e6, Requests: 3, PerMinute: 0.1}},
		"small rise": {rows: []store.FillRow{row("a", 0, 100, 1000), row("a", 60, 399, 1000)}},
		"fastest wins, other account not counted": {rows: []store.FillRow{
			row("a", 0, 0, 1000), row("b", 1, 0, 2000), row("a", 2, 100, 1000),
			row("b", 31, 500, 2000),  // b: 500 in 30 min -> 60 min
			row("a", 100, 500, 1000), // a: 500 in 100 min -> 200 min
		}, want: &WindowFill{AccountID: "b", Reset: "1970-01-01T00:33:20Z",
			FromPermille: 0, ToPermille: 500, ObservedUs: 30 * 60e6, FullUs: 60 * 60e6, Requests: 2, PerMinute: 0.1}},
		"tie goes to the older window": {rows: []store.FillRow{
			row("b", 0, 0, 2000), row("a", 0, 0, 3000), row("b", 10, 500, 2000), row("a", 10, 500, 3000),
		}, want: &WindowFill{AccountID: "b", Reset: "1970-01-01T00:33:20Z",
			FromPermille: 0, ToPermille: 500, ObservedUs: 10 * 60e6, FullUs: 20 * 60e6, Requests: 2, PerMinute: 0.2}},
		"no rows": {},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := fastestFill(c.rows)
			if (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
				t.Fatalf("got %+v want %+v", got, c.want)
			}
		})
	}
}
