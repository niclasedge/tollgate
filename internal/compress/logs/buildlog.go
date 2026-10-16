// Ported from Headroom
// crates/headroom-core/src/transforms/pipeline/reformats/log_template.rs
// (LogTemplate: run mining, extends_run, merge_into_template, flush_run) and
// from crates/headroom-core/src/transforms/log_compressor.rs
// (normalize_for_dedupe for repeated warnings).
// Headroom (Apache-2.0, Copyright 2025 Headroom Contributors), Stand 0947a28,
// portiert und verändert: runs share the first token, digits are masked and
// wildcard positions are ignored when lines are compared, a template needs
// one word-like constant instead of two, long variant tables are shortened, errors, warnings and summaries never
// join a run, and a collapse that would not shrink the text is skipped.

package logs

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

const (
	minRun        = 3 // lines that make a run
	variantsHead  = 2 // variant lines kept at the start of a long run
	variantsTail  = 2 // ... and at the end
	tailKeep      = 3 // last lines stay verbatim (final summary)
	wildcard      = "<*>"
	similarityPct = 40 // share of matching positions (Drain default)
)

var (
	reWordLike  = regexp.MustCompile(`[A-Za-z]{3,}`)
	reNumPipe   = regexp.MustCompile(`^\d+\s*\|`)
	reBuildSum  = regexp.MustCompile(`^\s*((added|removed|changed|audited) \d+ packages?|up to date|Done in |Lockfile is up to date|Packages: |Successfully (built|tagged)|Finished\s|found \d+ vulnerabilit)`)
	reDigitMask = regexp.MustCompile(`\d+`)
)

type kind int

const (
	kindPlain kind = iota // may join a run
	kindKeep              // verbatim
	kindDrop              // repeated warning
)

// BuildLog is the strategy build_log: runs of at least three similar lines
// (progress, downloads, "Compiling x v1.2") collapse into one template line
// plus a short variant table, repeated warnings appear once. Errors,
// warnings and the final summary stay intact.
func BuildLog(out string, _ strat.Params, t strat.Trace) strat.Result {
	lines := splitLines(out)
	n := len(lines)
	if n < minLines {
		return strat.Result{Text: out}
	}
	infos := make([]info, n)
	for i, l := range lines {
		infos[i] = classifyLine(l)
	}

	// Pass 1: mark errors, warnings and summaries as verbatim; drop
	// repeated warning blocks.
	kinds := make([]kind, n)
	seen := map[string]bool{} // membership only
	nErr, warnDropped := 0, 0
	for i := 0; i < n; {
		in := infos[i]
		if i >= n-tailKeep || strings.TrimSpace(in.text) == "" {
			kinds[i] = kindKeep
			i++
			continue
		}
		special := in.level == levelError || in.level == levelFail || in.level == levelWarn ||
			in.summary || in.passSum || reBuildSum.MatchString(in.text)
		if !special || in.noise {
			i++
			continue
		}
		j := blockEnd(lines, i)
		if in.level == levelWarn {
			key := blockKey(lines[i:j])
			if seen[key] {
				for k := i; k < j; k++ {
					kinds[k] = kindDrop
				}
				if j < n-tailKeep && strings.TrimSpace(lines[j]) == "" {
					kinds[j] = kindDrop
					j++
				}
				warnDropped++
				i = j
				continue
			}
			seen[key] = true
		}
		if in.level == levelError || in.level == levelFail {
			nErr++
		}
		for k := i; k < j; k++ {
			kinds[k] = kindKeep
		}
		i = j
	}
	t.Step("dedupe", joinLines(lines, func(i int) bool { return kinds[i] != kindDrop }))

	// Pass 2: collapse runs of similar plain lines.
	var res []string
	nextID, collapsed := 1, 0
	for i := 0; i < n; {
		if kinds[i] == kindDrop {
			i++
			continue
		}
		toks := strings.Fields(lines[i])
		if kinds[i] != kindPlain || len(toks) == 0 {
			res = append(res, lines[i])
			i++
			continue
		}
		r := newRun(i, toks)
		j := i + 1
		for j < n && kinds[j] == kindPlain {
			tk := strings.Fields(lines[j])
			if len(tk) == 0 || !r.extends(tk) {
				break
			}
			r.add(j, tk)
			j++
		}
		if block, ok := r.collapse(lines, nextID); ok {
			res = append(res, block...)
			nextID++
			collapsed += len(r.idx)
		} else {
			for k := i; k < j; k++ {
				res = append(res, lines[k])
			}
		}
		i = j
	}
	text := strings.Join(res, "\n")
	t.Step("template", text)

	if len(text) >= len(strings.Join(lines, "\n")) {
		return strat.Result{Text: out}
	}
	var details []string
	if nErr > 0 {
		details = append(details, strconv.Itoa(nErr)+" Fehler")
	}
	if warnDropped > 0 {
		details = append(details, strconv.Itoa(warnDropped)+" Warnungen zusammengefasst")
	}
	if collapsed > 0 {
		details = append(details, strconv.Itoa(collapsed)+" gleichartige Zeilen")
	}
	return strat.Result{Text: text, Omitted: max(0, n-len(res)), Details: details}
}

// blockEnd returns the end (exclusive) of the diagnostic block that starts at
// i. Only compiler-style headers ("warning: ...", "error[E0425]: ...") own the
// indented lines below them; every other line is a block of one.
func blockEnd(lines []string, i int) int {
	j := i + 1
	if !reCargoDiag.MatchString(lines[i]) {
		return j
	}
	for j < len(lines) && strings.TrimSpace(lines[j]) != "" &&
		(reIndented.MatchString(lines[j]) || reNumPipe.MatchString(lines[j])) {
		j++
	}
	return j
}

// blockKey is the dedupe key of a block: the normalized lines.
func blockKey(block []string) string {
	norm := make([]string, len(block))
	for i, l := range block {
		norm[i] = normalizeForDedupe(l)
	}
	return strings.Join(norm, "\n")
}

// run is a candidate for collapsing: line indices and the accumulated
// template (wild[i] marks a varying position).
type run struct {
	idx  []int
	toks [][]string
	tmpl []string
	wild []bool
}

func newRun(i int, toks []string) *run {
	return &run{idx: []int{i}, toks: [][]string{toks}, tmpl: append([]string(nil), toks...), wild: make([]bool, len(toks))}
}

func mask(tok string) string { return reDigitMask.ReplaceAllString(tok, "0") }

// extends reports whether toks belongs to the run: same length, same first
// token, and enough of the still constant positions match (digit-masked).
// Wildcard positions are not counted: otherwise an old run drifts to all
// wildcards and swallows unrelated lines of the same length.
func (r *run) extends(toks []string) bool {
	if len(toks) != len(r.tmpl) || toks[0] != r.tmpl[0] {
		return false
	}
	matches, constants := 0, 0
	for pos, tok := range toks {
		if r.wild[pos] {
			continue
		}
		constants++
		if mask(tok) == mask(r.tmpl[pos]) {
			matches++
		}
	}
	return matches*100 >= constants*similarityPct
}

func (r *run) add(i int, toks []string) {
	r.idx = append(r.idx, i)
	r.toks = append(r.toks, toks)
	for pos, tok := range toks {
		if !r.wild[pos] && r.tmpl[pos] != tok {
			r.wild[pos] = true
		}
	}
}

// collapse renders the run as template header plus variant lines. It
// returns false when the run is too short, has no word-like constant or the
// result would not be smaller.
func (r *run) collapse(lines []string, id int) ([]string, bool) {
	if len(r.idx) < minRun {
		return nil, false
	}
	anchors, varying := 0, 0
	parts := make([]string, len(r.tmpl))
	for pos, tok := range r.tmpl {
		if r.wild[pos] {
			varying++
			parts[pos] = wildcard
			continue
		}
		parts[pos] = tok
		if reWordLike.MatchString(tok) {
			anchors++
		}
	}
	if anchors < 1 || varying == 0 {
		return nil, false
	}
	block := []string{"[Template T" + strconv.Itoa(id) + ": " + strings.Join(parts, " ") +
		"] (" + strconv.Itoa(len(r.idx)) + " Zeilen)"}
	variant := func(k int) string {
		var v []string
		for pos, tok := range r.toks[k] {
			if r.wild[pos] {
				v = append(v, tok)
			}
		}
		return strings.Join(v, " ")
	}
	total := len(r.idx)
	if total > variantsHead+variantsTail+2 {
		for k := range variantsHead {
			block = append(block, variant(k))
		}
		block = append(block, "[… "+strconv.Itoa(total-variantsHead-variantsTail)+" weitere]")
		for k := range variantsTail {
			block = append(block, variant(total-variantsTail+k))
		}
	} else {
		for k := 0; k < total; k++ {
			block = append(block, variant(k))
		}
	}
	orig := make([]string, total)
	for k, i := range r.idx {
		orig[k] = lines[i]
	}
	if len(strings.Join(block, "\n")) >= len(strings.Join(orig, "\n")) {
		return nil, false
	}
	return block, true
}
