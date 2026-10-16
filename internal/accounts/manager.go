package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/niclasedge/tollgate/internal/cryptobox"
	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/store"
	"github.com/niclasedge/tollgate/internal/upstream"
)

type vaultStore interface {
	ListAccounts(context.Context) ([]store.AccountRecord, error)
	SaveAccount(context.Context, store.AccountRecord) error
	DeleteAccount(context.Context, string) error
	AuditAccount(context.Context, string, string) error
	SaveAccountUsage(context.Context, string, []byte) error
	AccountUsage(context.Context, string) ([]byte, error)
}
type management interface {
	Credentials(context.Context, string) ([]upstream.Credential, error)
	PauseCredential(context.Context, string, string, bool) error
	DownloadCredential(context.Context, string, string) ([]byte, error)
	DeleteCredential(context.Context, string, string) error
	UploadCredential(context.Context, string, string, []byte) error
	TestAccount(context.Context, string) (upstream.AccountTest, error)
}
type Manager struct {
	observedUsage   sync.Map // immutable *UsageSnapshot by account ID
	usagePending    sync.Map
	usagePersisting atomic.Bool
	mu              sync.Mutex
	st              vaultStore
	box             *cryptobox.Box
	up              management
	mgmtKey, apiKey string
	gate            *Gate
	initialized     atomic.Bool
	OnSwitch        func()
	// OnLimits receives the parsed limits of every response with limit
	// headers and the account they belong to. It must not block (#122).
	OnLimits      func(id string, l meter.Limits)
	usageMu       sync.Mutex
	usageAttempts map[string]usageAttempt
	fetchUsage    func(context.Context, string) (UsageSnapshot, error)
}

func New(st vaultStore, box *cryptobox.Box, up management, mgmtKey, apiKey string, gate *Gate) *Manager {
	return &Manager{st: st, box: box, up: up, mgmtKey: mgmtKey, apiKey: apiKey, gate: gate}
}

type tokenMetadata struct {
	Type         string `json:"type"`
	Email        string `json:"email"`
	Access       string `json:"access_token"`
	Refresh      string `json:"refresh_token"`
	Organization string `json:"organization_uuid"`
	Expires      string `json:"expired"`
	LastRefresh  string `json:"last_refresh"`
}

func parseToken(b []byte) (tokenMetadata, error) {
	var t tokenMetadata
	if len(b) > 1<<20 || json.Unmarshal(b, &t) != nil || t.Type != "claude" || t.Email == "" || len(t.Email) > 320 || strings.ContainsAny(t.Email, "\r\n\x00") || t.Access == "" || t.Refresh == "" {
		return t, errors.New("Keine gültige CLIProxyAPI Claude-Token-Datei")
	}
	return t, nil
}
func tokenID(t tokenMetadata) string {
	hash := sha256.Sum256([]byte(strings.ToLower(t.Email) + "\x00" + t.Organization))
	return hex.EncodeToString(hash[:])
}
func validFilename(n string) bool {
	return n != "" && len(n) < 256 && !strings.ContainsAny(n, "/\\\x00\r\n") && strings.HasSuffix(n, ".json")
}
func public(a store.AccountRecord, t tokenMetadata, active bool, status string) Account {
	return Account{ID: a.ID, Name: a.Name, Email: a.Email, Active: active, Status: status, ExpiresAt: t.Expires, LastRefresh: t.LastRefresh, UpdatedAt: time.UnixMilli(a.UpdatedAt).UTC().Format(time.RFC3339)}
}
func (m *Manager) credentials(ctx context.Context) ([]upstream.Credential, error) {
	if m.mgmtKey == "" {
		return nil, errors.New("Account-Verwaltung benötigt den Management-Key")
	}
	c, err := m.up.Credentials(ctx, m.mgmtKey)
	if err != nil {
		return nil, errors.New("Accounts nicht verfügbar: CLIProxyAPI Management nicht erreichbar")
	}
	if len(c) > 1 {
		return nil, errors.New("Mehrere Dateien im auth-dir; bitte zuerst auf genau einen aktiven Account korrigieren")
	}
	if len(c) == 1 && (c[0].Provider != "claude" || !validFilename(c[0].Name)) {
		return nil, errors.New("Aktiver Account ist kein unterstützter Claude-Account")
	}
	return c, nil
}

// reconcile adopts an existing deployment without changing its active token.
func (m *Manager) reconcile(ctx context.Context) ([]store.AccountRecord, []upstream.Credential, error) {
	c, err := m.credentials(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !m.initialized.Load() {
		if len(c) == 1 {
			if err := m.up.PauseCredential(ctx, m.mgmtKey, c[0].Name, false); err != nil {
				return nil, nil, errors.New("Refresh-Barriere nicht verfügbar; bitte CPA-Image mit just setup neu bauen")
			}
		}
		m.initialized.Store(true)
	}
	records, err := m.st.ListAccounts(ctx)
	if err != nil {
		return nil, nil, errors.New("Account-Tresor nicht lesbar")
	}
	if len(c) == 1 {
		found := false
		for _, a := range records {
			if a.Filename == c[0].Name {
				found = true
				break
			}
		}
		if !found {
			b, err := m.up.DownloadCredential(ctx, m.mgmtKey, c[0].Name)
			if err != nil {
				return nil, nil, errors.New("Aktiver Account konnte nicht gesichert werden")
			}
			t, err := parseToken(b)
			if err != nil {
				return nil, nil, err
			}
			id := tokenID(t)
			name := t.Email
			for _, a := range records {
				if a.ID == id {
					name = a.Name
				}
			}
			_, err = m.save(ctx, id, c[0].Name, name, t, b)
			if err != nil {
				return nil, nil, err
			}
			records, err = m.st.ListAccounts(ctx)
			if err != nil {
				return nil, nil, errors.New("Account-Tresor nicht lesbar")
			}
		}
	}
	return records, c, nil
}
func (m *Manager) save(ctx context.Context, id, filename, name string, t tokenMetadata, b []byte) (store.AccountRecord, error) {
	cipher, err := m.box.Seal(id, b)
	if err != nil {
		return store.AccountRecord{}, err
	}
	a := store.AccountRecord{ID: id, Filename: filename, Name: name, Email: t.Email, Cipher: cipher, UpdatedAt: time.Now().UnixMilli()}
	if m.st.SaveAccount(ctx, a) != nil {
		return a, errors.New("Account konnte nicht verschlüsselt gespeichert werden")
	}
	return a, nil
}
func (m *Manager) List(ctx context.Context) ([]Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	records, c, err := m.reconcile(ctx)
	if err != nil {
		return nil, err
	}
	out := []Account{}
	for _, a := range records {
		b, err := m.box.Unseal(a.ID, a.Cipher)
		if err != nil {
			return nil, err
		}
		t, err := parseToken(b)
		if err != nil {
			return nil, err
		}
		active := len(c) == 1 && c[0].Name == a.Filename
		status := "gespeichert"
		if active {
			status = c[0].Status
		}
		out = append(out, public(a, t, active, status))
	}
	return out, nil
}
func (m *Manager) Import(ctx context.Context, name string, b []byte) (Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, err := parseToken(b)
	if err != nil {
		return Account{}, err
	}
	records, c, err := m.reconcile(ctx)
	if err != nil {
		return Account{}, err
	}
	id := tokenID(t)
	filename := "claude-" + id + ".json"
	for _, a := range records {
		if a.ID == id {
			if len(c) == 1 && c[0].Name == a.Filename {
				return Account{}, errors.New("Dieser Account ist bereits aktiv; bitte zuerst einen anderen aktivieren")
			}
			filename = a.Filename
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = t.Email
	}
	if len(name) > 120 || strings.ContainsAny(name, "\x00\r\n") {
		return Account{}, errors.New("Account-Name muss höchstens 120 Zeichen ohne Zeilenumbruch enthalten")
	}
	a, err := m.save(ctx, id, filename, name, t, b)
	return public(a, t, false, "gespeichert"), err
}
func findRecord(records []store.AccountRecord, id string) (store.AccountRecord, error) {
	for _, a := range records {
		if a.ID == id {
			return a, nil
		}
	}
	return store.AccountRecord{}, errors.New("Account nicht gefunden")
}
func (m *Manager) Remove(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	records, c, err := m.reconcile(ctx)
	if err != nil {
		return err
	}
	a, err := findRecord(records, id)
	if err != nil {
		return err
	}
	if len(c) == 1 && c[0].Name == a.Filename {
		return errors.New("Der aktive Account kann nicht entfernt werden")
	}
	if m.st.DeleteAccount(ctx, id) != nil {
		return errors.New("Account konnte nicht entfernt werden")
	}
	return nil
}
func (m *Manager) verify(ctx context.Context, name string, expectedEmail ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		c, err := m.credentials(ctx)
		if err != nil {
			return err
		}
		if name == "" && len(c) == 0 {
			return nil
		}
		if len(c) == 1 && c[0].Name == name && !c[0].Disabled && (len(expectedEmail) == 0 || c[0].Email == expectedEmail[0]) {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("Aktiver Account konnte nicht bestätigt werden")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func (m *Manager) restore(old *store.AccountRecord, oldToken []byte, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, err := m.credentials(ctx)
	if err != nil {
		return err
	}
	if len(c) == 1 {
		if old != nil && c[0].Name == old.Filename {
			return m.verify(ctx, old.Filename)
		}
		if c[0].Name != target {
			return errors.New("Unerwartete aktive Datei; Wiederherstellung angehalten")
		}
		if m.up.DeleteCredential(ctx, m.mgmtKey, target) != nil {
			return errors.New("Neue Datei konnte zur Wiederherstellung nicht entfernt werden")
		}
	}
	// An unsuccessful upload may have written a file before registration failed.
	// Its refresh was reserved-paused before upload, so removing that filename
	// also cleans such an orphan safely before restoring the previous file.
	if len(c) == 0 {
		if err := m.up.DeleteCredential(ctx, m.mgmtKey, target); err != nil {
			return errors.New("Unvollständige neue Datei konnte nicht entfernt werden")
		}
	}
	if err := m.verify(ctx, ""); err != nil {
		return err
	}
	if old == nil {
		return nil
	}
	if m.up.UploadCredential(ctx, m.mgmtKey, old.Filename, oldToken) != nil {
		return errors.New("Vorheriger Account konnte nicht wiederhergestellt werden; im Tresor gesichert")
	}
	return m.verify(ctx, old.Filename)
}

const cacheNotice = "Prompt-Cache startet beim Wechsel der Organisation neu."

func (m *Manager) Activate(ctx context.Context, id string) (result Activation, retErr error) {
	end, err := m.gate.WaitExclusive(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return Activation{}, errors.New("Vorgemerkter Account-Wechsel wurde abgebrochen")
		}
		return Activation{}, err
	}
	defer end()
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()
	records, c, err := m.reconcile(ctx)
	if err != nil {
		return Activation{}, err
	}
	// Resume the surviving active credential even after a canceled request or
	// rollback. The proxy gate remains held until this cleanup completes.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		current, err := m.credentials(cleanup)
		if err == nil && len(current) == 1 {
			err = m.up.PauseCredential(cleanup, m.mgmtKey, current[0].Name, false)
		}
		if err != nil {
			retErr = errors.New("Account-Zustand muss geprüft werden: Refresh konnte nicht freigegeben werden; bitte erneut aktivieren")
		}
	}()
	target, err := findRecord(records, id)
	if err != nil {
		return Activation{}, err
	}
	data, err := m.box.Unseal(id, target.Cipher)
	if err != nil {
		return Activation{}, err
	}
	t, err := parseToken(data)
	if err != nil {
		return Activation{}, err
	}
	if len(c) == 1 && c[0].Name == target.Filename {
		if err := m.up.PauseCredential(ctx, m.mgmtKey, target.Filename, false); err != nil {
			return Activation{}, errors.New("Account ist aktiv, Refresh konnte nicht freigegeben werden; bitte erneut aktivieren")
		}
		return Activation{Account: public(target, t, true, c[0].Status), Test: m.runTest(ctx, id), CacheNotice: cacheNotice}, nil
	}
	var old *store.AccountRecord
	var oldToken []byte
	if len(c) == 1 {
		for _, a := range records {
			if a.Filename == c[0].Name {
				copy := a
				old = &copy
			}
		}
		if old == nil {
			return Activation{}, errors.New("Bisheriger Account fehlt im Tresor")
		}
		if err = m.up.PauseCredential(ctx, m.mgmtKey, old.Filename, true); err != nil {
			return Activation{}, errors.New("Wechsel angehalten: laufender Token-Refresh konnte nicht sicher pausiert werden")
		}
		oldToken, err = m.up.DownloadCredential(ctx, m.mgmtKey, old.Filename)
		if err != nil {
			return Activation{}, errors.New("Aktueller Token konnte nicht zurückgesichert werden")
		}
		meta, err := parseToken(oldToken)
		if err != nil {
			return Activation{}, err
		}
		if tokenID(meta) != old.ID {
			return Activation{}, errors.New("Identität des aktiven Tokens hat sich geändert")
		}
		if _, err = m.save(ctx, old.ID, old.Filename, old.Name, meta, oldToken); err != nil {
			return Activation{}, err
		}
	}
	if m.st.AuditAccount(ctx, id, "started") != nil {
		return Activation{}, errors.New("Wechsel konnte nicht im Audit protokolliert werden")
	}
	mutationErr := m.up.PauseCredential(ctx, m.mgmtKey, target.Filename, true)
	if old != nil && mutationErr == nil {
		mutationErr = m.up.DeleteCredential(ctx, m.mgmtKey, old.Filename)
	}
	if mutationErr == nil {
		mutationErr = m.verify(ctx, "")
	}
	if mutationErr == nil {
		mutationErr = m.up.UploadCredential(ctx, m.mgmtKey, target.Filename, data)
	}
	if mutationErr == nil {
		mutationErr = m.verify(ctx, target.Filename, target.Email)
	}
	audit := func(result string) error {
		auditCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return m.st.AuditAccount(auditCtx, id, result)
	}
	if mutationErr != nil {
		restoreErr := m.restore(old, oldToken, target.Filename)
		result := "restored"
		if restoreErr != nil {
			result = "restore_failed"
		}
		auditErr := audit(result)
		if restoreErr != nil {
			return Activation{}, errors.New("Wechsel fehlgeschlagen; Wiederherstellung fehlgeschlagen. Tokens sind im Tresor gesichert; bitte erneut aktivieren")
		}
		if auditErr != nil {
			return Activation{}, errors.New("Wechsel fehlgeschlagen; vorheriger Account wiederhergestellt, Audit nicht verfügbar")
		}
		return Activation{}, errors.New("Wechsel fehlgeschlagen; vorheriger Zustand wurde wiederhergestellt")
	}
	if m.OnSwitch != nil {
		m.OnSwitch()
	}
	if audit("activated") != nil {
		return Activation{}, errors.New("Account aktiviert, aber Abschluss-Audit nicht verfügbar; bitte Liste aktualisieren")
	}
	if err := m.up.PauseCredential(ctx, m.mgmtKey, target.Filename, false); err != nil {
		return Activation{}, errors.New("Account aktiviert, aber Refresh konnte nicht freigegeben werden; bitte erneut aktivieren")
	}
	return Activation{Account: public(target, t, true, "active"), Test: m.runTest(ctx, id), CacheNotice: cacheNotice}, nil
}
func (m *Manager) runTest(ctx context.Context, id string) TestResult {
	r, err := m.up.TestAccount(ctx, m.apiKey)
	if r.Limits != nil {
		m.ObserveLimits(id, *r.Limits)
	}
	out := TestResult{OK: r.OK, Status: r.Status, Model: r.Model}
	if err != nil {
		out.Message = "Beispielaufruf fehlgeschlagen oder Zeitlimit überschritten"
	} else if r.FailureKind == "oauth_not_allowed" {
		out.Message = "Anthropic erlaubt OAuth-Nutzung für diese Organisation nicht (HTTP 403). Der Account ist gespeichert, kann aber derzeit keine Anfragen ausführen."
	} else if r.OK {
		out.Message = "Beispielaufruf erfolgreich: der aktive Account antwortet."
	} else {
		out.Message = fmt.Sprintf("Account aktiv, Beispielaufruf fehlgeschlagen (HTTP %d).", r.Status)
	}
	return out
}
func (m *Manager) Test(ctx context.Context, id string) (TestResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	end, ok := m.gate.Exclusive()
	if !ok {
		return TestResult{}, errors.New("Es läuft noch eine Anfrage; bitte danach erneut testen")
	}
	defer end()
	records, c, err := m.reconcile(ctx)
	if err != nil {
		return TestResult{}, err
	}
	a, err := findRecord(records, id)
	if err != nil {
		return TestResult{}, err
	}
	if len(c) != 1 || c[0].Name != a.Filename {
		return TestResult{}, errors.New("Nur der aktive Account kann getestet werden")
	}
	return m.runTest(ctx, id), nil
}

// Initialize resumes the surviving active credential after a gateway restart.
// It is retried before proxy forwarding if CPA was unavailable at startup.
func (m *Manager) Initialize(ctx context.Context) error {
	if m.initialized.Load() {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initialized.Load() {
		return nil
	}
	_, _, err := m.reconcile(ctx)
	return err
}

// ActiveID resolves the actual CPA credential. The caller must hold a request
// slot in Gate so activation cannot race this snapshot or its later forward.
func (m *Manager) ActiveID(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	records, creds, err := m.reconcile(ctx)
	if err != nil {
		return "", err
	}
	if len(creds) == 0 {
		return "", nil
	}
	for _, a := range records {
		if a.Filename == creds[0].Name {
			return a.ID, nil
		}
	}
	return "", errors.New("Aktiver Account konnte nicht zugeordnet werden")
}

func (m *Manager) SwitchStatus() SwitchStatus { return m.gate.Status() }

// ForceSwitch aborts the running requests of a reserved switch (#142).
func (m *Manager) ForceSwitch() (SwitchStatus, error) {
	if _, err := m.gate.Force(); err != nil {
		return SwitchStatus{}, err
	}
	return m.gate.Status(), nil
}
