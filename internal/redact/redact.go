package redact

import (
	"bytes"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// Finding is one resolved secret. Start and End are byte offsets of the
// secret group in the original text.
type Finding struct {
	Start    int
	End      int
	Rule     string // detector name, e.g. "anthropic_key"; visible in traces only
	Type     string // placeholder type, e.g. "api_key"
	Priority int
	order    int // position in the rule table, final tie-breaker
}

// placeholderRE recognises placeholders of the ADR-0006 grammar. Text in
// this form is never treated as a secret. Group 1 is the type, group 2 the
// number.
var placeholderRE = regexp.MustCompile(`<redacted:([a-z][a-z0-9_]{0,31})_([1-9][0-9]{0,5})>`)

// Redactor runs the enabled built-in detectors. It is immutable and safe for
// concurrent use.
type Redactor struct {
	rules []indexedRule
	allow map[string]struct{} // allowlist (issue #15), exact secret values; lookup only
	// noPrefilter runs every regex regardless of need (tests only, #84).
	noPrefilter bool
}

type indexedRule struct {
	rule
	order int
}

// New returns a Redactor for the given switches.
func New(opts Options) *Redactor {
	r := &Redactor{}
	for i, rl := range builtinRules {
		if rl.enabled(opts) {
			r.rules = append(r.rules, indexedRule{rule: rl, order: i})
		}
	}
	return r
}

// Find runs all enabled detectors on the original text, drops false
// positives and candidates that touch an existing placeholder, resolves
// overlaps (priority, then longer span, then earlier start, then rule order)
// and returns the accepted findings sorted by start. The uncovered rest of a
// displaced generic candidate is accepted as findings of its own (#83).
func (r *Redactor) Find(text string) []Finding {
	placeholders := placeholderRE.FindAllStringIndex(text, -1)
	var cands []Finding
	// keep holds the non-secret parts of every candidate match (prefix and
	// suffix around the secret group). Residuals inside them stay (#83).
	var keep [][2]int
	pf := r.newPrefilter(text)
	for _, rl := range r.rules {
		if !pf.pass(rl.rule) {
			continue
		}
		for _, m := range rl.matches(text) {
			start, end := m[2*rl.group], m[2*rl.group+1]
			if rl.trim != nil && start >= 0 {
				start, end = rl.trim(text, start, end)
			}
			if start < 0 || start == end {
				continue
			}
			before, after := contextAround(text, m[0], m[1])
			if isFalsePositive(text[m[0]:m[1]], rl.name, before, after) {
				continue
			}
			if overlapsAny(start, end, placeholders) {
				continue
			}
			if _, ok := r.allow[text[start:end]]; ok {
				continue
			}
			if m[0] < start {
				keep = append(keep, [2]int{m[0], start})
			}
			if end < m[1] {
				keep = append(keep, [2]int{end, m[1]})
			}
			cands = append(cands, Finding{
				Start: start, End: end,
				Rule: rl.name, Type: rl.typ, Priority: rl.priority,
				order: rl.order,
			})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if la, lb := a.End-a.Start, b.End-b.Start; la != lb {
			return la > lb
		}
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		return a.order < b.order
	})
	// covered marks bytes of accepted secrets. Matches of one rule never
	// overlap each other, so marking and checking costs at most
	// len(rules) * len(text) steps in total.
	covered := make([]bool, len(text))
	var accepted []Finding
	accept := func(f Finding) {
		for i := f.Start; i < f.End; i++ {
			covered[i] = true
		}
		accepted = append(accepted, f)
	}
	for _, c := range cands {
		if !slices.Contains(covered[c.Start:c.End], true) {
			accept(c)
			continue
		}
		if c.Priority != prioGeneric {
			continue
		}
		// A displaced generic finding: its uncovered rest must not stay in
		// plain text (#83). Each rest becomes a finding of the same rule.
		for _, rest := range r.residuals(text, c, covered, keep) {
			accept(rest)
		}
	}
	sort.Slice(accepted, func(i, j int) bool { return accepted[i].Start < accepted[j].Start })
	return accepted
}

// residuals returns the uncovered rests of the displaced candidate c (#83,
// ADR-0005 Nachtrag). Every maximal uncovered run of c's secret span is
// trimmed of surrounding whitespace and dropped if it holds no letter or
// digit, lies completely inside the non-secret part of a candidate match
// (e.g. the scheme "Bearer" of bearer_token or the key name "password=" of
// generic_assign), is an auth scheme of bearer_token (also when the token
// is too short for bearer_token to match) or is allowlisted.
func (r *Redactor) residuals(text string, c Finding, covered []bool, keep [][2]int) []Finding {
	var out []Finding
	for i := c.Start; i < c.End; {
		if covered[i] {
			i++
			continue
		}
		j := i
		for j < c.End && !covered[j] {
			j++
		}
		s, e := trimSpace(text, i, j)
		i = j
		if s >= e || !hasLetterOrDigit(text[s:e]) || insideAny(s, e, keep) || isAuthScheme(text[s:e]) {
			continue
		}
		if _, ok := r.allow[text[s:e]]; ok {
			continue
		}
		rest := c
		rest.Start, rest.End = s, e
		out = append(out, rest)
	}
	return out
}

// authSchemes are the schemes bearer_token keeps as prefix.
var authSchemes = []string{"Bearer", "Basic", "Digest"}

func isAuthScheme(s string) bool {
	for _, a := range authSchemes {
		if strings.EqualFold(s, a) {
			return true
		}
	}
	return false
}

// trimSpace narrows [s, e) by Unicode whitespace on both ends.
func trimSpace(text string, s, e int) (int, int) {
	t := strings.TrimLeftFunc(text[s:e], unicode.IsSpace)
	s = e - len(t)
	return s, s + len(strings.TrimRightFunc(t, unicode.IsSpace))
}

func hasLetterOrDigit(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
}

// insideAny reports whether [s, e) lies completely inside one of spans.
func insideAny(s, e int, spans [][2]int) bool {
	for _, sp := range spans {
		if sp[0] <= s && e <= sp[1] {
			return true
		}
	}
	return false
}

// prefilter decides per rule whether its regex can match text at all
// (#84): a rule runs only if text contains one of its need literals.
type prefilter struct {
	text   string
	folded []byte // text folded for (?i) literals, built on first use
	ready  bool
	off    bool
}

func (r *Redactor) newPrefilter(text string) *prefilter {
	return &prefilter{text: text, off: r.noPrefilter}
}

// pass reports whether rl may match. Rules without need always run.
func (p *prefilter) pass(rl rule) bool {
	if p.off || len(rl.need) == 0 {
		return true
	}
	if !rl.fold {
		for _, k := range rl.need {
			if strings.Contains(p.text, k) {
				return true
			}
		}
		return false
	}
	if !p.ready {
		p.folded, p.ready = foldASCII(p.text), true
	}
	for _, k := range rl.need {
		if bytes.Contains(p.folded, []byte(k)) {
			return true
		}
	}
	return false
}

// foldASCII lower-cases A-Z and maps the only non-ASCII runes that Go's
// (?i) folds to ASCII letters, U+017F (long s) and U+212A (Kelvin sign), to
// s and k (TestFoldOrbitsToASCII). Every (?i) match of a lower-case ASCII
// literal in text is then a plain substring of the result.
func foldASCII(text string) []byte {
	b := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case 'A' <= c && c <= 'Z':
			c += 'a' - 'A'
		case c == 0xC5 && strings.HasPrefix(text[i:], "\u017f"):
			c, i = 's', i+1
		case c == 0xE2 && strings.HasPrefix(text[i:], "\u212a"):
			c, i = 'k', i+2
		}
		b = append(b, c)
	}
	return b
}

// overlapsAny reports whether [start, end) overlaps one of the sorted,
// disjoint spans.
func overlapsAny(start, end int, spans [][]int) bool {
	i := sort.Search(len(spans), func(i int) bool { return spans[i][1] > start })
	return i < len(spans) && spans[i][0] < end
}

// Replace substitutes every finding with placeholder(f) in one pass.
// Findings must not overlap and must be sorted by start, as returned by Find.
func Replace(text string, findings []Finding, placeholder func(Finding) string) string {
	if len(findings) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	last := 0
	for _, f := range findings {
		b.WriteString(text[last:f.Start])
		b.WriteString(placeholder(f))
		last = f.End
	}
	b.WriteString(text[last:])
	return b.String()
}

// Redact replaces all findings with the unnumbered form <redacted:TYPE>.
// Numbered, reversible placeholders (ADR-0006) are produced by Number and the
// redact stage.
func (r *Redactor) Redact(text string) string {
	return Replace(text, r.Find(text), func(f Finding) string { return "<redacted:" + f.Type + ">" })
}
