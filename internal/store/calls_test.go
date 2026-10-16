package store

import (
	"context"
	"testing"
	"time"
)

func TestCountTracesHalfOpenRange(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	from, to := t0, t0.Add(5*time.Minute)
	// Unfinished traces count too: a call is every recorded request.
	perfTrace(t, s, "before", "a", from.Add(-time.Millisecond), TraceResult{})
	perfTrace(t, s, "at-from", "a", from, TraceResult{})
	perfTrace(t, s, "inside", "a", from.Add(time.Minute), okResult(1_000, 500))
	perfTrace(t, s, "last-ms", "a", to.Add(-time.Millisecond), TraceResult{Status: 500, DurationMicros: 10})
	perfTrace(t, s, "at-to", "a", to, TraceResult{})

	for _, tc := range []struct {
		name     string
		from, to time.Time
		want     int64
	}{
		{"from inclusive, to exclusive", from, to, 3},
		{"empty range", to.Add(time.Hour), to.Add(2 * time.Hour), 0},
		{"zero width", from, from, 0},
		{"everything", from.Add(-time.Hour), to.Add(time.Hour), 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.CountTraces(ctx, tc.from, tc.to)
			if err != nil || got != tc.want {
				t.Fatalf("CountTraces = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
}
