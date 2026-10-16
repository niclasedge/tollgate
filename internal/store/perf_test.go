package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/perf"
)

func i64(v int64) *int64 { return &v }

// perfTrace records and finishes a messages trace of model "m" at started.
func perfTrace(t *testing.T, s *Store, id, client string, started time.Time, res TraceResult) {
	t.Helper()
	ctx := context.Background()
	tr := Trace{ID: id, ClientID: client, StartedAt: started, Endpoint: "messages", Model: "m", Stream: true}
	if err := s.RecordTrace(ctx, tr); err != nil {
		t.Fatalf("RecordTrace %s: %v", id, err)
	}
	if res.Status == 0 {
		return
	}
	if err := s.FinishTrace(ctx, id, res); err != nil {
		t.Fatalf("FinishTrace %s: %v", id, err)
	}
}

// okResult is a finished stream whose generation started with the first token.
func okResult(dur, ttft int64) TraceResult {
	return TraceResult{Status: 200, DurationMicros: dur, TTFTMicros: i64(ttft), GenStartMicros: i64(ttft), Usage: &Usage{Input: 10, Output: 300, CacheRead: 5, CacheCreation: 1}}
}

func TestPerfColumnsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	res := okResult(5_000_000, 1_200_000)
	res.GenStartMicros = i64(300_000)
	res.ReturnMicros, res.ReturnReplacements = i64(800), i64(2)
	perfTrace(t, s, "tr-1", "a", t0, res)
	got, err := s.GetTrace(ctx, "tr-1")
	if err != nil {
		t.Fatal(err)
	}
	if *got.TTFTMicros != 1_200_000 || *got.GenStartMicros != 300_000 || *got.ReturnMicros != 800 || *got.ReturnReplacements != 2 {
		t.Fatalf("got %+v", got)
	}
	perfTrace(t, s, "tr-2", "a", t0, TraceResult{Status: 429, DurationMicros: 10})
	got, _ = s.GetTrace(ctx, "tr-2")
	if got.TTFTMicros != nil || got.GenStartMicros != nil || got.ReturnMicros != nil || got.ReturnReplacements != nil {
		t.Fatalf("unknown values not NULL: %+v", got)
	}
	bad := okResult(1, 1)
	bad.TTFTMicros = i64(-1)
	if err := s.FinishTrace(ctx, "tr-1", bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("negative ttft: %v", err)
	}
	bad = okResult(1, 1)
	bad.GenStartMicros = i64(-1)
	if err := s.FinishTrace(ctx, "tr-1", bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("negative generation start: %v", err)
	}
}

// A database at migration 0005 keeps its traces; the new columns are NULL.
func TestPerfMigrationOnExistingDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	migs, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureFile(path); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migs[:5]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO clients (id, name, key_hash, key_hint, status, created_at) VALUES ('a', 'a', ?, 'x', 'aktiv', 1)`, hash("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO traces (id, client_id, started_at, endpoint, model, stream, estimated_input_tokens, status, duration_us)
		VALUES ('old', 'a', 1, 'messages', 'm', 1, 0, 200, 99)`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	s, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.GetTrace(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	if got.DurationMicros != 99 || got.TTFTMicros != nil || got.ReturnMicros != nil || got.Perf != nil {
		t.Fatalf("old trace after migration: %+v", got)
	}
}

func TestPutPerfValidatesAndCascades(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	perfTrace(t, s, "tr-1", "a", t0, okResult(9_800_000, 1_000_000))
	slow := PerfFromJudgement(perf.Judgement{Metric: perf.MetricDuration, Class: "i0o1", Samples: 37, Median: 4_200_000, P90: 6_100_000, Value: 9_800_000, Verdict: perf.Slow})
	unknown := PerfFromJudgement(perf.Judgement{Metric: perf.MetricTTFT, Class: "i0", Samples: 7, Value: 1_000_000, Verdict: perf.Unknown})
	for _, bad := range []Perf{
		{Metric: "ttft", Class: "i0", Verdict: "rot"},
		{Metric: "ttft", Class: "i0", Verdict: "normal", Samples: -1},
		{Metric: "cpu", Class: "i0", Verdict: "normal"},
		{Metric: "ttft", Class: "x9", Verdict: "normal"},
	} {
		if err := s.PutPerf(ctx, "tr-1", []Perf{bad}); !errors.Is(err, ErrInvalid) {
			t.Errorf("PutPerf(%+v) = %v, want ErrInvalid", bad, err)
		}
	}
	if err := s.PutPerf(ctx, "tr-1", []Perf{slow, unknown}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetTrace(ctx, "tr-1")
	if len(got.Perf) != 2 || got.Perf[0].Metric != "duration" || *got.Perf[0].MedianMicros != 4_200_000 ||
		got.Perf[1].Verdict != "unbekannt" || got.Perf[1].MedianMicros != nil || got.Perf[1].Samples != 7 {
		t.Fatalf("perf: %+v", got.Perf)
	}
	list, err := s.ListTracesWithClient(ctx, ListTracesFilter{})
	if err != nil || len(list) != 1 || len(list[0].Perf) != 2 {
		t.Fatalf("list: %+v %v", list, err)
	}
	if _, err := s.PurgeTraces(ctx, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.r.QueryRow(`SELECT COUNT(*) FROM trace_perf`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("trace_perf after purge: %d %v", n, err)
	}
}

func TestPerfSamplesFilters(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	now := t0.Add(30 * 24 * time.Hour)
	perfTrace(t, s, "ok-1", "a", now.Add(-time.Hour), okResult(4_000_000, 1_000_000))
	perfTrace(t, s, "ok-2", "a", now.Add(-13*24*time.Hour), okResult(5_000_000, 0))
	perfTrace(t, s, "old", "a", now.Add(-15*24*time.Hour), okResult(1, 1))
	perfTrace(t, s, "status", "a", now.Add(-time.Hour), TraceResult{Status: 500, DurationMicros: 1, Usage: &Usage{}})
	errRes := okResult(1, 1)
	errRes.ErrorKind = "stream_aborted"
	perfTrace(t, s, "errkind", "a", now.Add(-time.Hour), errRes)
	perfTrace(t, s, "nousage", "a", now.Add(-time.Hour), TraceResult{Status: 200, DurationMicros: 1})
	perfTrace(t, s, "speed", SpeedtestClientID, now.Add(-time.Hour), okResult(1, 1))
	perfTrace(t, s, "later", "a", now.Add(time.Millisecond), okResult(1, 1))
	perfTrace(t, s, "running", "a", now.Add(-time.Hour), TraceResult{})
	noTTFT := okResult(6_000_000, 0)
	noTTFT.TTFTMicros = nil
	perfTrace(t, s, "nottft", "a", now.Add(-2*time.Hour), noTTFT)

	dur, err := s.PerfSamples(ctx, "m", now, perf.MetricDuration)
	if err != nil {
		t.Fatal(err)
	}
	if len(dur) != 3 || dur[0].Duration != 4_000_000 || dur[0].InputTotal != 16 || dur[0].Output != 300 || dur[2].Duration != 5_000_000 {
		t.Fatalf("duration samples: %+v", dur)
	}
	ttft, err := s.PerfSamples(ctx, "m", now, perf.MetricTTFT)
	if err != nil {
		t.Fatal(err)
	}
	if len(ttft) != 2 || !ttft[0].HasTTFT || ttft[0].TTFT != 1_000_000 {
		t.Fatalf("ttft samples: %+v", ttft)
	}
	if other, _ := s.PerfSamples(ctx, "other", now, perf.MetricDuration); len(other) != 0 {
		t.Fatalf("other model: %+v", other)
	}
}

func TestPerfSamplesNewest500(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	for i := 0; i < 600; i++ {
		perfTrace(t, s, fmt.Sprintf("tr-%03d", i), "a", t0.Add(time.Duration(i)*time.Second), okResult(int64(i+1), 1))
	}
	got, err := s.PerfSamples(ctx, "m", t0.Add(time.Hour), perf.MetricDuration)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 500 || got[0].Duration != 600 || got[499].Duration != 101 {
		t.Fatalf("got %d samples, first %d last %d", len(got), got[0].Duration, got[len(got)-1].Duration)
	}
}

func TestReservedSpeedtestClient(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	if _, err := s.CreateClient(ctx, Client{ID: "x", Name: "tollgate:test", KeyHash: hash("x"), KeyHint: "x", Status: ClientActive}); !errors.Is(err, ErrInvalid) {
		t.Errorf("reserved name: %v", err)
	}
	if _, err := s.CreateClient(ctx, Client{ID: SpeedtestClientID, Name: "x", KeyHash: hash("x"), KeyHint: "x", Status: ClientActive}); !errors.Is(err, ErrInvalid) {
		t.Errorf("reserved id: %v", err)
	}
	if err := s.SetClientStatus(ctx, SpeedtestClientID, ClientActive); !errors.Is(err, ErrNotFound) {
		t.Errorf("activate: %v", err)
	}
	if err := s.RotateClientKey(ctx, SpeedtestClientID, hash("k"), "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rotate: %v", err)
	}
	c, err := s.ClientByKeyHash(ctx, make([]byte, 32))
	if err != nil || c.ID != SpeedtestClientID || c.Status != ClientBlocked {
		t.Fatalf("reserved row: %+v %v", c, err)
	}
}

func TestPerfRowsAndSpeedtestRows(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	tr := Trace{ID: "r-1", ClientID: "a", StartedAt: t0, Endpoint: "messages", Model: "m", Stream: true, Stages: []Stage{
		{Name: "intake", DurationMicros: 100, HashIn: hexHash("1"), HashOut: hexHash("1")},
		{Name: "redact", DurationMicros: 400, BytesIn: 1000, BytesOut: 1010, TokensIn: 250, TokensOut: 253, HashIn: hexHash("1"), HashOut: hexHash("2"),
			Matches: []Match{{Offset: 1, Length: 20, Type: "email", Rule: "email"}, {Offset: 30, Length: 20, Type: "email", Rule: "email"}}},
	}}
	if err := s.RecordTrace(ctx, tr); err != nil {
		t.Fatal(err)
	}
	res := okResult(3_000_000, 900_000)
	res.ReturnMicros, res.ReturnReplacements = i64(50), i64(1)
	if err := s.FinishTrace(ctx, "r-1", res); err != nil {
		t.Fatal(err)
	}
	if err := s.PutPerf(ctx, "r-1", []Perf{PerfFromJudgement(perf.Judgement{Metric: "ttft", Class: "i0", Samples: 20, Median: 1, P90: 2, Value: 900_000, Verdict: perf.Slow})}); err != nil {
		t.Fatal(err)
	}
	perfTrace(t, s, "r-2", "a", t0.Add(time.Second), okResult(2_000_000, 500_000))
	perfTrace(t, s, "r-err", "a", t0.Add(time.Second), TraceResult{Status: 529, DurationMicros: 1})
	perfTrace(t, s, "sp-1", SpeedtestClientID, t0.Add(2*time.Second), okResult(1_000_000, 300_000))

	rows, err := s.PerfRows(ctx, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != "r-1" || rows[1].ID != "r-2" {
		t.Fatalf("rows: %+v", rows)
	}
	r := rows[0]
	if r.StageMicros != 500 || r.Redact == nil || r.Redact.DurationMicros != 400 || r.Redact.Matches != 2 ||
		r.Redact.TokensOut-r.Redact.TokensIn != 3 || !r.Slow || !r.Judged || *r.ReturnMicros != 50 {
		t.Fatalf("row: %+v redact %+v", r, r.Redact)
	}
	if rows[1].Redact != nil || rows[1].Slow || rows[1].Judged {
		t.Fatalf("row 2: %+v", rows[1])
	}
	sp, err := s.SpeedtestRows(ctx, 10)
	if err != nil || len(sp) != 1 || sp[0].ID != "sp-1" {
		t.Fatalf("speedtest rows: %+v %v", sp, err)
	}
}
