package alert

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/meter"
)

var t0 = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

type fakeAccounts struct {
	list      []accounts.Account
	usage     map[string]accounts.UsageSnapshot
	listErr   error
	activated []string
	actErr    error
}

func (f *fakeAccounts) List(context.Context) ([]accounts.Account, error) {
	return f.list, f.listErr
}
func (f *fakeAccounts) Usage(_ context.Context, id string) (accounts.UsageSnapshot, error) {
	u, ok := f.usage[id]
	if !ok {
		return accounts.UsageSnapshot{}, errors.New("unknown")
	}
	return u, nil
}
func (f *fakeAccounts) Activate(_ context.Context, id string) (accounts.Activation, error) {
	f.activated = append(f.activated, id)
	return accounts.Activation{}, f.actErr
}

func pct(v float64) *float64 { return &v }

func window(util *float64, reset time.Time) accounts.UsageWindow {
	w := accounts.UsageWindow{Utilization: util}
	if !reset.IsZero() {
		w.Reset = reset.Format(time.RFC3339)
	}
	return w
}

// threeAccounts: A is active, B has a full week, C has room.
func threeAccounts() *fakeAccounts {
	return &fakeAccounts{
		list: []accounts.Account{
			{ID: "a", Name: "Alpha", Active: true},
			{ID: "b", Name: "Beta"},
			{ID: "c", Name: "Gamma"},
		},
		usage: map[string]accounts.UsageSnapshot{
			"b": {SevenDay: window(pct(100), t0.Add(48*time.Hour))},
			"c": {SevenDay: window(pct(40), t0.Add(72*time.Hour))},
		},
	}
}

// limits builds the unified rate-limit headers; util < 0 leaves it out.
func limits(status string, five float64, fiveReset time.Time, fiveStatus string) http.Header {
	h := http.Header{}
	h.Set("Anthropic-Ratelimit-Unified-Status", status)
	if five >= 0 {
		h.Set("Anthropic-Ratelimit-Unified-5h-Utilization", fmt.Sprint(five))
	}
	h.Set("Anthropic-Ratelimit-Unified-5h-Reset", fmt.Sprint(fiveReset.Unix()))
	if fiveStatus != "" {
		h.Set("Anthropic-Ratelimit-Unified-5h-Status", fiveStatus)
	}
	h.Set("Anthropic-Ratelimit-Unified-7d-Utilization", "0.3")
	h.Set("Anthropic-Ratelimit-Unified-7d-Reset", fmt.Sprint(t0.Add(96*time.Hour).Unix()))
	return h
}

type harness struct {
	w    *Watcher
	acc  *fakeAccounts
	now  time.Time
	sent []string
}

func newHarness(acc *fakeAccounts, autoSwitch bool) *harness {
	h := &harness{acc: acc, now: t0}
	h.w = NewWatcher(Options{
		Accounts: acc, AutoSwitch: autoSwitch, Threshold: 90,
		Notify: func(text string) error { h.sent = append(h.sent, text); return nil },
		Now:    func() time.Time { return h.now },
	})
	return h
}

// observe feeds one response like the proxy does: headers first, then the
// status of the finished trace.
func (h *harness) observe(t *testing.T, status int, hdr http.Header) {
	t.Helper()
	if l, ok := meter.ParseLimits(hdr, h.now); ok {
		h.w.Observe("a", l)
	}
	h.w.ObserveStatus("a", status)
	h.w.drain(context.Background())
}

func TestWarningOncePerPoint(t *testing.T) {
	reset := t0.Add(80 * time.Minute)
	h := newHarness(threeAccounts(), true)
	for _, u := range []float64{0.89, 0.90, 0.904, 0.91, 0.90, 0.29} {
		h.observe(t, 200, limits("allowed_warning", u, reset, ""))
	}
	if len(h.sent) != 2 {
		t.Fatalf("messages = %d, want 2: %q", len(h.sent), h.sent)
	}
	want := "⚠️ 5-h-Limit von *Alpha* bei 90 % (Reset in 1 h 20 min). Ausweichen: *Gamma* (Woche 40 %, 5 h unbekannt)."
	if h.sent[0] != want {
		t.Errorf("message\n got %q\nwant %q", h.sent[0], want)
	}
	if !strings.Contains(h.sent[1], "bei 91 %") {
		t.Errorf("second message = %q", h.sent[1])
	}
}

func TestWarningJumpAndNewWindow(t *testing.T) {
	h := newHarness(threeAccounts(), true)
	r1, r2 := t0.Add(time.Hour), t0.Add(6*time.Hour)
	h.observe(t, 200, limits("allowed", 0.90, r1, ""))
	h.observe(t, 200, limits("allowed", 0.93, r1, ""))
	h.observe(t, 200, limits("allowed", 0.95, r1, ""))
	h.observe(t, 200, limits("allowed", 0.92, r2, ""))
	var got []string
	for _, s := range h.sent {
		got = append(got, s[strings.Index(s, "bei"):strings.Index(s, " %")])
	}
	if strings.Join(got, ",") != "bei 90,bei 93,bei 95,bei 92" {
		t.Fatalf("points = %v", got)
	}
}

func TestWarningWithoutCandidate(t *testing.T) {
	acc := threeAccounts()
	acc.usage["c"] = accounts.UsageSnapshot{SevenDay: window(pct(20), t0.Add(time.Hour)), FiveHour: window(pct(100), t0.Add(time.Hour))}
	h := newHarness(acc, true)
	h.observe(t, 200, limits("allowed", 0.95, t0.Add(30*time.Minute), ""))
	if len(h.sent) != 1 || !strings.HasSuffix(h.sent[0], "Kein anderes Abo mit freiem Wochenlimit.") {
		t.Fatalf("messages = %q", h.sent)
	}
}

func TestRejectedSwitches(t *testing.T) {
	h := newHarness(threeAccounts(), true)
	h.observe(t, 429, limits("rejected", 1.0, t0.Add(2*time.Hour), "rejected"))
	if strings.Join(h.acc.activated, ",") != "c" {
		t.Fatalf("activated = %v, want [c]", h.acc.activated)
	}
	want := "🔁 *Alpha* abgelehnt (5-h-Limit). Gewechselt auf *Gamma* (Woche 40 %, 5 h unbekannt)."
	if len(h.sent) != 1 || h.sent[0] != want {
		t.Fatalf("messages = %q, want %q", h.sent, want)
	}

	// Cooldown: a second rejection five minutes later does nothing.
	h.now = t0.Add(5 * time.Minute)
	h.observe(t, 429, limits("rejected", 1.0, t0.Add(2*time.Hour), "rejected"))
	if len(h.acc.activated) != 1 || len(h.sent) != 1 {
		t.Fatalf("cooldown ignored: activated %v, sent %q", h.acc.activated, h.sent)
	}
	// After the cooldown the next rejection is handled again.
	h.now = t0.Add(11 * time.Minute)
	h.observe(t, 429, limits("rejected", 1.0, t0.Add(2*time.Hour), "rejected"))
	if len(h.acc.activated) != 2 {
		t.Fatalf("after cooldown activated = %v", h.acc.activated)
	}
}

func TestRejectedWithoutHeadersNearLimit(t *testing.T) {
	h := newHarness(threeAccounts(), true)
	// A plain 429 without history is a short rate limit, not the subscription.
	h.observe(t, 429, http.Header{})
	h.observe(t, 200, limits("allowed", 0.89, t0.Add(time.Hour), ""))
	h.observe(t, 429, http.Header{})
	if len(h.acc.activated) != 0 || len(h.sent) != 0 {
		t.Fatalf("429 below the threshold: activated %v, sent %q", h.acc.activated, h.sent)
	}
	h.observe(t, 200, limits("allowed_warning", 0.97, t0.Add(time.Hour), ""))
	h.sent = nil
	h.observe(t, 429, http.Header{})
	if strings.Join(h.acc.activated, ",") != "c" {
		t.Fatalf("activated = %v, want [c]", h.acc.activated)
	}
	if len(h.sent) != 1 || !strings.Contains(h.sent[0], "*Alpha* abgelehnt (Limit)") {
		t.Fatalf("messages = %q", h.sent)
	}
}

func TestRejectedVariants(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*fakeAccounts)
		auto     bool
		want     string
		activate string
	}{
		{"no candidate", func(f *fakeAccounts) { delete(f.usage, "c"); f.list = f.list[:2] }, true,
			"⛔ *Alpha* abgelehnt (5-h-Limit). Kein anderes Abo frei.", ""},
		{"switch off", nil, false,
			"⛔ *Alpha* abgelehnt (5-h-Limit). Automatischer Wechsel ist aus. Ausweichen: *Gamma* (Woche 40 %, 5 h unbekannt).", ""},
		{"activate fails", func(f *fakeAccounts) { f.actErr = errors.New("Test fehlgeschlagen") }, true,
			"❌ *Alpha* abgelehnt (5-h-Limit). Wechsel auf *Gamma* fehlgeschlagen: Test fehlgeschlagen", "c"},
		{"already switched", func(f *fakeAccounts) { f.list[0].Active, f.list[2].Active = false, true }, true, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acc := threeAccounts()
			if tc.mutate != nil {
				tc.mutate(acc)
			}
			h := newHarness(acc, tc.auto)
			h.observe(t, 429, limits("rejected", 1.0, t0.Add(time.Hour), "rejected"))
			if got := strings.Join(acc.activated, ","); got != tc.activate {
				t.Errorf("activated = %q, want %q", got, tc.activate)
			}
			if tc.want == "" {
				if len(h.sent) != 0 {
					t.Errorf("messages = %q, want none", h.sent)
				}
				return
			}
			if len(h.sent) != 1 || h.sent[0] != tc.want {
				t.Errorf("messages = %q\nwant %q", h.sent, tc.want)
			}
		})
	}
}

func TestObserveNeverBlocks(t *testing.T) {
	h := newHarness(threeAccounts(), true)
	done := make(chan struct{})
	go func() {
		// Nobody drains: the queue fills up and further jobs are dropped.
		for i := range 200 {
			l, _ := meter.ParseLimits(limits("allowed", 0.90+float64(i%10)/1000, t0.Add(time.Duration(i)*time.Minute), ""), t0)
			h.w.Observe("a", l)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Observe blocked")
	}
}

func TestPickCandidate(t *testing.T) {
	past := t0.Add(-time.Minute)
	acc := &fakeAccounts{
		list: []accounts.Account{
			{ID: "a", Name: "Aktiv", Active: true},
			{ID: "b", Name: "Bravo"},   // week unknown
			{ID: "c", Name: "Charlie"}, // 5h full but reset passed: free, week 70
			{ID: "d", Name: "Delta"},   // 5h 30, week 10
			{ID: "e", Name: "Echo"},    // needs a new login
			{ID: "f", Name: "Foxtrot"}, // 5h full
		},
		usage: map[string]accounts.UsageSnapshot{
			"c": {FiveHour: window(pct(100), past), SevenDay: window(pct(70), t0.Add(time.Hour))},
			"d": {FiveHour: window(pct(30), t0.Add(time.Hour)), SevenDay: window(pct(10), t0.Add(time.Hour))},
			"e": {SevenDay: window(pct(0), t0.Add(time.Hour)), License: &accounts.License{Token: accounts.TokenRelogin}},
			"f": {FiveHour: window(pct(100), t0.Add(time.Hour)), SevenDay: window(pct(5), t0.Add(time.Hour))},
		},
	}
	got := pickCandidates(context.Background(), acc, acc.list, "a", t0)
	var names []string
	for _, c := range got {
		names = append(names, c.account.Name)
	}
	// Known week first; then lower 5h (expired = 0); then lower week; unknown week last.
	if strings.Join(names, ",") != "Charlie,Delta,Bravo" {
		t.Fatalf("order = %v", names)
	}
}

func TestResetText(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{80 * time.Minute, " (Reset in 1 h 20 min)"},
		{59*time.Minute + 30*time.Second, " (Reset in 59 min)"},
		{20 * time.Second, " (Reset in < 1 min)"},
		{-time.Minute, ""},
	} {
		r := t0.Add(tc.d)
		if got := resetText(&r, t0); got != tc.want {
			t.Errorf("resetText(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
	if got := resetText(nil, t0); got != "" {
		t.Errorf("resetText(nil) = %q", got)
	}
}
