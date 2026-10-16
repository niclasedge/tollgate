package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestModelAndEndpointNormalization(t *testing.T) {
	ctx := context.Background()
	s, _, path := openTest(t)
	mustClient(t, s, "a")
	secret := "sk-ant-api03-LEAKCANARY hello"
	tests := []struct {
		name, model, endpoint, wantModel, wantEndpoint string
	}{
		{"bracket suffix kept", "claude-opus-4-1[1m]", "/v1/messages", "claude-opus-4-1[1m]", "/v1/messages"},
		{"secret model", secret, "/v1/messages", "unknown", "/v1/messages"},
		{"secret endpoint", "claude-sonnet-4-5", secret, "claude-sonnet-4-5", "unknown"},
		{"empty model", "", "/v1/messages", "unknown", "/v1/messages"},
		{"too long model", strings.Repeat("a", 65), "/v1/messages", "unknown", "/v1/messages"},
	}
	for i, tc := range tests {
		tr := sampleTrace(fmt.Sprintf("tr-%d", i), "a", 0)
		tr.Model, tr.Endpoint = tc.model, tc.endpoint
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got, err := s.GetTrace(ctx, tr.ID)
		if err != nil || got.Model != tc.wantModel || got.Endpoint != tc.wantEndpoint {
			t.Errorf("%s: model=%q endpoint=%q err=%v", tc.name, got.Model, got.Endpoint, err)
		}
	}
	_, _ = s.w.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	_ = s.Close()
	b, _ := os.ReadFile(path)
	if bytes.Contains(b, []byte("LEAKCANARY")) {
		t.Error("untrusted model/endpoint value reached the database")
	}
}

func TestFilePermissions(t *testing.T) {
	ctx := context.Background()
	check := func(t *testing.T, path string) {
		t.Helper()
		for _, f := range []string{path, path + "-wal", path + "-shm"} {
			fi, err := os.Stat(f)
			if err != nil {
				if f == path {
					t.Fatal(err)
				}
				continue
			}
			if fi.Mode().Perm() != 0o600 {
				t.Errorf("%s mode %v want 0600", filepath.Base(f), fi.Mode().Perm())
			}
		}
	}
	t.Run("fresh", func(t *testing.T) {
		s, _, path := openTest(t)
		mustClient(t, s, "a")
		check(t, path)
	})
	t.Run("existing loose file is tightened", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "old.db")
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		s, err := Open(ctx, path, Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		mustClient(t, s, "a")
		check(t, path)
	})
}

func TestListTracesKeysetPagination(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	for i := 0; i < 5; i++ {
		tr := sampleTrace(fmt.Sprintf("same-%d", i), "a", 0)
		tr.Stages = nil
		tr.StartedAt = t0 // identical millisecond
		if err := s.RecordTrace(ctx, tr); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	var order []string
	f := ListTracesFilter{Limit: 2}
	for page := 0; page < 10; page++ {
		got, err := s.ListTraces(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 {
			break
		}
		for _, x := range got {
			if seen[x.ID] {
				t.Fatalf("duplicate %s", x.ID)
			}
			seen[x.ID] = true
			order = append(order, x.ID)
		}
		last := got[len(got)-1]
		f.Before, f.BeforeID = last.StartedAt, last.ID
	}
	if len(seen) != 5 {
		t.Fatalf("paged through %d traces, want 5: %v", len(seen), order)
	}
	if !sort.IsSorted(sort.Reverse(sort.StringSlice(order))) {
		t.Errorf("order not id DESC within equal time: %v", order)
	}
}

func TestListTracesQueryPlanNoTempSort(t *testing.T) {
	s, _, _ := openTest(t)
	cases := []struct {
		name string
		f    ListTracesFilter
		idx  string
	}{
		{"all", ListTracesFilter{}, "traces_started_at"},
		{"all keyset", ListTracesFilter{Before: t0, BeforeID: "x"}, "traces_started_at"},
		{"all before only", ListTracesFilter{Before: t0}, "traces_started_at"},
		{"client", ListTracesFilter{ClientID: "a"}, "traces_client"},
		{"client keyset", ListTracesFilter{ClientID: "a", Before: t0, BeforeID: "x"}, "traces_client"},
		{"client before only", ListTracesFilter{ClientID: "a", Before: t0}, "traces_client"},
	}
	froms := map[string]string{"plain": tracesFrom, "join": tracesWithClientFrom}
	for _, tc := range cases {
		for fromName, from := range froms {
			t.Run(tc.name+"/"+fromName, func(t *testing.T) {
				testQueryPlan(t, s, tc.f, tc.idx, from)
			})
		}
	}
}

func testQueryPlan(t *testing.T, s *Store, f ListTracesFilter, idx, from string) {
	q, args := listTracesQuery(f, 10, qualifiedTraceCols, from)
	rows, err := s.r.Query("EXPLAIN QUERY PLAN "+q, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var a, b, c int
		var d string
		if err := rows.Scan(&a, &b, &c, &d); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, d)
	}
	all := strings.Join(plan, " | ")
	if strings.Contains(all, "TEMP B-TREE") {
		t.Errorf("temp sort in plan: %s", all)
	}
	if !strings.Contains(all, idx) {
		t.Errorf("plan does not use %s: %s", idx, all)
	}
}

func TestFilePermissionsExistingSidecars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	for _, f := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(f, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureFile(path); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{path, path + "-wal", path + "-shm"} {
		fi, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v want 0600", filepath.Base(f), fi.Mode().Perm())
		}
	}
}

func TestSidecarsAreNeverLoosenedOrFollowed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db")
	if err := os.WriteFile(path+"-wal", nil, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := ensureFile(path); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(path + "-wal"); fi.Mode().Perm() != 0o400 {
		t.Errorf("-wal mode %v, a stricter mode must stay", fi.Mode().Perm())
	}

	foreign := filepath.Join(dir, "foreign")
	if err := os.WriteFile(foreign, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(foreign, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, path+"-shm"); err != nil {
		t.Fatal(err)
	}
	if err := ensureFile(path); err == nil {
		t.Error("symlinked -shm accepted")
	}
	if fi, _ := os.Stat(foreign); fi.Mode().Perm() != 0o644 {
		t.Errorf("symlink target changed to %v", fi.Mode().Perm())
	}
}

func TestIsBusy(t *testing.T) {
	cases := map[string]bool{
		"database is locked (5) (SQLITE_BUSY)": true,
		"constraint failed":                    false,
	}
	for msg, want := range cases {
		if got := isBusy(errors.New(msg)); got != want {
			t.Errorf("isBusy(%q) = %v", msg, got)
		}
	}
}

func TestParallelOpenFreshFileMigratesOnce(t *testing.T) {
	ctx := context.Background()
	migs, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 5; round++ {
		path := filepath.Join(t.TempDir(), "fresh.db")
		const n = 4
		stores := make([]*Store, n)
		errs := make([]error, n)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				stores[i], errs[i] = Open(ctx, path, Options{})
			}()
		}
		close(start)
		wg.Wait()
		for i := 0; i < n; i++ {
			if errs[i] != nil {
				t.Fatalf("round %d open %d: %v", round, i, errs[i])
			}
			defer stores[i].Close()
		}
		var count, distinct int
		if err := stores[0].r.QueryRow(`SELECT count(*), count(DISTINCT version) FROM schema_migrations`).Scan(&count, &distinct); err != nil {
			t.Fatal(err)
		}
		if count != len(migs) || distinct != len(migs) {
			t.Fatalf("round %d: schema_migrations has %d rows (%d distinct), want %d", round, count, distinct, len(migs))
		}
	}
}

func TestConfigVersionsInsertOrReplaceRejected(t *testing.T) {
	s, _, _ := openTest(t)
	v := mustConfig(t, s, "orig")
	for _, stmt := range []string{"INSERT OR REPLACE", "REPLACE", "INSERT OR IGNORE"} {
		_, err := s.w.Exec(stmt+` INTO config_versions (id, created_at, author, comment, content, words_enc, diff) VALUES (?, 1, 'x', 'evil', x'00', x'00', '')`, v.ID)
		if err == nil && stmt != "INSERT OR IGNORE" {
			t.Errorf("%s on existing id succeeded", stmt)
		}
	}
	back, err := s.ConfigVersionByID(context.Background(), v.ID)
	if err != nil || back.Comment != "orig" {
		t.Fatalf("version modified: %+v %v", back, err)
	}
}

func TestConcurrentWritersTwoStores(t *testing.T) {
	ctx := context.Background()
	s1, _, path := openTest(t)
	s2, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := map[int64]bool{}
	var errs []error
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st := s1
			if i%2 == 1 {
				st = s2
			}
			v, err := st.CreateConfigVersion(ctx, NewConfigVersion{Author: "admin", Content: []byte("c")})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if ids[v.ID] {
				errs = append(errs, fmt.Errorf("duplicate id %d", v.ID))
			}
			ids[v.ID] = true
		}()
	}
	wg.Wait()
	if len(errs) > 0 || len(ids) != 40 {
		t.Fatalf("ids=%d errs=%v", len(ids), errs)
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	v := mustConfig(t, s, "1")
	var wg sync.WaitGroup
	errc := make(chan error, 300)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr := sampleTrace(fmt.Sprintf("c-%d", i), "a", v.ID)
			tr.StartedAt = t0.Add(time.Duration(i) * time.Millisecond)
			errc <- s.RecordTrace(ctx, tr)
			errc <- s.FinishTrace(ctx, tr.ID, TraceResult{Status: 200, Usage: &Usage{Input: 1}})
			_, err := s.PinSession(ctx, Session{KeyHash: hash(fmt.Sprint("k", i%7)), ClientID: "a", ConfigVersionID: v.ID})
			errc <- err
			_, err = s.ListTraces(ctx, ListTracesFilter{Limit: 5})
			errc <- err
			_, err = s.CreateConfigVersion(ctx, NewConfigVersion{Author: "admin", Content: []byte("c")})
			errc <- err
		}()
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		if err != nil {
			t.Errorf("concurrent op: %v", err)
		}
	}
	list, err := s.ListTraces(ctx, ListTracesFilter{Limit: 1000})
	if err != nil || len(list) != 50 {
		t.Fatalf("traces=%d err=%v", len(list), err)
	}
}

func TestTimestampsTruncatedToMillis(t *testing.T) {
	ctx := context.Background()
	s, clk, _ := openTest(t)
	clk.now = time.Unix(1_760_000_000, 123_456_789).In(time.FixedZone("x", 3600))
	c := mustClient(t, s, "a")
	v := mustConfig(t, s, "1")
	got, _ := s.ClientByKeyHash(ctx, c.KeyHash)
	back, _ := s.ConfigVersionByID(ctx, v.ID)
	if !c.CreatedAt.Equal(got.CreatedAt) || c.CreatedAt.Location() != time.UTC {
		t.Errorf("client: returned %v stored %v", c.CreatedAt, got.CreatedAt)
	}
	if !v.CreatedAt.Equal(back.CreatedAt) || v.CreatedAt.Location() != time.UTC {
		t.Errorf("config: returned %v stored %v", v.CreatedAt, back.CreatedAt)
	}
}

func TestInputRangeValidation(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	v := mustConfig(t, s, "1")

	zero := sampleTrace("z", "a", 0)
	zero.StartedAt = time.Time{}
	if err := s.RecordTrace(ctx, zero); !errors.Is(err, ErrInvalid) {
		t.Errorf("zero StartedAt: %v", err)
	}
	neg := sampleTrace("n", "a", 0)
	neg.EstimatedInputTokens = -1
	if err := s.RecordTrace(ctx, neg); !errors.Is(err, ErrInvalid) {
		t.Errorf("negative estimate: %v", err)
	}

	if err := s.RecordTrace(ctx, sampleTrace("f", "a", 0)); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		r    TraceResult
	}{
		{"status 0", TraceResult{Status: 0}},
		{"status 99", TraceResult{Status: 99}},
		{"status 600", TraceResult{Status: 600}},
		{"negative duration", TraceResult{Status: 200, DurationMicros: -1}},
		{"negative input", TraceResult{Status: 200, Usage: &Usage{Input: -1}}},
		{"negative output", TraceResult{Status: 200, Usage: &Usage{Output: -1}}},
		{"negative cache read", TraceResult{Status: 200, Usage: &Usage{CacheRead: -1}}},
		{"negative cache creation", TraceResult{Status: 200, Usage: &Usage{CacheCreation: -1}}},
	}
	for _, tc := range tests {
		if err := s.FinishTrace(ctx, "f", tc.r); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	for _, st := range []int{100, 599} {
		if err := s.FinishTrace(ctx, "f", TraceResult{Status: st}); err != nil {
			t.Errorf("status %d: %v", st, err)
		}
	}

	pins := []struct {
		name string
		in   Session
	}{
		{"bad client id", Session{KeyHash: hash("p"), ClientID: "a b", ConfigVersionID: v.ID}},
		{"empty client id", Session{KeyHash: hash("p"), ConfigVersionID: v.ID}},
		{"zero version", Session{KeyHash: hash("p"), ClientID: "a"}},
		{"negative version", Session{KeyHash: hash("p"), ClientID: "a", ConfigVersionID: -1}},
	}
	for _, tc := range pins {
		if _, err := s.PinSession(ctx, tc.in); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

func TestConfigDiffLimitAndListLimit(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	if _, err := s.CreateConfigVersion(ctx, NewConfigVersion{Author: "a", Content: []byte("c"), Diff: strings.Repeat("d", MaxDiffBytes)}); err != nil {
		t.Errorf("diff at limit: %v", err)
	}
	if _, err := s.CreateConfigVersion(ctx, NewConfigVersion{Author: "a", Content: []byte("c"), Diff: strings.Repeat("d", MaxDiffBytes+1)}); !errors.Is(err, ErrInvalid) {
		t.Errorf("diff over limit: %v", err)
	}
	if _, err := s.CreateConfigVersion(ctx, NewConfigVersion{Author: "a", Content: []byte("c"), Comment: strings.Repeat("c", maxComment+1)}); !errors.Is(err, ErrInvalid) {
		t.Errorf("comment over limit: %v", err)
	}
	// list limit caps
	for i := 0; i < 5; i++ {
		mustConfig(t, s, "x")
	}
	if l, _ := s.ListConfigVersions(ctx, 3); len(l) != 3 {
		t.Errorf("limit 3 returned %d", len(l))
	}
	if l, err := s.ListConfigVersions(ctx, 1<<30); err != nil || len(l) != 6 {
		t.Errorf("huge limit: %d %v", len(l), err)
	}
}

func TestPurgeBoundaries(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	v := mustConfig(t, s, "1")
	if _, err := s.PinSession(ctx, Session{KeyHash: hash("s"), ClientID: "a", ConfigVersionID: v.ID}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.PurgeSessions(ctx, t0); err != nil || n != 0 {
		t.Errorf("last_seen == threshold must survive: %d %v", n, err)
	}
	if n, err := s.PurgeSessions(ctx, t0.Add(time.Millisecond)); err != nil || n != 1 {
		t.Errorf("last_seen < threshold must go: %d %v", n, err)
	}
	if err := s.RecordTrace(ctx, sampleTrace("p", "a", 0)); err != nil {
		t.Fatal(err)
	}
	if n, err := s.PurgeTraces(ctx, t0); err != nil || n != 0 {
		t.Errorf("trace == threshold must survive: %d %v", n, err)
	}
	if n, err := s.PurgeTraces(ctx, t0.Add(time.Millisecond)); err != nil || n != 1 {
		t.Errorf("trace < threshold must go: %d %v", n, err)
	}
}

func TestStageOrderFollowsSeq(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	tr := sampleTrace("o", "a", 0)
	tr.Stages = tr.Stages[:1]
	tr.Stages[0].Name = "s0"
	if err := s.RecordTrace(ctx, tr); err != nil {
		t.Fatal(err)
	}
	for _, seq := range []int{2, 1} { // insert order differs from seq order
		if _, err := s.w.Exec(`INSERT INTO trace_stages (trace_id, seq, name, duration_us, bytes_in, bytes_out, tokens_in, tokens_out, hash_in, hash_out, diff, matches)
			VALUES ('o', ?, ?, 0, 0, 0, 0, 0, ?, ?, '', '[]')`, seq, fmt.Sprintf("s%d", seq), hexHash("a"), hexHash("b")); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.GetTrace(ctx, "o")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, st := range got.Stages {
		names = append(names, st.Name)
	}
	if strings.Join(names, ",") != "s0,s1,s2" {
		t.Fatalf("order %v", names)
	}
}
