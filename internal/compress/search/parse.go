// Line parser of the search strategy.
//
// Source: crates/headroom-core/src/transforms/search_compressor.rs of
// Headroom (Apache-2.0, Copyright 2025 Headroom Contributors), Stand
// 0947a28 (parse_match_line); portiert und verändert.
//
// Changes: the line number must follow the first colon of the path (a ":12:"
// inside the content never moves the split); "-N-" context lines are only
// accepted for files known from ":N:" lines (file names like "a-1-b.go");
// added are the formats without line numbers, rg --heading and plain path
// lists. Unparseable lines become raw records and are never dropped.
package search

import "strings"

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// validPath rejects empty prefixes and prefixes with white space; those are
// prose ("Binary file x matches"), not file names.
func validPath(s string) bool {
	return s != "" && !strings.ContainsAny(s, " \t\r")
}

// firstColon is the index of the colon that ends the path, skipping the
// drive colon of a Windows path (C:\ or C:/); -1 if there is none.
func firstColon(line string) int {
	start := 0
	if len(line) >= 3 && isAlpha(line[0]) && line[1] == ':' && (line[2] == '\\' || line[2] == '/') {
		start = 2
	}
	i := strings.IndexByte(line[start:], ':')
	if i < 0 {
		return -1
	}
	return start + i
}

// splitLineNo parses "path:N:content".
func splitLineNo(line string) (string, bool) {
	i := firstColon(line)
	if i <= 0 {
		return "", false
	}
	j := i + 1
	for j < len(line) && isDigit(line[j]) {
		j++
	}
	if j == i+1 || j >= len(line) || line[j] != ':' {
		return "", false
	}
	if !validPath(line[:i]) {
		return "", false
	}
	return line[:i], true
}

// splitContext parses "path-N-content" for a file known from a match line.
func splitContext(line string, known map[string]struct{}) (string, bool) {
	for i := 1; i < len(line); i++ {
		if line[i] != '-' {
			continue
		}
		j := i + 1
		for j < len(line) && isDigit(line[j]) {
			j++
		}
		if j == i+1 || j >= len(line) || line[j] != '-' {
			continue
		}
		if _, ok := known[line[:i]]; ok {
			return line[:i], true
		}
	}
	return "", false
}

// splitNoNumber parses "path:content" (grep -r without -n). The path must
// look like one (a '/', '\' or '.'), so prose like "error: x" stays raw.
func splitNoNumber(line string) (string, bool) {
	i := firstColon(line)
	if i <= 0 {
		return "", false
	}
	p := line[:i]
	if !validPath(p) || !strings.ContainsAny(p, "/\\.") {
		return "", false
	}
	return p, true
}

// numPrefix reports whether the line starts with "N:" or "N-" (rg --heading).
func numPrefix(line string) bool {
	j := 0
	for j < len(line) && isDigit(line[j]) {
		j++
	}
	return j > 0 && j < len(line) && (line[j] == ':' || line[j] == '-')
}

// pathish reports whether the line looks like one entry of a path listing
// (grep -l, rg --files). Lines with whitespace are text, e.g. the matches of
// grep -h or of a single file, and never count as paths.
func pathish(line string) bool {
	if line == "" || strings.ContainsAny(line, ": \t") {
		return false
	}
	return strings.ContainsAny(line, "/.")
}

func dirOf(path string) string {
	switch i := strings.LastIndexByte(path, '/'); {
	case i < 0:
		return "."
	case i == 0:
		return "/"
	default:
		return path[:i]
	}
}

// parse classifies the lines of one output. ok is false when the output is
// not recognisably search output (fewer than half of the non-empty lines
// parsed); the caller then keeps it unchanged.
func parse(lines []string) (recs []rec, w words, ok bool) {
	nonEmpty := 0
	for _, l := range lines {
		if l != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		return nil, w, false
	}
	accept := func(recs []rec, w words) ([]rec, words, bool) {
		parsed, items := 0, 0
		for _, r := range recs {
			if r.kind == kItem || r.kind == kHeader || r.kind == kPath {
				parsed++
			}
			if r.kind == kItem || r.kind == kPath {
				items++
			}
		}
		if items == 0 || parsed*2 < nonEmpty {
			return nil, w, false
		}
		return recs, w, true
	}

	// grep -n / rg -n / git grep -n: "path:N:content", context "path-N-content".
	known := map[string]struct{}{}
	files := make([]string, len(lines))
	numbered := 0
	for i, l := range lines {
		if f, ok := splitLineNo(l); ok {
			files[i] = f
			known[f] = struct{}{}
			numbered++
		}
	}
	if numbered > 0 {
		recs = make([]rec, 0, len(lines))
		last := ""
		for i, l := range lines {
			switch f := files[i]; {
			case f != "":
				recs = append(recs, rec{l, kItem, f})
				last = f
			case l == "--" && last != "":
				recs = append(recs, rec{l, kSoft, last})
			default:
				if f, ok := splitContext(l, known); ok {
					recs = append(recs, rec{l, kItem, f})
					last = f
				} else {
					recs = append(recs, rec{l, kRaw, ""})
				}
			}
		}
		return accept(recs, wordsGrep)
	}

	// rg --heading: "path", then "N:content" / "N-context" rows, blank line.
	cur, headers := "", 0
	recs = make([]rec, 0, len(lines))
	for i, l := range lines {
		switch {
		case l == "" && cur != "":
			recs = append(recs, rec{l, kSoft, cur})
			cur = ""
		case cur != "" && l == "--":
			recs = append(recs, rec{l, kSoft, cur})
		case cur != "" && numPrefix(l):
			recs = append(recs, rec{l, kItem, cur})
		case cur == "" && l != "" && i+1 < len(lines) && numPrefix(lines[i+1]):
			cur = l
			headers++
			recs = append(recs, rec{l, kHeader, l})
		default:
			recs = append(recs, rec{l, kRaw, ""})
		}
	}
	if headers > 0 {
		return accept(recs, wordsGrep)
	}

	// grep -r without line numbers: "path:content".
	recs = recs[:0]
	for _, l := range lines {
		if f, ok := splitNoNumber(l); ok {
			recs = append(recs, rec{l, kItem, f})
		} else {
			recs = append(recs, rec{l, kRaw, ""})
		}
	}
	if r, w2, ok := accept(recs, wordsGrep); ok {
		return r, w2, ok
	}

	// find: plain path list, grouped by directory.
	paths := 0
	recs = recs[:0]
	for _, l := range lines {
		if pathish(l) {
			recs = append(recs, rec{l, kPath, dirOf(l)})
			paths++
		} else {
			recs = append(recs, rec{l, kRaw, ""})
		}
	}
	if paths*10 < nonEmpty*8 {
		return nil, w, false
	}
	return accept(recs, wordsPath)
}
