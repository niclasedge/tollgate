package redact

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

// splitRunes cuts s into pieces at rune boundaries chosen by seed, so that
// every piece is valid UTF-8 like the deltas Anthropic sends.
func splitRunes(s string, seed uint64) []string {
	var out []string
	for len(s) > 0 {
		n := int(seed%7) + 1
		seed = seed*6364136223846793005 + 1442695040888963407
		i := 0
		for k := 0; k < n && i < len(s); k++ {
			_, w := utf8.DecodeRuneInString(s[i:])
			i += w
		}
		out = append(out, s[:i])
		s = s[i:]
	}
	return out
}

// FuzzRehydrateStream: redact arbitrary text, send the redacted text as
// text_delta and as a JSON string inside input_json_delta, cut at arbitrary
// rune boundaries; the rehydrated block equals the original text.
func FuzzRehydrateStream(f *testing.F) {
	for _, c := range loadCorpus(f) {
		f.Add(c.input, uint64(1))
	}
	f.Add("mail "+mailA+" and <redacted:email_1> and "+mailB+" "+mailA, uint64(3))
	f.Add("key "+antKey+" <redacted:api_key_", uint64(7))
	f.Add(connURL+"\n\"quoted\" \\ "+mailA+"<", uint64(11))
	r := New(corpusOptions())
	f.Fuzz(func(t *testing.T, a string, seed uint64) {
		body := []byte(msgs(`{"role":"user","content":` + js(a) + `}`))
		in, err := jsonsplice.Scan(body)
		if err != nil {
			t.Skip("not a valid body")
		}
		res, err := Number(r, in.Targets)
		if err != nil {
			t.Fatalf("Number: %v", err)
		}
		if len(in.Targets) != 1 {
			t.Fatalf("targets = %d", len(in.Targets))
		}
		a = in.Targets[0].Value // invalid UTF-8 was replaced by the encoder
		redacted := a
		if len(res.Replacements) == 1 {
			redacted = res.Replacements[0].New
		}

		var stream strings.Builder
		stream.WriteString(blockStart(0, "text") + blockStart(1, "tool_use"))
		frag := `{"command":"` + string(jsonsplice.Escape(nil, redacted)) + `"}`
		tp, jp := splitRunes(redacted, seed), splitRunes(frag, seed^0x9e3779b97f4a7c15)
		for i := 0; i < max(len(tp), len(jp)); i++ {
			if i < len(tp) {
				stream.WriteString(textDelta(0, tp[i]))
			}
			if i < len(jp) {
				stream.WriteString(jsonDelta(1, jp[i]))
			}
		}
		stream.WriteString(blockStop(0) + blockStop(1))
		out := rewrite(t, stream.String(), res.Placeholders)

		if got, n := blockText(t, out, 0); got != a {
			t.Fatalf("text block:\n got %q\nwant %q", got, a)
		} else if n < len(tp) || n > len(tp)+1 {
			t.Fatalf("text deltas %d for %d upstream deltas", n, len(tp))
		}
		got, _ := blockText(t, out, 1)
		var v struct{ Command string }
		if err := json.Unmarshal([]byte(got), &v); err != nil {
			t.Fatalf("partial_json invalid: %v\n%s", err, got)
		}
		if v.Command != a {
			t.Fatalf("tool_use input:\n got %q\nwant %q", v.Command, a)
		}
		if again := rewrite(t, stream.String(), res.Placeholders); again != out {
			t.Fatalf("not deterministic")
		}
	})
}
