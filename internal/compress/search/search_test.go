package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/niclasedge/tollgate/internal/compress/compresstest"
	"github.com/niclasedge/tollgate/internal/compress/strat"
)

func gen(n int, f func(i int) string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(f(i))
		b.WriteString("\n")
	}
	return b.String()
}

func TestSearchFormats(t *testing.T) {
	small := strat.Params{MaxErrors: 2, MaxFiles: 2}
	tests := []struct {
		name        string
		in          string
		p           strat.Params
		want        string
		wantOmitted int
		wantDetails []string
	}{
		{
			name:        "grep -rn per-file cap",
			in:          gen(5, func(i int) string { return fmt.Sprintf("src/a.go:%d:foo %d", i, i) }),
			p:           small,
			want:        "src/a.go:1:foo 1\nsrc/a.go:2:foo 2\n… src/a.go: 3 weitere Treffer\n",
			wantOmitted: 3,
			wantDetails: []string{"3 Treffer in 1 Datei ausgelassen"},
		},
		{
			name:        "grep -rn context lines resolved via known files",
			in:          "src/a-b.go:4:x\nsrc/a-b.go-5-ctx\nsrc/a-b.go-6-ctx\n--\nsrc/c.go:9:y\n",
			p:           small,
			want:        "src/a-b.go:4:x\nsrc/a-b.go-5-ctx\n… src/a-b.go: 1 weiterer Treffer\n--\nsrc/c.go:9:y\n",
			wantOmitted: 1,
			wantDetails: []string{"1 Treffer in 1 Datei ausgelassen"},
		},
		{
			name:        "grep -r without line numbers",
			in:          "src/a.go:one\nsrc/a.go:two\nsrc/a.go:three: with colon\nsrc/b.go:x\n",
			p:           small,
			want:        "src/a.go:one\nsrc/a.go:two\n… src/a.go: 1 weiterer Treffer\nsrc/b.go:x\n",
			wantOmitted: 1,
			wantDetails: []string{"1 Treffer in 1 Datei ausgelassen"},
		},
		{
			name:        "colons in content do not move the split",
			in:          "src/a.go:10:url http://x:80/p:12:z\nsrc/a.go:11:b\nsrc/a.go:12:c\n",
			p:           small,
			want:        "src/a.go:10:url http://x:80/p:12:z\nsrc/a.go:11:b\n… src/a.go: 1 weiterer Treffer\n",
			wantOmitted: 1,
			wantDetails: []string{"1 Treffer in 1 Datei ausgelassen"},
		},
		{
			name:        "windows drive path",
			in:          "C:\\work\\a.go:1:x\nC:\\work\\a.go:2:y\nC:\\work\\a.go:3:z\n",
			p:           small,
			want:        "C:\\work\\a.go:1:x\nC:\\work\\a.go:2:y\n… C:\\work\\a.go: 1 weiterer Treffer\n",
			wantOmitted: 1,
			wantDetails: []string{"1 Treffer in 1 Datei ausgelassen"},
		},
		{
			name:        "rg heading",
			in:          "src/a.go\n1:x\n2:y\n3:z\n\nsrc/b.go\n7:q\n",
			p:           small,
			want:        "src/a.go\n1:x\n2:y\n… src/a.go: 1 weiterer Treffer\n\nsrc/b.go\n7:q\n",
			wantOmitted: 1,
			wantDetails: []string{"1 Treffer in 1 Datei ausgelassen"},
		},
		{
			name:        "rg heading file cap drops block and blank",
			in:          "src/a.go\n1:x\n\nsrc/b.go\n2:y\n\nsrc/c.go\n3:z\n4:w\n\nsrc/d.go\n5:v\n",
			p:           small,
			want:        "src/a.go\n1:x\n\nsrc/b.go\n2:y\n\n… 2 weitere Dateien (3 Treffer) ausgelassen\n",
			wantOmitted: 6,
			wantDetails: []string{"3 Treffer in 2 Dateien ausgelassen"},
		},
		{
			name:        "file cap with unparsed line kept in place",
			in:          "a/x.go:1:a\nb/y.go:1:b\nc/z.go:1:c\ngrep: d/p.go: Permission denied\ne/w.go:1:e\n",
			p:           small,
			want:        "a/x.go:1:a\nb/y.go:1:b\n… 2 weitere Dateien (2 Treffer) ausgelassen\ngrep: d/p.go: Permission denied\n",
			wantOmitted: 2,
			wantDetails: []string{"2 Treffer in 2 Dateien ausgelassen"},
		},
		{
			name:        "find path list per directory",
			in:          "./src/a.go\n./src/b.go\n./src/c.go\n./src/sub/d.go\n./src/e.go\n./lib/f.go\n",
			p:           strat.Params{MaxErrors: 2, MaxFiles: 5},
			want:        "./src/a.go\n./src/b.go\n./src/sub/d.go\n… ./src: 2 weitere Pfade\n./lib/f.go\n",
			wantOmitted: 2,
			wantDetails: []string{"2 Pfade in 1 Verzeichnis ausgelassen"},
		},
		{
			name:        "find directory cap",
			in:          "a/1\nb/1\nc/1\nc/2\nd/1\n",
			p:           small,
			want:        "a/1\nb/1\n… 2 weitere Verzeichnisse (3 Pfade) ausgelassen\n",
			wantOmitted: 3,
			wantDetails: []string{"3 Pfade in 2 Verzeichnissen ausgelassen"},
		},
		{
			name:        "binary file matches line kept",
			in:          "src/a.go:1:x\nsrc/a.go:2:y\nsrc/a.go:3:z\nBinary file src/blob.bin matches\n",
			p:           small,
			want:        "src/a.go:1:x\nsrc/a.go:2:y\n… src/a.go: 1 weiterer Treffer\nBinary file src/blob.bin matches\n",
			wantOmitted: 1,
			wantDetails: []string{"1 Treffer in 1 Datei ausgelassen"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Search(tc.in, tc.p, nil)
			if r.Text != tc.want {
				t.Errorf("text:\n got %q\nwant %q", r.Text, tc.want)
			}
			if r.Omitted != tc.wantOmitted {
				t.Errorf("omitted = %d, want %d", r.Omitted, tc.wantOmitted)
			}
			if strings.Join(r.Details, "|") != strings.Join(tc.wantDetails, "|") {
				t.Errorf("details = %q, want %q", r.Details, tc.wantDetails)
			}
		})
	}
}

func TestSearchUnchanged(t *testing.T) {
	tests := map[string]string{
		"empty":         "",
		"under caps":    "src/a.go:1:x\nsrc/b.go:2:y\n",
		"not search":    "hello world\nthis is not search output\nat all\n",
		"mostly errors": "grep: a: Permission denied\ngrep: b: Permission denied\ngrep: c: Permission denied\nsrc/a.go:1:x\n",
		// Matches of a single file carry no path; slashes in them are text.
		"single file": gen(20, func(i int) string { return fmt.Sprintf("\t// step %d of src/a/b", i) }),
		"test names":  gen(20, func(i int) string { return fmt.Sprintf("=== RUN   TestX/case_%d", i) }),
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			r := Search(in, strat.Params{MaxErrors: 2, MaxFiles: 2}, nil)
			if r.Text != in || r.Omitted != 0 || len(r.Details) != 0 {
				t.Errorf("want unchanged, got %+v", r)
			}
		})
	}
}

func TestSearchLongLineTruncation(t *testing.T) {
	long := strings.Repeat("a", 400)
	in := "src/a.js:1:" + long + "\nsrc/a.js:2:short\n"
	r := Search(in, strat.Params{}, nil)
	want := ("src/a.js:1:" + long)[:200] + "…" + "\nsrc/a.js:2:short\n"
	if r.Text != want {
		t.Fatalf("got %q", r.Text)
	}
	if r.Omitted != 0 {
		t.Errorf("omitted = %d", r.Omitted)
	}
	if strings.Join(r.Details, "|") != "Zeilen gekürzt: 1" {
		t.Errorf("details = %q", r.Details)
	}
}

func TestSearchTruncationUTF8Boundary(t *testing.T) {
	// "ä" is two bytes; the prefix "src/a.js:1:" is 11 bytes so byte 200
	// lands inside a rune when the content starts with an odd offset.
	for pad := 0; pad < 3; pad++ {
		in := "src/a.js:1:" + strings.Repeat("x", pad) + strings.Repeat("ä", 200) + "\n"
		r := Search(in, strat.Params{}, nil)
		line := strings.TrimSuffix(r.Text, "\n")
		if !utf8.ValidString(line) {
			t.Fatalf("pad %d: invalid UTF-8 %q", pad, line)
		}
		if !strings.HasSuffix(line, "…") || len(line) > 200+len("…") {
			t.Errorf("pad %d: len %d", pad, len(line))
		}
	}
}

func TestSearchDefaultsAndMapping(t *testing.T) {
	// defaults: 8 matches per file, 40 files
	in := gen(10, func(i int) string { return fmt.Sprintf("src/a.go:%d:x", i) })
	r := Search(in, strat.Params{}, nil)
	if got := strings.Count(r.Text, "\n"); got != 9 { // 8 matches + summary
		t.Errorf("default per-file cap: %d lines\n%s", got, r.Text)
	}
	in = gen(45, func(i int) string { return fmt.Sprintf("src/f%02d.go:1:x", i) })
	r = Search(in, strat.Params{}, nil)
	if !strings.Contains(r.Text, "… 5 weitere Dateien (5 Treffer) ausgelassen") || r.Omitted != 5 {
		t.Errorf("default file cap:\n%s omitted=%d", r.Text, r.Omitted)
	}
	// MaxErrors maps to matches per file
	r = Search(in, strat.Params{MaxFiles: 100, MaxErrors: 1}, nil)
	if r.Omitted != 0 {
		t.Errorf("explicit MaxFiles ignored: %d", r.Omitted)
	}
}

func TestSearchTrace(t *testing.T) {
	in := gen(5, func(i int) string { return fmt.Sprintf("src/a.go:%d:x", i) })
	var steps []string
	Search(in, strat.Params{MaxErrors: 2}, func(step, text string) { steps = append(steps, step) })
	if strings.Join(steps, ",") != "group,limits,truncate" {
		t.Errorf("steps = %v", steps)
	}
}

func TestSearchNoNewlineAtEnd(t *testing.T) {
	r := Search("src/a.go:1:x\nsrc/a.go:2:y\nsrc/a.go:3:z", strat.Params{MaxErrors: 1}, nil)
	want := "src/a.go:1:x\n… src/a.go: 2 weitere Treffer"
	if r.Text != want {
		t.Errorf("got %q", r.Text)
	}
}

func TestDetailsNeverBreakMarker(t *testing.T) {
	in := gen(30, func(i int) string { return fmt.Sprintf("d%d/f.go:1:%s", i, strings.Repeat("y", 400)) })
	r := Search(in, strat.Params{MaxFiles: 3}, nil)
	for _, d := range r.Details {
		if strings.ContainsAny(d, "]\n") {
			t.Errorf("detail %q", d)
		}
	}
}

func TestGolden(t *testing.T) {
	compresstest.Golden(t, "search", Search)
}

func TestGoldenInputsAreSizedAndDeterministic(t *testing.T) {
	inputs, _ := filepath.Glob(filepath.Join("testdata", "search", "*.input"))
	if len(inputs) == 0 {
		t.Fatal("no inputs")
	}
	for _, in := range inputs {
		raw, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) < 2048 {
			t.Errorf("%s is %d bytes, want >= 2 KiB", in, len(raw))
		}
		a := Search(string(raw), strat.Params{}, nil)
		b := Search(string(raw), strat.Params{}, nil)
		if a.Text != b.Text || a.Omitted != b.Omitted || strings.Join(a.Details, "|") != strings.Join(b.Details, "|") {
			t.Errorf("%s: not deterministic", in)
		}
		if len(a.Text) >= len(raw) {
			t.Errorf("%s: no savings (%d -> %d)", in, len(raw), len(a.Text))
		}
	}
}
