// Ported from Headroom crates/headroom-core/src/transforms/log_compressor.rs
// (LevelClassifier, StackTraceDetector, is_summary_line, score_log_line,
// parse_lines).
// Headroom (Apache-2.0, Copyright 2025 Headroom Contributors), Stand 0947a28,
// portiert und verändert: no format detection (the command picked the rule),
// extra line shapes for go test, cargo test, pytest and vitest/jest, a Go
// goroutine and a Rust backtrace flavor, and failure markers that are always
// kept.

package logs

import (
	"regexp"
	"strings"
)

type level int

const (
	levelUnknown level = iota
	levelError
	levelFail
	levelWarn
	levelInfo
	levelDebug
	levelTrace
)

// levelRe finds level words; leftmost-longest so "WARNING" wins over "WARN".
var levelRe = func() *regexp.Regexp {
	r := regexp.MustCompile(`ERROR|error|Error|FATAL|fatal|Fatal|CRITICAL|critical|` +
		`FAILED|FAIL|failed|fail|Failed|Fail|` +
		`WARNING|WARN|warning|warn|Warning|Warn|` +
		`INFO|info|Info|DEBUG|debug|Debug|TRACE|trace|Trace`)
	r.Longest()
	return r
}()

func levelOfWord(w string) level {
	switch w {
	case "ERROR", "error", "Error", "FATAL", "fatal", "Fatal", "CRITICAL", "critical":
		return levelError
	case "FAIL", "FAILED", "fail", "failed", "Fail", "Failed":
		return levelFail
	case "WARN", "WARNING", "warn", "warning", "Warn", "Warning":
		return levelWarn
	case "INFO", "info", "Info":
		return levelInfo
	case "DEBUG", "debug", "Debug":
		return levelDebug
	case "TRACE", "trace", "Trace":
		return levelTrace
	}
	return levelUnknown
}

func isWordByte(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_'
}

func wordBoundary(s string, start, end int) bool {
	return (start == 0 || !isWordByte(s[start-1])) && (end == len(s) || !isWordByte(s[end]))
}

// classifyLevel returns the level of the first level word that sits on a
// word boundary. Counts such as "0 failed" are no failure.
func classifyLevel(line string) level {
	for _, m := range levelRe.FindAllStringIndex(line, -1) {
		s, e := m[0], m[1]
		if !wordBoundary(line, s, e) || zeroCount(line, s) {
			continue
		}
		return levelOfWord(line[s:e])
	}
	return levelUnknown
}

// zeroCount reports "0 <word>" where the 0 is a whole number.
func zeroCount(line string, at int) bool {
	if at < 2 || line[at-1] != ' ' || line[at-2] != '0' {
		return false
	}
	if at >= 3 {
		c := line[at-3]
		return !(c >= '0' && c <= '9') && c != '.'
	}
	return true
}

var (
	reANSI = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

	// Lines that look like errors by accident (passing tests, progress).
	reGoEvent   = regexp.MustCompile(`^\s*(=== (RUN|PAUSE|CONT|NAME)\b|--- (PASS|SKIP|BENCH)\b)`)
	rePassLine  = regexp.MustCompile(`(\.\.\. (ok|ignored)$|\bPASSED\b( +\[ *\d+%\])?$|\bSKIPPED\b|^\s*(✓|✔|√)|^\s*PASS\s+\S)`)
	reProgress  = regexp.MustCompile(`^\s*(Compiling|Checking|Downloaded|Downloading|Fresh|Running|Doc-tests|Updating|Locking|Adding|Fetching|Installing|Unpacking|Packaging|Resolving)\s`)
	rePassSum   = regexp.MustCompile(`^(ok\s|PASS$|\?\s+\S+\s+\[no test files\]|test result: ok\.)`)
	reGoOK      = regexp.MustCompile(`^ok\s`)
	reErrExtra  = regexp.MustCompile(`^\s*panic: |panicked at |^Traceback \(most recent|^E {2,}\S|^\s*[A-Z]\w*(Error|Exception)\b|^\s*[\w./-]+\.(go|rs|c|cc|cpp|h|hpp|ts|tsx|js|jsx):\d+:\d+:\s`)
	reSumDigits = regexp.MustCompile(`^\d+ (passed|failed|skipped|error|warning)`)
	reSumTests  = regexp.MustCompile(`^(Tests?|Suites?|Test Files|Test Suites|Snapshots|Duration):?\s+\d`)
	reSumTotal  = regexp.MustCompile(`^(TOTAL|Total|Summary)`)
	reSumBuild  = regexp.MustCompile(`^(Build|Compile|Test).*(succeeded|failed|complete)`)
	reSumTool   = regexp.MustCompile(`^(ok\s|FAIL(\s|$)|PASS$|exit status \d+|test result:|Finished\s|error: (test failed|could not compile))`)

	// Failure markers: kept in any case, they carry the failing test name.
	reFailMark = regexp.MustCompile(`^\s*--- FAIL:|^FAIL(\s|$)|^\s*panic: |^fatal error: |\[(build|setup) failed\]|` +
		`^\s*thread '.*' panicked at|^test .* \.\.\. FAILED$|^test result: FAILED|^-{4} .* -{4}$|` +
		`^_{3,} .+ _{3,}$|^(FAILED|ERROR) \S+::|^\s*FAIL\s+\S|^\s*●\s|^\s*[×✗✘]\s|` +
		`^E {2,}\S|^>\s{2,}\S|^[\w./-]+\.py:\d+: |^\s*[A-Z]\w*(Error|Exception)\b|` +
		`^error: (test failed|could not compile)`)
	reFailHead  = regexp.MustCompile(`^\s*(--- FAIL:|●\s|FAIL\s+\S|[×✗✘]\s)`)
	reFailName  = regexp.MustCompile(`^    [\w:<>]+$`)
	reIndented  = regexp.MustCompile(`^(\s|\t)`)
	reCargoDiag = regexp.MustCompile(`^(warning|error)(\[\w+\])?:`)
)

// info is one classified line.
type info struct {
	text    string // original line
	level   level
	trace   bool
	summary bool
	passSum bool // ok/PASS lines of passing packages: kept only as a tail
	noise   bool // passing test or progress line, never an error
	hard    bool // failure marker, always kept
	score   float64
}

// classifyLine does the stateless part of the classification.
func classifyLine(raw string) info {
	line := strings.TrimRight(reANSI.ReplaceAllString(raw, ""), "\r")
	in := info{text: raw}
	switch {
	case reGoEvent.MatchString(line) || rePassLine.MatchString(line) || reProgress.MatchString(line):
		in.noise = true
		in.level = levelInfo
	case rePassSum.MatchString(line):
		in.passSum = true
		in.level = levelInfo
	default:
		in.level = classifyLevel(line)
		if reErrExtra.MatchString(line) {
			in.level = levelError
		}
		in.summary = isSummaryLine(line)
	}
	return in
}

// isSummaryLine detects the result lines of test runners and builds.
func isSummaryLine(line string) bool {
	if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") {
		return true
	}
	t := strings.TrimLeft(line, " \t")
	return reSumDigits.MatchString(line) || reSumTests.MatchString(t) ||
		reSumTotal.MatchString(line) || reSumBuild.MatchString(line) ||
		reSumTool.MatchString(line)
}

// isResultLine reports the run-level result lines ("FAIL pkg", "=== 2 failed
// ===", "Tests 2 failed"): they repeat what the failure lines already say, so
// they are not counted as separate failures.
func isResultLine(line string) bool {
	t := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(line, "===") || reSumTool.MatchString(line) ||
		reSumTests.MatchString(t) || reSumDigits.MatchString(line) || reSumBuild.MatchString(line)
}

// ---- stack traces ----

type flavor int

const (
	flavorNone flavor = iota
	flavorPython
	flavorJS
	flavorJava
	flavorRustArrow
	flavorGoDigits
	flavorGoRoutine
	flavorRustBacktrace
)

var (
	reGoroutine  = regexp.MustCompile(`^goroutine \d+ \[`)
	reRustBT     = regexp.MustCompile(`^\s+(\d+: |at )`)
	reGoCall     = regexp.MustCompile(`^\S+\(.*\)$`)
	reGoDigits   = regexp.MustCompile(`^\s*\d+:\s*0x[0-9a-fA-F]+`)
	rePyFrame    = regexp.MustCompile(`^File "[^"]*", line \d+`)
	reLineCol    = regexp.MustCompile(`:\d+:\d+`)
	reJavaMethod = regexp.MustCompile(`^at [A-Za-z0-9._$]+\(`)
)

func flavorFor(line string) flavor {
	t := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(t, "Traceback (most recent call last)") || isPythonFrame(t):
		return flavorPython
	case isJSFrame(t):
		return flavorJS
	case reJavaMethod.MatchString(t):
		return flavorJava
	case strings.HasPrefix(t, "--> ") && reLineCol.MatchString(t):
		return flavorRustArrow
	case reGoroutine.MatchString(line):
		return flavorGoRoutine
	case strings.HasPrefix(line, "stack backtrace:"):
		return flavorRustBacktrace
	case reGoDigits.MatchString(line):
		return flavorGoDigits
	}
	return flavorNone
}

func isPythonFrame(t string) bool {
	return rePyFrame.MatchString(t)
}

// isJSFrame accepts "at fn (file:1:2)", "at file:1:2" and vitest "❯ file:1:2".
func isJSFrame(t string) bool {
	if strings.HasPrefix(t, "at ") {
		return reLineCol.MatchString(t)
	}
	return strings.HasPrefix(t, "❯ ") && reLineCol.MatchString(t)
}

type traceState struct {
	flavor flavor
	lines  int
	sawExc bool // python: the exception line was already seen
}

const stackTraceMaxLines = 20

// terminates reports whether line ends the current run.
func (s *traceState) terminates(line string) bool {
	t := strings.TrimLeft(line, " \t")
	switch s.flavor {
	case flavorPython:
		// Blank lines and chained-exception markers continue the trace. A
		// non-indented line is the exception line (one allowed per block),
		// anything else ends the trace.
		cont := strings.HasPrefix(t, "Traceback") || strings.HasPrefix(t, "File ") ||
			strings.HasPrefix(t, "During handling") || strings.HasPrefix(t, "The above exception")
		if cont {
			s.sawExc = false
			return false
		}
		if line == "" || reIndented.MatchString(line) {
			return false
		}
		if !s.sawExc && t != "" && t[0] >= 'A' && t[0] <= 'Z' {
			s.sawExc = true
			return false
		}
		return true
	case flavorJS:
		return !strings.HasPrefix(t, "at ") && !strings.HasPrefix(t, "❯ ") && line != ""
	case flavorJava:
		return !strings.HasPrefix(t, "at ") && !strings.HasPrefix(t, "Caused by:") &&
			!strings.HasPrefix(t, "... ") && line != ""
	case flavorRustArrow:
		return !strings.HasPrefix(t, "--> ") && line != ""
	case flavorGoDigits:
		return !(t != "" && t[0] >= '0' && t[0] <= '9') && line != ""
	case flavorGoRoutine:
		// frames: "pkg.Func(args)", tab-indented "file.go:12 +0x1f", "created by ..."
		return !(reIndented.MatchString(line) || reGoCall.MatchString(line) ||
			strings.HasPrefix(line, "created by ") || strings.HasPrefix(line, "[originating"))
	case flavorRustBacktrace:
		return !reRustBT.MatchString(line)
	}
	return true
}

// ---- whole-log parse ----

const failBodyLines = 12

// parse classifies all lines, including the stateful parts (stack traces,
// failure bodies, cargo failure list).
func parse(lines []string) []info {
	out := make([]info, len(lines))
	var tr traceState
	failBody := 0
	inFailures := false
	for i, raw := range lines {
		in := classifyLine(raw)
		line := strings.TrimRight(reANSI.ReplaceAllString(raw, ""), "\r")

		if !in.noise && !in.passSum {
			in.hard = reFailMark.MatchString(line)
			switch {
			case reFailHead.MatchString(line):
				failBody = failBodyLines
			case failBody > 0:
				switch {
				case strings.TrimSpace(line) == "":
					failBody--
				case reIndented.MatchString(line):
					in.hard = true
					failBody--
				default:
					failBody = 0
				}
			}
			switch {
			case line == "failures:":
				inFailures = true
			case inFailures && reFailName.MatchString(line):
				in.hard = true
			case inFailures && line != "":
				inFailures = false
			}
		}

		// Stack-trace state machine: open on a flavor match, mark lines
		// until the flavor terminates or the cap is reached.
		if tr.flavor != flavorNone {
			if tr.lines >= stackTraceMaxLines || tr.terminates(line) {
				tr = traceState{}
				if f := flavorFor(line); f != flavorNone {
					tr = traceState{flavor: f, lines: 1}
					in.trace = true
				}
			} else {
				in.trace = true
				tr.lines++
			}
		} else if f := flavorFor(line); f != flavorNone {
			tr = traceState{flavor: f, lines: 1}
			in.trace = true
		}

		in.score = scoreLine(in)
		out[i] = in
	}
	return out
}

func scoreLine(in info) float64 {
	if in.hard {
		return 2
	}
	var s float64
	switch in.level {
	case levelError, levelFail:
		s = 1
	case levelWarn:
		s = 0.5
	case levelInfo, levelUnknown:
		s = 0.1
	case levelDebug:
		s = 0.05
	case levelTrace:
		s = 0.02
	}
	if in.trace {
		s += 0.3
	}
	if in.summary {
		s += 0.4
	}
	if s > 1 {
		s = 1
	}
	return s
}
