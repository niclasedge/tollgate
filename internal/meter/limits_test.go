package meter

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func header(kv ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(kv); i += 2 {
		h.Set(kv[i], kv[i+1])
	}
	return h
}

func TestParseLimits(t *testing.T) {
	now := time.Date(2026, 10, 4, 18, 0, 0, 0, time.UTC)
	full := header(
		"Anthropic-Ratelimit-Unified-Status", "allowed",
		"Anthropic-Ratelimit-Unified-Reset", "1790000000",
		"Anthropic-Ratelimit-Unified-Representative-Claim", "five_hour",
		"Anthropic-Ratelimit-Unified-5h-Utilization", "0.42",
		"Anthropic-Ratelimit-Unified-5h-Reset", "1790000000",
		"Anthropic-Ratelimit-Unified-5h-Status", "allowed",
		"Anthropic-Ratelimit-Unified-7d-Utilization", "0.1",
		"Anthropic-Ratelimit-Unified-7d-Reset", "2026-10-06T00:00:00Z",
		"Anthropic-Ratelimit-Unified-7d-Status", "Allowed_Warning",
	)
	l, ok := ParseLimits(full, now)
	if !ok {
		t.Fatal("ok = false for full headers")
	}
	if l.Status != "allowed" || l.RepresentativeClaim != "five_hour" || !l.ObservedAt.Equal(now) {
		t.Fatalf("top level: %+v", l)
	}
	if l.FiveHour.Utilization == nil || *l.FiveHour.Utilization != 0.42 || l.FiveHour.Status != "allowed" {
		t.Fatalf("five hour: %+v", l.FiveHour)
	}
	if l.FiveHour.Reset == nil || l.FiveHour.Reset.Unix() != 1790000000 || l.Reset == nil || l.Reset.Unix() != 1790000000 {
		t.Fatalf("five hour reset: %v / %v", l.FiveHour.Reset, l.Reset)
	}
	if l.SevenDay.Reset == nil || !l.SevenDay.Reset.Equal(time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("seven day reset (RFC 3339): %v", l.SevenDay.Reset)
	}
	if l.SevenDay.Status != "allowed_warning" {
		t.Fatalf("status is not lower-cased: %q", l.SevenDay.Status)
	}

	if _, ok := ParseLimits(header("Content-Type", "application/json"), now); ok {
		t.Fatal("ok = true without unified headers")
	}

	bad := header(
		"Anthropic-Ratelimit-Unified-5h-Utilization", "abc",
		"Anthropic-Ratelimit-Unified-7d-Utilization", "-0.1",
		"Anthropic-Ratelimit-Unified-5h-Reset", "0",
		"Anthropic-Ratelimit-Unified-7d-Reset", "tomorrow",
		"Anthropic-Ratelimit-Unified-Status", "x"+string(make([]byte, 200)),
	)
	l, ok = ParseLimits(bad, now)
	if !ok {
		t.Fatal("ok = false although unified headers are present")
	}
	if l.FiveHour.Utilization != nil || l.SevenDay.Utilization != nil || l.FiveHour.Reset != nil || l.SevenDay.Reset != nil {
		t.Fatalf("bad values must be unknown (nil), got %+v", l)
	}
	if len(l.Status) != maxLimitTextLen {
		t.Fatalf("status not capped: %d bytes", len(l.Status))
	}
	for _, v := range []string{"NaN", "Inf", "11"} {
		l, _ := ParseLimits(header("Anthropic-Ratelimit-Unified-5h-Utilization", v), now)
		if l.FiveHour.Utilization != nil {
			t.Errorf("utilization %q = %v, want unknown", v, *l.FiveHour.Utilization)
		}
	}
}

func TestLimitTracker(t *testing.T) {
	now := time.Date(2026, 10, 4, 18, 0, 0, 0, time.UTC)
	tr := NewLimitTracker(func() time.Time { return now })
	if _, ok := tr.Latest(); ok {
		t.Fatal("ok before the first observation")
	}
	tr.Observe(header("Anthropic-Ratelimit-Unified-5h-Utilization", "0.5"))
	tr.Observe(header("Content-Type", "application/json")) // e.g. count_tokens
	l, ok := tr.Latest()
	if !ok || l.FiveHour.Utilization == nil || *l.FiveHour.Utilization != 0.5 || !l.ObservedAt.Equal(now) {
		t.Fatalf("a response without headers must keep the state: %+v, %v", l, ok)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() { defer wg.Done(); tr.Observe(header("Anthropic-Ratelimit-Unified-Status", "allowed")) }()
		go func() { defer wg.Done(); _, _ = tr.Latest() }()
	}
	wg.Wait()
}

func FuzzParseLimits(f *testing.F) {
	f.Add("0.42", "1790000000", "allowed")
	f.Add("abc", "2026-10-06T00:00:00Z", "")
	f.Add("-1e309", "-5", "rejected")
	f.Fuzz(func(t *testing.T, util, reset, status string) {
		h := header(
			"Anthropic-Ratelimit-Unified-5h-Utilization", util,
			"Anthropic-Ratelimit-Unified-5h-Reset", reset,
			"Anthropic-Ratelimit-Unified-Status", status,
		)
		l, _ := ParseLimits(h, time.Unix(0, 0))
		if u := l.FiveHour.Utilization; u != nil && (*u < 0 || *u > maxUtilization) {
			t.Fatalf("utilization out of range: %v", *u)
		}
		if len(l.Status) > maxLimitTextLen {
			t.Fatalf("status too long: %d", len(l.Status))
		}
	})
}
