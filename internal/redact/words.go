package redact

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// matches returns the submatch index slices of rl in text. Rules with a
// custom matcher (word list) use it; all others use their regex.
func (rl rule) matches(text string) [][]int {
	if rl.find != nil {
		return rl.find(text)
	}
	return rl.re.FindAllStringSubmatchIndex(text, -1)
}

// sortedWords deduplicates the non-blank entries and sorts them by byte
// length descending, then lexicographically (ADR-0008), so the rule order
// never depends on the configured order.
func sortedWords(list []string) []string {
	seen := make(map[string]bool, len(list))
	var out []string
	for _, w := range list {
		if strings.TrimSpace(w) == "" || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// wordSpace joins the parts of a multi-word entry: any run of whitespace,
// including line breaks and Unicode spaces such as NBSP.
const wordSpace = `[\s\p{Z}]+`

// wordRule builds the detector for one entry of a word list. The rule name is
// the same for every entry of a list so traces never reveal the word.
func wordRule(w string, opts WordList, name, typ string) rule {
	parts := strings.FieldsFunc(w, func(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Z, r) })
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	expr := strings.Join(parts, wordSpace)
	if !opts.CaseSensitive {
		expr = "(?i)" + expr
	}
	re := regexp.MustCompile(expr) // quoted literals always compile
	first, _ := utf8.DecodeRuneInString(w)
	last, _ := utf8.DecodeLastRuneInString(w)
	checkLeft := opts.WholeWord && isWordRune(first)
	checkRight := opts.WholeWord && isWordRune(last)
	return rule{
		name:     name,
		re:       re,
		typ:      typ,
		priority: prioWords,
		find: func(text string) [][]int {
			return findWord(re, text, checkLeft, checkRight)
		},
	}
}

// findWord returns all matches of re in text whose neighbours pass the
// Unicode-aware word-boundary check (Go's \b is ASCII only). After a rejected
// candidate the search restarts one rune later, so a valid occurrence that
// overlaps a rejected one is not lost. re has no anchors, so searching a
// suffix of text is equivalent to searching from an offset.
func findWord(re *regexp.Regexp, text string, checkLeft, checkRight bool) [][]int {
	var out [][]int
	for pos := 0; pos < len(text); {
		loc := re.FindStringIndex(text[pos:])
		if loc == nil {
			break
		}
		s, e := pos+loc[0], pos+loc[1]
		if boundaryOK(text, s, e, checkLeft, checkRight) {
			out = append(out, []int{s, e})
			pos = max(e, s+1)
			continue
		}
		_, size := utf8.DecodeRuneInString(text[s:])
		pos = s + max(size, 1)
	}
	return out
}

func boundaryOK(text string, s, e int, checkLeft, checkRight bool) bool {
	if checkLeft && s > 0 {
		if r, _ := utf8.DecodeLastRuneInString(text[:s]); isWordRune(r) {
			return false
		}
	}
	if checkRight && e < len(text) {
		if r, _ := utf8.DecodeRuneInString(text[e:]); isWordRune(r) {
			return false
		}
	}
	return true
}

// isWordRune reports letters, digits and combining marks (ADR-0005).
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}
