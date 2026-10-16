package jsonsplice

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"github.com/go-json-experiment/json/jsontext"
)

// FuzzApply checks the ADR-0004 invariants: no replacements keep the bytes
// identical; every replacement yields valid JSON whose target decodes to the
// new value; bytes outside the replaced ranges stay identical.
func FuzzApply(f *testing.F) {
	f.Add([]byte(sampleBody), "<redacted:email_1>")
	f.Add([]byte(`{"messages":[{"role":"user","content":"a"}]}`), "line\nbreak \"q\" \\ \x01 ä")
	f.Add([]byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"😀"}]}]}`), "")
	f.Fuzz(func(t *testing.T, body []byte, repl string) {
		doc, err := Scan(body)
		if err != nil {
			return
		}
		same, err := Apply(body, nil)
		if err != nil || !bytes.Equal(same, body) {
			t.Fatalf("identity broken: %v", err)
		}
		if !utf8.ValidString(repl) {
			return
		}
		reps := make([]Replacement, len(doc.Targets))
		for i, tg := range doc.Targets {
			reps[i] = Replacement{Start: tg.Start, End: tg.End, New: repl}
		}
		out, err := Apply(body, reps)
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if !jsontext.Value(out).IsValid() {
			t.Fatalf("invalid JSON: %q", out)
		}
		again, err := Scan(out)
		if err != nil || len(again.Targets) != len(doc.Targets) {
			t.Fatalf("rescan: %v", err)
		}
		for _, tg := range again.Targets {
			if tg.Value != repl {
				t.Fatalf("decoded %q, want %q", tg.Value, repl)
			}
		}
		// Segments between targets are byte-identical.
		prevOld, prevNew := 0, 0
		for i, tg := range doc.Targets {
			gap := body[prevOld:tg.Start]
			if !bytes.Equal(out[prevNew:prevNew+len(gap)], gap) {
				t.Fatalf("gap %d changed", i)
			}
			prevNew += len(gap) + len(Escape([]byte(`"`), repl)) + 1
			prevOld = tg.End
		}
		if !bytes.Equal(out[prevNew:], body[prevOld:]) {
			t.Fatal("tail changed")
		}
	})
}
