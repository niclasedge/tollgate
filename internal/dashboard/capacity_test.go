package dashboard

import (
	"reflect"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/store"
)

func TestCapacity(t *testing.T) {
	base := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour
	at := func(h float64) time.Time { return base.Add(time.Duration(h * float64(time.Hour))) }
	p := func(v int64) *int64 { return &v }
	f := func(v float64) *float64 { return &v }
	// claude-opus-5-5 input costs 4000 nano-USD per token.
	row := func(acc string, h float64, five int64, input int64) store.FillRow {
		return store.FillRow{AccountID: acc, StartedAt: at(h), Permille: p(five), Reset: p(1000),
			Model: "claude-opus-5-5", Usage: store.Usage{Input: input}}
	}
	plain := func(acc string, h float64, input int64) store.FillRow {
		return store.FillRow{AccountID: acc, StartedAt: at(h), Model: "claude-opus-5-5", Usage: store.Usage{Input: input}}
	}
	// One window: 500 permille for 4e6 nano-USD, so a full window costs 8e6.
	window := []store.FillRow{row("a", 0, 0, 0), row("a", 2, 500, 1000), plain("a", 3, 9000)}
	marks := []weekMark{
		{account: "a", used: 0.5, start: base, observed: at(3), reset: base.Add(week)},         // 40e6 = 5 windows at 50 %
		{account: "b", used: 0.1, start: base, observed: at(3), reset: base.Add(week)},         // too little use
		{account: "c", used: 0.8, start: at(-1), observed: at(3), reset: at(-1).Add(week)},     // window not covered
		{account: "d", used: 1, start: at(-200), observed: at(-40), reset: at(-200).Add(week)}, // week over
	}
	cases := map[string]struct {
		rows  []store.FillRow
		since time.Time
		marks []weekMark
		want  Capacity
	}{
		"weekly limit holds ten windows": {rows: window, since: base, marks: marks, want: Capacity{
			WindowsPerWeek: f(10), FillsPerDay: f(6), MeasuredHours: 2, SampleWindows: 1,
			WindowCostNanoUSD: p(8_000_000), SampleAccounts: 1, Accounts: 4, FreeThisWeek: f(16)}},
		"no window over 30 points": {rows: []store.FillRow{row("a", 0, 0, 0), row("a", 2, 200, 1000)}, since: base, marks: marks,
			want: Capacity{FillsPerDay: f(2.4), MeasuredHours: 2, Accounts: 4}},
		"period after the window still prices it": {rows: window, since: at(1), marks: marks[:1], want: Capacity{
			WindowsPerWeek: f(10), SampleWindows: 1, WindowCostNanoUSD: p(8_000_000), SampleAccounts: 1, Accounts: 4, FreeThisWeek: f(5)}},
		"no rows": {since: base, marks: marks, want: Capacity{Accounts: 4}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := capacity(c.rows, c.since, at(48), c.marks, 4); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %+v want %+v", show(got), show(c.want))
			}
		})
	}
}

func TestWeekMarks(t *testing.T) {
	u := func(v float64) *float64 { return &v }
	got := weekMarks(map[string]accounts.UsageSnapshot{
		"b": {SevenDay: accounts.UsageWindow{Utilization: u(40), Reset: "2026-10-14T20:00:00Z"}, ObservedAt: "2026-10-08T17:00:00Z"},
		"a": {SevenDay: accounts.UsageWindow{Utilization: u(100), Reset: "2026-10-10T11:00:00Z", ObservedAt: "2026-10-06T14:52:11Z"}},
		"c": {SevenDay: accounts.UsageWindow{Reset: "2026-10-10T11:00:00Z"}},                                       // no utilization
		"d": {SevenDay: accounts.UsageWindow{Utilization: u(5), Reset: "bad"}, ObservedAt: "2026-10-08T17:00:00Z"}, // no reset
	})
	reset := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	want := []weekMark{
		{account: "a", used: 1, start: reset("2026-10-03T11:00:00Z"), observed: reset("2026-10-06T14:52:11Z"), reset: reset("2026-10-10T11:00:00Z")},
		{account: "b", used: 0.4, start: reset("2026-10-07T20:00:00Z"), observed: reset("2026-10-08T17:00:00Z"), reset: reset("2026-10-14T20:00:00Z")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func show(c Capacity) []any {
	v := func(f *float64) any {
		if f == nil {
			return nil
		}
		return *f
	}
	var cost any
	if c.WindowCostNanoUSD != nil {
		cost = *c.WindowCostNanoUSD
	}
	return []any{v(c.WindowsPerWeek), v(c.FillsPerDay), c.MeasuredHours, c.SampleWindows, cost, c.SampleAccounts, c.Accounts, v(c.FreeThisWeek)}
}
