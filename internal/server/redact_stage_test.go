package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/redact"
	"github.com/niclasedge/tollgate/internal/store"
)

// TestRedactStageLeaksNothing is the leak test of #13: with the redact stage
// on, no plaintext secret reaches the upstream, the database files or the
// logs, while the stored trace carries the redacted diff and the matches.
func TestRedactStageLeaksNothing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "leak.db")
	st, err := store.Open(context.Background(), dbPath, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.CreateClient(context.Background(), store.Client{ID: "client-1", Name: "c",
		KeyHash: bytes.Repeat([]byte{7}, 32), KeyHint: "tg_...cdef", Status: store.ClientActive}); err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	up := newUpstream(t, okJSON)
	ts := newProxy(t, up, proxyOpts{store: st, logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))},
		redact.NewStage(redact.Builtin()))

	mail := "jane.roe@mailhost.de"
	key := strings.Join([]string{"sk-ant-api03-", "abcdefghijklmnopqrstuvwxyz0123"}, "")
	pass := "s3cretpw"
	body := `{"model":"claude-test","stream":false,"messages":[` +
		`{"role":"user","content":"mail ` + mail + ` key ` + key + `"},` +
		`{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"Bash","input":{"command":"psql postgres://app:` + pass + `@db.prod.io/x"}}]}]}`
	if resp, b := doReq(t, http.MethodPost, ts.URL+"/v1/messages", body, auth(nil)); resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	tr := waitFinished(t, st)

	seen := up.seen()
	if len(seen) != 1 {
		t.Fatalf("upstream saw %d requests", len(seen))
	}
	for _, ph := range []string{"<redacted:email_1>", "<redacted:api_key_1>", "<redacted:connection_password_1>"} {
		if !bytes.Contains(seen[0].Body, []byte(ph)) {
			t.Errorf("upstream body lacks %s: %s", ph, seen[0].Body)
		}
	}

	var rs *store.Stage
	for i := range tr.Stages {
		if tr.Stages[i].Name == redact.StageName {
			rs = &tr.Stages[i]
		}
	}
	if rs == nil || len(rs.Matches) != 3 || !strings.Contains(rs.Diff, "<redacted:email_1>") {
		t.Fatalf("stored redact stage = %+v", rs)
	}

	var files []byte
	for _, suffix := range []string{"", "-wal", "-shm"} {
		b, err := os.ReadFile(dbPath + suffix)
		if err == nil {
			files = append(files, b...)
		}
	}
	if !bytes.Contains(files, []byte("<redacted:email_1>")) {
		t.Fatal("positive control: diff not found in the database files")
	}
	for _, secret := range []string{mail, key, pass} {
		if bytes.Contains(seen[0].Body, []byte(secret)) {
			t.Errorf("upstream body contains %q", secret)
		}
		if bytes.Contains(files, []byte(secret)) {
			t.Errorf("database files contain %q", secret)
		}
		if strings.Contains(logs.String(), secret) {
			t.Errorf("logs contain %q", secret)
		}
	}
}
