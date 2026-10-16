package accounts

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/upstream"
)

func tokenWithExpiry(email, access string, expires time.Time) []byte {
	return []byte(`{"type":"claude","email":"` + email + `","access_token":"` + access + `","refresh_token":"refresh-` + access + `","expired":"` + expires.UTC().Format(time.RFC3339) + `"}`)
}

func TestLicenseFromProfileWithoutSwitchAndAfterRestart(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx := context.Background()
	if _, err := m.List(ctx); err != nil {
		t.Fatal(err)
	}
	b, err := m.Import(ctx, "Zweites Abo", tokenB)
	if err != nil {
		t.Fatal(err)
	}
	m.fetchUsage = func(_ context.Context, token string) (UsageSnapshot, error) {
		if token != "private-access-b" {
			t.Fatal("wrong token")
		}
		u := 30.0
		return UsageSnapshot{SevenDay: UsageWindow{Utilization: &u, Reset: time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)}, License: &License{OrganizationType: "claude_max", RateLimitTier: "default_claude_max_20x", SubscriptionStatus: "active"}}, nil
	}
	got, err := m.Usage(ctx, b.ID)
	if err != nil || got.License == nil {
		t.Fatal(got, err)
	}
	l := got.License
	if l.Token != TokenValid || l.OrganizationType != "claude_max" || l.RateLimitTier != "default_claude_max_20x" || l.CheckedAt == "" || l.ProfileObservedAt == "" {
		t.Fatalf("%+v", l)
	}
	if len(f.files) != 1 || f.files["claude-a.json"] == nil || f.tests != 0 {
		t.Fatal("license read changed the active account")
	}
	data, _ := json.Marshal(got)
	if strings.Contains(string(data), "private-access") || strings.Contains(string(data), "refresh") {
		t.Fatal("token leaked into snapshot")
	}
	restarted := New(m.st, m.box, f, "mgmt", "api", new(Gate))
	restarted.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		t.Fatal("persistent cache ignored")
		return UsageSnapshot{}, nil
	}
	cached, err := restarted.Usage(ctx, b.ID)
	if err != nil || cached.License == nil || cached.License.OrganizationType != "claude_max" || cached.License.Token != TokenValid {
		t.Fatal(cached, err)
	}
}

func TestLicenseTokenStateFromUnauthorizedReads(t *testing.T) {
	for _, tc := range []struct {
		name    string
		expires time.Time
		want    string
	}{
		{"token not yet expired", time.Now().Add(6 * time.Hour), TokenRelogin},
		{"access token expired", time.Now().Add(-time.Hour), TokenExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _ := managerEnv(t)
			ctx := context.Background()
			if _, err := m.List(ctx); err != nil {
				t.Fatal(err)
			}
			b, err := m.Import(ctx, "Zweites Abo", tokenWithExpiry("b@example.com", "access-b", tc.expires))
			if err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
			if err := m.saveUsage(ctx, b.ID, UsageSnapshot{ObservedAt: old, License: &License{Token: TokenValid, CheckedAt: old, OrganizationType: "claude_pro", ProfileObservedAt: old}}); err != nil {
				t.Fatal(err)
			}
			m.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
				return UsageSnapshot{}, &upstream.UsageError{Status: 401}
			}
			got, err := m.Usage(ctx, b.ID)
			if err != nil || got.License == nil {
				t.Fatal(got, err)
			}
			if got.License.Token != tc.want || got.License.OrganizationType != "claude_pro" || got.License.ProfileObservedAt != old || got.License.CheckedAt == old {
				t.Fatalf("%+v", got.License)
			}
			restarted := New(m.st, m.box, m.up, "mgmt", "api", new(Gate))
			restarted.fetchUsage = m.fetchUsage
			again, _ := restarted.Usage(ctx, b.ID)
			if again.License == nil || again.License.Token != tc.want {
				t.Fatal("token state not persisted", again.License)
			}
		})
	}
}

func TestLicenseValidWhenUsageDeniedButProfileReadable(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		return UsageSnapshot{License: &License{OrganizationType: "claude_pro"}}, &upstream.UsageError{Status: 403}
	}
	got, err := m.Usage(ctx, list[0].ID)
	if err != nil || got.License == nil || got.License.Token != TokenValid || got.License.OrganizationType != "claude_pro" || got.Error != "Usage-Zugriff nicht erlaubt" {
		t.Fatal(got, err)
	}
}

func TestLicenseUnknownOnOtherFailures(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, _ := m.List(ctx)
	m.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		return UsageSnapshot{}, &upstream.UsageError{Status: 500}
	}
	got, _ := m.Usage(ctx, list[0].ID)
	if got.License == nil || got.License.Token != TokenUnknown {
		t.Fatal(got.License)
	}
}

func TestLicenseSurvivesHeaderObservations(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	list, _ := m.List(ctx)
	now := time.Now().UTC()
	u := 0.5
	if err := m.saveUsage(ctx, list[0].ID, UsageSnapshot{SevenDay: UsageWindow{Utilization: &u, Reset: now.Add(time.Hour).Format(time.RFC3339)}, ObservedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), License: &License{Token: TokenValid, OrganizationType: "claude_max"}}); err != nil {
		t.Fatal(err)
	}
	reset := now.Add(2 * time.Hour)
	share := 0.6
	m.ObserveLimits(list[0].ID, meter.Limits{SevenDay: meter.Window{Utilization: &share, Reset: &reset}, ObservedAt: now})
	m.fetchUsage = func(context.Context, string) (UsageSnapshot, error) {
		t.Fatal("cache ignored")
		return UsageSnapshot{}, nil
	}
	got, err := m.Usage(ctx, list[0].ID)
	if err != nil || got.SevenDay.Utilization == nil || *got.SevenDay.Utilization != 60 {
		t.Fatal(got, err)
	}
	if got.License == nil || got.License.OrganizationType != "claude_max" {
		t.Fatal("header observation dropped the license", got.License)
	}
}
