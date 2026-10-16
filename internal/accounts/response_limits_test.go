package accounts

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/upstream"
)

func TestResponseLimitsRecoverFromUsageThrottleAndStayAccountBound(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, _ := m.List(ctx)
	b, err := m.Import(ctx, "Second", tokenB)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	m.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		calls++
		return UsageSnapshot{}, &upstream.UsageError{Status: 429}
	}
	initial, err := m.Usage(ctx, list[0].ID)
	if err != nil || initial.Error == "" {
		t.Fatal(initial, err)
	}
	h := http.Header{}
	h.Set("Anthropic-Ratelimit-Unified-7d-Utilization", "0.42")
	h.Set("Anthropic-Ratelimit-Unified-7d-Reset", time.Now().Add(5*24*time.Hour).UTC().Format(time.RFC3339))
	m.ObserveUsage(list[0].ID, h)
	got, err := m.Usage(ctx, list[0].ID)
	if err != nil || got.Error != "" || got.SevenDay.Utilization == nil || *got.SevenDay.Utilization != 42 {
		t.Fatal(got, err)
	}
	other, err := m.Usage(ctx, b.ID)
	if err != nil || other.SevenDay.Utilization != nil || other.Error == "" {
		t.Fatal("limits leaked to another account", other, err)
	}
	u := 0.1
	reset := time.Now().Add(time.Hour)
	m.ObserveLimits(list[0].ID, meter.Limits{SevenDay: meter.Window{Utilization: &u, Reset: &reset}, ObservedAt: time.Now().Add(-time.Hour)})
	latest, err := m.Usage(ctx, list[0].ID)
	if err != nil || *latest.SevenDay.Utilization != 42 {
		t.Fatal("old observation replaced fresh", latest, err)
	}
	restarted := New(m.st, m.box, m.up, "mgmt", "api", new(Gate))
	restarted.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		t.Fatal("saved response cache ignored")
		return UsageSnapshot{}, nil
	}
	saved, err := restarted.Usage(ctx, list[0].ID)
	if err != nil || *saved.SevenDay.Utilization != 42 {
		t.Fatal(saved, err)
	}
	if calls != 2 {
		t.Fatal("unneeded API reads", calls)
	}
}

func TestResponsePersistenceStartsTimeoutAfterUsageLockContention(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, _ := m.List(ctx)
	// Hold the same critical section as a slow Usage read. There is deliberately
	// no later Usage call to perform persistence on the worker's behalf.
	m.usageMu.Lock()
	h := http.Header{}
	h.Set("Anthropic-Ratelimit-Unified-7d-Utilization", "0.42")
	h.Set("Anthropic-Ratelimit-Unified-7d-Reset", time.Now().Add(5*24*time.Hour).UTC().Format(time.RFC3339))
	m.ObserveUsage(list[0].ID, h)
	time.Sleep(6 * time.Second)
	m.usageMu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for m.usagePersisting.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if m.usagePersisting.Load() {
		t.Fatal("worker did not complete")
	}
	restarted := New(m.st, m.box, m.up, "mgmt", "api", new(Gate))
	restarted.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		t.Fatal("persisted fallback missing")
		return UsageSnapshot{}, nil
	}
	got, err := restarted.Usage(ctx, list[0].ID)
	if err != nil || got.SevenDay.Utilization == nil || *got.SevenDay.Utilization != 42 {
		t.Fatal(got, err)
	}
}
func TestPartialUsagePreservesWeeklyReadingAndItsMeasurementTime(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, _ := m.List(ctx)
	oldTime := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	weekUsed := 75.0
	fiveUsed := 15.0
	old := UsageSnapshot{ObservedAt: oldTime, SevenDay: UsageWindow{Utilization: &weekUsed, Reset: time.Now().Add(5 * 24 * time.Hour).UTC().Format(time.RFC3339)}}
	if err := m.saveUsage(ctx, list[0].ID, old); err != nil {
		t.Fatal(err)
	}
	m.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		return UsageSnapshot{FiveHour: UsageWindow{Utilization: &fiveUsed, Reset: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}, nil
	}
	got, err := m.Usage(ctx, list[0].ID)
	if err != nil || got.SevenDay.Utilization == nil || *got.SevenDay.Utilization != 75 || got.SevenDay.ObservedAt != oldTime || got.FiveHour.Utilization == nil || *got.FiveHour.Utilization != 15 {
		t.Fatal(got, err)
	}
}

type flakyUsageStore struct {
	vaultStore
	calls atomic.Int32
}

func (s *flakyUsageStore) SaveAccountUsage(ctx context.Context, id string, data []byte) error {
	if s.calls.Add(1) <= 2 {
		return errors.New("temporary store error")
	}
	return s.vaultStore.SaveAccountUsage(ctx, id, data)
}
func TestResponsePersistenceRetriesTransientWrites(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, _ := m.List(ctx)
	flaky := &flakyUsageStore{vaultStore: m.st}
	m.st = flaky
	h := http.Header{}
	h.Set("Anthropic-Ratelimit-Unified-7d-Utilization", "0.42")
	h.Set("Anthropic-Ratelimit-Unified-7d-Reset", time.Now().Add(5*24*time.Hour).UTC().Format(time.RFC3339))
	m.ObserveUsage(list[0].ID, h)
	deadline := time.Now().Add(2 * time.Second)
	for m.usagePersisting.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if m.usagePersisting.Load() || flaky.calls.Load() != 3 {
		t.Fatal("transient persistence not retried", flaky.calls.Load())
	}
	data, err := m.st.AccountUsage(ctx, list[0].ID)
	if err != nil || len(data) == 0 {
		t.Fatal("fallback not persisted", err)
	}
}
