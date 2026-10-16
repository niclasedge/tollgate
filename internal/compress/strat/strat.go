// Package strat is the contract between the compress stage and its
// strategies (ADR-0007). Each strategy lives in its own package and depends
// only on this one.
package strat

// Params are the limits of one rule (compress.rules[] of the configuration).
// Zero means the strategy default.
type Params struct {
	MaxErrors    int
	ContextLines int
	MaxFiles     int
	MaxHunks     int
}

// Result is what a strategy made of one output.
type Result struct {
	// Text is the shortened output without the marker line.
	Text string
	// Omitted counts the input lines that are not in Text.
	Omitted int
	// Details are short facts for the marker line in a fixed order, e.g.
	// "2 Fehler" or "Lockfiles: pnpm-lock.yaml". They never contain "]" or a
	// line break.
	Details []string
}

// Trace receives the intermediate text after each step of a strategy, for
// the playground (#21). It may be nil.
type Trace func(step, text string)

// Step reports one intermediate text; a nil Trace ignores it.
func (t Trace) Step(name, text string) {
	if t != nil {
		t(name, text)
	}
}

// Strategy turns one Bash output into a shorter one. It must be pure and
// deterministic: no time, no randomness, no map iteration order. Returning
// the input unchanged is always allowed; the stage keeps a result only when
// it is strictly smaller (ADR-0007 gates).
type Strategy func(out string, p Params, t Trace) Result
