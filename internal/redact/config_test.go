package redact

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func mustCompile(t testing.TB, c Config) *Redactor {
	t.Helper()
	r, err := Compile(c)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return r
}

func customerRule() CustomRule {
	return CustomRule{Name: "kundennummer", Pattern: `KD-\d{6}`, Placeholder: "customer_id", Examples: []string{"KD-123456"}}
}

func TestCompileCustomRule(t *testing.T) {
	r := mustCompile(t, Config{Builtin: DefaultOptions(), Custom: []CustomRule{customerRule()}})
	got := r.Redact("Ticket KD-123456 offen")
	if got != "Ticket <redacted:customer_id> offen" {
		t.Fatalf("got %q", got)
	}
	fs := r.Find("Ticket KD-123456 offen")
	if len(fs) != 1 || fs[0].Rule != "custom:kundennummer" || fs[0].Priority != prioCustom {
		t.Fatalf("findings %+v", fs)
	}
}

func TestCompileCustomBeatsBuiltin(t *testing.T) {
	// A custom rule that covers a whole e-mail address wins over the
	// built-in email rule by priority.
	c := Config{Builtin: DefaultOptions(), Custom: []CustomRule{{
		Name: "staff", Pattern: `[a-z]+@muster-gmbh\.invalid`, Placeholder: "staff_mail", Examples: []string{"jane@muster-gmbh.invalid"},
	}}}
	got := mustCompile(t, c).Redact("an jane@muster-gmbh.invalid")
	if got != "an <redacted:staff_mail>" {
		t.Fatalf("got %q", got)
	}
}

func TestCompileErrors(t *testing.T) {
	builtinTypes := []string{TypeEmail, TypeAPIKey, TypeJWT, TypeConnectionPassword, TypeIP, TypePrivateKey, TypeWord}
	tests := []struct {
		name string
		cfg  Config
		path string
	}{
		{"example misses", Config{Custom: []CustomRule{{Name: "k", Pattern: `KD-\d{6}`, Placeholder: "customer_id", Examples: []string{"KD-12"}}}}, "custom[0].examples[0]"},
		{"no example", Config{Custom: []CustomRule{{Name: "k", Pattern: `KD-\d{6}`, Placeholder: "customer_id"}}}, "custom[0].examples"},
		{"bad regex", Config{Custom: []CustomRule{{Name: "k", Pattern: `KD-(`, Placeholder: "customer_id", Examples: []string{"KD-"}}}}, "custom[0].pattern"},
		{"empty match", Config{Custom: []CustomRule{{Name: "k", Pattern: `x*`, Placeholder: "customer_id", Examples: []string{"xx"}}}}, "custom[0].pattern"},
		{"bad placeholder", Config{Custom: []CustomRule{{Name: "k", Pattern: `KD-\d{6}`, Placeholder: "Kunde-ID", Examples: []string{"KD-123456"}}}}, "custom[0].placeholder"},
		{"long placeholder", Config{Custom: []CustomRule{{Name: "k", Pattern: `KD-\d{6}`, Placeholder: "a" + strings.Repeat("b", 32), Examples: []string{"KD-123456"}}}}, "custom[0].placeholder"},
		{"bad name", Config{Custom: []CustomRule{{Name: "Kunde", Pattern: `KD-\d{6}`, Placeholder: "customer_id", Examples: []string{"KD-123456"}}}}, "custom[0].name"},
		{"duplicate name", Config{Custom: []CustomRule{customerRule(), customerRule()}}, "custom[1].name"},
		{"blank word", Config{Words: WordList{List: []string{"Projekt Falke", "  "}}}, "words.list[1]"},
	}
	for _, typ := range builtinTypes {
		tests = append(tests, struct {
			name string
			cfg  Config
			path string
		}{"placeholder collides " + typ, Config{Custom: []CustomRule{{Name: "k", Pattern: `KD-\d{6}`, Placeholder: typ, Examples: []string{"KD-123456"}}}}, "custom[0].placeholder"})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(tt.cfg)
			var ce ConfigErrors
			if !errors.As(err, &ce) {
				t.Fatalf("err = %v, want ConfigErrors", err)
			}
			if ce[0].Path != tt.path {
				t.Fatalf("path = %q, want %q (%v)", ce[0].Path, tt.path, err)
			}
		})
	}
}

func TestCompileErrorsNeverQuoteWords(t *testing.T) {
	_, err := Compile(Config{
		Words:  WordList{List: []string{"Projekt Falke", ""}},
		Custom: []CustomRule{{Name: "k", Pattern: `Muster GmbH`, Placeholder: "firm", Examples: []string{"Muster AG"}}},
	})
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "Projekt") || strings.Contains(err.Error(), "Falke") {
		t.Fatalf("error quotes a word: %v", err)
	}
}

func TestWordListWholeWordUnicode(t *testing.T) {
	text := "Müllers Akte, Herr Müller. 2Müller Müller3 Müller_x"
	whole := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"Müller"}}})
	if got := whole.Redact(text); got != "Müllers Akte, Herr <redacted:word>. 2Müller Müller3 <redacted:word>_x" {
		t.Errorf("whole_word: %q", got)
	}
	part := mustCompile(t, Config{Words: WordList{WholeWord: false, List: []string{"Müller"}}})
	if got := part.Redact(text); strings.Contains(got, "Müller") {
		t.Errorf("substring: %q", got)
	}
}

func TestWordListCombiningMarkIsWordChar(t *testing.T) {
	// "Mu" followed by a combining diaeresis is a different word.
	r := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"Mu"}}})
	text := "Mu\u0308ller und Mu."
	if got := r.Redact(text); got != "Mu\u0308ller und <redacted:word>." {
		t.Errorf("got %q", got)
	}
}

func TestWordListCaseSensitivity(t *testing.T) {
	text := "Vertrag mit MUSTER GMBH und Muster GmbH, mÜller"
	insensitive := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"Muster GmbH", "Müller"}}})
	if got := insensitive.Redact(text); got != "Vertrag mit <redacted:word> und <redacted:word>, <redacted:word>" {
		t.Errorf("case-insensitive: %q", got)
	}
	sensitive := mustCompile(t, Config{Words: WordList{CaseSensitive: true, WholeWord: true, List: []string{"Muster GmbH", "Müller"}}})
	if got := sensitive.Redact(text); got != "Vertrag mit MUSTER GMBH und <redacted:word>, mÜller" {
		t.Errorf("case-sensitive: %q", got)
	}
}

func TestWordListMultiWordWhitespace(t *testing.T) {
	r := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"Projekt Falke"}}})
	for _, text := range []string{"Projekt\nFalke", "Projekt \t  Falke", "Projekt Falke"} {
		if got := r.Redact("Status " + text + " gut"); got != "Status <redacted:word> gut" {
			t.Errorf("%q: got %q", text, got)
		}
	}
	if got := r.Redact("ProjektFalke"); got != "ProjektFalke" {
		t.Errorf("no whitespace must not match: %q", got)
	}
}

func TestWordListLongestWins(t *testing.T) {
	// The longer entry fails the boundary check; the shorter one at the same
	// start must still be found.
	r := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"Muster", "Muster GmbH"}}})
	if got := r.Redact("Muster GmbHx"); got != "<redacted:word> GmbHx" {
		t.Errorf("got %q", got)
	}
	if got := r.Redact("Muster GmbH."); got != "<redacted:word>." {
		t.Errorf("got %q", got)
	}
	// Restart one rune after a rejected candidate.
	r2 := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"a b"}}})
	if got := r2.Redact("ca b a b"); got != "ca b <redacted:word>" {
		t.Errorf("got %q", got)
	}
}

func TestWordListPunctuationEdges(t *testing.T) {
	// Boundaries are only checked at edges that are letters or digits.
	r := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"C++"}}})
	if got := r.Redact("Wir nutzen C++x und C++."); got != "Wir nutzen <redacted:word>x und <redacted:word>." {
		t.Errorf("got %q", got)
	}
}

func TestWordListOrderIndependent(t *testing.T) {
	a := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"Falke", "Projekt Falke", "Muster GmbH", "Falke"}}})
	b := mustCompile(t, Config{Words: WordList{WholeWord: true, List: []string{"Muster GmbH", "Projekt Falke", "Falke"}}})
	text := "Projekt Falke bei Muster GmbH, Falke"
	if fa, fb := a.Find(text), b.Find(text); !reflect.DeepEqual(fa, fb) {
		t.Fatalf("order dependent:\n%+v\n%+v", fa, fb)
	}
	if got := a.Redact(text); got != "<redacted:word> bei <redacted:word>, <redacted:word>" {
		t.Errorf("got %q", got)
	}
}

func TestWordsBeatCustomAndBuiltin(t *testing.T) {
	c := Config{
		Builtin: DefaultOptions(),
		Custom:  []CustomRule{{Name: "firm", Pattern: `Muster \w+`, Placeholder: "firm", Examples: []string{"Muster AG"}}},
		Words:   WordList{WholeWord: true, List: []string{"Muster GmbH"}},
	}
	fs := mustCompile(t, c).Find("Muster GmbH")
	if len(fs) != 1 || fs[0].Type != TypeWord || fs[0].Rule != "words" {
		t.Fatalf("findings %+v", fs)
	}
}

func TestAllowlistWins(t *testing.T) {
	c := Config{
		Builtin:   DefaultOptions(),
		Custom:    []CustomRule{customerRule()},
		Words:     WordList{WholeWord: true, List: []string{"Projekt Falke"}},
		Allowlist: []string{"team@muster-gmbh.invalid", "KD-000000", "Projekt Falke"},
	}
	r := mustCompile(t, c)
	text := "team@muster-gmbh.invalid jane@muster-gmbh.invalid KD-000000 KD-123456 Projekt Falke"
	want := "team@muster-gmbh.invalid <redacted:email> KD-000000 <redacted:customer_id> Projekt Falke"
	if got := r.Redact(text); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	// The built-in e-mail exceptions stay active.
	if got := r.Redact("noreply@github.com"); got != "noreply@github.com" {
		t.Errorf("builtin exception: %q", got)
	}
	// Issue #83: an allowlisted rest of a displaced generic finding stays.
	r = mustCompile(t, Config{Builtin: DefaultOptions(), Allowlist: []string{"weiterer-text"}})
	in := "Authorization: Bearer eyJa.eyJb.c weiterer-text"
	if got, want := r.Redact(in), "Authorization: Bearer <redacted:jwt> weiterer-text"; got != want {
		t.Errorf("allowlisted residual: got %q, want %q", got, want)
	}
}

func TestCompileMatchesNewForBuiltinOnly(t *testing.T) {
	opts := corpusOptions()
	a, b := New(opts), mustCompile(t, Config{Builtin: opts})
	for _, tc := range loadCorpus(t) {
		if fa, fb := a.Find(tc.input), b.Find(tc.input); !reflect.DeepEqual(fa, fb) {
			t.Errorf("%s: New and Compile differ", tc.name)
		}
	}
}

func TestCompileDeterministic(t *testing.T) {
	c := Config{
		Builtin: DefaultOptions(), Custom: []CustomRule{customerRule()},
		Words: WordList{WholeWord: true, List: []string{"Projekt Falke", "Muster GmbH"}}, Allowlist: []string{"KD-000000"},
	}
	text := "KD-123456 Projekt Falke Muster GmbH KD-000000 jane@muster-gmbh.invalid"
	first := mustCompile(t, c).Redact(text)
	for range 5 {
		if got := mustCompile(t, c).Redact(text); got != first {
			t.Fatalf("not deterministic: %q vs %q", got, first)
		}
	}
}
