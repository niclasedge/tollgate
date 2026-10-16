// Package perf holds the pure statistics of the performance view (#87):
// size classes, nearest-rank percentiles and the "slow" verdict. Everything
// works on integers, so equal inputs give byte-identical results.
package perf

import (
	"slices"
	"time"
)

const (
	// MinSamples is the least number of comparison values for a verdict.
	MinSamples = 20
	// Window is how far back comparison values reach.
	Window = 14 * 24 * time.Hour
	// MaxSamples caps the comparison values (newest first).
	MaxSamples = 500
	// slowGapMicros is the least distance to the median for "slow".
	slowGapMicros = 1_000_000
)

// Metric names stored in trace_perf.
const (
	MetricTTFT     = "ttft"
	MetricDuration = "duration"
)

// Verdict is the judgement of one metric.
type Verdict string

const (
	Normal  Verdict = "normal"
	Slow    Verdict = "langsam"
	Unknown Verdict = "unbekannt"
)

// Sample is one comparison request: sizes and measurements in microseconds.
type Sample struct {
	InputTotal int64 // input + cache read + cache creation
	Output     int64
	TTFT       int64
	HasTTFT    bool
	Duration   int64
}

// Judgement is the verdict of one metric with the numbers that explain it.
// Median and P90 are 0 when the verdict is Unknown.
type Judgement struct {
	Metric  string
	Class   string
	Samples int
	Median  int64
	P90     int64
	Value   int64
	Verdict Verdict
}

// Percentile returns the nearest-rank percentile x[ceil(q*n)-1] of sorted
// values, q given in permille. It returns 0 for no values.
func Percentile(sorted []int64, permille int) int64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	i := (permille*n+999)/1000 - 1
	return sorted[max(0, min(i, n-1))]
}

// InputClass is the size class of the input tokens including the cache.
func InputClass(inputTotal int64) string {
	switch {
	case inputTotal < 8_000:
		return "i0"
	case inputTotal < 32_000:
		return "i1"
	case inputTotal < 128_000:
		return "i2"
	}
	return "i3"
}

// OutputClass is the size class of the output tokens.
func OutputClass(output int64) string {
	switch {
	case output < 256:
		return "o0"
	case output < 1_024:
		return "o1"
	case output < 4_096:
		return "o2"
	}
	return "o3"
}

// Judge compares value with the comparison values (any order; not modified).
// It is slow when value is above p90, at least 1.5 x the median and at least
// 1 s above it; with fewer than MinSamples values it is Unknown.
func Judge(values []int64, value int64) Judgement {
	j := Judgement{Samples: len(values), Value: value, Verdict: Unknown}
	if len(values) < MinSamples {
		return j
	}
	s := slices.Clone(values)
	slices.Sort(s)
	j.Median, j.P90 = Percentile(s, 500), Percentile(s, 900)
	j.Verdict = Normal
	if value > j.P90 && 2*value >= 3*j.Median && value-j.Median >= slowGapMicros {
		j.Verdict = Slow
	}
	return j
}

// JudgeTTFT judges a time to first token against samples of the same input
// class.
func JudgeTTFT(samples []Sample, inputTotal, ttft int64) Judgement {
	class := InputClass(inputTotal)
	var vals []int64
	for _, s := range samples {
		if s.HasTTFT && InputClass(s.InputTotal) == class {
			vals = append(vals, s.TTFT)
		}
	}
	j := Judge(vals, ttft)
	j.Metric, j.Class = MetricTTFT, class
	return j
}

// JudgeDuration judges a total duration against samples of the same input and
// output class, falling back once to the output class alone when the fine
// class has fewer than MinSamples values.
func JudgeDuration(samples []Sample, inputTotal, output, duration int64) Judgement {
	in, out := InputClass(inputTotal), OutputClass(output)
	var fine, coarse []int64
	for _, s := range samples {
		if OutputClass(s.Output) != out {
			continue
		}
		coarse = append(coarse, s.Duration)
		if InputClass(s.InputTotal) == in {
			fine = append(fine, s.Duration)
		}
	}
	class, vals := in+out, fine
	if len(fine) < MinSamples {
		class, vals = out, coarse
	}
	j := Judge(vals, duration)
	j.Metric, j.Class = MetricDuration, class
	return j
}

// AnySlow reports whether one of the judgements is Slow.
func AnySlow(js []Judgement) bool {
	for _, j := range js {
		if j.Verdict == Slow {
			return true
		}
	}
	return false
}

// OutputTPS is output tokens per second after the generation start (the
// flush of message_start), rounded half up to one decimal. It is only defined
// for streams with output, a known generation start and a duration beyond it.
// The first content delta is no base: with hidden thinking it only arrives
// after all thinking tokens, which still count as output.
func OutputTPS(output, duration, genStart int64, hasStart, stream bool) (float64, bool) {
	gen := duration - genStart
	if !stream || !hasStart || output < 1 || gen <= 0 {
		return 0, false
	}
	tenths := (output*20_000_000 + gen) / (2 * gen) // output*10^7/gen, rounded
	return float64(tenths) / 10, true
}

// TollgateMicros is the time tollgate itself spends: all stages before the
// forward plus the return path.
func TollgateMicros(stageMicros []int64, returnMicros int64) int64 {
	sum := returnMicros
	for _, v := range stageMicros {
		sum += v
	}
	return sum
}
