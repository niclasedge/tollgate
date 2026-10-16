package meter

import (
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/store"
)

func TestFromJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
		want *store.Usage
	}{
		{"full", `{"id":"m","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":7,"service_tier":"standard"}}`,
			&store.Usage{Input: 10, Output: 5, CacheRead: 100, CacheCreation: 7}},
		{"without cache fields", `{"usage":{"input_tokens":3,"output_tokens":4}}`, &store.Usage{Input: 3, Output: 4}},
		{"null cache fields", `{"usage":{"input_tokens":3,"output_tokens":4,"cache_read_input_tokens":null}}`, &store.Usage{Input: 3, Output: 4}},
		{"no usage", `{"type":"error","error":{"type":"overloaded_error"}}`, nil},
		{"not json", `<html>`, nil},
		{"negative ignored", `{"usage":{"input_tokens":-1,"output_tokens":2}}`, &store.Usage{Output: 2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FromJSON([]byte(tc.body))
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

const stream = "event: message_start\n" +
	`data: {"type":"message_start","message":{"id":"m","usage":{"input_tokens":12,"cache_creation_input_tokens":30,"cache_read_input_tokens":400,"output_tokens":1}}}` + "\n\n" +
	"event: content_block_delta\n" +
	`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"\"usage\":{\"output_tokens\":999}"}}` + "\n\n" +
	"event: ping\ndata: {\"type\": \"ping\"}\n\n" +
	"event: message_delta\r\n" +
	`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":57}}` + "\r\n\r\n" +
	"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

func events(s string) []string {
	var out []string
	for s != "" {
		i := strings.Index(s, "\n\n")
		j := strings.Index(s, "\r\n\r\n")
		switch {
		case j >= 0 && (i < 0 || j < i):
			out, s = append(out, s[:j+4]), s[j+4:]
		case i >= 0:
			out, s = append(out, s[:i+2]), s[i+2:]
		default:
			out, s = append(out, s), ""
		}
	}
	return out
}

func TestStreamCollectsStartAndDelta(t *testing.T) {
	var s Stream
	for _, ev := range events(stream) {
		s.Observe([]byte(ev))
	}
	want := store.Usage{Input: 12, Output: 57, CacheRead: 400, CacheCreation: 30}
	if got := s.Usage(); got == nil || *got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStreamDeltaUpdatesInputWhenPresent(t *testing.T) {
	var s Stream
	s.Observe([]byte(`data: {"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":1}}}` + "\n\n"))
	s.Observe([]byte(`data: {"type":"message_delta","usage":{"input_tokens":6,"cache_read_input_tokens":2,"output_tokens":9}}` + "\n\n"))
	want := store.Usage{Input: 6, Output: 9, CacheRead: 2}
	if got := s.Usage(); got == nil || *got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStreamWithoutUsageEvents(t *testing.T) {
	var s Stream
	for _, ev := range []string{"event: error\ndata: {\"type\":\"error\"}\n\n", ": comment\n\n", "data: not json\n\n", "\n"} {
		s.Observe([]byte(ev))
	}
	if s.Usage() != nil {
		t.Fatalf("usage without usage events: %+v", s.Usage())
	}
}

func TestStreamHostileValues(t *testing.T) {
	var s Stream
	s.Observe([]byte(`data:{"type":"message_start","message":{}}` + "\n\n")) // no usage, no space after data:
	if s.Usage() != nil {
		t.Fatalf("usage from message_start without usage: %+v", s.Usage())
	}
	s.Observe([]byte(`data:{"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":3}}}` + "\n\n"))
	s.Observe([]byte(`data: {"type":"message_delta","usage":{"input_tokens":"7"}}` + "\n\n"))                  // string: event dropped
	s.Observe([]byte(`data: {"type":"message_delta","usage":{"input_tokens":1099511627777}}` + "\n\n"))        // > 1<<40: ignored
	s.Observe([]byte(`data: {"type":"message_delta",` + "\n" + `data: "usage":{"output_tokens":0}}` + "\n\n")) // zero is a value
	want := store.Usage{Input: 5}
	if got := s.Usage(); got == nil || *got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func FuzzMeterNeverPanics(f *testing.F) {
	f.Add([]byte(stream))
	f.Add([]byte(`{"usage":{"input_tokens":1}}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		var s Stream
		for _, ev := range events(string(b)) {
			s.Observe([]byte(ev))
		}
		if u := s.Usage(); u != nil {
			if c, ok := Cost("claude-fable-5-1", *u); ok && c < 0 {
				t.Fatalf("negative cost %d for %+v", c, u)
			}
		}
		if u := FromJSON(b); u != nil {
			if c, _ := Cost("claude-fable-5-1", *u); c < 0 {
				t.Fatalf("negative cost %d for %+v", c, u)
			}
		}
	})
}

func TestEveryPriceKeyResolvesToItself(t *testing.T) {
	for k, want := range prices {
		if got, ok := PriceFor(k); !ok || got != want {
			t.Errorf("%s resolves to %+v", k, got)
		}
	}
	if p, _ := PriceFor("claude-fable-5-1"); p.CacheRead != 250 {
		t.Errorf("fable 5.1 matched fable 5: %+v", p)
	}
}

func TestEventDataJoinsLines(t *testing.T) {
	got := string(eventData([]byte("event: x\ndata:{\"a\":\ndata: 1}\n\n")))
	if got != "{\"a\":\n1}" {
		t.Fatalf("got %q", got)
	}
}

func TestPriceFor(t *testing.T) {
	tests := []struct {
		model string
		input int64
		ok    bool
	}{
		{"claude-opus-5-5", 4000, true},
		{"claude-opus-5-5[1m]", 4000, true},
		{"claude-opus-5", 5000, true},
		{"claude-opus-5-20260101", 5000, true},
		{"claude-haiku-4-5-20251001", 1000, true},
		{"claude-haiku-5-5", 100, true},
		{"claude-opus-4", 15000, true},
		{"claude-opus-4-1-20250805", 15000, true},
		{"claude-opus-40", 0, false},
		{"claude-unknown", 0, false},
		{"", 0, false},
	}
	for _, tc := range tests {
		p, ok := PriceFor(tc.model)
		if ok != tc.ok || p.Input != tc.input {
			t.Errorf("%q: got %+v %v, want input %d %v", tc.model, p, ok, tc.input, tc.ok)
		}
	}
}

func TestCost(t *testing.T) {
	// Opus 5.5: 1M input $4, 1M output $20, 1M cache read $0.20, 1M cache write $5.
	u := store.Usage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000, CacheCreation: 1_000_000}
	c, ok := Cost("claude-opus-5-5", u)
	if !ok || c != 29_200_000_000 {
		t.Fatalf("cost %d %v, want 29.2 USD in nano-USD", c, ok)
	}
	// Haiku 5.5 below 100K prompt tokens: $0.10 + $0.50 + $0.01 + $0.125 (#138).
	if c, ok := Cost("claude-haiku-5-5", u); !ok || c != 735_000_000 {
		t.Fatalf("haiku 5.5 cost %d %v, want 0.735 USD in nano-USD", c, ok)
	}
	if _, ok := Cost("gpt-5", u); ok {
		t.Fatal("unknown model priced")
	}
}

func TestCacheSavings(t *testing.T) {
	tests := []struct {
		name  string
		model string
		u     store.Usage
		want  int64
		ok    bool
	}{
		// Opus 5.5: input $4, cache read $0.20 per MTok -> $3.80 saved per MTok read.
		{"opus 5.5", "claude-opus-5-5", store.Usage{CacheRead: 1_000_000}, 3_800_000_000, true},
		{"input, output and writes do not count", "claude-opus-5-5", store.Usage{Input: 7, Output: 9, CacheCreation: 1_000}, 0, true},
		{"haiku 4.5", "claude-haiku-4-5", store.Usage{CacheRead: 10}, 10 * (1000 - 100), true},
		// Sonnet 5.5 reads the cache at 0.05x input ($0.10 of $2 per MTok, #138).
		{"sonnet 5.5", "claude-sonnet-5-5", store.Usage{CacheRead: 1_000_000}, 1_900_000_000, true},
		{"haiku 5.5", "claude-haiku-5-5", store.Usage{CacheRead: 1_000_000}, 90_000_000, true},
		{"unknown model", "claude-unknown", store.Usage{CacheRead: 1_000}, 0, false},
	}
	for _, tc := range tests {
		got, ok := CacheSavings(tc.model, tc.u)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: got %d %v, want %d %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
