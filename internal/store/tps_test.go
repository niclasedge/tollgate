package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/perf"
)

func TestPutTPSValidatesReadsAndCascades(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	perfTrace(t, s, "tr-1", "a", t0, okResult(5_000_000, 1_000_000))
	perfTrace(t, s, "tr-2", "a", t0.Add(time.Second), okResult(5_000_000, 1_000_000))
	for _, bad := range []TPS{
		{Class: "i0", Verdict: "rot"},
		{Class: "x", Verdict: "üblich"},
		{Class: "i0", Verdict: "üblich", TPSMilli: -1},
	} {
		if err := s.PutTPS(ctx, "tr-1", bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("PutTPS(%+v) = %v, want ErrInvalid", bad, err)
		}
	}
	slower := TPSFromJudgement(perf.TPSJudgement{Class: "*", Samples: 87, TPSMilli: 48_200, P10: 51_300, Median: 62_000, P90: 70_100, DeviationPermille: -222, Verdict: perf.TPSSlower})
	if err := s.PutTPS(ctx, "tr-1", slower); err != nil {
		t.Fatal(err)
	}
	if err := s.PutTPS(ctx, "tr-2", TPSFromJudgement(perf.TPSJudgement{Class: "i0", Samples: 3, TPSMilli: 1, Verdict: perf.TPSUnknown})); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetTrace(ctx, "tr-1")
	if got.TPS == nil || got.TPS.Class != "*" || *got.TPS.MedianMilli != 62_000 || *got.TPS.DeviationPermille != -222 || got.TPS.Verdict != "langsamer" {
		t.Fatalf("tps: %+v", got.TPS)
	}
	list, err := s.ListTracesWithClient(ctx, ListTracesFilter{})
	if err != nil || len(list) != 2 || list[0].TPS == nil || list[0].TPS.Verdict != "unbekannt" || list[0].TPS.MedianMilli != nil || list[1].TPS == nil {
		t.Fatalf("list: %+v %v", list, err)
	}
	if _, err := s.PurgeTraces(ctx, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.r.QueryRow(`SELECT COUNT(*) FROM trace_tps`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("trace_tps after purge: %d %v", n, err)
	}
}

func TestTPSSamplesFilters(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openTest(t)
	mustClient(t, s, "a")
	at := t0.Add(perf.Window)
	perfTrace(t, s, "ok", "a", at.Add(-time.Hour), okResult(5_000_000, 1_000_000)) // 300 out / 4 s = 75 t/s
	short := okResult(5_000_000, 1_000_000)
	short.Usage = &Usage{Input: 10, Output: 63}
	perfTrace(t, s, "short", "a", at.Add(-time.Hour), short)
	perfTrace(t, s, "nottft", "a", at.Add(-time.Hour), TraceResult{Status: 200, DurationMicros: 5, Usage: &Usage{Output: 300}})
	// Older traces have a TTFT but no generation start: no tokens/s.
	nogen := okResult(5_000_000, 1_000_000)
	nogen.GenStartMicros = nil
	perfTrace(t, s, "nogen", "a", at.Add(-time.Hour), nogen)
	perfTrace(t, s, "old", "a", at.Add(-perf.Window-time.Millisecond), okResult(5_000_000, 1_000_000))
	perfTrace(t, s, "later", "a", at, okResult(5_000_000, 1_000_000))
	perfTrace(t, s, "speed", SpeedtestClientID, at.Add(-time.Hour), okResult(5_000_000, 1_000_000))
	if err := s.RecordTrace(ctx, Trace{ID: "json", ClientID: "a", StartedAt: at.Add(-time.Hour), Endpoint: "messages", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishTrace(ctx, "json", okResult(5_000_000, 1_000_000)); err != nil {
		t.Fatal(err)
	}
	got, err := s.TPSSamples(ctx, "m", at)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TPSMilli != 75_000 || got[0].InputTotal != 16 {
		t.Fatalf("samples %+v", got)
	}
}
