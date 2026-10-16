package compress

import (
	"github.com/niclasedge/tollgate/internal/compress/diff"
	"github.com/niclasedge/tollgate/internal/compress/logs"
	"github.com/niclasedge/tollgate/internal/compress/search"
	"github.com/niclasedge/tollgate/internal/compress/strat"
)

// strategies maps the strategy names of the configuration schema to their
// implementation (ADR-0007, ports of Headroom in the subpackages).
var strategies = map[string]strat.Strategy{
	"test_log":    logs.TestLog,
	"build_log":   logs.BuildLog,
	"diff":        diff.Diff,
	"search":      search.Search,
	"passthrough": func(out string, _ strat.Params, _ strat.Trace) strat.Result { return strat.Result{Text: out} },
}

// Lookup returns the strategy of a name.
func Lookup(name string) (strat.Strategy, bool) {
	s, ok := strategies[name]
	return s, ok
}

// Applied describes one compressed output for the trace.
type Applied struct {
	Rule        string
	BytesBefore int
	BytesAfter  int
	Omitted     int
}

// Compress applies the rule of command to the output of that command. It
// returns the output unchanged and false unless all gates of ADR-0007 pass:
// out has at least MinBytes, a rule matches, its strategy exists and the
// result including the marker line is strictly smaller.
func (c *Config) Compress(command, out string) (string, Applied, bool) {
	if len(out) < c.MinBytes {
		return out, Applied{}, false
	}
	r, ok := c.Match(command)
	if !ok {
		return out, Applied{}, false
	}
	s, ok := Lookup(r.Strategy)
	if !ok {
		return out, Applied{}, false
	}
	res := s(out, r.Params, nil)
	if res.Text == out {
		return out, Applied{}, false
	}
	got := strat.Render(r.Name, res)
	if len(got) >= len(out) {
		return out, Applied{}, false
	}
	return got, Applied{Rule: r.Name, BytesBefore: len(out), BytesAfter: len(got), Omitted: res.Omitted}, true
}
