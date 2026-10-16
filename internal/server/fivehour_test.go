package server

import (
	"net/http"
	"testing"
)

func TestWindowMarks(t *testing.T) {
	const (
		util5, reset5 = "Anthropic-Ratelimit-Unified-5h-Utilization", "Anthropic-Ratelimit-Unified-5h-Reset"
		util7, reset7 = "Anthropic-Ratelimit-Unified-7d-Utilization", "Anthropic-Ratelimit-Unified-7d-Reset"
	)
	ptr := func(v int64) *int64 { return &v }
	for name, c := range map[string]struct {
		h                        map[string]string
		fiveP, fiveR, sevP, sevR *int64
	}{
		"both windows": {h: map[string]string{util5: "0.4236", reset5: "1760000000", util7: "0.31", reset7: "1760500000"},
			fiveP: ptr(424), fiveR: ptr(1_760_000_000), sevP: ptr(310), sevR: ptr(1_760_500_000)},
		"over limit":    {h: map[string]string{util5: "1.02", reset5: "1760000000"}, fiveP: ptr(1020), fiveR: ptr(1_760_000_000)},
		"no 5h reset":   {h: map[string]string{util5: "0.5", util7: "0.2", reset7: "1760500000"}, sevP: ptr(200), sevR: ptr(1_760_500_000)},
		"no headers":    {h: map[string]string{}},
		"garbage value": {h: map[string]string{util5: "x", reset5: "1760000000"}},
	} {
		t.Run(name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range c.h {
				h.Set(k, v)
			}
			fp, fr, sp, sr := windowMarks(h)
			for i, pair := range [][2]*int64{{fp, c.fiveP}, {fr, c.fiveR}, {sp, c.sevP}, {sr, c.sevR}} {
				got, want := pair[0], pair[1]
				if (got == nil) != (want == nil) || got != nil && *got != *want {
					t.Fatalf("value %d: got %v want %v", i, got, want)
				}
			}
		})
	}
}
