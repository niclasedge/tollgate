package redact

import (
	"reflect"
	"slices"
	"testing"
	"unicode"
)

// withoutPrefilter returns a copy of r that runs every regex (#84).
func withoutPrefilter(r *Redactor) *Redactor {
	c := *r
	c.noPrefilter = true
	return &c
}

// TestPrefilterKeepsCorpusFindings: the prefilter must never lose a
// finding. Over the whole corpus, Find with and without prefilter returns
// identical findings, for the default and the corpus switches.
func TestPrefilterKeepsCorpusFindings(t *testing.T) {
	for _, opts := range []Options{DefaultOptions(), corpusOptions()} {
		r := New(opts)
		raw := withoutPrefilter(r)
		for _, c := range loadCorpus(t) {
			if got, want := r.Find(c.input), raw.Find(c.input); !reflect.DeepEqual(got, want) {
				t.Errorf("%s (%+v): prefilter changes findings\n with: %+v\n  raw: %+v", c.name, opts, got, want)
			}
		}
	}
}

// TestPrefilterSkipsRules: text without any keyword runs no built-in regex;
// a keyword enables exactly the rules that need it.
func TestPrefilterSkipsRules(t *testing.T) {
	r := New(corpusOptions())
	running := func(text string) []string {
		p := r.newPrefilter(text)
		var out []string
		for _, rl := range r.rules {
			if p.pass(rl.rule) {
				out = append(out, rl.name)
			}
		}
		return out
	}
	if got := running("hello world without anything"); len(got) != 0 {
		t.Errorf("plain text runs %q", got)
	}
	if got := running("PASSWORD is here"); !slices.Equal(got, []string{"generic_assign"}) {
		t.Errorf("PASSWORD runs %q, want [generic_assign]", got)
	}
	if got := running("mail me @ home"); !slices.Equal(got, []string{"email"}) {
		t.Errorf("@ runs %q, want [email]", got)
	}
	for _, rl := range builtinRules {
		if len(rl.need) == 0 {
			t.Errorf("%s: built-in rule without prefilter keywords", rl.name)
		}
		for _, k := range rl.need {
			if rl.fold && k != asciiLower(k) {
				t.Errorf("%s: case-insensitive keyword %q must be lower-case", rl.name, k)
			}
		}
	}
}

// TestPrefilterCaseFolding: Go's (?i) folds U+017F (long s) to s and
// U+212A (Kelvin sign) to k. The prefilter must not drop such matches.
func TestPrefilterCaseFolding(t *testing.T) {
	r := New(DefaultOptions())
	raw := withoutPrefilter(r)
	for _, in := range []string{
		"paſſword=abcdefgh12",           // generic_assign via long s
		"api_Key: abcdefgh12",           // generic_assign via Kelvin sign
		"Clientſecret=abcdef1234567890", // client_secret
		"X-API-KEY: abc123def456",       // auth_header
	} {
		want := raw.Find(in)
		if len(want) == 0 {
			t.Fatalf("premise: no finding without prefilter for %q", in)
		}
		if got := r.Find(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: with prefilter %+v, without %+v", in, got, want)
		}
	}
}

// TestFoldOrbitsToASCII pins the premise of the case-insensitive prefilter:
// apart from ASCII letters, only U+017F and U+212A simple-fold to an ASCII
// letter.
func TestFoldOrbitsToASCII(t *testing.T) {
	var got []rune
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < 0x80 && unicode.IsLetter(f) {
				got = append(got, r)
				break
			}
		}
	}
	if want := []rune{0x017F, 0x212A}; !slices.Equal(got, want) {
		t.Errorf("non-ASCII runes folding to ASCII = %U, want %U", got, want)
	}
}

// FuzzPrefilter: for arbitrary input, Find with and without prefilter
// returns identical findings.
func FuzzPrefilter(f *testing.F) {
	for _, c := range loadCorpus(f) {
		f.Add(c.input)
	}
	f.Add("paſſword=abcdefgh12")
	f.Add("api_Key: abcdefgh12")
	r := New(corpusOptions())
	raw := withoutPrefilter(r)
	f.Fuzz(func(t *testing.T, in string) {
		if got, want := r.Find(in), raw.Find(in); !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: with prefilter %+v, without %+v", in, got, want)
		}
	})
}
