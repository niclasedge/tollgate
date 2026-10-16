package compress

import (
	"strconv"
	"strings"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

// RuleList lists the rules of c for the compress view.
func (c *Config) RuleList(version int64, enabled bool) RuleList {
	l := RuleList{Version: version, Enabled: enabled, MinBytes: c.MinBytes, Rules: make([]RuleInfo, 0, len(c.Rules))}
	for _, r := range c.Rules {
		l.Rules = append(l.Rules, RuleInfo{Name: r.Name, Match: r.Match, Strategy: r.Strategy, MaxErrors: r.Params.MaxErrors,
			ContextLines: r.Params.ContextLines, MaxFiles: r.Params.MaxFiles, MaxHunks: r.Params.MaxHunks, Builtin: r.Builtin})
	}
	return l
}

// Explain runs command and out through the same gates as Compress and
// records the text after every step, for the playground (#21).
func (c *Config) Explain(command, out string) Explanation {
	e := Explanation{Output: out, BytesBefore: len(out), BytesAfter: len(out), Steps: []Step{step("input", out)}}
	n, ok := Normalize(command)
	if !ok {
		e.Reason = "Kein einzelner Befehl: Pipes, Ketten, Heredocs und Ersetzungen bleiben unverändert."
		return e
	}
	e.Normalized = n
	r, ok := c.Match(command)
	if !ok {
		e.Reason = "Keine Regel passt zu diesem Befehl."
		return e
	}
	e.Rule, e.Strategy = r.Name, r.Strategy
	s, ok := Lookup(r.Strategy)
	if !ok {
		e.Reason = "Unbekannte Strategie " + r.Strategy + "."
		return e
	}
	res := s(out, r.Params, func(name, text string) { e.Steps = append(e.Steps, step(name, text)) })
	got := strat.Render(r.Name, res)
	e.Steps = append(e.Steps, step("result", got))
	switch {
	case len(out) < c.MinBytes:
		e.Reason = "Ausgabe hat weniger als " + strconv.Itoa(c.MinBytes) + " Bytes und bleibt unverändert."
	case res.Text == out:
		e.Reason = "Die Strategie ändert nichts an dieser Ausgabe."
	case len(got) >= len(out):
		e.Reason = "Das Ergebnis mit Markerzeile wäre nicht kleiner."
	default:
		e.Output, e.BytesAfter, e.Omitted, e.Applied = got, len(got), res.Omitted, true
	}
	return e
}

func step(name, text string) Step {
	lines := strings.Count(text, "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		lines++
	}
	return Step{Name: name, Text: text, Bytes: len(text), Lines: lines}
}
