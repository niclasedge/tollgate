package redact

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

// Invented names only.
func nameConfig() Config {
	return Config{Words: WordList{
		WholeWord:  true,
		Customers:  []string{"Muster GmbH", "Anna Müller GmbH"},
		FirstNames: []string{"Anna", "Bernd"},
		LastNames:  []string{"Müller", "Schmidt"},
	}}
}

func TestNameListsTypes(t *testing.T) {
	r := mustCompile(t, nameConfig())
	got := r.Redact("Anna Müller von Muster GmbH")
	if want := "<redacted:vorname> <redacted:nachname> von <redacted:kunde>"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	fs := r.Find("Bernd Schmidt")
	if len(fs) != 2 || fs[0].Rule != "first_names" || fs[1].Rule != "last_names" || fs[0].Priority != prioWords {
		t.Fatalf("findings %+v", fs)
	}
}

func TestNameListsLongestWins(t *testing.T) {
	r := mustCompile(t, nameConfig())
	fs := r.Find("Anna Müller GmbH")
	if len(fs) != 1 || fs[0].Type != TypeCustomer {
		t.Fatalf("findings %+v", fs)
	}
}

func TestNameListsSemanticsLikeWordList(t *testing.T) {
	// whole_word and case_sensitive apply to the three lists, too.
	text := "Müllers Akte, Herr MÜLLER. Müller_x"
	whole := mustCompile(t, Config{Words: WordList{WholeWord: true, LastNames: []string{"Müller"}}})
	if got := whole.Redact(text); got != "Müllers Akte, Herr <redacted:nachname>. <redacted:nachname>_x" {
		t.Errorf("whole_word: %q", got)
	}
	sens := mustCompile(t, Config{Words: WordList{WholeWord: true, CaseSensitive: true, LastNames: []string{"Müller"}}})
	if got := sens.Redact(text); got != "Müllers Akte, Herr MÜLLER. <redacted:nachname>_x" {
		t.Errorf("case_sensitive: %q", got)
	}
	part := mustCompile(t, Config{Words: WordList{FirstNames: []string{"Anna"}}})
	if got := part.Redact("Hannah Annabelle"); got != "H<redacted:vorname>h <redacted:vorname>belle" {
		t.Errorf("substring: %q", got)
	}
}

func TestNameListsAllowlistAndEmpty(t *testing.T) {
	c := nameConfig()
	c.Allowlist = []string{"Schmidt"}
	if got := mustCompile(t, c).Redact("Bernd Schmidt"); got != "<redacted:vorname> Schmidt" {
		t.Errorf("allowlist: %q", got)
	}
	bad := Config{Words: WordList{Customers: []string{"ok", " "}, FirstNames: []string{""}}}
	_, err := Compile(bad)
	var ce ConfigErrors
	if !errors.As(err, &ce) || len(ce) != 2 || ce[0].Path != "words.customers[1]" || ce[1].Path != "words.first_names[0]" {
		t.Fatalf("errors %v", err)
	}
	if strings.Contains(err.Error(), "ok") {
		t.Errorf("error quotes an entry: %v", err)
	}
}

func TestNameListsOrderIndependent(t *testing.T) {
	a := mustCompile(t, nameConfig())
	c := nameConfig()
	c.Words.Customers = []string{"Anna Müller GmbH", "Muster GmbH"}
	c.Words.FirstNames = []string{"Bernd", "Anna", "Bernd"}
	c.Words.LastNames = []string{"Schmidt", "Müller"}
	b := mustCompile(t, c)
	text := "Anna Müller GmbH: Bernd Schmidt, Anna Müller, Muster GmbH"
	if fa, fb := a.Find(text), b.Find(text); !reflect.DeepEqual(fa, fb) {
		t.Fatalf("order dependent:\n%+v\n%+v", fa, fb)
	}
}

func TestNameListsSameEntryInTwoLists(t *testing.T) {
	// The fixed rule order (words, customers, first names, last names)
	// decides: the same text always gets the same type.
	c := Config{Words: WordList{WholeWord: true, Customers: []string{"Falke"}, LastNames: []string{"Falke"}}}
	for range 5 {
		if got := mustCompile(t, c).Redact("Falke"); got != "<redacted:kunde>" {
			t.Fatalf("got %q", got)
		}
	}
}

func TestNameListsDoNotReserveCustomTypes(t *testing.T) {
	// Existing configurations with a custom rule of this type stay valid.
	for _, typ := range []string{TypeCustomer, TypeFirstName, TypeLastName} {
		c := Config{Custom: []CustomRule{{Name: "k", Pattern: `KD-\d{6}`, Placeholder: typ, Examples: []string{"KD-123456"}}}}
		if _, err := Compile(c); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
}

func TestIsListType(t *testing.T) {
	for _, typ := range []string{TypeWord, TypeCustomer, TypeFirstName, TypeLastName} {
		if !IsListType(typ) {
			t.Errorf("%s is a list type", typ)
		}
	}
	for _, typ := range []string{TypeEmail, TypeAPIKey, "customer_id", ""} {
		if IsListType(typ) {
			t.Errorf("%q is no list type", typ)
		}
	}
}

func TestNameListsMaskShowsLengthOnly(t *testing.T) {
	for _, tc := range []struct{ rule, typ, value, want string }{
		{"customers", TypeCustomer, "Muster GmbH", "••• (11)"},
		{"first_names", TypeFirstName, "Anna", "••• (4)"},
		{"last_names", TypeLastName, "Müller", "••• (6)"},
	} {
		if got := Mask(tc.rule, tc.typ, tc.value); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}

// bodyWith returns a request body with the text in a user message.
func bodyWith(text string) []byte {
	return []byte(msgs(`{"role":"user","content":` + js(text) + `}`))
}

func TestNameListsNumberingAndRoundTrip(t *testing.T) {
	r := mustCompile(t, nameConfig())
	body := bodyWith("Anna Müller (Muster GmbH) und Bernd Schmidt, wieder Anna und Muster GmbH")
	in, err := jsonsplice.Scan(body)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Number(r, in.Targets)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jsonsplice.Apply(body, res.Replacements)
	if err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{"Anna", "Müller", "Muster GmbH", "Bernd", "Schmidt"} {
		if bytes.Contains(out, []byte(plain)) {
			t.Fatalf("%s left in the body: %s", plain, out)
		}
	}
	for ph, v := range map[string]string{
		"<redacted:vorname_1>":  "Anna",
		"<redacted:vorname_2>":  "Bernd",
		"<redacted:nachname_1>": "Müller",
		"<redacted:nachname_2>": "Schmidt",
		"<redacted:kunde_1>":    "Muster GmbH",
	} {
		if res.Placeholders[ph] != v {
			t.Errorf("%s = %q, want %q", ph, res.Placeholders[ph], v)
		}
	}
	// Deterministic: a second run is byte-identical.
	again, _ := Number(r, in.Targets)
	if !reflect.DeepEqual(res, again) {
		t.Fatal("Number not deterministic")
	}
	got, err := jsonsplice.Scan(out)
	if err != nil {
		t.Fatal(err)
	}
	if back := rehydrate(got.Targets[0].Value, res.Placeholders); back != in.Targets[0].Value {
		t.Fatalf("round trip:\n got %q\nwant %q", back, in.Targets[0].Value)
	}
}

func TestNameListsRehydrateAcrossChunks(t *testing.T) {
	mapping := map[string]string{
		"<redacted:kunde_1>":    "Muster GmbH",
		"<redacted:vorname_1>":  "Anna",
		"<redacted:nachname_1>": "Müller",
	}
	text := "Hallo <redacted:vorname_1> <redacted:nachname_1> von <redacted:kunde_1>!"
	want := "Hallo Anna Müller von Muster GmbH!"
	for cut := 1; cut < len(text); cut++ {
		h := NewHoldback(mapping, false)
		got := h.Write(text[:cut]) + h.Write(text[cut:]) + h.Flush()
		if got != want {
			t.Fatalf("split at %d: got %q", cut, got)
		}
	}
	// input_json_delta: the plaintext is inserted JSON-escaped.
	h := NewHoldback(map[string]string{"<redacted:kunde_1>": `Muster "Ost" GmbH`}, true)
	got := h.Write("{\"c\":\"<redacted:ku") + h.Write("nde_1>\"}") + h.Flush()
	if got != `{"c":"Muster \"Ost\" GmbH"}` {
		t.Fatalf("json: %q", got)
	}
}

func FuzzNameListsRoundTrip(f *testing.F) {
	f.Add("Anna Müller von Muster GmbH", "<redacted:kunde_1> Bernd")
	f.Add("Müllers Anna_x \xffSchmidt", "")
	f.Add("Anna Müller GmbH Anna", "<redacted:vorname_1><redacted:nachname_3>")
	r := mustCompile(f, nameConfig())
	f.Fuzz(func(t *testing.T, a, b string) {
		body := []byte(msgs(
			`{"role":"user","content":`+js(a)+`}`,
			`{"role":"assistant","content":[{"type":"text","text":`+js(b)+`}]}`,
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
		again, _ := Number(r, in.Targets)
		if !reflect.DeepEqual(res, again) {
			t.Fatal("not deterministic")
		}
		got, err := jsonsplice.Scan(out)
		if err != nil || len(got.Targets) != len(in.Targets) {
			t.Fatalf("output is not a valid body: %v", err)
		}
		for i := range in.Targets {
			if back := rehydrate(got.Targets[i].Value, res.Placeholders); back != in.Targets[i].Value {
				t.Fatalf("target %d: rehydrate\n got %q\nwant %q", i, back, in.Targets[i].Value)
			}
		}
	})
}
