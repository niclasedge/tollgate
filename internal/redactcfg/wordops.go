package redactcfg

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of one word list; equal to words.list in config/schema
// (TestWordLimitsMatchSchema).
const (
	maxWords     = 5000
	maxWordRunes = 200
)

// WordOpError is an invalid word-list operation (status 400). Its message
// names indices only, never a word.
type WordOpError struct{ msg string }

func (e *WordOpError) Error() string { return e.msg }

func (o WordOps) empty() bool {
	return len(o.WordsRemove) == 0 && len(o.WordsAdd) == 0 && o.ListsSet == nil
}

// applyWordOps returns base without the entries at WordsRemove, followed by
// the trimmed WordsAdd in input order. Empty new entries and new entries
// already in the result (per caseSensitive) are dropped; duplicates within
// base stay. base is not modified. The result depends on the input only.
func applyWordOps(base []string, ops WordOps, caseSensitive bool) ([]string, error) {
	drop := make(map[int]bool, len(ops.WordsRemove))
	for i, idx := range ops.WordsRemove {
		switch {
		case idx < 0 || idx >= len(base):
			return nil, &WordOpError{fmt.Sprintf("words_remove[%d]: Index %d gibt es nicht (Wortliste hat %d Einträge)", i, idx, len(base))}
		case drop[idx]:
			return nil, &WordOpError{fmt.Sprintf("words_remove[%d]: Index %d ist doppelt", i, idx)}
		}
		drop[idx] = true
	}
	key := func(w string) string {
		if caseSensitive {
			return w
		}
		return foldKey(w)
	}
	out := make([]string, 0, len(base)+len(ops.WordsAdd))
	seen := make(map[string]bool, len(base)+len(ops.WordsAdd))
	for i, w := range base {
		if !drop[i] {
			out = append(out, w)
			seen[key(w)] = true
		}
	}
	var issues []Issue
	for i, w := range ops.WordsAdd {
		w = strings.TrimSpace(w)
		if w == "" || seen[key(w)] {
			continue
		}
		if msg := entryProblem(w); msg != "" {
			issues = append(issues, Issue{Path: "words_add[" + strconv.Itoa(i) + "]", Message: msg})
			continue
		}
		seen[key(w)] = true
		out = append(out, w)
	}
	if len(issues) == 0 && len(out) > maxWords {
		issues = append(issues, Issue{Path: "words_add", Message: fmt.Sprintf("Wortliste hätte %d Einträge, erlaubt sind %d", len(out), maxWords)})
	}
	if len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	return out, nil
}

// entryProblem names what is wrong with a trimmed, non-empty list entry, or
// returns "". The message never quotes the entry.
func entryProblem(w string) string {
	switch {
	case utf8.RuneCountInString(w) > maxWordRunes:
		return fmt.Sprintf("Eintrag ist länger als %d Zeichen", maxWordRunes)
	case strings.Contains(w, maskDots):
		return "maskierter Eintrag – nur Klartext-Wörter hinzufügen"
	}
	return ""
}

// foldKey maps every rune to the smallest member of its simple case-folding
// orbit, so foldKey(a) == foldKey(b) exactly when strings.EqualFold(a, b),
// the folding of the redactor's (?i) word rules.
func foldKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		m := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			m = min(m, f)
		}
		b.WriteRune(m)
	}
	return b.String()
}
