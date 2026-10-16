package store

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestCostByHour(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	// 2025-10-10T00:00:00Z is 1_760_054_400_000 ms.
	midnight := time.UnixMilli(1_760_054_400_000)
	h := func(n int) time.Time { return midnight.Add(time.Duration(n) * time.Hour) }
	type rec struct {
		id, account, model string
		at                 time.Time
		usage              *Usage
		finish             bool
	}
	u := func(i, o, cr, cc int64) *Usage { return &Usage{Input: i, Output: o, CacheRead: cr, CacheCreation: cc} }
	for _, r := range []rec{
		{"1", "acc-a", "claude-opus-5-5", h(5).Add(-time.Millisecond), u(1, 2, 3, 4), true},  // 04:59:59.999 -> hour 04
		{"2", "acc-a", "claude-opus-5-5", h(5), u(10, 20, 30, 40), true},                     // 05:00 boundary -> hour 05
		{"3", "acc-a", "claude-opus-5-5", h(5).Add(59 * time.Minute), u(100, 0, 0, 0), true}, // same bucket as 2
		{"4", "acc-b", "claude-opus-5-5", h(5), u(5, 5, 5, 5), true},                         // other account
		{"5", "acc-a", "unknown-model", h(5), u(1, 1, 1, 1), true},                           // other model
		{"6", "", "claude-opus-5-5", h(-1), nil, true},                                       // previous UTC day, no account, no usage
		{"7", "acc-a", "claude-opus-5-5", h(7), u(9, 9, 9, 9), false},                        // unfinished: excluded
		{"8", "acc-a", "claude-opus-5-5", h(-30), u(7, 7, 7, 7), true},                       // before since
	} {
		tr := sampleTrace(r.id, "a", 0)
		tr.StartedAt, tr.Model, tr.AccountID = r.at, r.model, r.account
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
		if r.finish {
			if err := s.FinishTrace(ctx, r.id, TraceResult{Status: 200, Usage: r.usage}); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := s.CostByHour(ctx, midnight.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Hours 00..03 and 06 have no traces and are absent.
	want := []HourAggregate{
		{Hour: "2025-10-09T23:00:00Z", AccountID: "", Model: "claude-opus-5-5", Requests: 1},
		{Hour: "2025-10-10T04:00:00Z", AccountID: "acc-a", Model: "claude-opus-5-5", Requests: 1, Usage: Usage{Input: 1, Output: 2, CacheRead: 3, CacheCreation: 4}},
		{Hour: "2025-10-10T05:00:00Z", AccountID: "acc-a", Model: "claude-opus-5-5", Requests: 2, Usage: Usage{Input: 110, Output: 20, CacheRead: 30, CacheCreation: 40}},
		{Hour: "2025-10-10T05:00:00Z", AccountID: "acc-a", Model: "unknown-model", Requests: 1, Usage: Usage{Input: 1, Output: 1, CacheRead: 1, CacheCreation: 1}},
		{Hour: "2025-10-10T05:00:00Z", AccountID: "acc-b", Model: "claude-opus-5-5", Requests: 1, Usage: Usage{Input: 5, Output: 5, CacheRead: 5, CacheCreation: 5}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CostByHour =\n%+v\nwant\n%+v", got, want)
	}
}
