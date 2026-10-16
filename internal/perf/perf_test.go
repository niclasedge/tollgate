package perf

import (
	"slices"
	"testing"
)

func TestPercentileNearestRank(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in       []int64
		permille int
		want     int64
	}{
		{"one value median", []int64{7}, 500, 7},
		{"one value p90", []int64{7}, 900, 7},
		{"two values median", []int64{1, 2}, 500, 1},
		{"two values p90", []int64{1, 2}, 900, 2},
		{"twenty median", seq(1, 20), 500, 10},
		{"twenty p90", seq(1, 20), 900, 18},
		{"twenty p10", seq(1, 20), 100, 2},
		{"twentyone median", seq(1, 21), 500, 11},
		{"twentyone p90", seq(1, 21), 900, 19},
		{"equal values", []int64{5, 5, 5, 5}, 900, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Percentile(tc.in, tc.permille); got != tc.want {
				t.Fatalf("Percentile(%v, %d) = %d, want %d", tc.in, tc.permille, got, tc.want)
			}
		})
	}
}

func TestClasses(t *testing.T) {
	for _, tc := range []struct {
		n       int64
		in, out string
	}{
		{0, "i0", "o0"}, {255, "i0", "o0"}, {256, "i0", "o1"}, {1023, "i0", "o1"}, {1024, "i0", "o2"},
		{4095, "i0", "o2"}, {4096, "i0", "o3"}, {7999, "i0", "o3"}, {8000, "i1", "o3"},
		{31999, "i1", "o3"}, {32000, "i2", "o3"}, {127999, "i2", "o3"}, {128000, "i3", "o3"},
	} {
		if got := InputClass(tc.n); got != tc.in {
			t.Errorf("InputClass(%d) = %s, want %s", tc.n, got, tc.in)
		}
		if got := OutputClass(tc.n); got != tc.out {
			t.Errorf("OutputClass(%d) = %s, want %s", tc.n, got, tc.out)
		}
	}
}

func TestJudgeThreshold(t *testing.T) {
	// 20 values with median 4.0 s and p90 5.0 s.
	base := make([]int64, 0, 20)
	for i := 0; i < 10; i++ {
		base = append(base, 3_000_000+int64(i)*100_000) // 3.0 .. 3.9 s
	}
	for i := 0; i < 10; i++ {
		base = append(base, 4_000_000+int64(i)*111_111) // 4.0 .. 5.0 s
	}
	med, p90 := Percentile(sorted(base), 500), Percentile(sorted(base), 900)
	for _, tc := range []struct {
		name  string
		value int64
		want  Verdict
	}{
		{"far above", 9_800_000, Slow},
		{"only above p90", p90 + 1, Normal},       // below 1.5 x median
		{"above p90 and 1.5x", med*3/2 + 1, Slow}, // 1 s gap holds as well
		{"at p90", p90, Normal},                   // not above p90
		{"below median", 1_000_000, Normal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := Judge(base, tc.value)
			if j.Verdict != tc.want {
				t.Fatalf("verdict %s, want %s (median %d, p90 %d)", j.Verdict, tc.want, j.Median, j.P90)
			}
			if j.Samples != 20 || j.Median != med || j.P90 != p90 {
				t.Fatalf("stats %+v", j)
			}
		})
	}
}

func TestJudgeSmallValuesNeedOneSecond(t *testing.T) {
	vals := make([]int64, 20)
	for i := range vals {
		vals[i] = 400_000 + int64(i)*5_000 // 0.40 .. 0.495 s
	}
	if j := Judge(vals, 900_000); j.Verdict != Normal {
		t.Fatalf("0.9 s against median 0.45 s: %s, want normal (gap < 1 s)", j.Verdict)
	}
}

func TestJudgeUnknownBelowMinSamples(t *testing.T) {
	j := Judge(seq(1, 19), 1_000_000_000)
	if j.Verdict != Unknown || j.Samples != 19 || j.Median != 0 || j.P90 != 0 {
		t.Fatalf("got %+v, want unknown without stats", j)
	}
}

func TestJudgeOrderIndependent(t *testing.T) {
	a := seq(1_000_000, 1_000_029)
	b := slices.Clone(a)
	slices.Reverse(b)
	b[3], b[17] = b[17], b[3]
	if Judge(a, 5_000_000) != Judge(b, 5_000_000) {
		t.Fatal("permuted input changes the result")
	}
	if a[0] != 1_000_000 {
		t.Fatal("Judge modified its input")
	}
}

func TestJudgeDurationFallback(t *testing.T) {
	var samples []Sample
	// 12 values in i2o1, 28 more in other input classes with o1: o1 has 40.
	for i := 0; i < 12; i++ {
		samples = append(samples, Sample{InputTotal: 40_000, Output: 500, Duration: 4_000_000})
	}
	for i := 0; i < 28; i++ {
		samples = append(samples, Sample{InputTotal: 1_000, Output: 500, Duration: 4_000_000})
	}
	j := JudgeDuration(samples, 40_000, 500, 9_800_000)
	if j.Class != "o1" || j.Samples != 40 || j.Verdict != Slow {
		t.Fatalf("got %+v, want class o1 with 40 samples, slow", j)
	}
	// Enough in the fine class: no fallback.
	for i := 0; i < 8; i++ {
		samples = append(samples, Sample{InputTotal: 40_000, Output: 500, Duration: 4_000_000})
	}
	if j := JudgeDuration(samples, 40_000, 500, 4_000_000); j.Class != "i2o1" || j.Samples != 20 {
		t.Fatalf("got %+v, want class i2o1 with 20 samples", j)
	}
}

func TestJudgeTTFTUsesInputClassAndKnownValues(t *testing.T) {
	var samples []Sample
	for i := 0; i < 20; i++ {
		samples = append(samples, Sample{InputTotal: 10_000, TTFT: 1_000_000, HasTTFT: true})
	}
	samples = append(samples, Sample{InputTotal: 10_000}) // no TTFT: ignored
	samples = append(samples, Sample{InputTotal: 100, TTFT: 50, HasTTFT: true})
	j := JudgeTTFT(samples, 9_000, 3_000_000)
	if j.Class != "i1" || j.Samples != 20 || j.Verdict != Slow || j.Metric != MetricTTFT {
		t.Fatalf("got %+v", j)
	}
}

func TestOutputTPS(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		output, duration, ttft int64
		hasTTFT, stream        bool
		want                   float64
		ok                     bool
	}{
		{"spec example", 200, 5_000_000, 1_000_000, true, true, 50.0, true},
		{"rounds to tenths", 100, 4_000_000, 1_000_000, true, true, 33.3, true},
		{"rounds half up", 1, 1_600_000, 0, true, true, 0.6, true},
		{"json", 200, 5_000_000, 1_000_000, true, false, 0, false},
		{"no output", 0, 5_000_000, 1_000_000, true, true, 0, false},
		{"no ttft", 200, 5_000_000, 0, false, true, 0, false},
		{"ttft equals duration", 200, 5_000_000, 5_000_000, true, true, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := OutputTPS(tc.output, tc.duration, tc.ttft, tc.hasTTFT, tc.stream)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("got %v %v, want %v %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestTollgateMicros(t *testing.T) {
	if got := TollgateMicros([]int64{300, 1_200}, 0); got != 1_500 {
		t.Fatalf("got %d, want 1500", got)
	}
	if got := TollgateMicros([]int64{300}, 700); got != 1_000 {
		t.Fatalf("got %d, want 1000", got)
	}
}

func seq(from, to int64) []int64 {
	var out []int64
	for v := from; v <= to; v++ {
		out = append(out, v)
	}
	return out
}

func sorted(v []int64) []int64 {
	c := slices.Clone(v)
	slices.Sort(c)
	return c
}
