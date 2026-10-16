// Package diff is the compress strategy "diff" for the output of git diff,
// git show and git log -p (ADR-0007, strategies 3 and 4).
//
// Source: Headroom crates/headroom-core/src/transforms/diff_compressor.rs
// (DiffCompressor::compress_with_store, format_output) and
// pipeline/offloads/diff_noise.rs. Headroom (Apache-2.0, Copyright 2025
// Headroom Contributors), Stand 0947a28, portiert und verändert.
//
// Steps, each reported to the trace:
//
//	noise    lockfile hunks and whitespace-only hunks are dropped
//	limits   at most MaxFiles files, at most MaxHunks hunks per file
//	context  at most ContextLines unchanged lines around each change
//
// Every other line stays byte-identical: commit headers, "diff --git",
// "index", "---", "+++", "@@", mode, rename and binary lines, and every
// "+"/"-" line of a kept hunk. Dropped hunks leave a one-line note at their
// place; dropped files are named in the details.
//
// Differences to Headroom: no query scoring, no CCR, fixed limits, no
// normalization of headers, no summary footer in the text (the stage appends
// the marker line), and the details name what was removed.
package diff

import (
	"strconv"
	"strings"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

// Defaults used when a Params field is 0. Headroom uses 2 context lines,
// 20 files and 10 hunks per file; git's own default context is 3 lines, so
// the default only trims diffs made with a larger -U.
const (
	DefaultContextLines = 3
	DefaultMaxFiles     = 30
	DefaultMaxHunks     = 20
)

// maxListed is how many names a detail lists before it ends with "…".
const maxListed = 5

// Diff shortens a unified diff. Input that holds no "diff --git" section is
// returned unchanged.
func Diff(out string, p strat.Params, t strat.Trace) strat.Result {
	ctx := orDefault(p.ContextLines, DefaultContextLines)
	maxFiles := orDefault(p.MaxFiles, DefaultMaxFiles)
	maxHunks := orDefault(p.MaxHunks, DefaultMaxHunks)

	d := parse(out)
	hasFile := false
	for _, b := range d.blocks {
		if b.f != nil {
			hasFile = true
			break
		}
	}
	if !hasFile {
		return strat.Result{Text: out}
	}

	nf := dropNoise(d)
	t.Step("noise", d.render())
	lf := applyLimits(d, maxFiles, maxHunks)
	t.Step("limits", d.render())
	trimmed := trimContext(d, ctx)
	text := d.render()
	t.Step("context", text)

	var details []string
	if len(nf.lockfiles) > 0 {
		details = append(details, "Lockfiles: "+nameList(nf.lockfiles))
	}
	if nf.wsHunks > 0 {
		details = append(details, strconv.Itoa(nf.wsHunks)+" Whitespace-Hunks")
	}
	if len(lf.files) > 0 {
		details = append(details, strconv.Itoa(len(lf.files))+" Dateien ausgelassen: "+nameList(lf.files))
	}
	if lf.hunks > 0 {
		details = append(details, strconv.Itoa(lf.hunks)+" Hunks ausgelassen")
	}
	if trimmed > 0 {
		details = append(details, strconv.Itoa(trimmed)+" Kontextzeilen gekürzt")
	}
	return strat.Result{Text: text, Omitted: d.omitted(), Details: details}
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// nameList joins the first names with ", " and ends with "…" when more
// follow. Names are made safe for the marker line.
func nameList(names []string) string {
	shown := names
	if len(shown) > maxListed {
		shown = shown[:maxListed]
	}
	clean := make([]string, len(shown))
	for i, n := range shown {
		clean[i] = sanitize(n)
	}
	s := strings.Join(clean, ", ")
	if len(names) > maxListed {
		s += ", …"
	}
	return s
}

// sanitize removes what must not appear in a detail: "]" and line breaks.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ']':
			return ')'
		case '\n', '\r':
			return ' '
		}
		return r
	}, s)
}

// render writes the document with all drops and trims applied. Dropped hunks
// that follow each other become one note.
func (d *document) render() string {
	var lines []string
	d.walk(func(line string, orig bool) { lines = append(lines, line) })
	s := strings.Join(lines, "\n")
	if d.trailingNL {
		s += "\n"
	}
	return s
}

// omitted counts the input lines that render does not write.
func (d *document) omitted() int {
	kept := 0
	d.walk(func(_ string, orig bool) {
		if orig {
			kept++
		}
	})
	return d.inputLines - kept
}

// walk calls emit for every output line; orig is false for the notes.
func (d *document) walk(emit func(line string, orig bool)) {
	for _, b := range d.blocks {
		if b.f == nil {
			for _, l := range b.raw {
				emit(l, true)
			}
			continue
		}
		f := b.f
		if f.dropped {
			continue
		}
		for _, l := range f.header {
			emit(l, true)
		}
		var run noteRun
		flush := func() {
			if run.hunks > 0 {
				emit(run.note(), false)
			}
			run = noteRun{}
		}
		for _, h := range f.hunks {
			if h.drop != keepHunk {
				run.add(h)
				continue
			}
			flush()
			emit(h.header, true)
			for i, l := range h.lines {
				if h.trim == nil || !h.trim[i] {
					emit(l, true)
				}
			}
		}
		flush()
	}
}

// noteRun collects consecutive dropped hunks of one file.
type noteRun struct {
	hunks, lines int
	reason       dropReason
}

func (r *noteRun) add(h *hunk) {
	if r.hunks == 0 {
		r.reason = h.drop
	} else if r.reason != h.drop {
		r.reason = keepHunk // mixed
	}
	r.hunks++
	r.lines += len(h.lines) + 1
}

func (r *noteRun) note() string {
	what := "Hunks"
	if r.hunks == 1 {
		what = "Hunk"
	}
	why := "Limit"
	switch r.reason {
	case dropLockfile:
		why = "Lockfile"
	case dropWhitespace:
		why = "nur Whitespace"
	case keepHunk:
		why = "Lockfile, Whitespace oder Limit"
	}
	return "[tollgate: " + strconv.Itoa(r.hunks) + " " + what + " ausgelassen (" + why + ", " +
		strconv.Itoa(r.lines) + " Zeilen)]"
}
