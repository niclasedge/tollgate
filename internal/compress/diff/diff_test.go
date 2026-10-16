package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/compress/compresstest"
	"github.com/niclasedge/tollgate/internal/compress/strat"
)

func TestGolden(t *testing.T) {
	compresstest.Golden(t, "diff", Diff)
}

// TestDeterministic runs every golden input twice and compares the bytes of
// the result and of every trace step.
func TestDeterministic(t *testing.T) {
	inputs, err := filepath.Glob("testdata/diff/*.input")
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no inputs: %v", err)
	}
	run := func(s string) string {
		var b strings.Builder
		r := Diff(s, strat.Params{}, func(step, text string) { b.WriteString(step + "\x00" + text + "\x00") })
		b.WriteString(strat.Render("diff", r))
		return b.String()
	}
	for _, in := range inputs {
		raw, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		first := run(string(raw))
		for i := 0; i < 3; i++ {
			if run(string(raw)) != first {
				t.Fatalf("%s: output differs between runs", in)
			}
		}
	}
}

func lines(ss ...string) string { return strings.Join(ss, "\n") + "\n" }

func TestParse(t *testing.T) {
	in := lines(
		"commit 0123456789abcdef",
		"Author: Dev <dev@example.com>",
		"",
		"    msg",
		"",
		"diff --git a/x.go b/x.go",
		"index 111..222 100644",
		"--- a/x.go",
		"+++ b/x.go",
		"@@ -1,3 +1,3 @@",
		" a",
		"-b",
		"+c",
		"\\ No newline at end of file",
		"diff --git a/img.png b/img.png",
		"index 333..444 100644",
		"Binary files a/img.png and b/img.png differ",
		"diff --git a/old.go b/new.go",
		"similarity index 90%",
		"rename from old.go",
		"rename to new.go",
		"commit fedcba9876543210",
		"Author: Dev <dev@example.com>",
	)
	d := parse(in)
	if !d.trailingNL || d.inputLines != 23 {
		t.Fatalf("trailingNL=%v inputLines=%d", d.trailingNL, d.inputLines)
	}
	if len(d.blocks) != 5 {
		t.Fatalf("blocks = %d, want raw+3 files+raw", len(d.blocks))
	}
	if got := d.blocks[0].raw; len(got) != 5 || got[0] != "commit 0123456789abcdef" {
		t.Errorf("preamble = %q", got)
	}
	f := d.blocks[1].f
	if f.path != "x.go" || len(f.hunks) != 1 || len(f.header) != 4 {
		t.Fatalf("file 1 = %+v", f)
	}
	if h := f.hunks[0]; len(h.lines) != 4 || h.lines[3] != "\\ No newline at end of file" {
		t.Errorf("hunk lines = %q", h.lines)
	}
	if f := d.blocks[2].f; len(f.hunks) != 0 || len(f.header) != 3 {
		t.Errorf("binary file = %+v", f)
	}
	if f := d.blocks[3].f; f.path != "new.go" || len(f.header) != 4 {
		t.Errorf("rename file = %+v", f)
	}
	if got := d.blocks[4].raw; len(got) != 2 || !strings.HasPrefix(got[0], "commit fedcba") {
		t.Errorf("next commit header = %q", got)
	}
	if got := d.render(); got != in {
		t.Errorf("untouched document does not round trip:\n%s", got)
	}
}

func TestParseKeepsCountsAuthoritative(t *testing.T) {
	// The format-patch signature starts with "-" but is not a hunk line.
	in := lines("diff --git a/x b/x", "--- a/x", "+++ b/x", "@@ -1 +1 @@", "-a", "+b", "-- ", "2.43.0")
	d := parse(in)
	if got := len(d.blocks[1].raw); got != 2 {
		t.Fatalf("raw tail = %d lines, want 2", got)
	}
}

func TestParseCombined(t *testing.T) {
	in := lines("diff --cc m.py", "index abc,def..ghi", "--- a/m.py", "+++ b/m.py", "@@@ -1,3 -1,3 +1,4 @@@", "  a", "+ b", " +c", "- d", "  e")
	d := parse(in)
	f := d.blocks[0].f
	if f.path != "m.py" || len(f.hunks) != 1 || f.hunks[0].width != 2 || len(f.hunks[0].lines) != 5 {
		t.Fatalf("combined file = %+v hunk=%+v", f, f.hunks[0])
	}
	if f.hunks[0].changes() != 3 {
		t.Errorf("changes = %d, want 3", f.hunks[0].changes())
	}
}

func TestIsLockfile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"package-lock.json", true},
		{"web/pnpm-lock.yaml", true},
		{"yarn.lock", true},
		{"go.sum", true},
		{"crates/x/Cargo.lock", true},
		{"poetry.lock", true},
		{"uv.lock", true},
		{"Pipfile.lock", true},
		{"composer.lock", true},
		{"MyCargo.lock", false},
		{"docs/go.sum.md", false},
		{"lock.go", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isLockfile(tt.path); got != tt.want {
			t.Errorf("isLockfile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func hunkOf(body ...string) *hunk {
	return &hunk{header: "@@ -1 +1 @@", lines: body, width: 1}
}

func TestWhitespaceOnly(t *testing.T) {
	tests := []struct {
		name string
		body []string
		want bool
	}{
		{"reindent", []string{"-\tfoo()", "+    foo()"}, true},
		{"trailing space", []string{"-foo() ", "+foo()"}, true},
		{"blank lines added", []string{" a", "+", " b"}, true},
		{"real change", []string{"-foo()", "+bar()"}, false},
		{"swapped order", []string{"-a", "-b", "+b", "+a"}, false},
		{"only context", []string{" a", " b"}, false},
		{"unbalanced", []string{"-a", "+a", "+b"}, false},
	}
	for _, tt := range tests {
		if got := hunkOf(tt.body...).isWhitespaceOnly(); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
	h := hunkOf("-a", "+a")
	h.width = 2
	if h.isWhitespaceOnly() {
		t.Error("combined hunk must never be whitespace-only")
	}
}

func TestTrimContext(t *testing.T) {
	body := []string{" c1", " c2", " c3", " c4", "-old", "+new", " d1", " d2", " d3", " d4"}
	h := hunkOf(body...)
	if n := h.trimContext(2); n != 4 {
		t.Fatalf("trimmed = %d, want 4", n)
	}
	d := &document{trailingNL: true, blocks: []block{{f: &file{header: []string{"diff --git a/x b/x"}, hunks: []*hunk{h}}}}}
	want := lines("diff --git a/x b/x", "@@ -1 +1 @@", " c3", " c4", "-old", "+new", " d1", " d2")
	if got := d.render(); got != want {
		t.Errorf("render:\n%s\nwant:\n%s", got, want)
	}
}

func TestTrimContextMarkers(t *testing.T) {
	// The marker after a kept line stays, the marker after a trimmed line goes.
	h := hunkOf("-a", "+b", "\\ No newline at end of file", " c1", " c2", " c3", " c4", "\\ No newline at end of file")
	h.trimContext(1)
	d := &document{blocks: []block{{f: &file{hunks: []*hunk{h}}}}}
	got := d.render()
	if strings.Count(got, "\\ No newline") != 1 {
		t.Errorf("markers: %q", got)
	}
}

func TestNeverDropsChangedLines(t *testing.T) {
	raw, err := os.ReadFile("testdata/diff/many_files_and_hunks.input")
	if err != nil {
		t.Fatal(err)
	}
	// Only the context step: every +/- line must survive, even with 1 line of context.
	d := parse(string(raw))
	trimContext(d, 1)
	got := d.render()
	changed := func(s string) (n int) {
		for _, l := range strings.Split(s, "\n") {
			if (strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-")) && !strings.HasPrefix(l, "+++") && !strings.HasPrefix(l, "---") {
				n++
			}
		}
		return
	}
	if changed(got) != changed(string(raw)) {
		t.Errorf("changed lines: got %d, want %d", changed(got), changed(string(raw)))
	}
	if len(got) >= len(raw) {
		t.Error("expected trimming to shrink the input")
	}
}

func TestLimitsFiles(t *testing.T) {
	var b strings.Builder
	for i, n := range []int{1, 5, 3, 5} { // changes per file
		name := "f" + string(rune('a'+i)) + ".go"
		b.WriteString(lines("diff --git a/"+name+" b/"+name, "--- a/"+name, "+++ b/"+name, "@@ -1 +1 @@"))
		for j := 0; j < n; j++ {
			b.WriteString("+x\n")
		}
	}
	r := Diff(b.String(), strat.Params{MaxFiles: 2, ContextLines: 3}, nil)
	for _, name := range []string{"fb.go", "fd.go"} { // the two largest, in input order
		if !strings.Contains(r.Text, "diff --git a/"+name) {
			t.Errorf("%s missing", name)
		}
	}
	if strings.Contains(r.Text, "fa.go") || strings.Contains(r.Text, "fc.go") {
		t.Errorf("small files kept:\n%s", r.Text)
	}
	if strings.Index(r.Text, "fb.go") > strings.Index(r.Text, "fd.go") {
		t.Error("kept files are not in input order")
	}
	if len(r.Details) != 1 || r.Details[0] != "2 Dateien ausgelassen: fa.go, fc.go" {
		t.Errorf("details = %q", r.Details)
	}
}

func TestLimitsHunks(t *testing.T) {
	var b strings.Builder
	b.WriteString(lines("diff --git a/x.go b/x.go", "--- a/x.go", "+++ b/x.go"))
	for i := 0; i < 6; i++ {
		b.WriteString("@@ -" + string(rune('1'+i)) + "0 +" + string(rune('1'+i)) + "0 @@\n")
		b.WriteString("-keep\n+h" + string(rune('0'+i)) + "\n")
		if i == 3 { // the only hunk with a priority word wins a middle slot
			b.WriteString("+// fix: panic on nil\n")
		}
	}
	r := Diff(b.String(), strat.Params{MaxHunks: 3, ContextLines: 3}, nil)
	for _, want := range []string{"+h0", "+h3", "+h5"} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("%s missing:\n%s", want, r.Text)
		}
	}
	for _, gone := range []string{"+h1", "+h2", "+h4"} {
		if strings.Contains(r.Text, gone) {
			t.Errorf("%s kept:\n%s", gone, r.Text)
		}
	}
	if !strings.Contains(r.Text, "[tollgate: 2 Hunks ausgelassen (Limit, 6 Zeilen)]") {
		t.Errorf("note missing:\n%s", r.Text)
	}
	if !strings.Contains(r.Text, "[tollgate: 1 Hunk ausgelassen (Limit, 3 Zeilen)]") {
		t.Errorf("single note missing:\n%s", r.Text)
	}
	if len(r.Details) != 1 || r.Details[0] != "3 Hunks ausgelassen" {
		t.Errorf("details = %q", r.Details)
	}
	one := Diff(b.String(), strat.Params{MaxHunks: 1}, nil)
	if strings.Count(one.Text, "\n@@ ") != 1 || !strings.Contains(one.Text, "+h0") {
		t.Errorf("MaxHunks 1 must keep exactly the first hunk:\n%s", one.Text)
	}
}

func TestNoiseResult(t *testing.T) {
	in := lines(
		"diff --git a/pnpm-lock.yaml b/pnpm-lock.yaml",
		"index 1..2 100644",
		"--- a/pnpm-lock.yaml",
		"+++ b/pnpm-lock.yaml",
		"@@ -1,2 +1,2 @@",
		"-  a: 1",
		"+  a: 2",
		"diff --git a/x.go b/x.go",
		"--- a/x.go",
		"+++ b/x.go",
		"@@ -1,2 +1,2 @@",
		"-\tx := 1",
		"+    x := 1",
		"@@ -9,2 +9,2 @@",
		"-y",
		"+z",
	)
	r := Diff(in, strat.Params{}, nil)
	want := lines(
		"diff --git a/pnpm-lock.yaml b/pnpm-lock.yaml",
		"index 1..2 100644",
		"--- a/pnpm-lock.yaml",
		"+++ b/pnpm-lock.yaml",
		"[tollgate: 1 Hunk ausgelassen (Lockfile, 3 Zeilen)]",
		"diff --git a/x.go b/x.go",
		"--- a/x.go",
		"+++ b/x.go",
		"[tollgate: 1 Hunk ausgelassen (nur Whitespace, 3 Zeilen)]",
		"@@ -9,2 +9,2 @@",
		"-y",
		"+z",
	)
	if r.Text != want {
		t.Errorf("text:\n%s\nwant:\n%s", r.Text, want)
	}
	if r.Omitted != 6 {
		t.Errorf("omitted = %d, want 6", r.Omitted)
	}
	if got := strings.Join(r.Details, "|"); got != "Lockfiles: pnpm-lock.yaml|1 Whitespace-Hunks" {
		t.Errorf("details = %q", got)
	}
}

func TestSteps(t *testing.T) {
	raw, err := os.ReadFile("testdata/diff/multi_file_pnpm_lock.input")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	var texts []string
	r := Diff(string(raw), strat.Params{}, func(step, text string) {
		names = append(names, step)
		texts = append(texts, text)
	})
	if strings.Join(names, ",") != "noise,limits,context" {
		t.Errorf("steps = %v", names)
	}
	if texts[2] != r.Text {
		t.Error("last step must equal the result text")
	}
	if len(texts[0]) >= len(raw) {
		t.Error("noise step should already shrink this input")
	}
}

func TestUnchangedInput(t *testing.T) {
	for _, in := range []string{
		"",
		"just some text\nwithout a diff\n",
		lines("diff --git a/x b/x", "--- a/x", "+++ b/x", "@@ -1 +1 @@", "-a", "+b"),
	} {
		r := Diff(in, strat.Params{}, nil)
		if r.Text != in || r.Omitted != 0 || len(r.Details) != 0 {
			t.Errorf("input %q changed: %+v", in, r)
		}
	}
}

func TestNoTrailingNewlinePreserved(t *testing.T) {
	in := strings.TrimSuffix(lines("diff --git a/pnpm-lock.yaml b/pnpm-lock.yaml", "--- a/pnpm-lock.yaml", "+++ b/pnpm-lock.yaml", "@@ -1 +1 @@", "-a", "+b"), "\n")
	r := Diff(in, strat.Params{}, nil)
	if strings.HasSuffix(r.Text, "\n") {
		t.Errorf("trailing newline added: %q", r.Text)
	}
}

func TestCommitHeadersIntact(t *testing.T) {
	raw, err := os.ReadFile("testdata/diff/git_log_p_3.input")
	if err != nil {
		t.Fatal(err)
	}
	r := Diff(string(raw), strat.Params{}, nil)
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(l, "commit ") || strings.HasPrefix(l, "Author: ") || strings.HasPrefix(l, "Date: ") || strings.HasPrefix(l, "    ") && !strings.HasPrefix(l, "     ") && strings.Contains(l, "(") {
			if !strings.Contains(r.Text, l+"\n") {
				t.Errorf("header line lost: %q", l)
			}
		}
	}
	if got := strings.Count(r.Text, "\ncommit ") + 1; got != 3 {
		t.Errorf("commits in output = %d, want 3", got)
	}
}

func TestDetailsSafe(t *testing.T) {
	var names []string
	for i := 0; i < 8; i++ {
		names = append(names, "dir/file"+string(rune('a'+i))+"]x\n.lock")
	}
	got := nameList(names)
	if strings.ContainsAny(got, "]\n") {
		t.Errorf("unsafe: %q", got)
	}
	if !strings.HasSuffix(got, ", …") || strings.Count(got, ", ") != 5 {
		t.Errorf("not truncated to 5 + ellipsis: %q", got)
	}
}

func TestOmittedCountsInputLines(t *testing.T) {
	raw, err := os.ReadFile("testdata/diff/multi_file_pnpm_lock.input")
	if err != nil {
		t.Fatal(err)
	}
	r := Diff(string(raw), strat.Params{}, nil)
	in := strings.Count(string(raw), "\n")
	notes := strings.Count(r.Text, "[tollgate: ")
	out := strings.Count(r.Text, "\n") - notes
	if r.Omitted != in-out {
		t.Errorf("omitted = %d, want %d", r.Omitted, in-out)
	}
}
