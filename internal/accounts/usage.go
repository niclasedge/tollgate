package accounts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/upstream"
)

const usageCacheTTL = 5 * time.Minute

type usageAttempt struct {
	at      time.Time
	message string
}

func usageWindow(w meter.Window) UsageWindow {
	out := UsageWindow{}
	if w.Utilization != nil {
		u := *w.Utilization * 100
		out.Utilization = &u
	}
	if w.Reset != nil {
		out.Reset = w.Reset.UTC().Format(time.RFC3339)
	}
	return out
}

// fetchSubscriptionUsage reads limits and the plan with the same in-memory
// token. A non-nil License means the profile read succeeded; the error is the
// usage error, or the profile 401 when both reads failed.
func fetchSubscriptionUsage(ctx context.Context, token string) (UsageSnapshot, error) {
	var out UsageSnapshot
	l, err := upstream.ReadUsage(ctx, token)
	if err == nil {
		out.FiveHour, out.SevenDay = usageWindow(l.FiveHour), usageWindow(l.SevenDay)
	}
	p, perr := upstream.ReadProfile(ctx, token)
	var status *upstream.UsageError
	switch {
	case perr == nil:
		out.License = &License{OrganizationType: p.OrganizationType, RateLimitTier: p.RateLimitTier, SubscriptionStatus: p.SubscriptionStatus}
	case err != nil && errors.As(perr, &status) && status.Status == 401:
		err = perr
	}
	return out, err
}

// nextLicense records the token state of one read attempt. Plan fields only
// change after a successful profile read and keep their own measurement time.
func nextLicense(old, profile *License, err error, expires string, now time.Time) *License {
	l := License{}
	if old != nil {
		l = *old
	}
	l.CheckedAt = now.UTC().Format(time.RFC3339Nano)
	if profile != nil {
		l.OrganizationType, l.RateLimitTier, l.SubscriptionStatus = profile.OrganizationType, profile.RateLimitTier, profile.SubscriptionStatus
		l.ProfileObservedAt = l.CheckedAt
	}
	var status *upstream.UsageError
	switch {
	case err == nil || profile != nil:
		l.Token = TokenValid
	case errors.As(err, &status) && status.Status == 401:
		exp, perr := time.Parse(time.RFC3339, expires)
		switch {
		case perr != nil:
			l.Token = TokenUnknown
		case exp.After(now):
			l.Token = TokenRelogin
		default:
			l.Token = TokenExpired
		}
	default:
		l.Token = TokenUnknown
	}
	return &l
}
func (m *Manager) saveUsage(ctx context.Context, id string, u UsageSnapshot) error {
	u.Error = ""
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return m.st.SaveAccountUsage(ctx, id, data)
}

// Usage reads each subscription independently; it never activates, refreshes or
// uploads credentials. The active credential may have rotated since its backup.
func (m *Manager) Usage(ctx context.Context, id string) (UsageSnapshot, error) {
	m.usageMu.Lock()
	defer m.usageMu.Unlock()
	// Resolve the ID even on a cache hit, so removed accounts cannot reappear.
	m.mu.Lock()
	records, creds, err := m.reconcile(ctx)
	if err != nil {
		m.mu.Unlock()
		return UsageSnapshot{}, err
	}
	a, err := findRecord(records, id)
	m.mu.Unlock()
	if err != nil {
		return UsageSnapshot{}, err
	}
	var cached UsageSnapshot
	data, err := m.st.AccountUsage(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return cached, errors.New("Usage-Messstand nicht lesbar")
	}
	if len(data) > 0 && json.Unmarshal(data, &cached) != nil {
		return UsageSnapshot{}, errors.New("Usage-Messstand ungültig")
	}
	cached = m.applyObservedUsage(ctx, id, cached)
	now := time.Now()
	at, _ := time.Parse(time.RFC3339, cached.ObservedAt)
	if !at.IsZero() && now.Sub(at) < usageCacheTTL {
		return cached, nil
	}
	if attempt, ok := m.usageAttempts[id]; ok && now.Sub(attempt.at) < usageCacheTTL {
		cached.Error = attempt.message
		return cached, nil
	}
	// Snapshot the right token under the switch mutex, then release it before HTTPS.
	m.mu.Lock()
	records, creds, err = m.reconcile(ctx)
	if err == nil {
		a, err = findRecord(records, id)
	}
	var token []byte
	if err == nil {
		if len(creds) == 1 && creds[0].Name == a.Filename {
			token, err = m.up.DownloadCredential(ctx, m.mgmtKey, a.Filename)
		} else {
			token, err = m.box.Unseal(a.ID, a.Cipher)
		}
	}
	var t tokenMetadata
	if err == nil {
		t, err = parseToken(token)
	}
	m.mu.Unlock()
	if err != nil {
		cached.Error = "Limits momentan nicht abrufbar"
		return cached, nil
	}
	fetch := m.fetchUsage
	if fetch == nil {
		fetch = fetchSubscriptionUsage
	}
	u, err := fetch(ctx, t.Access)
	if m.usageAttempts == nil {
		m.usageAttempts = make(map[string]usageAttempt)
	}
	message := ""
	license := nextLicense(cached.License, u.License, err, t.Expires, time.Now())
	if err != nil {
		message = "Limits momentan nicht abrufbar"
		var status *upstream.UsageError
		if errors.As(err, &status) {
			switch status.Status {
			case 401:
				message = "Usage-Zugriff abgelaufen · Account neu anmelden"
			case 403:
				message = "Usage-Zugriff nicht erlaubt"
			case 429:
				message = "Usage-Abfrage begrenzt · später erneut versuchen"
			}
		}
		cached.License = license
		_ = m.saveUsage(ctx, id, cached)
		cached.Error = message
	} else {
		u.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
		u.Error = ""
		u = mergeUsage(cached, u)
		u.License = license
		if m.saveUsage(ctx, id, u) != nil {
			message = "Messstand konnte nicht gespeichert werden"
			u.Error = message
		}
		cached = u
	}
	m.usageAttempts[id] = usageAttempt{at: now, message: message}
	cached = m.applyObservedUsage(ctx, id, cached)
	return cached, nil
}

// ObserveUsage never waits for storage or remote HTTP. Only validated numeric
// limits are kept, not response headers. The request snapshot supplies the ID.
func (m *Manager) ObserveUsage(id string, h http.Header) {
	if l, ok := meter.ParseLimits(h, time.Now()); ok {
		m.ObserveLimits(id, l)
	}
}
func (m *Manager) ObserveLimits(id string, l meter.Limits) {
	if id != "" && m.OnLimits != nil {
		m.OnLimits(id, l)
	}
	if id == "" || l.SevenDay.Utilization == nil || l.SevenDay.Reset == nil || l.ObservedAt.IsZero() {
		return
	}
	u := &UsageSnapshot{FiveHour: usageWindow(l.FiveHour), SevenDay: usageWindow(l.SevenDay), ObservedAt: l.ObservedAt.UTC().Format(time.RFC3339Nano)}
	for {
		old, ok := m.observedUsage.Load(id)
		if ok {
			at, _ := time.Parse(time.RFC3339Nano, old.(*UsageSnapshot).ObservedAt)
			if !l.ObservedAt.After(at) {
				return
			}
			if !m.observedUsage.CompareAndSwap(id, old, u) {
				continue
			}
		} else {
			if _, loaded := m.observedUsage.LoadOrStore(id, u); loaded {
				continue
			}
		}
		break
	}
	m.usagePending.Store(id, true)
	if m.usagePersisting.CompareAndSwap(false, true) {
		go m.persistObservedUsage()
	}
}

// Each window retains its own measurement time when an API returns only a
// partial result. A new 5h value must not make an old week look current.
func windowTime(w UsageWindow, fallback string) time.Time {
	if w.Utilization == nil || w.Reset == "" {
		return time.Time{}
	}
	stamp := w.ObservedAt
	if stamp == "" {
		stamp = fallback
	}
	at, _ := time.Parse(time.RFC3339Nano, stamp)
	return at
}
func mergeUsage(old, incoming UsageSnapshot) UsageSnapshot {
	merge := func(previous, next UsageWindow) UsageWindow {
		previousAt := windowTime(previous, old.ObservedAt)
		nextAt := windowTime(next, incoming.ObservedAt)
		if next.Utilization != nil && next.Reset != "" && (previous.Utilization == nil || previous.Reset == "" || nextAt.After(previousAt)) {
			next.ObservedAt = nextAt.UTC().Format(time.RFC3339Nano)
			return next
		}
		if previous.ObservedAt == "" && !previousAt.IsZero() {
			previous.ObservedAt = previousAt.UTC().Format(time.RFC3339Nano)
		}
		return previous
	}
	if incoming.License == nil {
		incoming.License = old.License
	}
	incoming.FiveHour = merge(old.FiveHour, incoming.FiveHour)
	incoming.SevenDay = merge(old.SevenDay, incoming.SevenDay)
	oldAt, _ := time.Parse(time.RFC3339Nano, old.ObservedAt)
	newAt, _ := time.Parse(time.RFC3339Nano, incoming.ObservedAt)
	if oldAt.After(newAt) {
		incoming.ObservedAt = old.ObservedAt
	}
	return incoming
}
func (m *Manager) applyObservedUsage(ctx context.Context, id string, cached UsageSnapshot) UsageSnapshot {
	value, ok := m.observedUsage.Load(id)
	if !ok {
		return cached
	}
	incoming := mergeUsage(cached, *value.(*UsageSnapshot))
	if !windowTime(incoming.SevenDay, incoming.ObservedAt).After(windowTime(cached.SevenDay, cached.ObservedAt)) && !windowTime(incoming.FiveHour, incoming.ObservedAt).After(windowTime(cached.FiveHour, cached.ObservedAt)) {
		return cached
	}
	if m.saveUsage(ctx, id, incoming) != nil {
		incoming.Error = "Messstand konnte nicht gespeichert werden"
	}
	return incoming
}

// One short-lived worker per manager coalesces concurrent observations. It
// exits after the pending set is empty; there is no persistent polling loop.
func (m *Manager) persistObservedUsage() {
	retries := map[string]int{}
	for {
		m.usagePending.Range(func(key, value any) bool {
			id := key.(string)
			m.usagePending.Delete(id)
			m.usageMu.Lock()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var cached UsageSnapshot
			data, err := m.st.AccountUsage(ctx, id)
			if err == nil {
				err = json.Unmarshal(data, &cached)
			}
			if err == nil || errors.Is(err, sql.ErrNoRows) {
				saved := m.applyObservedUsage(ctx, id, cached)
				if saved.Error != "" {
					err = errors.New("persist failed")
				} else {
					err = nil
				}
			}
			m.usageMu.Unlock()
			cancel()
			if err != nil {
				retries[id]++
				if retries[id] < 3 {
					m.usagePending.Store(id, true)
				}
			} else {
				delete(retries, id)
			}
			return true
		})
		m.usagePersisting.Store(false)
		pending := false
		m.usagePending.Range(func(_, _ any) bool { pending = true; return false })
		if !pending || !m.usagePersisting.CompareAndSwap(false, true) {
			return
		}
	}
}

// CachedUsage returns the last known usage of a subscription without asking
// upstream; zero when none is stored (#136).
func (m *Manager) CachedUsage(ctx context.Context, id string) (UsageSnapshot, error) {
	m.usageMu.Lock()
	defer m.usageMu.Unlock()
	var cached UsageSnapshot
	data, err := m.st.AccountUsage(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return cached, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cached); err != nil {
			return UsageSnapshot{}, err
		}
	}
	return m.applyObservedUsage(ctx, id, cached), nil
}
