package redact

import (
	"reflect"
	"strings"
	"testing"
)

// FuzzRedact checks, for arbitrary input: no panic, deterministic findings
// and output, sorted non-overlapping findings inside the input, and no
// plaintext of an accepted secret left in the output.
func FuzzRedact(f *testing.F) {
	for _, c := range loadCorpus(f) {
		f.Add(c.input)
	}
	f.Add("password=<redacted:api_key_1>x")
	f.Add("\xff\xfepassword=\xffabcdefgh")
	r := New(corpusOptions())
	f.Fuzz(func(t *testing.T, in string) {
		fs := r.Find(in)
		if again := r.Find(in); !reflect.DeepEqual(fs, again) {
			t.Fatalf("findings not deterministic for %q", in)
		}
		out := r.Redact(in)
		if again := r.Redact(in); out != again {
			t.Fatalf("output not deterministic for %q", in)
		}
		last := 0
		for i, x := range fs {
			if x.Start < last || x.Start >= x.End || x.End > len(in) {
				t.Fatalf("finding %d invalid or overlapping: %+v", i, fs)
			}
			last = x.End
		}
		// The unredacted remainder of the input: every secret becomes a line
		// break, so placeholder text cannot hide a leak. A secret that occurs
		// only once in the input must be gone; repeated values may stay where
		// no detector fired (same as the Rust original).
		rest := Replace(in, fs, func(Finding) string { return "\n" })
		for _, x := range fs {
			secret := in[x.Start:x.End]
			if strings.Count(in, secret) == 1 && strings.Contains(rest, secret) {
				t.Fatalf("secret %q (%s) survives in %q", secret, x.Rule, out)
			}
		}
	})
}
