package compress

import (
	"os"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

func stepNames(e Explanation) string {
	names := make([]string, len(e.Steps))
	for i, s := range e.Steps {
		names[i] = s.Name
	}
	return strings.Join(names, ",")
}

// Explain must decide exactly like Compress, whatever gate applies.
func TestExplainMatchesCompress(t *testing.T) {
	withStrategy(t, "keep_first", keepFirst)
	withStrategy(t, "same", func(out string, _ strat.Params, _ strat.Trace) strat.Result {
		return strat.Result{Text: out[:len(out)-1]}
	})
	cfg := &Config{MinBytes: 2048, Rules: []Rule{
		{Name: "off", Match: []string{"go test -run X"}, Strategy: "passthrough"},
		{Name: "go_test", Match: []string{"go test"}, Strategy: "keep_first"},
		{Name: "unknown", Match: []string{"rg"}, Strategy: "no_such_strategy"},
		{Name: "same", Match: []string{"make"}, Strategy: "same"},
	}}
	big := bigOutput(200)
	for _, c := range []struct{ name, cmd, out, reason, steps string }{
		{"compressed", "cd /src && go test ./...", big, "", "input,result"},
		{"below min_bytes", "go test ./...", bigOutput(10), "weniger als 2048 Bytes", "input,result"},
		{"no rule", "ls -la", big, "Keine Regel", "input"},
		{"passthrough", "go test -run X ./...", big, "ändert nichts", "input,result"},
		{"pipeline", "go test ./... | tail", big, "Kein einzelner Befehl", "input"},
		{"unknown strategy", "rg foo", big, "Unbekannte Strategie", "input"},
		{"not smaller", "make", big, "nicht kleiner", "input,result"},
	} {
		want, a, ok := cfg.Compress(c.cmd, c.out)
		e := cfg.Explain(c.cmd, c.out)
		if e.Output != want || e.Applied != ok || e.BytesAfter != len(want) || e.BytesBefore != len(c.out) {
			t.Errorf("%s: explain %+v differs from compress (%v, %+v)", c.name, e.Applied, ok, a)
		}
		if ok && (e.Rule != a.Rule || e.Omitted != a.Omitted) {
			t.Errorf("%s: rule %s omitted %d, want %+v", c.name, e.Rule, e.Omitted, a)
		}
		if (c.reason == "") != (e.Reason == "") || !strings.Contains(e.Reason, c.reason) {
			t.Errorf("%s: reason %q, want %q", c.name, e.Reason, c.reason)
		}
		if got := stepNames(e); got != c.steps {
			t.Errorf("%s: steps %s, want %s", c.name, got, c.steps)
		}
	}
	if e := cfg.Explain("cd /src && go test ./...", big); e.Normalized != "go test ./..." || e.Strategy != "keep_first" {
		t.Errorf("normalized %q strategy %q", e.Normalized, e.Strategy)
	}
}

// The steps of a real strategy show up between input and result.
func TestExplainShowsStrategySteps(t *testing.T) {
	in, err := os.ReadFile("logs/testdata/test_log/go_test_fail.input")
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := os.ReadFile("logs/testdata/test_log/go_test_fail.cmd")
	if err != nil {
		t.Fatal(err)
	}
	e := Default().Explain(strings.TrimSpace(string(cmd)), string(in))
	if got := stepNames(e); got != "input,dedupe,select,context,result" {
		t.Fatalf("steps = %s", got)
	}
	last := e.Steps[len(e.Steps)-1]
	if !e.Applied || last.Text != e.Output || last.Bytes != len(e.Output) || e.Rule != "go_test" {
		t.Errorf("explanation = rule %s applied %v reason %q", e.Rule, e.Applied, e.Reason)
	}
	if first := e.Steps[0]; first.Text != string(in) || first.Lines != strings.Count(string(in), "\n") {
		t.Errorf("input step = %d bytes, %d lines", first.Bytes, first.Lines)
	}
}

func TestStepLines(t *testing.T) {
	for text, want := range map[string]int{"": 0, "a": 1, "a\n": 1, "a\nb": 2, "a\nb\n": 2} {
		if got := step("x", text).Lines; got != want {
			t.Errorf("%q: %d lines, want %d", text, got, want)
		}
	}
}

func TestRuleList(t *testing.T) {
	min := 100
	c := WithDefaults(&min, []Rule{{Name: "mine", Match: []string{"go test"}, Strategy: "diff", Params: strat.Params{MaxFiles: 3}}})
	l := c.RuleList(7, true)
	if l.Version != 7 || !l.Enabled || l.MinBytes != 100 || len(l.Rules) != len(c.Rules) {
		t.Fatalf("list = %+v", l)
	}
	if r := l.Rules[0]; r.Name != "mine" || r.Builtin || r.MaxFiles != 3 || r.Strategy != "diff" {
		t.Errorf("own rule = %+v", r)
	}
	for _, r := range l.Rules[1:] {
		if !r.Builtin {
			t.Errorf("%s must be builtin", r.Name)
		}
	}
}
