// Ported from Headroom crates/headroom-core/src/transforms/log_compressor.rs
// (LogCompressor::select_lines, select_with_first_last, dedupe_similar,
// format_output).
// Headroom (Apache-2.0, Copyright 2025 Headroom Contributors), Stand 0947a28,
// portiert und verändert: fixed limits instead of adaptive_sizer, no CCR, the
// footer is the marker line the stage appends, failure markers and pass
// summaries are handled separately (see classify.go).

package logs

import (
	"sort"
	"strconv"
	"strings"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

const (
	defaultMaxErrors    = 5
	defaultContextLines = 3
	minLines            = 10  // shorter outputs stay as they are
	maxWarnings         = 5   // unique warnings kept
	maxStackTraces      = 3   // stack traces kept
	maxTotalLines       = 150 // fixed replacement of adaptive_sizer
	passSummaryTail     = 4   // ok/PASS lines of passing packages kept
)

// splitLines splits on "\n" and drops the empty element after a final
// newline.
func splitLines(s string) []string {
	lines := strings.Split(s, "\n")
	if n := len(lines); n > 1 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// TestLog is the strategy test_log: it keeps failures, errors, unique
// warnings, stack traces and summary lines of test and build output, each
// with context, and drops the rest.
func TestLog(out string, p strat.Params, t strat.Trace) strat.Result {
	lines := splitLines(out)
	if len(lines) < minLines {
		return strat.Result{Text: out}
	}
	maxErrors, ctx := p.MaxErrors, p.ContextLines
	if maxErrors <= 0 {
		maxErrors = defaultMaxErrors
	}
	if ctx <= 0 {
		ctx = defaultContextLines
	}
	n := len(lines)
	infos := parse(lines)

	// Step 1: drop repeated warnings.
	dropped := make([]bool, n)
	seen := map[string]bool{} // membership only
	warnDropped := 0
	var warns []int
	for i, in := range infos {
		if in.level != levelWarn || in.noise || in.passSum {
			continue
		}
		key := normalizeForDedupe(in.text)
		if seen[key] {
			dropped[i] = true
			warnDropped++
			continue
		}
		seen[key] = true
		warns = append(warns, i)
	}
	t.Step("dedupe", joinLines(lines, func(i int) bool { return !dropped[i] }))

	// Step 2: select by category.
	sel := make([]bool, n)
	var errs, fails, sums, passSums []int
	var runs [][]int
	var cur []int
	goOK := 0
	for i, in := range infos {
		if dropped[i] {
			continue
		}
		switch {
		case in.noise:
		case in.passSum:
			passSums = append(passSums, i)
			if reGoOK.MatchString(in.text) {
				goOK++
			}
		case in.level == levelError:
			errs = append(errs, i)
		case in.level == levelFail:
			fails = append(fails, i)
		}
		if in.summary {
			sums = append(sums, i)
		}
		if in.hard {
			sel[i] = true
		}
		if in.trace {
			cur = append(cur, i)
		} else if len(cur) > 0 {
			runs = append(runs, cur)
			cur = nil
		}
	}
	if len(cur) > 0 {
		runs = append(runs, cur)
	}
	for _, i := range pickFirstLast(errs, infos, maxErrors) {
		sel[i] = true
	}
	for _, i := range pickFirstLast(fails, infos, maxErrors) {
		sel[i] = true
	}
	if len(warns) > maxWarnings {
		warns = warns[:maxWarnings]
	}
	for _, i := range warns {
		sel[i] = true
	}
	for r := 0; r < len(runs) && r < maxStackTraces; r++ {
		for k := 0; k < len(runs[r]) && k < stackTraceMaxLines; k++ {
			sel[runs[r][k]] = true
		}
	}
	for _, i := range sums {
		sel[i] = true
	}
	if len(passSums) > passSummaryTail {
		passSums = passSums[len(passSums)-passSummaryTail:]
	}
	for _, i := range passSums {
		sel[i] = true
	}
	t.Step("select", joinLines(lines, func(i int) bool { return sel[i] }))

	// Step 3: context around errors, failures, warnings and trace starts.
	final := append([]bool(nil), sel...)
	for i := range lines {
		if !sel[i] {
			continue
		}
		in := infos[i]
		src := in.hard || in.level == levelError || in.level == levelFail || in.level == levelWarn
		if in.trace && (i == 0 || !infos[i-1].trace) {
			src = true
		}
		if !src || in.noise || in.passSum {
			continue
		}
		for j := max(0, i-ctx); j < min(n, i+ctx+1); j++ {
			if !dropped[j] && !infos[j].noise && !infos[j].passSum {
				final[j] = true
			}
		}
	}

	// Step 4: global cap by score, then restore line order.
	var idx []int
	for i, ok := range final {
		if ok {
			idx = append(idx, i)
		}
	}
	if len(idx) > maxTotalLines {
		sort.Slice(idx, func(a, b int) bool {
			sa, sb := infos[idx[a]].score, infos[idx[b]].score
			if sa != sb {
				return sa > sb
			}
			return idx[a] < idx[b]
		})
		idx = idx[:maxTotalLines]
		sort.Ints(idx)
	}
	if len(idx) == 0 { // nothing recognizable: keep the tail
		for i := max(0, n-10); i < n; i++ {
			idx = append(idx, i)
		}
	}
	kept := make([]string, len(idx))
	for k, i := range idx {
		kept[k] = lines[i]
	}
	text := strings.Join(kept, "\n")
	t.Step("context", text)

	// Facts in a fixed order.
	var nErr, nFail int
	for _, in := range infos {
		if in.noise || in.passSum || isResultLine(in.text) {
			continue
		}
		switch in.level {
		case levelError:
			nErr++
		case levelFail:
			nFail++
		}
	}
	var details []string
	if nErr > 0 {
		details = append(details, strconv.Itoa(nErr)+" Fehler")
	}
	if nFail > 0 {
		details = append(details, strconv.Itoa(nFail)+" Fehlschläge")
	}
	if warnDropped > 0 {
		details = append(details, strconv.Itoa(warnDropped)+" Warnungen zusammengefasst")
	}
	if len(runs) > 0 {
		details = append(details, strconv.Itoa(len(runs))+" Stacktraces")
	}
	if goOK > 0 {
		details = append(details, strconv.Itoa(goOK)+" Pakete ok")
	}
	return strat.Result{Text: text, Omitted: n - len(kept), Details: details}
}

func joinLines(lines []string, keep func(int) bool) string {
	var b strings.Builder
	first := true
	for i, l := range lines {
		if !keep(i) {
			continue
		}
		if !first {
			b.WriteByte('\n')
		}
		b.WriteString(l)
		first = false
	}
	return b.String()
}

// pickFirstLast keeps the first and the last entry and fills up to max with
// the highest scores (ties: earlier line first).
func pickFirstLast(idx []int, infos []info, max int) []int {
	if len(idx) <= max {
		return idx
	}
	out := []int{idx[0], idx[len(idx)-1]}
	rest := append([]int(nil), idx[1:len(idx)-1]...)
	sort.SliceStable(rest, func(a, b int) bool {
		sa, sb := infos[rest[a]].score, infos[rest[b]].score
		if sa != sb {
			return sa > sb
		}
		return rest[a] < rest[b]
	})
	for _, i := range rest {
		if len(out) >= max {
			break
		}
		out = append(out, i)
	}
	return out
}
