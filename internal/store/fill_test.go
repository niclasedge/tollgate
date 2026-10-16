package store

import (
	"context"
	"testing"
	"time"
)

func TestFillRows(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	type rec struct {
		id, account, endpoint string
		at                    time.Duration
		res                   TraceResult
	}
	five := TraceResult{Status: 200, Usage: &Usage{Input: 5, Output: 7, CacheRead: 11, CacheCreation: 13}, FiveHourPermille: i64(423), FiveHourReset: i64(1_760_000_000), SevenDayPermille: i64(310), SevenDayReset: i64(1_760_500_000)}
	for _, r := range []rec{
		{"1", "acc-a", "messages", time.Minute, five},
		{"2", "acc-a", "messages", 2 * time.Minute, TraceResult{Status: 429}},             // no headers: still a request
		{"3", "", "messages", 3 * time.Minute, five},                                      // no account: excluded
		{"4", "acc-a", "count_tokens", 4 * time.Minute, five},                             // other endpoint: excluded
		{"5", "acc-a", "messages", -time.Minute, five},                                    // before since: excluded
		{"6", "acc-b", "messages", 0, TraceResult{Status: 200, FiveHourPermille: i64(0)}}, // since is inclusive
	} {
		tr := sampleTrace(r.id, "a", 0)
		tr.StartedAt, tr.AccountID, tr.Endpoint = t0.Add(r.at), r.account, r.endpoint
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
		if err := s.FinishTrace(ctx, r.id, r.res); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.FillRows(ctx, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("rows %+v", got)
	}
	if got[0].AccountID != "acc-b" || *got[0].Permille != 0 || got[0].Reset != nil || got[0].SevenPermille != nil {
		t.Fatalf("row 0 %+v", got[0])
	}
	if r := got[1]; r.AccountID != "acc-a" || *r.Permille != 423 || *r.Reset != 1_760_000_000 || *r.SevenPermille != 310 || *r.SevenReset != 1_760_500_000 || !r.StartedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("row 1 %+v", r)
	}
	if r := got[1]; r.Model != "claude-sonnet-4-5" || r.Usage != (Usage{Input: 5, Output: 7, CacheRead: 11, CacheCreation: 13}) || got[2].Usage != (Usage{}) {
		t.Fatalf("model and usage %+v %+v", r, got[2])
	}
	if got[2].Permille != nil || got[2].Reset != nil {
		t.Fatalf("row 2 %+v", got[2])
	}
	bad := TraceResult{Status: 200, FiveHourPermille: i64(-1)}
	if err := s.FinishTrace(ctx, "1", bad); err == nil {
		t.Fatal("negative permille accepted")
	}
}
