package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

// Invented test values only.
const (
	phMail = "<redacted:email_1>"
	phKey  = "<redacted:api_key_2>"
	plMail = "anna.berg@mailhost.de"
	plKey  = "pa\"ss\\wo\nrd\t</x>"
)

var testMapping = map[string]string{phMail: plMail, phKey: plKey}

func TestRehydrate(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"none", "plain text", "plain text"},
		{"known", "to " + phMail + ".", "to " + plMail + "."},
		{"unknown stays", "<redacted:email_9> and " + phMail, "<redacted:email_9> and " + plMail},
		{"twice", phMail + phMail, plMail + plMail},
		{"broken stays", "<redacted:email_1", "<redacted:email_1"},
		{"value with escapes raw", phKey, plKey},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Rehydrate(c.in, testMapping); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
	if got := Rehydrate(phMail, nil); got != phMail {
		t.Fatalf("nil mapping changed text: %q", got)
	}
}

func TestRehydrateJSON(t *testing.T) {
	frag := `{"command":"echo ` + phKey + ` ` + phMail + `","n":"<redacted:email_9>"}`
	out := RehydrateJSON(frag, testMapping)
	var v struct{ Command, N string }
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, out)
	}
	if want := "echo " + plKey + " " + plMail; v.Command != want {
		t.Fatalf("command = %q, want %q", v.Command, want)
	}
	if v.N != "<redacted:email_9>" {
		t.Fatalf("unknown placeholder changed: %q", v.N)
	}
}

func TestHoldbackSteps(t *testing.T) {
	long := "<redacted:" + strings.Repeat("a", 32) + "_123456" // 49 bytes, still viable
	cases := []struct {
		name   string
		chunks []string
		want   []string // output of each Write
		flush  string
	}{
		{"split placeholder", []string{"hi <reda", "cted:email_1> x"}, []string{"hi ", plMail + " x"}, ""},
		{"lone lt", []string{"a <", " b"}, []string{"a ", "< b"}, ""},
		{"not viable", []string{"a < b <x"}, []string{"a < b <x"}, ""},
		{"uppercase type", []string{"<redacted:Email"}, []string{"<redacted:Email"}, ""},
		{"zero number", []string{"<redacted:email_0> y"}, []string{"<redacted:email_0> y"}, ""},
		{"type may contain digit", []string{"<redacted:email_0"}, []string{""}, "<redacted:email_0"},
		{"type with underscore", []string{"x<redacted:api_key_", "2>"}, []string{"x", plKey}, ""},
		{"unknown completes", []string{"<redacted:email_1", "2>"}, []string{"", "<redacted:email_12>"}, ""},
		{"rest at end", []string{"end <redac"}, []string{"end "}, "<redac"},
		{"max 49 bytes", []string{"x" + long}, []string{"x"}, long},
		{"over 49 bytes", []string{"x" + long + "7"}, []string{"x" + long + "7"}, ""},
		{"type too long", []string{"<redacted:" + strings.Repeat("a", 33)}, []string{"<redacted:" + strings.Repeat("a", 33)}, ""},
		{"full then partial", []string{phMail + "<redacted:"}, []string{plMail}, "<redacted:"},
		{"utf8 kept", []string{"äö<re", "x ü"}, []string{"äö", "<rex ü"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewHoldback(testMapping, false)
			for i, ch := range c.chunks {
				if got := h.Write(ch); got != c.want[i] {
					t.Fatalf("Write %d (%q) = %q, want %q", i, ch, got, c.want[i])
				}
			}
			if got := h.Flush(); got != c.flush {
				t.Fatalf("Flush = %q, want %q", got, c.flush)
			}
			if got := h.Flush(); got != "" {
				t.Fatalf("second Flush = %q, want empty", got)
			}
		})
	}
}

// TestHoldbackAnySplit checks that every split into two or three chunks
// yields the same text as rehydrating the whole.
func TestHoldbackAnySplit(t *testing.T) {
	in := "x " + phMail + " <y " + phKey + "<redacted:email_9> <redacted:email_1"
	want := Rehydrate(in, testMapping)
	for i := 0; i <= len(in); i++ {
		for j := i; j <= len(in); j++ {
			h := NewHoldback(testMapping, false)
			got := h.Write(in[:i]) + h.Write(in[i:j]) + h.Write(in[j:]) + h.Flush()
			if got != want {
				t.Fatalf("split %d/%d: got %q, want %q", i, j, got, want)
			}
		}
	}
}

func TestHoldbackJSONMode(t *testing.T) {
	h := NewHoldback(testMapping, true)
	got := h.Write(`{"c":"<redacted:api`) + h.Write(`_key_2>"}`) + h.Flush()
	var v struct{ C string }
	if err := json.Unmarshal([]byte(got), &v); err != nil || v.C != plKey {
		t.Fatalf("got %q (%v), want C=%q", got, err, plKey)
	}
}
