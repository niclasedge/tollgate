package compress

import (
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

// keepFirst is a test strategy: the first line plus the number of others.
func keepFirst(out string, _ strat.Params, _ strat.Trace) strat.Result {
	lines := strings.Split(out, "\n")
	return strat.Result{Text: lines[0], Omitted: len(lines) - 1, Details: []string{"Test"}}
}

func withStrategy(t *testing.T, name string, s strat.Strategy) {
	t.Helper()
	prev, had := strategies[name]
	strategies[name] = s
	t.Cleanup(func() {
		if had {
			strategies[name] = prev
		} else {
			delete(strategies, name)
		}
	})
}

func bigOutput(n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString("=== RUN   TestSomething/case_")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString("\n")
	}
	return b.String()
}

func TestCompressGates(t *testing.T) {
	withStrategy(t, "keep_first", keepFirst)
	cfg := &Config{MinBytes: 2048, Rules: []Rule{
		{Name: "off", Match: []string{"go test -run X"}, Strategy: "passthrough"},
		{Name: "go_test", Match: []string{"go test"}, Strategy: "keep_first"},
		{Name: "unknown", Match: []string{"rg"}, Strategy: "no_such_strategy"},
	}}
	big := bigOutput(200)

	got, a, ok := cfg.Compress("cd /src/app && go test ./...", big)
	if !ok {
		t.Fatal("expected compression")
	}
	want := "=== RUN   TestSomething/case_\n[tollgate: go_test · 200 Zeilen ausgelassen · Test]"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if a.Rule != "go_test" || a.BytesBefore != len(big) || a.BytesAfter != len(want) || a.Omitted != 200 {
		t.Errorf("applied = %+v", a)
	}

	for _, c := range []struct{ name, cmd, out string }{
		{"below min_bytes", "go test ./...", bigOutput(10)},
		{"no rule", "ls -la", big},
		{"passthrough rule first", "go test -run X ./...", big},
		{"pipeline", "go test ./... | tail", big},
		{"unknown strategy", "rg foo", big},
	} {
		if got, _, ok := cfg.Compress(c.cmd, c.out); ok || got != c.out {
			t.Errorf("%s: compressed to %q", c.name, got)
		}
	}
}

func TestCompressOnlyWhenSmaller(t *testing.T) {
	withStrategy(t, "same", func(out string, _ strat.Params, _ strat.Trace) strat.Result {
		return strat.Result{Text: out[:len(out)-1], Omitted: 0}
	})
	cfg := &Config{MinBytes: 1, Rules: []Rule{{Name: "x", Match: []string{"go test"}, Strategy: "same"}}}
	out := bigOutput(100)
	if got, _, ok := cfg.Compress("go test", out); ok || got != out {
		t.Fatal("result with marker is not smaller; the original must stay")
	}
}

func TestDefaultConfig(t *testing.T) {
	d := Default()
	if d.MinBytes != DefaultMinBytes {
		t.Errorf("min bytes = %d", d.MinBytes)
	}
	names := map[string]bool{}
	for _, r := range d.Rules {
		if names[r.Name] {
			t.Errorf("rule %s twice", r.Name)
		}
		names[r.Name] = true
		if _, ok := Lookup(r.Strategy); !ok {
			t.Errorf("rule %s: unknown strategy %s", r.Name, r.Strategy)
		}
	}
	for cmd, rule := range map[string]string{
		"go test ./...":          "go_test",
		"cargo test --workspace": "cargo_test",
		"pnpm vitest run":        "js_test",
		"uv run pytest -q":       "pytest",
		"git diff":               "git_diff",
		"git log -p -3":          "git_diff",
		"pnpm install":           "js_install",
		"docker build .":         "build",
		"make test":              "make_test",
		"make":                   "build",
	} {
		r, ok := d.Match(cmd)
		if !ok || r.Name != rule {
			t.Errorf("%s: rule %q (%v), want %s", cmd, r.Name, ok, rule)
		}
	}
	if _, ok := d.Match("cat README.md"); ok {
		t.Error("cat must not match a rule")
	}
}

func TestWithDefaults(t *testing.T) {
	min := 100
	c := WithDefaults(&min, []Rule{{Name: "mine", Match: []string{"go test"}, Strategy: "passthrough"}})
	if c.MinBytes != 100 || c.Rules[0].Name != "mine" || len(c.Rules) != len(Default().Rules)+1 {
		t.Fatalf("config = %+v", c)
	}
	if r, _ := c.Match("go test ./..."); r.Name != "mine" {
		t.Errorf("own rules come first, got %s", r.Name)
	}
	if c := WithDefaults(nil, nil); c.MinBytes != DefaultMinBytes {
		t.Errorf("default min bytes, got %d", c.MinBytes)
	}
}
