package logs

import (
	"fmt"
	"strings"
	"testing"
)

func buildLines() string {
	var l []string
	for i := 0; i < 12; i++ {
		l = append(l, fmt.Sprintf("   Compiling crate%d v1.%d.0", i, i))
	}
	l = append(l, "warning: unused variable: `x`", " --> src/a.rs:1:1", "", "Finished dev in 1s", "done", "end")
	return strings.Join(l, "\n") + "\n"
}

func TestCollapseRun(t *testing.T) {
	r := BuildLog(buildLines(), paramsZero, nil)
	for _, want := range []string{
		"[Template T1: Compiling <*> <*>] (12 Zeilen)",
		"crate0 v1.0.0", "crate1 v1.1.0", "[… 8 weitere]", "crate10 v1.10.0", "crate11 v1.11.0",
		"warning: unused variable: `x`", " --> src/a.rs:1:1", "Finished dev in 1s",
	} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("missing %q in\n%s", want, r.Text)
		}
	}
	if strings.Contains(r.Text, "crate5") {
		t.Error("middle variants must be elided")
	}
	if len(r.Details) != 1 || r.Details[0] != "12 gleichartige Zeilen" {
		t.Errorf("details = %v", r.Details)
	}
	if r.Omitted <= 0 {
		t.Errorf("omitted = %d", r.Omitted)
	}
}

func TestShortRunsStayVerbatim(t *testing.T) {
	var l []string
	for i := 0; i < 8; i++ { // alternating shapes never form a run of 3
		l = append(l, fmt.Sprintf("   Compiling crate%d v1.0.0", i), fmt.Sprintf("other %d line here now", i))
	}
	in := strings.Join(l, "\n") + "\nx\ny\nz\n"
	if r := BuildLog(in, paramsZero, nil); r.Text != in {
		t.Errorf("changed:\n%s", r.Text)
	}
}

func TestRunWithoutAnchorStays(t *testing.T) {
	var l []string
	for i := 0; i < 12; i++ {
		l = append(l, fmt.Sprintf("+ pkg%d 1.%d.0", i, i))
	}
	in := strings.Join(l, "\n") + "\nx\ny\nz\n"
	if r := BuildLog(in, paramsZero, nil); r.Text != in {
		t.Errorf("collapsed a run without a word-like constant:\n%s", r.Text)
	}
}

func TestRepeatedWarningBlocksAppearOnce(t *testing.T) {
	block := []string{"warning: unused variable: `idx`", "  --> src/lexer.rs:88:13", "   |", "88 |     let idx = 1;", "   |         ^^^"}
	var l []string
	l = append(l, block...)
	l = append(l, "")
	l = append(l, "   Compiling a v1.0.0")
	l = append(l, block...)
	l = append(l, "")
	l = append(l, "warning: unused variable: `other`", "  --> src/lexer.rs:90:13", "")
	l = append(l, "x", "y", "Finished in 1s")
	r := BuildLog(strings.Join(l, "\n")+"\n", paramsZero, nil)
	if got := strings.Count(r.Text, "unused variable: `idx`"); got != 1 {
		t.Errorf("idx warning kept %d times\n%s", got, r.Text)
	}
	if !strings.Contains(r.Text, "unused variable: `other`") {
		t.Error("distinct warning lost")
	}
	if r.Details[0] != "1 Warnungen zusammengefasst" {
		t.Errorf("details = %v", r.Details)
	}
}

func TestErrorsAndTailStayIntact(t *testing.T) {
	var l []string
	for i := 0; i < 10; i++ {
		l = append(l, fmt.Sprintf("#5 %d.1 Setting up lib%d (1.%d) ...", i, i, i))
	}
	l = append(l, "#5 ERROR: process did not complete successfully: exit code: 1", "#6 DONE 0.1s", "#7 DONE 0.2s", "#8 DONE 0.3s")
	r := BuildLog(strings.Join(l, "\n")+"\n", paramsZero, nil)
	for _, want := range []string{"#5 ERROR: process did not complete successfully: exit code: 1", "#6 DONE 0.1s", "#7 DONE 0.2s", "#8 DONE 0.3s"} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("lost %q", want)
		}
	}
	if r.Details[0] != "1 Fehler" {
		t.Errorf("details = %v", r.Details)
	}
}
