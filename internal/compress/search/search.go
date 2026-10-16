// Search strategy of the compress stage (ADR-0007, strategy 6).
//
// Source: crates/headroom-core/src/transforms/search_compressor.rs of
// Headroom (Apache-2.0, Copyright 2025 Headroom Contributors), Stand
// 0947a28; portiert und verändert.
//
// What is ported: the line parser idea of parse_match_line (separator/number
// marker, Windows drive prefix, "-" context lines), grouping by file in a
// first-appearance order, a per-file match cap with a "more matches"
// summary, a file cap, and the file-grouped (rg --heading) view.
//
// What differs on purpose: no query/relevance scoring, no importance
// detector, no adaptive_sizer, no CCR store and no marker with a hash. All
// limits are fixed. Kept lines stay byte-identical (Headroom re-formats
// them); the only edit is the documented truncation of overlong match lines.
// Lines that do not parse are kept verbatim and in place.
package search

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

// Fixed limits. strat.Params overrides them when non-zero:
//   - Params.MaxFiles  -> files (grep/rg) or directories (find) shown
//   - Params.MaxErrors -> matches (grep/rg) or paths (find) kept per file or
//     directory; reused because this strategy has no errors to cap
//   - Params.ContextLines and Params.MaxHunks are unused.
const (
	defaultMaxFiles = 40
	defaultPerFile  = 8

	// A match line longer than maxLineBytes is cut to keepLineBytes bytes
	// (on a UTF-8 boundary) plus an ellipsis.
	maxLineBytes  = 300
	keepLineBytes = 200
	ellipsis      = "…"
)

type kind uint8

const (
	kRaw    kind = iota // not understood: always kept, in place
	kItem               // grep/rg match or context line, counts toward the file cap
	kHeader             // rg --heading file line
	kSoft               // blank or "--" separator, follows the fate of its file
	kPath               // find-style path, counts toward the directory cap
)

type rec struct {
	line string
	kind kind
	key  string // file (grep/rg) or directory (find)
}

// words are the German nouns of one input flavour.
type words struct {
	item1, itemN string // Treffer / Pfad(e)
	cont1        string // Datei / Verzeichnis (dative singular)
	contNDat     string // Dateien / Verzeichnissen
	contNNom     string // Dateien / Verzeichnisse
	more1        string // "weitere Datei" / "weiteres Verzeichnis"
	moreItem1    string // "weiterer Treffer" / "weiterer Pfad"
}

var (
	wordsGrep = words{"Treffer", "Treffer", "Datei", "Dateien", "Dateien", "weitere Datei", "weiterer Treffer"}
	wordsPath = words{"Pfad", "Pfade", "Verzeichnis", "Verzeichnissen", "Verzeichnisse", "weiteres Verzeichnis", "weiterer Pfad"}
)

func (w words) items(n int) string {
	if n == 1 {
		return "1 " + w.item1
	}
	return strconv.Itoa(n) + " " + w.itemN
}

func (w words) containersDat(n int) string {
	if n == 1 {
		return "1 " + w.cont1
	}
	return strconv.Itoa(n) + " " + w.contNDat
}

// Search compresses the output of grep -rn, grep -r, rg (also --heading),
// git grep and find. Output it does not recognise is returned unchanged.
func Search(out string, p strat.Params, t strat.Trace) strat.Result {
	unchanged := strat.Result{Text: out}
	maxFiles, perFile := p.MaxFiles, p.MaxErrors
	if maxFiles <= 0 {
		maxFiles = defaultMaxFiles
	}
	if perFile <= 0 {
		perFile = defaultPerFile
	}
	body := strings.TrimSuffix(out, "\n")
	if body == "" {
		return unchanged
	}
	trailingNL := len(body) != len(out)
	recs, w, ok := parse(strings.Split(body, "\n"))
	if !ok {
		return unchanged
	}

	g := newGrouping(recs)
	if t != nil {
		var b strings.Builder
		for rk, key := range g.keys {
			b.WriteString(key + ": " + w.items(g.total[rk]) + "\n")
		}
		t.Step("group", b.String())
		t.Step("limits", join(emit(recs, g, w, maxFiles, perFile, false).lines, trailingNL))
	}
	res := emit(recs, g, w, maxFiles, perFile, true)
	text := join(res.lines, trailingNL)
	t.Step("truncate", text)
	if res.omitted == 0 && res.truncated == 0 {
		return unchanged
	}

	var details []string
	if res.droppedItems > 0 {
		details = append(details, w.items(res.droppedItems)+" in "+w.containersDat(res.affected)+" ausgelassen")
	}
	if res.truncated > 0 {
		details = append(details, "Zeilen gekürzt: "+strconv.Itoa(res.truncated))
	}
	return strat.Result{Text: text, Omitted: res.omitted, Details: details}
}

func join(lines []string, trailingNL bool) string {
	s := strings.Join(lines, "\n")
	if trailingNL && len(lines) > 0 {
		s += "\n"
	}
	return s
}

// grouping is the per-key bookkeeping, in order of first appearance.
type grouping struct {
	rank  map[string]int // lookup only, never iterated
	keys  []string
	total []int // items per key
	last  []int // index of the last non-soft record per key
}

func newGrouping(recs []rec) *grouping {
	g := &grouping{rank: map[string]int{}}
	for i, r := range recs {
		if r.kind == kRaw {
			continue
		}
		rk, seen := g.rank[r.key]
		if !seen {
			rk = len(g.keys)
			g.rank[r.key] = rk
			g.keys = append(g.keys, r.key)
			g.total = append(g.total, 0)
			g.last = append(g.last, -1)
		}
		if r.kind == kSoft {
			continue
		}
		g.last[rk] = i
		if r.kind == kItem || r.kind == kPath {
			g.total[rk]++
		}
	}
	return g
}

type outcome struct {
	lines        []string
	omitted      int // input lines not in the output
	droppedItems int
	affected     int // keys that lost at least one item
	truncated    int
}

// emit applies the caps and, when trunc is set, the line truncation. Kept
// lines keep their input order; summaries sit after the last item of a file.
func emit(recs []rec, g *grouping, w words, maxFiles, perFile int, trunc bool) outcome {
	var o outcome
	seen := make([]int, len(g.keys))
	hit := make([]bool, len(g.keys))
	droppedFiles, itemsInDropped := 0, 0
	for rk := maxFiles; rk < len(g.keys); rk++ {
		droppedFiles++
		itemsInDropped += g.total[rk]
	}
	aggregateDone := false
	prevSoft := false

	push := func(s string, soft bool) {
		o.lines = append(o.lines, s)
		prevSoft = soft
	}

	for i, r := range recs {
		if r.kind == kRaw {
			push(r.line, false)
			continue
		}
		rk := g.rank[r.key]
		if rk >= maxFiles {
			if !aggregateDone {
				aggregateDone = true
				more := "weitere " + w.contNNom
				if droppedFiles == 1 {
					more = w.more1
				}
				push("… "+strconv.Itoa(droppedFiles)+" "+more+" ("+w.items(itemsInDropped)+") ausgelassen", false)
			}
			o.omitted++
			if r.kind == kItem || r.kind == kPath {
				o.droppedItems++
				hit[rk] = true
			}
			continue
		}
		switch r.kind {
		case kHeader:
			push(r.line, false)
		case kItem, kPath:
			seen[rk]++
			if seen[rk] > perFile {
				o.omitted++
				o.droppedItems++
				hit[rk] = true
				break
			}
			line := r.line
			if trunc && r.kind == kItem && len(line) > maxLineBytes {
				line = cut(line)
				o.truncated++
			}
			push(line, false)
		case kSoft:
			if prevSoft {
				o.omitted++
			} else {
				push(r.line, true)
			}
		}
		if r.kind != kSoft && i == g.last[rk] && g.total[rk] > perFile {
			n := g.total[rk] - perFile
			more := "weitere " + w.itemN
			if n == 1 {
				more = w.moreItem1
			}
			push("… "+g.keys[rk]+": "+strconv.Itoa(n)+" "+more, false)
		}
	}
	for _, h := range hit {
		if h {
			o.affected++
		}
	}
	return o
}

// cut keeps the first keepLineBytes bytes, on a rune boundary, plus "…".
func cut(s string) string {
	n := keepLineBytes
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + ellipsis
}
