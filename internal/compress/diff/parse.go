// Parser for unified diffs as printed by git diff, git show and git log -p.
//
// Source: Headroom crates/headroom-core/src/transforms/diff_compressor.rs
// (parse_diff, hunk/file header recognition) and pipeline/offloads/diff_noise.rs
// (parse_new_path). Headroom (Apache-2.0, Copyright 2025 Headroom Contributors),
// Stand 0947a28, portiert und verändert.
//
// Deviations: the parser is line preserving. Every input line ends up in
// exactly one place (raw block, file header or hunk), so an untouched
// document renders to the original bytes. Hunk bodies are delimited by the
// counts of the "@@" header, which keeps commit headers of the following
// commit in "git log -p" out of the last hunk. Headroom regenerates the file
// headers and normalizes modes and binary notices; this port never does.

package diff

import (
	"regexp"
	"strconv"
	"strings"
)

type dropReason int

const (
	keepHunk dropReason = iota
	dropLockfile
	dropWhitespace
	dropLimit
)

// hunk is one "@@" section. Body lines are stored without the header.
type hunk struct {
	header string
	lines  []string
	// width is the number of prefix columns of a body line: 1 for a
	// regular diff, parents for a combined diff ("@@@" has 2).
	width int
	drop  dropReason
	// trim[i] is true when body line i is removed by the context step.
	trim []bool
}

// file is one "diff --git" / "diff --cc" section.
type file struct {
	header  []string // "diff ..." line and the extended header up to the first hunk
	path    string
	hunks   []*hunk
	dropped bool // removed by the file limit
}

// block is either raw text (commit headers, stat lines, anything that is not
// part of a file section) or a file.
type block struct {
	raw []string
	f   *file
}

type document struct {
	blocks     []block
	trailingNL bool
	inputLines int
}

var hunkCountsRe = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@`)

// extendedHeaderPrefixes are the lines git prints between "diff --git" and
// the first hunk.
var extendedHeaderPrefixes = []string{
	"index ", "--- ", "+++ ", "old mode ", "new mode ", "deleted file mode ",
	"new file mode ", "similarity index ", "dissimilarity index ",
	"rename from ", "rename to ", "copy from ", "copy to ", "Binary files ",
	"GIT binary patch",
}

var commitLineRe = regexp.MustCompile(`^commit [0-9a-f]{7,}`)

func isDiffHeader(line string) bool {
	return strings.HasPrefix(line, "diff --git ") ||
		strings.HasPrefix(line, "diff --cc ") ||
		strings.HasPrefix(line, "diff --combined ")
}

// parseHunkHeader recognizes "@@ -a,b +c,d @@" and the combined forms
// "@@@ ... @@@". oldRem/newRem are -1 when the counts are unknown.
func parseHunkHeader(line string) (width, oldRem, newRem int, ok bool) {
	n := 0
	for n < len(line) && line[n] == '@' {
		n++
	}
	if n < 2 || n >= len(line) || line[n] != ' ' {
		return 0, 0, 0, false
	}
	if !strings.Contains(line[n:], strings.Repeat("@", n)) {
		return 0, 0, 0, false
	}
	if n == 2 {
		if m := hunkCountsRe.FindStringSubmatch(line); m != nil {
			return 1, countOrOne(m[1]), countOrOne(m[2]), true
		}
	}
	return n - 1, -1, -1, true
}

func countOrOne(s string) int {
	if s == "" {
		return 1
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 1
	}
	return v
}

// hunkState tracks how many body lines a hunk header announced.
type hunkState struct {
	h              *hunk
	oldRem, newRem int
}

func (s *hunkState) exhausted() bool {
	return s.oldRem == 0 && s.newRem == 0
}

// accept consumes a body line. It trusts the header counts but tolerates a
// header that announced too few lines (hand-written diffs), except for the
// "-- " signature that git format-patch appends.
func (s *hunkState) accept(line string) bool {
	known := s.oldRem >= 0
	if line == "" {
		if known && !s.exhausted() {
			s.oldRem, s.newRem = dec(s.oldRem), dec(s.newRem)
			return true
		}
		return false
	}
	switch line[0] {
	case '\\':
		return true
	case ' ':
		if known {
			s.oldRem, s.newRem = dec(s.oldRem), dec(s.newRem)
		}
		return true
	case '+':
		if known {
			s.newRem = dec(s.newRem)
		}
		return true
	case '-':
		if known && s.exhausted() && (line == "-- " || line == "--") {
			return false
		}
		if known {
			s.oldRem = dec(s.oldRem)
		}
		return true
	}
	return false
}

func dec(v int) int {
	if v > 0 {
		return v - 1
	}
	return 0
}

func (d *document) appendRaw(line string) {
	if n := len(d.blocks); n > 0 && d.blocks[n-1].f == nil {
		d.blocks[n-1].raw = append(d.blocks[n-1].raw, line)
		return
	}
	d.blocks = append(d.blocks, block{raw: []string{line}})
}

// parse splits out into raw blocks and file sections.
func parse(out string) *document {
	lines := strings.Split(out, "\n")
	d := &document{}
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		d.trailingNL = true
		lines = lines[:len(lines)-1]
	}
	d.inputLines = len(lines)

	var cur *file
	var hs *hunkState
	binaryPatch := false
	for _, line := range lines {
		if hs != nil {
			if hs.accept(line) {
				hs.h.lines = append(hs.h.lines, line)
				continue
			}
			hs = nil
		}
		switch {
		case isDiffHeader(line):
			cur = &file{header: []string{line}}
			cur.path = headerPath(line)
			d.blocks = append(d.blocks, block{f: cur})
			binaryPatch = false
		case cur != nil && binaryPatch && !commitLineRe.MatchString(line):
			cur.header = append(cur.header, line)
		case cur != nil && strings.HasPrefix(line, "@@"):
			width, o, n, ok := parseHunkHeader(line)
			if !ok {
				width, o, n = 1, -1, -1
			}
			h := &hunk{header: line, width: width}
			cur.hunks = append(cur.hunks, h)
			hs = &hunkState{h: h, oldRem: o, newRem: n}
		case cur != nil && len(cur.hunks) == 0 && hasAnyPrefix(line, extendedHeaderPrefixes):
			cur.header = append(cur.header, line)
			if line == "GIT binary patch" {
				binaryPatch = true
			}
			if cur.path == "" && strings.HasPrefix(line, "+++ b/") {
				cur.path = line[len("+++ b/"):]
			}
		default:
			cur = nil
			binaryPatch = false
			d.appendRaw(line)
		}
	}
	return d
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// headerPath returns the new path of a file section header.
func headerPath(line string) string {
	switch {
	case strings.HasPrefix(line, "diff --cc "):
		return line[len("diff --cc "):]
	case strings.HasPrefix(line, "diff --combined "):
		return line[len("diff --combined "):]
	}
	if i := strings.LastIndex(line, " b/"); i >= 0 {
		return line[i+3:]
	}
	if i := strings.LastIndex(line, ` "b/`); i >= 0 {
		return strings.TrimSuffix(line[i+4:], `"`)
	}
	return ""
}

// isChange reports whether a body line adds or removes content.
func (h *hunk) isChange(line string) bool {
	if line == "" {
		return false
	}
	w := h.width
	if w > len(line) {
		w = len(line)
	}
	return strings.ContainsAny(line[:w], "+-")
}

func (h *hunk) changes() int {
	n := 0
	for _, l := range h.lines {
		if h.isChange(l) {
			n++
		}
	}
	return n
}

func (f *file) changes() int {
	n := 0
	for _, h := range f.hunks {
		if h.drop == keepHunk {
			n += h.changes()
		}
	}
	return n
}

func (f *file) liveHunks() []*hunk {
	var live []*hunk
	for _, h := range f.hunks {
		if h.drop == keepHunk {
			live = append(live, h)
		}
	}
	return live
}
