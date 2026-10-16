package perf

import (
	"slices"
	"testing"
)

func TestTPSMilli(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		output, duration, ttft int64
		hasTTFT, stream        bool
		want                   int64
		ok                     bool
	}{
		{"spec example", 200, 5_000_000, 1_000_000, true, true, 50_000, true},
		{"truncates", 100, 4_000_000, 1_000_000, true, true, 33_333, true},
		{"json", 200, 5_000_000, 1_000_000, true, false, 0, false},
		{"no output", 0, 5_000_000, 1_000_000, true, true, 0, false},
		{"no ttft", 200, 5_000_000, 0, false, true, 0, false},
		{"ttft equals duration", 200, 5_000_000, 5_000_000, true, true, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := TPSMilli(tc.output, tc.duration, tc.ttft, tc.hasTTFT, tc.stream)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("got %d %v, want %d %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// tpsSamples gives n samples of input class i1 from 50.000 to 50.000+n-1 ‰ t/s
// spread so that p10 ≈ 45, median 50, p90 ≈ 55 t/s.
func tpsSamples(n int, input int64) []TPSSample {
	var out []TPSSample
	for i := 0; i < n; i++ {
		out = append(out, TPSSample{InputTotal: input, TPSMilli: 40_000 + int64(i)*20_000/int64(n)})
	}
	return out
}

func TestJudgeTPS(t *testing.T) {
	s := tpsSamples(20, 10_000) // 40.0 .. 49.0 t/s, median 49.0? see stats check
	vals := make([]int64, len(s))
	for i, x := range s {
		vals[i] = x.TPSMilli
	}
	slices.Sort(vals)
	p10, med, p90 := Percentile(vals, 100), Percentile(vals, 500), Percentile(vals, 900)
	for _, tc := range []struct {
		name   string
		tps    int64
		output int64
		want   TPSVerdict
	}{
		{"far below", med * 7 / 10, 500, TPSSlower},
		{"below p10 but within 20 %", p10 - 1, 500, TPSUsual},
		{"far above", med * 13 / 10, 500, TPSFaster},
		{"above p90 but within 20 %", p90 + 1, 500, TPSUsual},
		{"median", med, 500, TPSUsual},
		{"output 63", med * 5 / 10, 63, TPSUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := JudgeTPS(s, 10_000, tc.output, tc.tps)
			if j.Verdict != tc.want {
				t.Fatalf("verdict %s, want %s (%+v)", j.Verdict, tc.want, j)
			}
			if tc.want != TPSUnknown && (j.Class != "i1" || j.Samples != 20 || j.Median != med || j.P10 != p10 || j.P90 != p90) {
				t.Fatalf("stats %+v", j)
			}
		})
	}
}

func TestJudgeTPSFallbackAndUnknown(t *testing.T) {
	s := append(tpsSamples(10, 10_000), tpsSamples(10, 100)...)
	j := JudgeTPS(s, 10_000, 500, 10_000)
	if j.Class != AllInputs || j.Samples != 20 || j.Verdict != TPSSlower {
		t.Fatalf("got %+v, want fallback to * with 20 samples, slower", j)
	}
	j = JudgeTPS(s[:19], 10_000, 500, 10_000)
	if j.Verdict != TPSUnknown || j.Samples != 19 || j.Median != 0 || j.DeviationPermille != 0 {
		t.Fatalf("got %+v, want unknown", j)
	}
}

func TestJudgeTPSOrderIndependent(t *testing.T) {
	a := tpsSamples(30, 10_000)
	b := slices.Clone(a)
	slices.Reverse(b)
	if JudgeTPS(a, 10_000, 500, 30_000) != JudgeTPS(b, 10_000, 500, 30_000) {
		t.Fatal("permuted input changes the result")
	}
}

func TestCacheClass(t *testing.T) {
	for _, tc := range []struct {
		read, total int64
		want        string
	}{
		{0, 0, "c0"}, {499, 1000, "c0"}, {500, 1000, "c1"}, {899, 1000, "c1"}, {900, 1000, "c2"}, {1000, 1000, "c2"},
	} {
		if got := CacheClass(tc.read, tc.total); got != tc.want {
			t.Errorf("CacheClass(%d, %d) = %s, want %s", tc.read, tc.total, got, tc.want)
		}
	}
}

func rows(n int, input, cacheRead, ttft, tps int64) []InfluenceRow {
	var out []InfluenceRow
	for i := 0; i < n; i++ {
		out = append(out, InfluenceRow{InputTotal: input, CacheRead: cacheRead, TTFT: ttft, HasTTFT: true,
			Duration: 2 * ttft, TPSMilli: tps, HasTPS: true})
	}
	return out
}

func TestMatrix(t *testing.T) {
	r := append(rows(5, 40_000, 0, 2_000_000, 50_000), rows(4, 40_000, 39_000, 800_000, 50_000)...)
	cells := Matrix(r)
	if len(cells) != 12 || cells[0].Input != "i0" || cells[0].Cache != "c0" || cells[11].Input != "i3" || cells[11].Cache != "c2" {
		t.Fatalf("cells %+v", cells)
	}
	cold, warm := cells[6], cells[8] // i2c0, i2c2
	if cold.Input != "i2" || cold.Cache != "c0" || cold.Samples != 5 || cold.TTFTMedian == nil || *cold.TTFTMedian != 2_000_000 ||
		cold.DurationMedian == nil || *cold.DurationMedian != 4_000_000 || cold.TPS == nil || *cold.TPS != 50_000 {
		t.Fatalf("cold %+v", cold)
	}
	if warm.Samples != 4 || warm.TTFTMedian != nil || warm.TPS != nil {
		t.Fatalf("warm below threshold must only count: %+v", warm)
	}
	if cells[0].Samples != 0 || cells[0].TTFTMedian != nil {
		t.Fatalf("empty cell %+v", cells[0])
	}
}

func TestInsights(t *testing.T) {
	r := append(rows(5, 40_000, 0, 2_000_000, 50_000), rows(5, 40_000, 39_000, 1_180_000, 50_000)...)
	r = append(r, rows(6, 1_000, 0, 500_000, 60_000)...)
	r = append(r, rows(6, 1_000, 950, 300_000, 60_000)...)
	r = append(r, rows(5, 200_000, 190_000, 3_000_000, 49_200)...)
	got := Insights(r)
	if len(got) != 2 {
		t.Fatalf("insights %+v", got)
	}
	// i0 has 12 values, i2 10: i0 wins.
	if c := got[0]; c.Kind != InsightCacheTTFT || c.Input != "i0" || c.DeltaPermille != -400 || c.SamplesFrom != 6 || c.SamplesTo != 6 {
		t.Fatalf("cache insight %+v", c)
	}
	if c := got[1]; c.Kind != InsightInputTPS || c.From != "i0" || c.To != "i3" || c.DeltaPermille != -180 {
		t.Fatalf("input insight %+v", c)
	}
	// Tie on count: the smaller class wins.
	tie := append(rows(5, 1_000, 0, 500_000, 1), rows(5, 1_000, 950, 250_000, 1)...)
	tie = append(tie, rows(5, 40_000, 0, 2_000_000, 1)...)
	tie = append(tie, rows(5, 40_000, 39_000, 1_000_000, 1)...)
	if got := Insights(tie); len(got) != 2 || got[0].Input != "i0" || got[0].DeltaPermille != -500 {
		t.Fatalf("tie %+v", got)
	}
	if got := Insights(rows(4, 1_000, 0, 1, 1)); len(got) != 0 {
		t.Fatalf("too few values must give no insight: %+v", got)
	}
}

func TestInsightsOrderIndependent(t *testing.T) {
	r := append(rows(5, 40_000, 0, 2_000_000, 50_000), rows(5, 40_000, 39_000, 1_000_000, 40_000)...)
	r[2].TTFT, r[7].TTFT = 1_900_000, 900_000
	b := slices.Clone(r)
	slices.Reverse(b)
	if !slices.Equal(Insights(r), Insights(b)) {
		t.Fatal("permuted input changes insights")
	}
}
