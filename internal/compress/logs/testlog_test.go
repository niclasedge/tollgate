package logs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

var paramsZero = strat.Params{}

// noisy builds a log with n errors spread among filler lines.
func noisy(n int) string {
	var l []string
	for i := 0; i < n; i++ {
		l = append(l, repeat("filler line without anything special", 8)...)
		l = append(l, fmt.Sprintf("ERROR worker %d crashed", i))
	}
	l = append(l, repeat("trailing filler", 8)...)
	return strings.Join(l, "\n") + "\n"
}

func countPrefix(text, prefix string) int {
	n := 0
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

func TestLimitMaxErrors(t *testing.T) {
	in := noisy(20)
	if got := countPrefix(TestLog(in, paramsZero, nil).Text, "ERROR"); got != 5 {
		t.Errorf("default keeps %d errors, want 5", got)
	}
	if got := countPrefix(TestLog(in, strat.Params{MaxErrors: 2}, nil).Text, "ERROR"); got < 2 || got > 2+2*0 {
		// context lines are fillers, so exactly the cap remains
		t.Errorf("MaxErrors 2 keeps %d errors", got)
	}
	text := TestLog(in, paramsZero, nil).Text
	if !strings.Contains(text, "ERROR worker 0 crashed") || !strings.Contains(text, "ERROR worker 19 crashed") {
		t.Error("first and last error must be kept")
	}
}

func TestLimitContextLines(t *testing.T) {
	in := noisy(1)
	for _, c := range []struct{ ctx, fillers int }{{0, 3}, {1, 1}, {3, 3}, {5, 5}} {
		got := TestLog(in, strat.Params{ContextLines: c.ctx}, nil)
		want := c.fillers
		if c.ctx == 0 { // zero means default 3
			want = 3
		}
		if f := strings.Count(got.Text, "filler"); f != 2*want {
			t.Errorf("ctx %d: %d filler lines kept, want %d around the error", c.ctx, f, 2*want)
		}
	}
}

func TestRepeatedWarningsCollapse(t *testing.T) {
	var l []string
	for i := 0; i < 30; i++ {
		l = append(l, "plain line")
		l = append(l, fmt.Sprintf("WARNING: slow query took %d ms at 0x%x", 100+i, 4096+i))
	}
	r := TestLog(strings.Join(l, "\n"), paramsZero, nil)
	if got := countPrefix(r.Text, "WARNING"); got != 1 {
		t.Errorf("kept %d identical warnings, want 1", got)
	}
	found := false
	for _, d := range r.Details {
		if d == "29 Warnungen zusammengefasst" {
			found = true
		}
	}
	if !found {
		t.Errorf("details = %v", r.Details)
	}
}

func TestTraceSteps(t *testing.T) {
	var steps []string
	TestLog(noisy(3), paramsZero, func(step, _ string) { steps = append(steps, step) })
	if strings.Join(steps, ",") != "dedupe,select,context" {
		t.Errorf("steps = %v", steps)
	}
	steps = nil
	BuildLog(buildLines(), paramsZero, func(step, _ string) { steps = append(steps, step) })
	if strings.Join(steps, ",") != "dedupe,template" {
		t.Errorf("steps = %v", steps)
	}
}

func TestOmittedAndDetails(t *testing.T) {
	in := noisy(10)
	r := TestLog(in, paramsZero, nil)
	if want := len(splitLines(in)) - len(strings.Split(r.Text, "\n")); r.Omitted != want {
		t.Errorf("Omitted = %d, want %d", r.Omitted, want)
	}
	if len(r.Details) == 0 || r.Details[0] != "10 Fehler" {
		t.Errorf("details = %v", r.Details)
	}
}

func TestAllGreenKeepsOnlyTail(t *testing.T) {
	var l []string
	for i := 0; i < 40; i++ {
		l = append(l, fmt.Sprintf("=== RUN   TestOK%d", i), fmt.Sprintf("--- PASS: TestOK%d (0.00s)", i))
	}
	l = append(l, "PASS", "ok  \texample.com/x\t0.2s")
	r := TestLog(strings.Join(l, "\n"), paramsZero, nil)
	if r.Text != "PASS\nok  \texample.com/x\t0.2s" {
		t.Errorf("text = %q", r.Text)
	}
}

func TestFailuresSurvive(t *testing.T) {
	var l []string
	for i := 0; i < 30; i++ {
		l = append(l, fmt.Sprintf("=== RUN   TestPass%d", i), fmt.Sprintf("--- PASS: TestPass%d (0.00s)", i))
	}
	for i := 0; i < 8; i++ { // more failures than MaxErrors
		l = append(l, fmt.Sprintf("--- FAIL: TestBad%d (0.00s)", i), fmt.Sprintf("    x_test.go:%d: want 1, got 2", i+10))
	}
	l = append(l, "FAIL", "FAIL\texample.com/x\t0.1s")
	text := TestLog(strings.Join(l, "\n"), paramsZero, nil).Text
	for i := 0; i < 8; i++ {
		for _, want := range []string{fmt.Sprintf("--- FAIL: TestBad%d", i), fmt.Sprintf("x_test.go:%d: want 1, got 2", i+10)} {
			if !strings.Contains(text, want) {
				t.Errorf("lost %q", want)
			}
		}
	}
	if strings.Count(text, "TestPass") > 0 {
		t.Error("passing tests must go, also from the context")
	}
}
