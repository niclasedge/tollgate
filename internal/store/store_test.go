package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

var t0 = time.UnixMilli(1_760_000_000_000)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func openTest(t *testing.T) (*Store, *fakeClock, string) {
	t.Helper()
	clk := &fakeClock{now: t0}
	path := filepath.Join(t.TempDir(), "tollgate.db")
	s, err := Open(context.Background(), path, Options{Now: clk.Now})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, clk, path
}

func hash(seed string) []byte {
	h := sha256.Sum256([]byte(seed))
	return h[:]
}

func hexHash(seed string) string { return hex.EncodeToString(hash(seed)) }

func mustClient(t *testing.T, s *Store, id string) Client {
	t.Helper()
	c, err := s.CreateClient(context.Background(), Client{
		ID: id, Name: "name-" + id, KeyHash: hash("key-" + id), KeyHint: "tg_Ab…wxyz", Status: ClientActive,
	})
	if err != nil {
		t.Fatalf("CreateClient: %v", err)
	}
	return c
}

func mustConfig(t *testing.T, s *Store, comment string) ConfigVersion {
	t.Helper()
	v, err := s.CreateConfigVersion(context.Background(), NewConfigVersion{
		Author: "admin", Comment: comment, Content: []byte("version: 1\n"), WordsEnc: []byte{1, 2, 3}, Diff: "+x",
	})
	if err != nil {
		t.Fatalf("CreateConfigVersion: %v", err)
	}
	return v
}

func schemaDump(t *testing.T, s *Store) []string {
	t.Helper()
	rows, err := s.r.Query(`SELECT type || ':' || name || ':' || COALESCE(sql, '') FROM sqlite_master ORDER BY type, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			t.Fatal(err)
		}
		out = append(out, l)
	}
	return out
}

func TestMigrationsIdempotentAndDeterministic(t *testing.T) {
	ctx := context.Background()
	s1, _, path := openTest(t)
	before := schemaDump(t, s1)
	var n int
	if err := s1.r.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n == 0 {
		t.Fatalf("schema_migrations count=%d err=%v", n, err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()
	var n2 int
	if err := s2.r.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n2); err != nil || n2 != n {
		t.Fatalf("second open changed migrations: %d -> %d (%v)", n, n2, err)
	}
	if got := schemaDump(t, s2); !reflect.DeepEqual(before, got) {
		t.Fatalf("schema differs after reopen")
	}

	s3, err := Open(ctx, filepath.Join(t.TempDir(), "other.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close()
	if got := schemaDump(t, s3); !reflect.DeepEqual(before, got) {
		t.Fatalf("two fresh databases have different schemas")
	}
}

func TestMigrationChecksumMismatch(t *testing.T) {
	s, _, path := openTest(t)
	if _, err := s.w.Exec(`UPDATE schema_migrations SET checksum = 'deadbeef' WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	_, err := Open(context.Background(), path, Options{})
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("want checksum error, got %v", err)
	}
}

func TestParseMigrations(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{"ok", map[string]string{"0001_a.sql": "x", "0002_b.sql": "y"}, ""},
		{"gap", map[string]string{"0001_a.sql": "x", "0003_b.sql": "y"}, "gap"},
		{"duplicate", map[string]string{"0001_a.sql": "x", "0001_b.sql": "y"}, "duplicate"},
		{"bad name", map[string]string{"init.sql": "x"}, "name"},
		{"empty", map[string]string{}, "no migrations"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMigrations(tc.files)
			if tc.wantErr == "" {
				if err != nil || len(got) != len(tc.files) {
					t.Fatalf("got %v, %v", got, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestOpenRejectsMemoryAndEmptyPath(t *testing.T) {
	for _, p := range []string{"", ":memory:", "file::memory:?cache=shared"} {
		if _, err := Open(context.Background(), p, Options{}); err == nil {
			t.Errorf("Open(%q) should fail", p)
		}
	}
}

func TestWALMode(t *testing.T) {
	s, _, _ := openTest(t)
	var mode string
	if err := s.r.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode=%q err=%v", mode, err)
	}
	if _, err := s.r.Exec(`DELETE FROM clients`); err == nil {
		t.Fatal("read pool must be query_only")
	}
}

func TestClients(t *testing.T) {
	ctx := context.Background()
	long := "tg_" + strings.Repeat("A", 43)
	tests := []struct {
		name    string
		c       Client
		wantErr error
	}{
		{"ok", Client{ID: "c1", Name: "laptop", KeyHash: hash("a"), KeyHint: "tg_Ab…wxyz", Status: ClientActive}, nil},
		{"plaintext key as hint", Client{ID: "c1", Name: "laptop", KeyHash: hash("a"), KeyHint: long, Status: ClientActive}, ErrInvalid},
		{"plaintext key as hash", Client{ID: "c1", Name: "laptop", KeyHash: []byte(long), KeyHint: "x", Status: ClientActive}, ErrInvalid},
		{"empty id", Client{Name: "laptop", KeyHash: hash("a"), KeyHint: "x", Status: ClientActive}, ErrInvalid},
		{"bad status", Client{ID: "c1", Name: "laptop", KeyHash: hash("a"), KeyHint: "x", Status: "weird"}, ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := openTest(t)
			got, err := s.CreateClient(ctx, tc.c)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if !got.CreatedAt.Equal(t0) {
				t.Errorf("CreatedAt=%v", got.CreatedAt)
			}
			found, err := s.ClientByKeyHash(ctx, tc.c.KeyHash)
			if err != nil || found.ID != tc.c.ID || found.Status != ClientActive || found.RotatedAt != nil {
				t.Fatalf("lookup: %+v %v", found, err)
			}
		})
	}

	t.Run("conflicts and not found", func(t *testing.T) {
		s, _, _ := openTest(t)
		mustClient(t, s, "a")
		_, err := s.CreateClient(ctx, Client{ID: "a", Name: "other", KeyHash: hash("z"), KeyHint: "x", Status: ClientActive})
		if !errors.Is(err, ErrConflict) {
			t.Errorf("duplicate id: %v", err)
		}
		_, err = s.CreateClient(ctx, Client{ID: "b", Name: "name-a", KeyHash: hash("z"), KeyHint: "x", Status: ClientActive})
		if !errors.Is(err, ErrConflict) {
			t.Errorf("duplicate name: %v", err)
		}
		if _, err := s.ClientByKeyHash(ctx, hash("unknown")); !errors.Is(err, ErrNotFound) {
			t.Errorf("unknown hash: %v", err)
		}
		if err := s.SetClientStatus(ctx, "nope", ClientBlocked); !errors.Is(err, ErrNotFound) {
			t.Errorf("status unknown client: %v", err)
		}
	})

	t.Run("block and rotate", func(t *testing.T) {
		s, clk, _ := openTest(t)
		c := mustClient(t, s, "a")
		if err := s.SetClientStatus(ctx, "a", ClientBlocked); err != nil {
			t.Fatal(err)
		}
		got, _ := s.ClientByKeyHash(ctx, c.KeyHash)
		if got.Status != ClientBlocked {
			t.Errorf("status=%s", got.Status)
		}
		clk.now = t0.Add(time.Hour)
		if err := s.RotateClientKey(ctx, "a", hash("new"), "tg_Zz…9999"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ClientByKeyHash(ctx, c.KeyHash); !errors.Is(err, ErrNotFound) {
			t.Errorf("old key still valid: %v", err)
		}
		got, err := s.ClientByKeyHash(ctx, hash("new"))
		if err != nil || got.RotatedAt == nil || !got.RotatedAt.Equal(t0.Add(time.Hour)) || got.KeyHint != "tg_Zz…9999" {
			t.Fatalf("rotated: %+v %v", got, err)
		}
		if err := s.RotateClientKey(ctx, "a", []byte("short"), "x"); !errors.Is(err, ErrInvalid) {
			t.Errorf("short hash: %v", err)
		}
		list, err := s.ListClients(ctx)
		// The migration adds the reserved speedtest client (#87).
		if err != nil || len(list) != 2 || list[0].ID != SpeedtestClientID {
			t.Fatalf("list: %v %v", list, err)
		}
	})
}

func TestConfigVersions(t *testing.T) {
	ctx := context.Background()
	s, clk, _ := openTest(t)

	if _, err := s.ActiveConfigVersionID(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no active yet: %v", err)
	}
	v1 := mustConfig(t, s, "first")
	clk.now = t0.Add(time.Minute)
	v2 := mustConfig(t, s, "second")
	if v1.ID != 1 || v2.ID != 2 || !v2.CreatedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("ids/time: %+v %+v", v1, v2)
	}

	t.Run("seal gets id and result is stored", func(t *testing.T) {
		var gotID int64
		v, err := s.CreateConfigVersion(ctx, NewConfigVersion{
			Author: "import", Comment: "sealed", Content: []byte("c"), Diff: "",
			Seal: func(id int64) ([]byte, error) { gotID = id; return []byte{9, byte(id)}, nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotID != v.ID || gotID != 3 {
			t.Fatalf("seal id=%d version id=%d", gotID, v.ID)
		}
		back, err := s.ConfigVersionByID(ctx, v.ID)
		if err != nil || !bytes.Equal(back.WordsEnc, []byte{9, 3}) {
			t.Fatalf("words_enc: %v %v", back.WordsEnc, err)
		}
	})

	t.Run("seal error aborts without gap", func(t *testing.T) {
		boom := errors.New("boom")
		_, err := s.CreateConfigVersion(ctx, NewConfigVersion{
			Author: "admin", Content: []byte("c"), Seal: func(int64) ([]byte, error) { return nil, boom },
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err=%v", err)
		}
		if v := mustConfig(t, s, "after"); v.ID != 4 {
			t.Fatalf("id=%d want 4", v.ID)
		}
	})

	t.Run("validation", func(t *testing.T) {
		for name, n := range map[string]NewConfigVersion{
			"no author":  {Content: []byte("c")},
			"bad author": {Author: "a b\n", Content: []byte("c")},
			"no content": {Author: "admin"},
		} {
			if _, err := s.CreateConfigVersion(ctx, n); !errors.Is(err, ErrInvalid) {
				t.Errorf("%s: %v", name, err)
			}
		}
	})

	t.Run("immutable", func(t *testing.T) {
		if _, err := s.w.Exec(`UPDATE config_versions SET comment = 'x' WHERE id = 1`); err == nil {
			t.Error("UPDATE must fail")
		}
		if _, err := s.w.Exec(`DELETE FROM config_versions WHERE id = 1`); err == nil {
			t.Error("DELETE must fail")
		}
	})

	t.Run("active", func(t *testing.T) {
		if err := s.SetActiveConfigVersion(ctx, 999); !errors.Is(err, ErrNotFound) {
			t.Errorf("unknown version: %v", err)
		}
		for _, id := range []int64{1, 2, 1} {
			if err := s.SetActiveConfigVersion(ctx, id); err != nil {
				t.Fatal(err)
			}
			if got, err := s.ActiveConfigVersionID(ctx); err != nil || got != id {
				t.Fatalf("active=%d err=%v want %d", got, err, id)
			}
		}
	})

	t.Run("list newest first", func(t *testing.T) {
		list, err := s.ListConfigVersions(ctx, 2)
		if err != nil || len(list) != 2 || list[0].ID < list[1].ID {
			t.Fatalf("list: %+v %v", list, err)
		}
	})

	if _, err := s.ConfigVersionByID(ctx, 12345); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
}

func TestSessions(t *testing.T) {
	ctx := context.Background()
	s, clk, _ := openTest(t)
	mustClient(t, s, "a")
	v1 := mustConfig(t, s, "1")
	v2 := mustConfig(t, s, "2")
	key := hash("session-1")

	got, err := s.PinSession(ctx, Session{KeyHash: key, ClientID: "a", ConfigVersionID: v1.ID})
	if err != nil || got.ConfigVersionID != v1.ID || !got.CreatedAt.Equal(t0) {
		t.Fatalf("first pin: %+v %v", got, err)
	}

	clk.now = t0.Add(time.Hour)
	got, err = s.PinSession(ctx, Session{KeyHash: key, ClientID: "a", ConfigVersionID: v2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfigVersionID != v1.ID {
		t.Fatalf("pinned version changed to %d", got.ConfigVersionID)
	}
	if !got.CreatedAt.Equal(t0) || !got.LastSeenAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("times: %+v", got)
	}

	tests := []struct {
		name string
		in   Session
		want error
	}{
		{"short key", Session{KeyHash: []byte("x"), ClientID: "a", ConfigVersionID: v1.ID}, ErrInvalid},
		{"unknown client", Session{KeyHash: hash("k2"), ClientID: "zzz", ConfigVersionID: v1.ID}, ErrNotFound},
		{"unknown version", Session{KeyHash: hash("k3"), ClientID: "a", ConfigVersionID: 99}, ErrNotFound},
	}
	for _, tc := range tests {
		if _, err := s.PinSession(ctx, tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v want %v", tc.name, err, tc.want)
		}
	}

	if _, err := s.PinSession(ctx, Session{KeyHash: hash("other"), ClientID: "a", ConfigVersionID: v2.ID}); err != nil {
		t.Fatal(err)
	}
	// session-1 last seen t0+1h, "other" at t0+1h too; purge idle before t0+2h removes both.
	n, err := s.PurgeSessions(ctx, t0.Add(30*time.Minute))
	if err != nil || n != 0 {
		t.Fatalf("purge early: %d %v", n, err)
	}
	n, err = s.PurgeSessions(ctx, t0.Add(2*time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("purge: %d %v", n, err)
	}
	// After purge a new request starts the session with the then-active version.
	got, err = s.PinSession(ctx, Session{KeyHash: key, ClientID: "a", ConfigVersionID: v2.ID})
	if err != nil || got.ConfigVersionID != v2.ID {
		t.Fatalf("re-pin: %+v %v", got, err)
	}
}

func sampleTrace(id, clientID string, cfg int64) Trace {
	return Trace{
		ID: id, ClientID: clientID, SessionKeyHash: hash("s"), ConfigVersionID: cfg,
		StartedAt: t0, Endpoint: "/v1/messages", Model: "claude-sonnet-4-5", Stream: true, EstimatedInputTokens: 1234,
		Stages: []Stage{
			{Name: "redact", DurationMicros: 150, BytesIn: 100, BytesOut: 110, TokensIn: 25, TokensOut: 28,
				HashIn: hexHash("in"), HashOut: hexHash("out"), ConfigVersionID: cfg,
				Diff: "-<removed>\n+<redacted:email_1>", Matches: []Match{{Offset: 3, Length: 12, Type: "email", Rule: "builtin.email"}, {Offset: 40, Length: 6, Type: "customer_id", Rule: "kundennummer"}}},
			{Name: "compress", DurationMicros: 90, BytesIn: 110, BytesOut: 60, TokensIn: 28, TokensOut: 15,
				HashIn: hexHash("out"), HashOut: hexHash("c"), Diff: ""},
		},
	}
}

func TestTraceRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	v := mustConfig(t, s, "1")
	in := sampleTrace("tr-1", "a", v.ID)
	if err := s.RecordTrace(ctx, in); err != nil {
		t.Fatalf("RecordTrace: %v", err)
	}

	got, err := s.GetTrace(ctx, "tr-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != 0 || got.Usage != nil {
		t.Errorf("unfinished trace has result: %+v", got)
	}
	want := in
	want.Stages[1].Matches = nil
	if !traceEqual(want, got) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}

	res := TraceResult{Status: 200, DurationMicros: 4321, Usage: &Usage{Input: 10, Output: 20, CacheRead: 30, CacheCreation: 40}}
	if err := s.FinishTrace(ctx, "tr-1", res); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetTrace(ctx, "tr-1")
	if got.Status != 200 || got.DurationMicros != 4321 || got.Usage == nil || *got.Usage != *res.Usage {
		t.Fatalf("finished: %+v", got)
	}
	if err := s.FinishTrace(ctx, "nope", res); !errors.Is(err, ErrNotFound) {
		t.Errorf("finish unknown: %v", err)
	}
	if _, err := s.GetTrace(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get unknown: %v", err)
	}
	if err := s.RecordTrace(ctx, in); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate trace: %v", err)
	}
}

func traceEqual(a, b Trace) bool {
	a.StartedAt, b.StartedAt = a.StartedAt.UTC(), b.StartedAt.UTC()
	return reflect.DeepEqual(a, b)
}

func TestTraceValidation(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	secret := "sk-ant-api03-" + strings.Repeat("Z", 60)
	mod := func(f func(*Trace)) Trace { tr := sampleTrace("tr-v", "a", 0); f(&tr); return tr }
	tests := []struct {
		name string
		tr   Trace
		want error
	}{
		{"valid without config version", sampleTrace("tr-v", "a", 0), nil},
		{"secret as hash in", mod(func(t *Trace) { t.Stages[0].HashIn = secret }), ErrInvalid},
		{"short hash out", mod(func(t *Trace) { t.Stages[0].HashOut = "abcd" }), ErrInvalid},
		{"non-hex hash", mod(func(t *Trace) { t.Stages[0].HashOut = strings.Repeat("z", 64) }), ErrInvalid},
		{"upper-case hex", mod(func(t *Trace) { t.Stages[0].HashOut = strings.ToUpper(hexHash("x")) }), ErrInvalid},
		{"secret as stage name", mod(func(t *Trace) { t.Stages[0].Name = secret }), ErrInvalid},
		{"secret as match type", mod(func(t *Trace) { t.Stages[0].Matches[0].Type = "a@b.de" }), ErrInvalid},
		{"secret as match rule", mod(func(t *Trace) { t.Stages[0].Matches[0].Rule = "KD 123456" }), ErrInvalid},
		{"oversized diff", mod(func(t *Trace) { t.Stages[0].Diff = strings.Repeat("a", MaxDiffBytes+1) }), ErrInvalid},
		{"negative match offset", mod(func(t *Trace) { t.Stages[0].Matches[0].Offset = -1 }), ErrInvalid},
		{"bad session hash", mod(func(t *Trace) { t.SessionKeyHash = []byte("x") }), ErrInvalid},
		{"no id", mod(func(t *Trace) { t.ID = "" }), ErrInvalid},
		{"unknown client", mod(func(t *Trace) { t.ClientID = "ghost" }), ErrNotFound},
		{"unknown config version", mod(func(t *Trace) { t.ConfigVersionID = 77 }), ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.RecordTrace(ctx, tc.tr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want %v", err, tc.want)
			}
			if err != nil && strings.Contains(err.Error(), secret) {
				t.Errorf("error message quotes the offending value")
			}
			if tc.want != nil {
				if _, gerr := s.GetTrace(ctx, "tr-v"); !errors.Is(gerr, ErrNotFound) {
					t.Errorf("rejected trace was partially stored: %v", gerr)
				}
			}
			if tc.want == nil {
				if _, err := s.deleteTrace("tr-v"); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func (s *Store) deleteTrace(id string) (int64, error) {
	r, err := s.w.Exec(`DELETE FROM traces WHERE id = ?`, id)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

func TestListAndPurgeTraces(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	mustClient(t, s, "b")
	for i, id := range []string{"t1", "t2", "t3", "t4"} {
		tr := sampleTrace(id, "a", 0)
		if i == 3 {
			tr.ClientID = "b"
		}
		tr.StartedAt = t0.Add(time.Duration(i) * time.Minute)
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(l []Trace) []string {
		var o []string
		for _, x := range l {
			o = append(o, x.ID)
		}
		return o
	}
	tests := []struct {
		name string
		f    ListTracesFilter
		want []string
	}{
		{"all newest first", ListTracesFilter{}, []string{"t4", "t3", "t2", "t1"}},
		{"limit", ListTracesFilter{Limit: 2}, []string{"t4", "t3"}},
		{"client", ListTracesFilter{ClientID: "a"}, []string{"t3", "t2", "t1"}},
		{"before", ListTracesFilter{Before: t0.Add(2 * time.Minute)}, []string{"t2", "t1"}},
	}
	for _, tc := range tests {
		got, err := s.ListTraces(ctx, tc.f)
		if err != nil || !reflect.DeepEqual(ids(got), tc.want) {
			t.Errorf("%s: %v %v want %v", tc.name, ids(got), err, tc.want)
		}
		for _, x := range got {
			if len(x.Stages) != 0 {
				t.Errorf("%s: list must not load stages", tc.name)
			}
		}
	}
	n, err := s.PurgeTraces(ctx, t0.Add(2*time.Minute))
	if err != nil || n != 2 {
		t.Fatalf("purge: %d %v", n, err)
	}
	var stages int
	if err := s.r.QueryRow(`SELECT count(*) FROM trace_stages WHERE trace_id IN ('t1','t2')`).Scan(&stages); err != nil || stages != 0 {
		t.Fatalf("stages not cascaded: %d %v", stages, err)
	}
}

// Guards the "only redacted data" rule by API shape: a new field on a stored
// type must be added to this allowlist consciously. Review it for anything that
// could carry raw content (body, text, value, plaintext key).
func TestStoredTypesHaveNoPlaintextFields(t *testing.T) {
	allowed := map[string][]string{
		"Client":           {"ID", "Name", "KeyHash", "KeyHint", "Status", "CreatedAt", "RotatedAt"},
		"ConfigVersion":    {"ID", "CreatedAt", "Author", "Comment", "Content", "WordsEnc", "Diff"},
		"NewConfigVersion": {"Author", "Comment", "Content", "WordsEnc", "Diff", "Seal"},
		"Session":          {"KeyHash", "ClientID", "ConfigVersionID", "CreatedAt", "LastSeenAt"},
		"Trace": {"ID", "ClientID", "AccountID", "SessionKeyHash", "ConfigVersionID", "StartedAt", "Endpoint", "Model", "Stream",
			"EstimatedInputTokens", "Status", "DurationMicros", "Usage", "ErrorKind", "Stages",
			"TTFTMicros", "GenStartMicros", "ReturnMicros", "ReturnReplacements", "Perf", "TPS"},
		"Stage": {"Name", "DurationMicros", "BytesIn", "BytesOut", "TokensIn", "TokensOut", "HashIn", "HashOut",
			"ConfigVersionID", "Diff", "Matches"},
		"Match":       {"Offset", "Length", "Type", "Rule", "Placeholder", "Mask"},
		"Usage":       {"Input", "Output", "CacheRead", "CacheCreation"},
		"TraceResult": {"Status", "DurationMicros", "Usage", "ErrorKind", "TTFTMicros", "GenStartMicros", "ReturnMicros", "ReturnReplacements", "FiveHourPermille", "FiveHourReset", "SevenDayPermille", "SevenDayReset"},
		"FillRow":     {"AccountID", "StartedAt", "Permille", "Reset", "SevenPermille", "SevenReset", "Model", "Usage"},
		"Perf":        {"Metric", "Class", "Samples", "MedianMicros", "P90Micros", "ValueMicros", "Verdict"},
		"PerfRow": {"ID", "ClientID", "Model", "StartedAt", "Stream", "Status", "ErrorKind", "Usage", "DurationMicros",
			"TTFTMicros", "GenStartMicros", "ReturnMicros", "ReturnReplacements", "StageMicros", "Redact", "Slow", "Judged"},
		"TPS": {"Class", "Samples", "TPSMilli", "P10Milli", "MedianMilli", "P90Milli", "DeviationPermille", "Verdict"},
	}
	types := map[string]reflect.Type{
		"Client": reflect.TypeOf(Client{}), "ConfigVersion": reflect.TypeOf(ConfigVersion{}),
		"NewConfigVersion": reflect.TypeOf(NewConfigVersion{}), "Session": reflect.TypeOf(Session{}),
		"Trace": reflect.TypeOf(Trace{}), "Stage": reflect.TypeOf(Stage{}), "Match": reflect.TypeOf(Match{}),
		"Usage": reflect.TypeOf(Usage{}), "TraceResult": reflect.TypeOf(TraceResult{}),
		"Perf": reflect.TypeOf(Perf{}), "PerfRow": reflect.TypeOf(PerfRow{}), "TPS": reflect.TypeOf(TPS{}),
		"FillRow": reflect.TypeOf(FillRow{}),
	}
	for name, typ := range types {
		var got []string
		for i := 0; i < typ.NumField(); i++ {
			got = append(got, typ.Field(i).Name)
		}
		want := append([]string(nil), allowed[name]...)
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s fields changed: got %v want %v (update the allowlist consciously)", name, got, want)
		}
	}
}

// Never-given secrets must not be found in the database files. The positive
// control proves the scan works: a marker smuggled through an allowed free-text
// field (diff) IS found, so a clean result for the secrets is meaningful.
func TestNoPlaintextInDatabaseFiles(t *testing.T) {
	ctx := context.Background()
	s, _, path := openTest(t)
	secrets := []string{"sk-ant-api03-LEAKCANARY", "hunter2-LEAKCANARY", "tg_" + strings.Repeat("Q", 43), "alice@example.com"}
	const canary = "tg_canary"

	mustClient(t, s, "a")
	// Plaintext attempts are rejected.
	for _, sec := range secrets {
		if _, err := s.CreateClient(ctx, Client{ID: "x", Name: "n", KeyHash: hash(sec), KeyHint: sec, Status: ClientActive}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("hint %q accepted", sec)
		}
		bad := sampleTrace("tr-bad", "a", 0)
		bad.Stages[0].HashIn = sec
		if err := s.RecordTrace(ctx, bad); err == nil {
			t.Fatalf("hash %q accepted", sec)
		}
	}

	// Fill every allowed path with redacted values.
	v, err := s.CreateConfigVersion(ctx, NewConfigVersion{
		Author: "admin", Comment: "<redacted:comment_1>", Content: []byte("words: ['<masked>']\n"),
		WordsEnc: []byte{0xde, 0xad, 0xbe, 0xef}, Diff: "-<redacted:word_1>\n+<redacted:word_2>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PinSession(ctx, Session{KeyHash: hash("session"), ClientID: "a", ConfigVersionID: v.ID}); err != nil {
		t.Fatal(err)
	}
	tr := sampleTrace("tr-ok", "a", v.ID)
	tr.Model = secrets[0] + " in a sentence" // untrusted model value: normalized, never stored
	tr.Stages[0].Diff = "-<redacted:email_1>\n+<redacted:email_1>"
	tr.Stages[1].Diff = "+" + canary // deliberate leak through an allowed free-text field
	if err := s.RecordTrace(ctx, tr); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishTrace(ctx, "tr-ok", TraceResult{Status: 200, Usage: &Usage{Input: 1}}); err != nil {
		t.Fatal(err)
	}
	// Checkpoint and close so that DB and WAL are fully on disk.
	if _, err := s.w.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	var all []byte
	for _, f := range []string{path, path + "-wal"} {
		if b, err := os.ReadFile(f); err == nil {
			all = append(all, b...)
		}
	}
	if !bytes.Contains(all, []byte(canary)) {
		t.Fatal("positive control failed: smuggled marker not found, the scan is blind")
	}
	for _, sec := range secrets {
		if bytes.Contains(all, []byte(sec)) {
			t.Errorf("database files contain secret %q", sec)
		}
	}
}

func TestListTracesWithClientJoinsNameAndSavedTokens(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	mustClient(t, s, "b")
	// sampleTrace stages: first tokens_in 25, last tokens_out 15 -> saved 10.
	withStages := sampleTrace("tr-1", "a", 0)
	noStages := sampleTrace("tr-2", "b", 0)
	noStages.Stages = nil
	noStages.StartedAt = t0.Add(time.Second)
	for _, tr := range []Trace{withStages, noStages} {
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.ListTracesWithClient(ctx, ListTracesFilter{})
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id, client string
		saved      int64
	}
	var rows []row
	for _, it := range got {
		rows = append(rows, row{it.ID, it.ClientName, it.SavedTokens})
	}
	want := []row{{"tr-2", "name-b", 0}, {"tr-1", "name-a", 10}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
	// Client filter and keyset cursor work through the join too.
	got, err = s.ListTracesWithClient(ctx, ListTracesFilter{ClientID: "a", Before: t0.Add(time.Second), BeforeID: "tr-2"})
	if err != nil || len(got) != 1 || got[0].ID != "tr-1" {
		t.Fatalf("filtered = %+v, %v", got, err)
	}
}

func TestCostByDay(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	mustClient(t, s, "b")
	// 2025-10-10T00:00:00Z is 1_760_054_400_000 ms.
	midnight := time.UnixMilli(1_760_054_400_000)
	type rec struct {
		id, client, model string
		at                time.Time
		usage             *Usage
		finish            bool
	}
	u := func(i, o, cr, cc int64) *Usage { return &Usage{Input: i, Output: o, CacheRead: cr, CacheCreation: cc} }
	for _, r := range []rec{
		{"1", "a", "claude-opus-5-5", midnight.Add(-time.Millisecond), u(1, 2, 3, 4), true}, // day 1
		{"2", "a", "claude-opus-5-5", midnight.Add(-time.Hour), u(10, 20, 30, 40), true},    // day 1, same group
		{"3", "a", "claude-opus-5-5", midnight, u(100, 0, 0, 0), true},                      // day 2 (boundary)
		{"4", "a", "some-unknown-model", midnight, u(5, 5, 5, 5), true},                     // other model
		{"5", "b", "claude-opus-5-5", midnight, nil, true},                                  // finished without usage
		{"6", "b", "claude-opus-5-5", midnight, u(9, 9, 9, 9), false},                       // unfinished: excluded
		{"7", "a", "claude-opus-5-5", midnight.Add(-72 * time.Hour), u(7, 7, 7, 7), true},   // before since
	} {
		tr := sampleTrace(r.id, r.client, 0)
		tr.StartedAt, tr.Model = r.at, r.model
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
		if r.finish {
			if err := s.FinishTrace(ctx, r.id, TraceResult{Status: 200, Usage: r.usage}); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := s.CostByDay(ctx, midnight.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Each sampleTrace saves 10 tokens and estimates 1234.
	want := []CostAggregate{
		{Day: "2025-10-09", ClientID: "a", ClientName: "name-a", Model: "claude-opus-5-5", Requests: 2,
			Usage: Usage{Input: 11, Output: 22, CacheRead: 33, CacheCreation: 44}, Estimated: 2468, SavedTokens: 20},
		{Day: "2025-10-10", ClientID: "a", ClientName: "name-a", Model: "claude-opus-5-5", Requests: 1,
			Usage: Usage{Input: 100}, Estimated: 1234, SavedTokens: 10},
		{Day: "2025-10-10", ClientID: "a", ClientName: "name-a", Model: "some-unknown-model", Requests: 1,
			Usage: Usage{Input: 5, Output: 5, CacheRead: 5, CacheCreation: 5}, Estimated: 1234, SavedTokens: 10},
		{Day: "2025-10-10", ClientID: "b", ClientName: "name-b", Model: "claude-opus-5-5", Requests: 1,
			Estimated: 1234, SavedTokens: 10},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CostByDay =\n%+v\nwant\n%+v", got, want)
	}
}
