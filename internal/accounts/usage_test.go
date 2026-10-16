package accounts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestUsageUsesEachTokenAndCachesAcrossRestartWithoutSwitch(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx := context.Background()
	list, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Import(ctx, "Second", tokenB)
	if err != nil {
		t.Fatal(err)
	}
	f.files["claude-a.json"] = []byte(strings.ReplaceAll(string(tokenA), "private-access-a", "rotated-access-a"))
	calls := 0
	m.fetchUsage = func(_ context.Context, token string) (UsageSnapshot, error) {
		calls++
		u := 12.0
		if token == "private-access-b" {
			u = 80
		} else if token != "rotated-access-a" {
			t.Fatal("wrong account token")
		}
		return UsageSnapshot{SevenDay: UsageWindow{Utilization: &u, Reset: time.Now().Add(6 * 24 * time.Hour).UTC().Format(time.RFC3339)}}, nil
	}
	first, err := m.Usage(ctx, list[0].ID)
	if err != nil || *first.SevenDay.Utilization != 12 {
		t.Fatal(first, err)
	}
	second, err := m.Usage(ctx, b.ID)
	if err != nil || *second.SevenDay.Utilization != 80 {
		t.Fatal(second, err)
	}
	if calls != 2 || len(f.files) != 1 || f.tests != 0 {
		t.Fatal("read mutated account or tested model")
	}
	restarted := New(m.st, m.box, f, "mgmt", "api", new(Gate))
	restarted.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		t.Fatal("persistent cache ignored")
		return UsageSnapshot{}, nil
	}
	cached, err := restarted.Usage(ctx, b.ID)
	if err != nil || *cached.SevenDay.Utilization != 80 {
		t.Fatal(cached, err)
	}
}

func TestUsageFailureKeepsLastObservationAndNeverLeaks(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, _ := m.List(ctx)
	u := 99.0
	old := UsageSnapshot{SevenDay: UsageWindow{Utilization: &u, Reset: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, ObservedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}
	if err := m.saveUsage(ctx, list[0].ID, old); err != nil {
		t.Fatal(err)
	}
	calls := 0
	m.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		calls++
		return UsageSnapshot{}, errors.New("private-access-token from upstream")
	}
	result, err := m.Usage(ctx, list[0].ID)
	if err != nil || result.Error == "" || strings.Contains(result.Error, "private") || *result.SevenDay.Utilization != 99 {
		t.Fatal(result, err)
	}
	_, _ = m.Usage(ctx, list[0].ID)
	if calls != 1 {
		t.Fatal("failed reads must be throttled")
	}
	if _, err = m.Usage(ctx, "unknown"); err == nil {
		t.Fatal("unknown account accepted")
	}
}

func TestCachedUsageNeverAsksUpstream(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		t.Fatal("cached read asked upstream")
		return UsageSnapshot{}, nil
	}
	none, err := m.CachedUsage(ctx, list[0].ID)
	if err != nil || none.SevenDay.Utilization != nil {
		t.Fatal(none, err)
	}
	u := 42.0
	if err := m.saveUsage(ctx, list[0].ID, UsageSnapshot{SevenDay: UsageWindow{Utilization: &u}}); err != nil {
		t.Fatal(err)
	}
	got, err := m.CachedUsage(ctx, list[0].ID)
	if err != nil || got.SevenDay.Utilization == nil || *got.SevenDay.Utilization != 42 {
		t.Fatal(got, err)
	}
}
