package perf

import "slices"

// minCellSamples is the least number of values for a median in the matrix.
const minCellSamples = 5

// InputClasses and CacheClasses are the axes of the influence matrix (#116).
var (
	InputClasses = []string{"i0", "i1", "i2", "i3"}
	CacheClasses = []string{"c0", "c1", "c2"}
)

// Insight kinds.
const (
	InsightCacheTTFT = "cache_ttft"
	InsightInputTPS  = "input_tps"
)

// CacheClass is the class of the cache-read share of the input in permille:
// c0 below 500, c1 up to 899, c2 from 900.
func CacheClass(cacheRead, inputTotal int64) string {
	if inputTotal <= 0 {
		return "c0"
	}
	switch share := cacheRead * 1000 / inputTotal; {
	case share < 500:
		return "c0"
	case share < 900:
		return "c1"
	}
	return "c2"
}

// InfluenceRow is one finished request for the matrix.
type InfluenceRow struct {
	InputTotal, CacheRead int64
	TTFT                  int64
	HasTTFT               bool
	Duration              int64
	TPSMilli              int64
	HasTPS                bool // tokens/s known and output >= MinTPSOutput
}

// Cell is one input × cache class. Medians are nil below 5 values.
type Cell struct {
	Input, Cache                    string
	Samples                         int
	TTFTMedian, DurationMedian, TPS *int64
}

// Insight is one deterministic statement from the matrix.
type Insight struct {
	Kind                   string
	Input, From, To        string
	DeltaPermille          int64
	SamplesFrom, SamplesTo int
}

func median(vals []int64) *int64 {
	if len(vals) < minCellSamples {
		return nil
	}
	s := slices.Clone(vals)
	slices.Sort(s)
	m := Percentile(s, 500)
	return &m
}

// Matrix returns all 12 cells, ordered by input then cache class.
func Matrix(rows []InfluenceRow) []Cell {
	type acc struct {
		n              int
		ttft, dur, tps []int64
	}
	accs := map[[2]string]*acc{}
	for _, r := range rows {
		k := [2]string{InputClass(r.InputTotal), CacheClass(r.CacheRead, r.InputTotal)}
		a := accs[k]
		if a == nil {
			a = &acc{}
			accs[k] = a
		}
		a.n++
		a.dur = append(a.dur, r.Duration)
		if r.HasTTFT {
			a.ttft = append(a.ttft, r.TTFT)
		}
		if r.HasTPS {
			a.tps = append(a.tps, r.TPSMilli)
		}
	}
	out := make([]Cell, 0, len(InputClasses)*len(CacheClasses))
	for _, in := range InputClasses {
		for _, c := range CacheClasses {
			cell := Cell{Input: in, Cache: c}
			if a := accs[[2]string{in, c}]; a != nil {
				cell.Samples = a.n
				cell.TTFTMedian, cell.DurationMedian, cell.TPS = median(a.ttft), median(a.dur), median(a.tps)
			}
			out = append(out, cell)
		}
	}
	return out
}

// Insights returns at most two statements: how a warm cache changes TTFT and
// how the input size changes tokens/s.
func Insights(rows []InfluenceRow) []Insight {
	var out []Insight
	ttft := map[[2]string][]int64{}
	tps := map[string][]int64{}
	for _, r := range rows {
		in := InputClass(r.InputTotal)
		if r.HasTTFT {
			k := [2]string{in, CacheClass(r.CacheRead, r.InputTotal)}
			ttft[k] = append(ttft[k], r.TTFT)
		}
		if r.HasTPS {
			tps[in] = append(tps[in], r.TPSMilli)
		}
	}
	best := -1
	var cache Insight
	for _, in := range InputClasses { // ascending: ties keep the smaller class
		cold, warm := ttft[[2]string{in, "c0"}], ttft[[2]string{in, "c2"}]
		mc, mw := median(cold), median(warm)
		if mc == nil || mw == nil || *mc == 0 {
			continue
		}
		if n := len(cold) + len(warm); n > best {
			best = n
			cache = Insight{Kind: InsightCacheTTFT, Input: in, From: "c0", To: "c2",
				DeltaPermille: (*mw - *mc) * 1000 / *mc, SamplesFrom: len(cold), SamplesTo: len(warm)}
		}
	}
	if best >= 0 {
		out = append(out, cache)
	}
	var lo, hi string
	for _, in := range InputClasses {
		if median(tps[in]) == nil {
			continue
		}
		if lo == "" {
			lo = in
		}
		hi = in
	}
	if lo != "" && lo != hi {
		ml, mh := *median(tps[lo]), *median(tps[hi])
		if ml > 0 {
			out = append(out, Insight{Kind: InsightInputTPS, From: lo, To: hi,
				DeltaPermille: (mh - ml) * 1000 / ml, SamplesFrom: len(tps[lo]), SamplesTo: len(tps[hi])})
		}
	}
	return out
}
