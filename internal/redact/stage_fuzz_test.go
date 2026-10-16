package redact

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

// rehydrate replaces every placeholder that has an entry in mapping, in one
// pass. Unknown and reserved placeholders stay (ADR-0006).
func rehydrate(s string, mapping map[string]string) string {
	return placeholderRE.ReplaceAllStringFunc(s, func(ph string) string {
		if v, ok := mapping[ph]; ok {
			return v
		}
		return ph
	})
}

// FuzzStage is the acceptance test of #13: for arbitrary text in every kind
// of target, redact followed by rehydrate yields exactly the input, and a
// second run is byte-identical.
func FuzzStage(f *testing.F) {
	for _, c := range loadCorpus(f) {
		f.Add(c.input, "")
	}
	f.Add("mail "+mailA+" and "+mailB+" and "+mailA, "<redacted:email_1> "+mailA)
	f.Add("<redacted:email_"+mailA+">", antKey+"<redacted:api_key_2>")
	f.Add("password=<redacted:api_key_1>x", connURL)
	f.Add("\xff\xfepassword=\xffabcdefgh", "a\"b\\c\n "+mailB)
	r := New(corpusOptions())
	f.Fuzz(func(t *testing.T, a, b string) {
		body := []byte(msgs(
			`{"role":"user","content":`+js(a)+`}`,
			`{"role":"assistant","content":[{"type":"thinking","thinking":`+js(b)+`,"signature":"s"},{"type":"text","text":`+js(b)+`},{"type":"tool_use","id":"t","name":"Bash","input":{"command":`+js(a)+`,"list":[`+js(b)+`]}}]}`,
			`{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":`+js(b)+`},{"type":"text","text":`+js(a+b)+`}]}`,
		))
		in, err := jsonsplice.Scan(body)
		if err != nil {
			t.Skip("not a valid body")
		}
		res, err := Number(r, in.Targets)
		if err != nil {
			t.Fatalf("Number: %v", err)
		}
		out, err := jsonsplice.Apply(body, res.Replacements)
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		again, err := Number(r, in.Targets)
		if err != nil || !reflect.DeepEqual(res, again) {
			t.Fatalf("Number not deterministic")
		}
		if out2, _ := jsonsplice.Apply(body, again.Replacements); !bytes.Equal(out, out2) {
			t.Fatalf("output not byte-identical")
		}
		got, err := jsonsplice.Scan(out)
		if err != nil {
			t.Fatalf("output is not a valid body: %v", err)
		}
		if len(got.Targets) != len(in.Targets) {
			t.Fatalf("targets %d -> %d", len(in.Targets), len(got.Targets))
		}
		for i := range in.Targets {
			if back := rehydrate(got.Targets[i].Value, res.Placeholders); back != in.Targets[i].Value {
				t.Fatalf("target %d (%s): rehydrate\n got %q\nwant %q", i, in.Targets[i].Path, back, in.Targets[i].Value)
			}
		}
		// Thinking (and everything outside the targets) is byte-identical.
		thinking := []byte(`{"type":"thinking","thinking":` + js(b) + `,"signature":"s"}`)
		if !bytes.Contains(out, thinking) {
			t.Fatalf("thinking block changed")
		}
		for ph, v := range res.Placeholders {
			if !placeholderRE.MatchString(ph) || len(ph) > 50 || v == "" {
				t.Fatalf("bad mapping entry %q", ph)
			}
		}
	})
}
