package strat

import (
	"strconv"
	"strings"
)

// Marker is the fixed last line of a compressed output, so the model knows
// that lines are missing (ADR-0007):
// "[tollgate: <rule> · <n> Zeilen ausgelassen · <details>]". It depends on
// the result only, never on time or version.
func Marker(rule string, r Result) string {
	var b strings.Builder
	b.WriteString("[tollgate: ")
	b.WriteString(rule)
	b.WriteString(" · ")
	b.WriteString(strconv.Itoa(r.Omitted))
	b.WriteString(" Zeilen ausgelassen")
	for _, d := range r.Details {
		b.WriteString(" · ")
		b.WriteString(d)
	}
	b.WriteString("]")
	return b.String()
}

// Render is the compressed output as it goes upstream: the text, a line
// break unless the text is empty or ends with one, and the marker.
func Render(rule string, r Result) string {
	if r.Text == "" || strings.HasSuffix(r.Text, "\n") {
		return r.Text + Marker(rule, r)
	}
	return r.Text + "\n" + Marker(rule, r)
}
