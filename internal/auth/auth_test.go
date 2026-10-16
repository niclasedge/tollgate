package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/auth"
	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/store"
)

const adminToken = "test-admin-token-0123456789abcdef"

type env struct {
	st     *store.Store
	dbPath string
	api    http.Handler
	ver    pipeline.KeyVerifier
	logs   *bytes.Buffer
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tollgate.db")
	st, err := store.Open(context.Background(), path, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return &env{st: st, dbPath: path, logs: logs,
		api: auth.NewAdminAPI(st, adminToken, logger),
		ver: auth.NewVerifier(st)}
}

func (e *env) do(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.api.ServeHTTP(rec, req)
	return rec
}

func (e *env) create(t *testing.T, name string) auth.CreateClientResponse {
	t.Helper()
	rec := e.do(t, "POST", "/api/clients", adminToken, `{"name":"`+name+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var r auth.CreateClientResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestGenerateKeyFormat(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		k, err := auth.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(k, "tg_") || len(k) != 46 {
			t.Fatalf("bad key shape: len=%d", len(k))
		}
		if seen[k] {
			t.Fatal("duplicate key")
		}
		seen[k] = true
	}
	if got := auth.KeyHint("tg_" + strings.Repeat("a", 39) + "wxyz"); got != "tg_…wxyz" {
		t.Fatalf("hint = %q", got)
	}
}

func TestVerifyLifecycle(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c := e.create(t, "laptop")

	id, err := e.ver.Verify(ctx, c.Key)
	if err != nil || id != c.Client.ID {
		t.Fatalf("verify fresh key: id=%q err=%v", id, err)
	}

	// Blocked: rejected immediately.
	if rec := e.do(t, "POST", "/api/clients/"+c.Client.ID+"/block", adminToken, ""); rec.Code != 200 {
		t.Fatalf("block: %d %s", rec.Code, rec.Body)
	}
	if _, err := e.ver.Verify(ctx, c.Key); !errors.Is(err, pipeline.ErrInvalidKey) {
		t.Fatalf("blocked key must be rejected, got %v", err)
	}

	// Rotated: old key rejected immediately, new key accepted.
	c2 := e.create(t, "desktop")
	rec := e.do(t, "POST", "/api/clients/"+c2.Client.ID+"/rotate", adminToken, "")
	if rec.Code != 200 {
		t.Fatalf("rotate: %d %s", rec.Code, rec.Body)
	}
	var rot auth.CreateClientResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &rot)
	if rot.Key == "" || rot.Key == c2.Key {
		t.Fatal("rotate must return a new key")
	}
	if _, err := e.ver.Verify(ctx, c2.Key); !errors.Is(err, pipeline.ErrInvalidKey) {
		t.Fatalf("old key after rotate must be rejected, got %v", err)
	}
	if id, err := e.ver.Verify(ctx, rot.Key); err != nil || id != c2.Client.ID {
		t.Fatalf("new key: id=%q err=%v", id, err)
	}
	if rot.Client.RotatedAt == nil {
		t.Fatal("rotated_at not set")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	e := newEnv(t)
	for _, k := range []string{"", "tg_", "x", "tg_short", strings.Repeat("a", 46), "tg_" + strings.Repeat("A", 43), "tg_" + strings.Repeat("!", 43)} {
		if _, err := e.ver.Verify(context.Background(), k); !errors.Is(err, pipeline.ErrInvalidKey) {
			t.Errorf("key %q: want ErrInvalidKey, got %v", k, err)
		}
	}
}

func TestAdminAuth(t *testing.T) {
	e := newEnv(t)
	for _, tc := range []struct{ name, token string }{{"none", ""}, {"wrong", "nope"}, {"prefix", adminToken[:10]}, {"longer", adminToken + "x"}} {
		rec := e.do(t, "GET", "/api/clients", tc.token, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: code %d", tc.name, rec.Code)
		}
	}
	// A client key must not open the admin API (strict separation).
	c := e.create(t, "x")
	if rec := e.do(t, "GET", "/api/clients", c.Key, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("client key on admin API: %d", rec.Code)
	}
	if rec := e.do(t, "GET", "/api/clients", adminToken, ""); rec.Code != http.StatusOK {
		t.Errorf("valid admin token: %d", rec.Code)
	}
}

func TestAdminAPIErrors(t *testing.T) {
	e := newEnv(t)
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/clients", `{"name":""}`, 400},
		{"POST", "/api/clients", `{"name":"   "}`, 400},
		{"POST", "/api/clients", `not json`, 400},
		{"POST", "/api/clients", `{"name":"a","extra":1}`, 400},
		{"POST", "/api/clients", `{"name":"a"}{"name":"b"}`, 400},
		{"POST", "/api/clients", `{"name":"` + strings.Repeat("a", 5000) + `"}`, 413},
		{"POST", "/api/clients/unknown/block", "", 404},
		{"POST", "/api/clients/unknown/rotate", "", 404},
		{"POST", "/api/clients/bad%20id/block", "", 400},
		{"DELETE", "/api/clients", "", 405},
	} {
		if rec := e.do(t, tc.method, tc.path, adminToken, tc.body); rec.Code != tc.want {
			t.Errorf("%s %s %q: got %d want %d (%s)", tc.method, tc.path, tc.body, rec.Code, tc.want, rec.Body)
		}
	}
}

func TestListHasNoKey(t *testing.T) {
	e := newEnv(t)
	c := e.create(t, "a")
	rec := e.do(t, "GET", "/api/clients", adminToken, "")
	if strings.Contains(rec.Body.String(), c.Key) || strings.Contains(rec.Body.String(), "key_hash") {
		t.Fatalf("list leaks key material: %s", rec.Body)
	}
	var l auth.ListClientsResponse
	// The reserved speedtest client (#87) is hidden: only "a" is listed.
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil || len(l.Clients) != 1 || strings.Contains(rec.Body.String(), "speedtest") {
		t.Fatalf("list: %v %+v", err, l)
	}
	if l.Clients[0].KeyHint != auth.KeyHint(c.Key) || l.Clients[0].Status != "aktiv" {
		t.Fatalf("list entry: %+v", l.Clients[0])
	}
}

func TestNoPlaintextKeyInDBOrLogs(t *testing.T) {
	e := newEnv(t)
	c := e.create(t, "a")
	rot := e.do(t, "POST", "/api/clients/"+c.Client.ID+"/rotate", adminToken, "")
	var r auth.CreateClientResponse
	_ = json.Unmarshal(rot.Body.Bytes(), &r)
	e.do(t, "POST", "/api/clients/"+c.Client.ID+"/block", adminToken, "")
	_, _ = e.ver.Verify(context.Background(), r.Key)
	e.do(t, "GET", "/api/clients", "wrong-token", "")
	e.do(t, "GET", "/api/clients", adminToken, "")

	secrets := []string{c.Key, r.Key, adminToken}
	if err := e.st.Close(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(e.dbPath + "*")
	if len(files) == 0 {
		t.Fatal("no db files")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range secrets {
			if bytes.Contains(b, []byte(s)) {
				t.Errorf("secret found in %s", filepath.Base(f))
			}
			// the key body without prefix must not appear either
			if bytes.Contains(b, []byte(strings.TrimPrefix(s, "tg_"))) && strings.HasPrefix(s, "tg_") {
				t.Errorf("key body found in %s", filepath.Base(f))
			}
		}
	}
	for _, s := range secrets {
		if strings.Contains(e.logs.String(), s) {
			t.Errorf("secret in logs")
		}
	}
}

func TestLoadOrCreateAdminToken(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "admin.token")
	tok, err := auth.LoadOrCreateAdminToken(p)
	if err != nil || len(tok) < 32 {
		t.Fatalf("create: %q %v", tok, err)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v", fi.Mode().Perm())
	}
	again, err := auth.LoadOrCreateAdminToken(p)
	if err != nil || again != tok {
		t.Fatalf("reload: %q %v", again, err)
	}
	if err := os.WriteFile(p, []byte("short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.LoadOrCreateAdminToken(p); err == nil {
		t.Fatal("short token must be rejected")
	}
}

func TestAdminTokenFileMustBePrivateRegularFile(t *testing.T) {
	dir := t.TempDir()
	loose := filepath.Join(dir, "loose.token")
	if err := os.WriteFile(loose, []byte(strings.Repeat("x", 40)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(loose, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.LoadOrCreateAdminToken(loose); err == nil {
		t.Error("group/world readable token file accepted")
	}
	link := filepath.Join(dir, "link.token")
	if err := os.Chmod(loose, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(loose, link); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.LoadOrCreateAdminToken(link); err == nil {
		t.Error("symlinked token file accepted")
	}
}

func TestEmptyAdminTokenNeverMatches(t *testing.T) {
	h := auth.RequireAdmin("", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/api/clients", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty token: %d", rec.Code)
	}
}

func TestVerifierInternalError(t *testing.T) {
	e := newEnv(t)
	c := e.create(t, "a")
	_ = e.st.Close()
	_, err := e.ver.Verify(context.Background(), c.Key)
	if err == nil || errors.Is(err, pipeline.ErrInvalidKey) {
		t.Fatalf("store failure must be an internal error, got %v", err)
	}
}

func FuzzVerifyNeverPanics(f *testing.F) {
	f.Add("tg_")
	f.Add("")
	f.Add("tg_" + strings.Repeat("a", 43))
	dir := f.TempDir()
	st, err := store.Open(context.Background(), filepath.Join(dir, "f.db"), store.Options{})
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() { _ = st.Close() })
	v := auth.NewVerifier(st)
	f.Fuzz(func(t *testing.T, k string) {
		if _, err := v.Verify(context.Background(), k); err != nil && !errors.Is(err, pipeline.ErrInvalidKey) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// The reserved speedtest client (#87) can neither be blocked, rotated nor
// created through the admin API.
func TestSpeedtestClientIsNotManageable(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/api/clients/speedtest/block", "/api/clients/speedtest/rotate"} {
		if rec := e.do(t, "POST", path, adminToken, ""); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
	if rec := e.do(t, "POST", "/api/clients", adminToken, `{"name":"tollgate:speedtest"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("reserved name: %d %s", rec.Code, rec.Body)
	}
}
