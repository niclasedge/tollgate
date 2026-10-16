package perf

import "slices"

// MinTPSOutput is the least output for a tokens/s value (#116): with shorter
// answers the last network flush dominates and tokens/s is noise.
const MinTPSOutput = 64

// tpsGapPermille is the least deviation from the median for faster/slower.
const tpsGapPermille = 200

// AllInputs is the class of the fallback to the whole model.
const AllInputs = "*"

// TPSVerdict is the judgement of tokens/s.
type TPSVerdict string

const (
	TPSFaster  TPSVerdict = "schneller"
	TPSUsual   TPSVerdict = "üblich"
	TPSSlower  TPSVerdict = "langsamer"
	TPSUnknown TPSVerdict = "unbekannt"
)

// TPSSample is one comparison value: input size and milli-tokens per second.
type TPSSample struct {
	InputTotal int64
	TPSMilli   int64
}

// TPSJudgement explains a tokens/s verdict. P10, Median, P90 and Deviation
// are 0 when the verdict is TPSUnknown.
type TPSJudgement struct {
	Class             string
	Samples           int
	TPSMilli          int64
	P10, Median, P90  int64
	DeviationPermille int64
	Verdict           TPSVerdict
}

// TPSMilli is output tokens per second after the generation start in
// milli-tokens (integer division). It has the same conditions as OutputTPS.
func TPSMilli(output, duration, genStart int64, hasStart, stream bool) (int64, bool) {
	gen := duration - genStart
	if !stream || !hasStart || output < 1 || gen <= 0 {
		return 0, false
	}
	return output * 1_000_000_000 / gen, true
}

// JudgeTPS compares tpsMilli with samples of the same input class, falling
// back once to all samples of the model (class "*") when the class has fewer
// than MinSamples values. Answers below MinTPSOutput are not judged.
func JudgeTPS(samples []TPSSample, inputTotal, output, tpsMilli int64) TPSJudgement {
	class := InputClass(inputTotal)
	j := TPSJudgement{Class: class, TPSMilli: tpsMilli, Verdict: TPSUnknown}
	if output < MinTPSOutput {
		return j
	}
	var fine, all []int64
	for _, s := range samples {
		all = append(all, s.TPSMilli)
		if InputClass(s.InputTotal) == class {
			fine = append(fine, s.TPSMilli)
		}
	}
	vals := fine
	if len(fine) < MinSamples {
		j.Class, vals = AllInputs, all
	}
	j.Samples = len(vals)
	if len(vals) < MinSamples {
		return j
	}
	s := slices.Clone(vals)
	slices.Sort(s)
	j.P10, j.Median, j.P90 = Percentile(s, 100), Percentile(s, 500), Percentile(s, 900)
	if j.Median > 0 {
		j.DeviationPermille = (tpsMilli - j.Median) * 1000 / j.Median
	}
	switch {
	case tpsMilli < j.P10 && j.DeviationPermille <= -tpsGapPermille:
		j.Verdict = TPSSlower
	case tpsMilli > j.P90 && j.DeviationPermille >= tpsGapPermille:
		j.Verdict = TPSFaster
	default:
		j.Verdict = TPSUsual
	}
	return j
}
