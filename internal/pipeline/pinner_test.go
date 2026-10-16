package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "p.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.CreateClient(context.Background(), store.Client{ID: "client-a", Name: "a",
		KeyHash: bytes.Repeat([]byte{7}, 32), KeyHint: "tg_...0000", Status: store.ClientActive}); err != nil {
		t.Fatal(err)
	}
	return st
}

func activate(t *testing.T, st *store.Store) int64 {
	t.Helper()
	ctx := context.Background()
	v, err := st.CreateConfigVersion(ctx, store.NewConfigVersion{Author: "test", Content: []byte("version: 1")})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetActiveConfigVersion(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func sessionKey(s string) string {
	h := sha256.Sum256([]byte("client-a\x00" + s))
	return hex.EncodeToString(h[:])
}

func TestSessionPinner(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	p := &SessionPinner{Store: st}

	// No active version: builtin (0), nothing written.
	if v, err := p.Pin(ctx, "client-a", sessionKey("s1")); err != nil || v != 0 {
		t.Fatalf("no active version: %d, %v", v, err)
	}
	if n, err := st.PurgeSessions(ctx, farFuture()); err != nil || n != 0 {
		t.Fatalf("sessions written without active version: %d, %v", n, err)
	}

	v1 := activate(t, st)
	if v, err := p.Pin(ctx, "client-a", sessionKey("s1")); err != nil || v != v1 {
		t.Fatalf("first pin: %d, %v", v, err)
	}
	v2 := activate(t, st)
	if v, err := p.Pin(ctx, "client-a", sessionKey("s1")); err != nil || v != v1 {
		t.Errorf("running session moved to %d (want %d), %v", v, v1, err)
	}
	if v, err := p.Pin(ctx, "client-a", sessionKey("s2")); err != nil || v != v2 {
		t.Errorf("new session: %d (want %d), %v", v, v2, err)
	}
	if v, err := p.Pin(ctx, "client-a", ""); err != nil || v != v2 {
		t.Errorf("client without session: %d (want %d), %v", v, v2, err)
	}
	if n, err := st.PurgeSessions(ctx, farFuture()); err != nil || n != 2 {
		t.Errorf("sessions stored = %d, want 2 (%v)", n, err)
	}
	if _, err := p.Pin(ctx, "client-a", "not-hex"); err == nil {
		t.Error("invalid session key accepted")
	}
}

func farFuture() time.Time { return time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC) }
