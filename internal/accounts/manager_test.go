package accounts

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/cryptobox"
	"github.com/niclasedge/tollgate/internal/store"
	"github.com/niclasedge/tollgate/internal/upstream"
)

var tokenA = []byte(`{"type":"claude","email":"a@example.com","access_token":"private-access-a","refresh_token":"private-refresh-a"}`)
var tokenB = []byte(`{"type":"claude","email":"b@example.com","access_token":"private-access-b","refresh_token":"private-refresh-b"}`)

type fakeCPA struct {
	files    map[string][]byte
	failNext bool
	tests    int
	maxFiles int
	paused   map[string]bool
	onPause  func(string, bool) error
}

func (f *fakeCPA) Credentials(context.Context, string) ([]upstream.Credential, error) {
	var c []upstream.Credential
	for n, b := range f.files {
		t, _ := parseToken(b)
		c = append(c, upstream.Credential{Name: n, Provider: "claude", Email: t.Email, Status: "active"})
	}
	return c, nil
}
func (f *fakeCPA) DownloadCredential(_ context.Context, _ string, n string) ([]byte, error) {
	b, ok := f.files[n]
	if !ok {
		return nil, errors.New("missing")
	}
	return b, nil
}
func (f *fakeCPA) DeleteCredential(_ context.Context, _ string, n string) error {
	delete(f.files, n)
	return nil
}
func (f *fakeCPA) UploadCredential(_ context.Context, _ string, n string, b []byte) error {
	if f.failNext {
		f.failNext = false
		return errors.New("upload failed")
	}
	f.files[n] = bytes.Clone(b)
	f.maxFiles = max(f.maxFiles, len(f.files))
	return nil
}
func (f *fakeCPA) TestAccount(context.Context, string) (upstream.AccountTest, error) {
	f.tests++
	return upstream.AccountTest{OK: true, Status: 200, Model: "claude-sonnet-5-5"}, nil
}
func managerEnv(t *testing.T) (*Manager, *fakeCPA, string) {
	t.Helper()
	dir := t.TempDir()
	db := filepath.Join(dir, "test.db")
	st, err := store.Open(context.Background(), db, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	box, err := cryptobox.Open(filepath.Join(dir, "crypto.key"), true)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeCPA{files: map[string][]byte{"claude-a.json": bytes.Clone(tokenA)}}
	m := New(st, box, f, "mgmt", "api", new(Gate))
	return m, f, db
}
func TestSwitchSavesRotatedTokenAndUsesOnlyOneFile(t *testing.T) {
	m, f, db := managerEnv(t)
	ctx := context.Background()
	a, err := m.List(ctx)
	if err != nil || len(a) != 1 || !a[0].Active {
		t.Fatal(a, err)
	}
	b, err := m.Import(ctx, "Second", tokenB)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.files) != 1 {
		t.Fatal("import changed active account")
	}
	rotated := bytes.ReplaceAll(tokenA, []byte("private-refresh-a"), []byte("rotated-refresh-a"))
	f.files["claude-a.json"] = rotated
	result, err := m.Activate(ctx, b.ID)
	if err != nil || !result.Test.OK {
		t.Fatal(result, err)
	}
	if len(f.files) != 1 || f.maxFiles > 1 || f.tests != 1 {
		t.Fatal("invariant", f.files, f.tests)
	}
	_, err = m.Activate(ctx, a[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(f.files["claude-a.json"], []byte("rotated-refresh-a")) {
		t.Fatal("rotation lost")
	}
	for _, path := range []string{db, db + "-wal"} {
		data, _ := os.ReadFile(path)
		for _, secret := range []string{"private-access", "private-refresh", "rotated-refresh"} {
			if bytes.Contains(data, []byte(secret)) {
				t.Fatal("DB plaintext", path)
			}
		}
	}
}
func TestFailedUploadRestoresAndCanceledWaitPreventsMutation(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx := context.Background()
	a, _ := m.List(ctx)
	b, err := m.Import(ctx, "Second", tokenB)
	if err != nil {
		t.Fatal(err)
	}
	done, _ := m.gate.Begin()
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	if _, err = m.Activate(waitCtx, b.ID); err == nil {
		t.Fatal("canceled wait ignored")
	}
	done()
	if !bytes.Equal(f.files["claude-a.json"], tokenA) {
		t.Fatal("busy switch changed file")
	}
	f.failNext = true
	if _, err = m.Activate(ctx, b.ID); err == nil {
		t.Fatal("upload failure ignored")
	}
	if len(f.files) != 1 || !bytes.Equal(f.files["claude-a.json"], tokenA) {
		t.Fatal("rollback failed")
	}
	list, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list {
		if item.ID == a[0].ID && !item.Active {
			t.Fatal("old account not active")
		}
	}
}
func TestRejectMultipleAndNeverDeleteActiveAccount(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx := context.Background()
	a, _ := m.List(ctx)
	if err := m.Remove(ctx, a[0].ID); err == nil {
		t.Fatal("deleted active")
	}
	f.files["other.json"] = tokenB
	if _, err := m.List(ctx); err == nil {
		t.Fatal("multiple credentials accepted")
	}
}

func (f *fakeCPA) PauseCredential(_ context.Context, _ string, name string, paused bool) error {
	if f.onPause != nil {
		if err := f.onPause(name, paused); err != nil {
			return err
		}
	}
	if f.paused == nil {
		f.paused = map[string]bool{}
	}
	f.paused[name] = paused
	return nil
}

func TestRestartResumesWithoutOpeningAccountUI(t *testing.T) {
	m, f, _ := managerEnv(t)
	f.paused = map[string]bool{"claude-a.json": true}
	if err := m.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.paused["claude-a.json"] {
		t.Fatal("surviving account still paused")
	}
}
func TestPauseWaitsForRotatedSnapshotBeforeDeleting(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx := context.Background()
	a, _ := m.List(ctx)
	b, _ := m.Import(ctx, "Second", tokenB)
	f.onPause = func(name string, paused bool) error {
		if name == "claude-a.json" && paused {
			f.files[name] = bytes.ReplaceAll(tokenA, []byte("private-refresh-a"), []byte("last-background-rotation"))
		}
		return nil
	}
	if _, err := m.Activate(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	f.onPause = nil
	if _, err := m.Activate(ctx, a[0].ID); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(f.files["claude-a.json"], []byte("last-background-rotation")) {
		t.Fatal("pre-barrier token was saved")
	}
}
func TestFailedResumeKeepsVerifiedTargetAndToken(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx := context.Background()
	m.List(ctx)
	b, _ := m.Import(ctx, "Second", tokenB)
	f.onPause = func(name string, paused bool) error {
		if name != "claude-a.json" && !paused {
			return errors.New("ambiguous resume failure")
		}
		return nil
	}
	if _, err := m.Activate(ctx, b.ID); err == nil {
		t.Fatal("resume failure not reported")
	}
	if len(f.files) != 1 {
		t.Fatal("incorrect file count")
	}
	for name, data := range f.files {
		if name == "claude-a.json" || !bytes.Equal(data, tokenB) {
			t.Fatal("verified target rolled back after resume")
		}
	}
}
