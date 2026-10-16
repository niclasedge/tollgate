package dashboard

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCallRate(t *testing.T) {
	now := midnight.Add(5 * time.Hour) // the env clock
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	// n traces spread inside one five-minute window k (0 = current).
	window := func(k, n int) []time.Time {
		var out []time.Time
		for i := range n {
			out = append(out, ago(time.Duration(k)*5*time.Minute+time.Duration(i+1)*time.Second))
		}
		return out
	}
	concat := func(parts ...[]time.Time) []time.Time {
		var out []time.Time
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	for _, tc := range []struct {
		name string
		at   []time.Time
		want CallRate
	}{
		{"no traces", nil,
			CallRate{WindowSeconds: 300, Windows: 10, Current: 0, PreviousTotal: 0, Average: 0, Trend: "equal"}},
		{"above: empty windows count as zero", concat(window(0, 4), window(1, 20)),
			CallRate{WindowSeconds: 300, Windows: 10, Current: 4, PreviousTotal: 20, Average: 2, Trend: "above"}},
		{"below", concat(window(0, 1), window(3, 7), window(10, 8)),
			CallRate{WindowSeconds: 300, Windows: 10, Current: 1, PreviousTotal: 15, Average: 1.5, Trend: "below"}},
		{"equal", concat(window(0, 2), window(2, 10), window(9, 10)),
			CallRate{WindowSeconds: 300, Windows: 10, Current: 2, PreviousTotal: 20, Average: 2, Trend: "equal"}},
		{"average with one decimal", concat(window(0, 1), window(4, 23)),
			CallRate{WindowSeconds: 300, Windows: 10, Current: 1, PreviousTotal: 23, Average: 2.3, Trend: "below"}},
		{"only previous windows", window(5, 3),
			CallRate{WindowSeconds: 300, Windows: 10, Current: 0, PreviousTotal: 3, Average: 0.3, Trend: "below"}},
		{"only current window", window(0, 1),
			CallRate{WindowSeconds: 300, Windows: 10, Current: 1, PreviousTotal: 0, Average: 0, Trend: "above"}},
		{"boundaries", []time.Time{
			now,                                    // not yet: the current window ends before now
			ago(time.Millisecond),                  // current
			ago(5 * time.Minute),                   // current: start inclusive
			ago(5*time.Minute + time.Millisecond),  // previous
			ago(55 * time.Minute),                  // previous: start inclusive
			ago(55*time.Minute + time.Millisecond), // too old
			ago(2 * time.Hour),                     // too old
		}, CallRate{WindowSeconds: 300, Windows: 10, Current: 2, PreviousTotal: 2, Average: 0.2, Trend: "above"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, nil)
			addClient(t, e.st, "c1")
			for i, at := range tc.at {
				// Unfinished and failed requests are calls as well.
				r := rec{id: fmt.Sprintf("t%03d", i), client: "c1", model: "m", at: at}
				record(t, e.st, r)
			}
			var got CallRate
			getJSON(t, e.srv.URL+"/api/calls", &got)
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCallRateNeedsAdminAndCarriesNoContent(t *testing.T) {
	e := newEnv(t, nil)
	key := addClient(t, e.st, "c1")
	record(t, e.st, rec{id: "t1", client: "c1", model: "secret-model", at: midnight.Add(5*time.Hour - time.Minute)})
	for _, tok := range []string{"", key, "nope"} {
		if code, _ := get(t, e.srv.URL+"/api/calls", tok); code != http.StatusUnauthorized {
			t.Fatalf("token %q: status %d, want 401", tok, code)
		}
	}
	code, body := get(t, e.srv.URL+"/api/calls", adminToken)
	if code != http.StatusOK || strings.Contains(string(body), "secret-model") || strings.Contains(string(body), "c1") {
		t.Fatalf("status %d body %s", code, body)
	}
}
