package redact

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// corpusCase is one txtar file under testdata/corpus (format: ADR-0005).
//
// Sections: input (required), must_contain and must_not_contain (one
// substring per non-empty line), want (exact Go output) and deviation (names
// of deliberate Rust/Go deviations, see knownDeviations in parity_test.go).
// The comment above the first section names the Rust origin.
//
// The marker "{{}}" is removed from every section when loading. It splits
// key-shaped test values so that secret scanners (GitHub push protection)
// do not flag the repository, while the loaded value still matches.
type corpusCase struct {
	name           string
	input          string
	mustContain    []string
	mustNotContain []string
	want           *string
	deviations     []string
}

const joinMarker = "{{}}"

var corpusSections = []string{"input", "must_contain", "must_not_contain", "want", "deviation"}

// parseTxtar reads the subset of the txtar format the corpus uses: an
// optional comment followed by "-- name --" sections. A tiny parser keeps
// golang.org/x/tools out of go.mod.
func parseTxtar(data string) (comment string, sections map[string]string, order []string) {
	sections = map[string]string{}
	cur := ""
	inSection := false
	var b strings.Builder
	flush := func() {
		if inSection {
			sections[cur] = b.String()
		} else {
			comment = b.String()
		}
		b.Reset()
	}
	for _, line := range strings.SplitAfter(data, "\n") {
		trimmed := strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") && len(trimmed) > 6 {
			flush()
			cur = strings.TrimSpace(trimmed[3 : len(trimmed)-3])
			order = append(order, cur)
			inSection = true
			continue
		}
		b.WriteString(line)
	}
	flush()
	return comment, sections, order
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func loadCorpus(t testing.TB) []corpusCase {
	t.Helper()
	root := filepath.Join("testdata", "corpus")
	var cases []corpusCase
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txtar") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		comment, secs, order := parseTxtar(strings.ReplaceAll(string(data), joinMarker, ""))
		name, _ := filepath.Rel(root, path)
		name = strings.TrimSuffix(filepath.ToSlash(name), ".txtar")
		if strings.TrimSpace(comment) == "" {
			t.Errorf("%s: missing origin comment", name)
		}
		for _, s := range order {
			if !slices.Contains(corpusSections, s) {
				t.Errorf("%s: unknown section %q", name, s)
			}
		}
		input, ok := secs["input"]
		if !ok {
			t.Errorf("%s: missing input section", name)
		}
		c := corpusCase{
			name:           name,
			input:          input,
			mustContain:    lines(secs["must_contain"]),
			mustNotContain: lines(secs["must_not_contain"]),
			deviations:     lines(secs["deviation"]),
		}
		if w, ok := secs["want"]; ok {
			c.want = &w
		}
		cases = append(cases, c)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("empty corpus")
	}
	return cases // WalkDir visits in lexical order
}

// corpusOptions are the switches used for the corpus and the parity test:
// defaults plus IP addresses (ADR-0005, Paritätstest).
func corpusOptions() Options {
	o := DefaultOptions()
	o.IPAddresses = true
	return o
}

func TestCorpus(t *testing.T) {
	r := New(corpusOptions())
	for _, c := range loadCorpus(t) {
		t.Run(c.name, func(t *testing.T) {
			got := r.Redact(c.input)
			for _, s := range c.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output lacks %q\noutput: %q", s, got)
				}
			}
			for _, s := range c.mustNotContain {
				if strings.Contains(got, s) {
					t.Errorf("output contains %q\noutput: %q", s, got)
				}
			}
			if c.want != nil && got != *c.want {
				t.Errorf("output mismatch\n got: %q\nwant: %q", got, *c.want)
			}
			if c.want == nil && len(c.mustContain)+len(c.mustNotContain) == 0 {
				t.Error("case asserts nothing")
			}
		})
	}
}

func TestParseTxtar(t *testing.T) {
	comment, secs, order := parseTxtar("origin\n-- input --\na\n-- x --\n-- want --\nb\nc\n")
	if comment != "origin\n" {
		t.Errorf("comment = %q", comment)
	}
	if secs["input"] != "a\n" || secs["x"] != "" || secs["want"] != "b\nc\n" {
		t.Errorf("sections = %q", secs)
	}
	if !slices.Equal(order, []string{"input", "x", "want"}) {
		t.Errorf("order = %q", order)
	}
}
