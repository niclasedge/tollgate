package logs

import (
	"strings"
	"testing"
)

func TestClassifyLevel(t *testing.T) {
	cases := []struct {
		line string
		want level
	}{
		{"ERROR something broke", levelError},
		{"a fatal condition", levelError},
		{"FAIL github.com/x/y", levelFail},
		{"test a ... FAILED", levelFail},
		{"WARNING: deprecated", levelWarn},
		{"npm warn deprecated x", levelWarn},
		{"INFO started", levelInfo},
		{"DEBUG details", levelDebug},
		{"TRACE enter", levelTrace},
		{"nothing special", levelUnknown},
		// word boundaries: substrings do not fire
		{"TestErrorHandling", levelUnknown},
		{"terrorism informed", levelUnknown},
		{"errors and failures", levelUnknown},
		// first valid match wins, longest word at one position
		{"warning: error ahead", levelWarn},
		{"xerror error", levelError},
		// zero counts are no failure
		{"Tests: 0 failed, 5 passed", levelUnknown},
		{"Tests: 10 failed", levelFail},
		{"summary 2.0 failed", levelFail},
	}
	for _, c := range cases {
		if got := classifyLevel(c.line); got != c.want {
			t.Errorf("classifyLevel(%q) = %d, want %d", c.line, got, c.want)
		}
	}
}

func TestClassifyLineNoise(t *testing.T) {
	// passing tests and progress lines are never errors
	for _, l := range []string{
		"--- PASS: TestParse/error (0.00s)",
		"=== RUN   TestFailureModes",
		"   Compiling error-chain v0.12.4",
		"test a::error ... ok",
		"tests/test_a.py::test_error PASSED [ 10%]",
		"ok  \texample.com/app/error\t0.1s",
		"test result: ok. 3 passed; 0 failed",
	} {
		in := classifyLine(l)
		if in.level != levelInfo || (in.hard) {
			t.Errorf("%q: level %d, want info", l, in.level)
		}
	}
}

func TestSummaryLines(t *testing.T) {
	yes := []string{
		"=== 2 failed, 52 passed in 4.31s ===",
		"2 failed, 52 passed",
		"      Tests  2 failed | 62 passed (64)",
		" Test Files  1 failed | 7 passed (8)",
		"Test Suites: 1 failed, 3 passed, 4 total",
		"test result: FAILED. 29 passed; 1 failed",
		"ok  \texample.com/x\t0.1s",
		"FAIL\texample.com/x\t0.1s",
		"exit status 2",
		"Total: 5",
		"Build succeeded",
	}
	no := []string{"some text", "--- PASS: TestX (0.00s)", "=== RUN TestX", "    Tests are great"}
	for _, l := range yes {
		if !classifyLine(l).summary && !classifyLine(l).passSum {
			t.Errorf("not a summary: %q", l)
		}
	}
	for _, l := range no {
		if classifyLine(l).summary {
			t.Errorf("wrongly a summary: %q", l)
		}
	}
}

func traceFlags(lines ...string) []bool {
	out := make([]bool, len(lines))
	for i, in := range parse(lines) {
		out[i] = in.trace
	}
	return out
}

func TestStackTraceFlavors(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  []bool
	}{
		{"python", []string{"ok", "Traceback (most recent call last):", `  File "a.py", line 3, in f`, "    x()", "ValueError: bad", "next line", "lower"},
			[]bool{false, true, true, true, true, false, false}},
		{"python chained", []string{"Traceback (most recent call last):", `  File "a.py", line 1, in <module>`, "ValueError: x", "", "During handling of the above exception, another exception occurred:", "", "Traceback (most recent call last):", `  File "b.py", line 2, in <module>`, "RuntimeError: y", "after"},
			[]bool{true, true, true, true, true, true, true, true, true, false}},
		{"js", []string{"Error: x", "    at fn (/src/a.js:10:5)", "    at Object.<anonymous> (/src/b.js:2:1)", "done"},
			[]bool{false, true, true, false}},
		{"java", []string{"Exception in thread main", "\tat com.x.Foo.bar(Foo.java:42)", "\tat com.x.Main.main(Main.java:7)", "Caused by: java.io.IOException", "\tat com.x.Io.read(Io.java:1)", "\t... 3 more", "end"},
			[]bool{false, true, true, true, true, true, false}},
		{"rust arrow", []string{"error[E0425]: nope", " --> src/lib.rs:12:9", "  |"},
			[]bool{false, true, false}},
		{"rust backtrace", []string{"panicked", "stack backtrace:", "   0: rust_begin_unwind", "             at /rustc/x/panicking.rs:1:1", "note: more"},
			[]bool{false, true, true, true, false}},
		{"go goroutine", []string{"panic: boom", "", "goroutine 7 [running]:", "main.f()", "\t/src/app/main.go:9 +0x1", "", "exit status 2"},
			[]bool{false, false, true, true, true, false, false}},
		{"go digits", []string{"x", "0:  0x4f2a31", "1:  0x4f2b00", "done"},
			[]bool{false, true, true, false}},
	}
	for _, c := range cases {
		got := traceFlags(c.lines...)
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: line %d %q trace=%v, want %v", c.name, i, c.lines[i], got[i], c.want[i])
			}
		}
	}
}

func TestStackTraceCap(t *testing.T) {
	lines := []string{"Traceback (most recent call last):"}
	for i := 0; i < 30; i++ {
		lines = append(lines, `  File "a.py", line 1, in f`)
	}
	n := 0
	for _, f := range traceFlags(lines...) {
		if f {
			n++
		}
	}
	if n != 31 { // the cap re-opens a run on the next frame; the run is contiguous
		t.Errorf("trace lines = %d", n)
	}
}

func TestNormalizeForDedupe(t *testing.T) {
	cases := []struct{ in, want string }{
		// hex is replaced before decimals
		{"leak: at 0x1F3a in 12 blocks", "leak: at ADDR in N blocks"},
		{"x=0xdeadbeef", "x=ADDR"},
		// prefix before the first ':' or '=' is kept verbatim
		{"segfault at 0xdeadbeef in main", "segfault at 0xdeadbeef in main"},
		{"warning: file /tmp/a/123 issue", "warning: file /PATH/N issue"},
		{"no separator 42", "no separator 42"},
	}
	for _, c := range cases {
		if got := normalizeForDedupe(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if normalizeForDedupe("warning: file /tmp/a/123 issue") != normalizeForDedupe("warning: file /tmp/b/999 issue") {
		t.Error("same warning with other numbers must collapse")
	}
	if normalizeForDedupe("warning: unused `a`") == normalizeForDedupe("warning: unused `b`") {
		t.Error("different names must stay distinct")
	}
}

func TestScoreCap(t *testing.T) {
	if s := scoreLine(info{level: levelError, trace: true, summary: true}); s != 1 {
		t.Errorf("score = %v, want 1", s)
	}
	if s := scoreLine(info{hard: true}); s != 2 {
		t.Errorf("hard score = %v", s)
	}
}

func repeat(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = prefix
	}
	return out
}

func TestTinyInputUnchanged(t *testing.T) {
	in := "a\nERROR b\nc\n"
	if r := TestLog(in, paramsZero, nil); r.Text != in || r.Omitted != 0 || r.Details != nil {
		t.Errorf("TestLog changed tiny input: %+v", r)
	}
	if r := BuildLog(in, paramsZero, nil); r.Text != in || r.Omitted != 0 {
		t.Errorf("BuildLog changed tiny input: %+v", r)
	}
	_ = strings.TrimSpace
}
