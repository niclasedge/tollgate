package store

import (
	"context"
	"testing"
	"time"
)

func TestTraceAccountPersistsAndCostsStaySeparated(t *testing.T) {
	ctx := context.Background()
	s, _, path := openTest(t)
	mustClient(t, s, "a")
	for _, id := range []string{"account-a", "account-b"} {
		if err := s.SaveAccount(ctx, AccountRecord{ID: id, Filename: id + ".json", Name: id, Email: "test@example.com", Cipher: []byte{1}}); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range []string{"account-a", "account-b", ""} {
		tr := sampleTrace([]string{"ta", "tb", "legacy"}[i], "a", 0)
		tr.AccountID = id
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
		if err := s.FinishTrace(ctx, tr.ID, TraceResult{Status: 200, Usage: &Usage{Input: int64(i+1) * 10, Output: 5}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteAccount(ctx, "account-a"); err != nil {
		t.Fatal(err)
	}
	remaining, err := s.ListAccounts(ctx)
	if err != nil || len(remaining) != 1 || remaining[0].ID != "account-b" {
		t.Fatal("account was not removed", remaining, err)
	}
	reopened, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	tr, err := reopened.GetTrace(ctx, "ta")
	if err != nil || tr.AccountID != "account-a" {
		t.Fatal(tr, err)
	}
	legacy, err := reopened.GetTrace(ctx, "legacy")
	if err != nil || legacy.AccountID != "" {
		t.Fatal(legacy, err)
	}
	groups, err := reopened.CostByDay(ctx, t0.Add(-time.Hour))
	if err != nil || len(groups) != 3 {
		t.Fatal(groups, err)
	}
	sums := map[string]int64{}
	for _, g := range groups {
		sums[g.AccountID] += g.Usage.Input
	}
	if sums["account-a"] != 10 || sums["account-b"] != 20 || sums[""] != 30 {
		t.Fatal(sums)
	}
}
