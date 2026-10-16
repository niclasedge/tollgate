package redact

import (
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// Values are invented and assembled from parts so that the source itself
// holds no detector match.
var (
	maskMail      = "nora.beispiel" + "@" + "firma-beispiel.de"
	maskMailShort = "ab" + "@" + "cd.ef"
	maskAnthropic = "sk-ant-" + "api03-" + "a" + strings.Repeat("Q", 94)
	maskAntShort  = "sk-ant-" + "b" + strings.Repeat("x", 19)
	maskOpenAI    = "sk-" + "proj-" + "Z" + strings.Repeat("y", 40)
	maskAWS       = "AKIA" + "QX7Z" + "EXAMPLE12345"
	maskGitHub    = "ghp" + "_" + strings.Repeat("k", 36)
	maskJWT       = "eyJ" + "hbGciOiJIUzI1NiJ9" + ".eyJ" + "zdWIiOiIxIn0" + "." + "c2lnbmF0dXJl"
	maskPEM       = "-----BEGIN " + "PRIVATE KEY-----\nMIIEvQIBADANBg\n-----END " + "PRIVATE KEY-----"
)

func TestMask(t *testing.T) {
	cases := []struct {
		name, rule, typ, value, want string
		public                       string // public literal that is not a secret segment
		hidden                       bool   // no clear character at all
	}{
		{name: "email", rule: "email", typ: TypeEmail, value: maskMail, want: "n•••@f•••.de", public: ".de"},
		{name: "email short drops TLD", rule: "email", typ: TypeEmail, value: "abc" + "@" + "def.gh", want: "a•••@d•••.•••"},
		{name: "email eight runes", rule: "email", typ: TypeEmail, value: maskMailShort, want: "a•••@c•••.•••"},
		{name: "email tiny keeps local initial", rule: "email", typ: TypeEmail, value: "a" + "@" + "b.cd", want: "a•••@•••"},
		{name: "email subdomain", rule: "email", typ: TypeEmail, value: "x" + "@" + "mail.example.co.uk", want: "x•••@m•••.uk", public: ".uk"},
		{name: "anthropic key", rule: "anthropic_key", typ: TypeAPIKey, value: maskAnthropic, want: "sk-ant-api03-a••• (108)", public: "sk-ant-api03-"},
		{name: "anthropic short", rule: "anthropic_key", typ: TypeAPIKey, value: maskAntShort, want: "sk-ant-b••• (27)", public: "sk-ant-"},
		{name: "anthropic api03 short hides prefix", rule: "anthropic_key", typ: TypeAPIKey, value: "sk-ant-" + "api03-" + strings.Repeat("c", 20), want: "••• (33)", hidden: true},
		{name: "slack drops extra char", rule: "slack_token", typ: TypeAPIKey, value: "xoxb" + "-" + strings.Repeat("9", 12), want: "xoxb-••• (17)", public: "xoxb-"},
		{name: "jwt short hides prefix", rule: "jwt", typ: TypeJWT, value: "eyJ" + "a.eyJ" + "b.c", want: "••• (11)", hidden: true},
		{name: "openai key", rule: "openai_key", typ: TypeAPIKey, value: maskOpenAI, want: "sk-proj-Z••• (49)", public: "sk-proj-"},
		{name: "aws key", rule: "aws_access_key", typ: TypeAPIKey, value: maskAWS, want: "AKIAQ••• (20)", public: "AKIA"},
		{name: "github token", rule: "github_token", typ: TypeAPIKey, value: maskGitHub, want: "ghp_k••• (40)", public: "ghp_"},
		{name: "jwt", rule: "jwt", typ: TypeJWT, value: maskJWT, want: "eyJh••• (49)", public: "eyJ"},
		{name: "ipv4", rule: "ipv4", typ: TypeIP, value: "203.0.113.7", want: "2•.•.•.•"},
		{name: "ipv4 short", rule: "ipv4", typ: TypeIP, value: "1.2.3.4", want: "1•.•.•.•"},
		{name: "ipv6", rule: "ipv6", typ: TypeIP, value: "2001:db8::7", want: "2•:•::•"},
		{name: "connection password", rule: "conn_string", typ: TypeConnectionPassword, value: "Sup3rGeheim!", want: "••• (12)", hidden: true},
		{name: "private key", rule: "pem_private", typ: TypePrivateKey, value: maskPEM, want: "••• (" + itoa(utf8.RuneCountInString(maskPEM)) + ")", hidden: true},
		{name: "word list", rule: "words", typ: TypeWord, value: "Projekt Falke", want: "••• (13)", hidden: true},
		{name: "word list unicode", rule: "words", typ: TypeWord, value: "Müller", want: "••• (6)", hidden: true},
		{name: "generic password", rule: "generic_assign", typ: TypeAPIKey, value: "hunter2hunter2", want: "••• (14)", hidden: true},
		{name: "bearer token", rule: "bearer_token", typ: TypeAPIKey, value: strings.Repeat("t", 24), want: "••• (24)", hidden: true},
		{name: "custom regex", rule: "kundennummer", typ: "customer_id", value: "KD-123456", want: "K••• (9)"},
		{name: "custom regex too short", rule: "kurz", typ: "short_id", value: "ab", want: "••• (2)", hidden: true},
		{name: "custom regex non-letter start", rule: "hash", typ: "tag", value: "#ab12cd34", want: "••• (9)", hidden: true},
		{name: "unknown builtin type", rule: "future_rule", typ: TypeAPIKey, value: "abcdefghijkl", want: "••• (12)", hidden: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Mask(c.rule, c.typ, c.value)
			if got != c.want {
				t.Fatalf("Mask = %q, want %q", got, c.want)
			}
			if again := Mask(c.rule, c.typ, c.value); again != got {
				t.Fatalf("not deterministic: %q then %q", got, again)
			}
			checkMaskInvariants(t, c.value, got, c.public, c.hidden)
		})
	}
}

// lengthSuffixRE is the length suffix " (N)" of a mask; it is no clear text.
var lengthSuffixRE = regexp.MustCompile(` \(\d+\)$`)

// checkMaskInvariants enforces the rules of issue #112 on one mask:
// checkMaskBasics plus at most one clear character per secret segment.
// public is the rule's public literal (prefix, TLD), which is no segment.
func checkMaskInvariants(t *testing.T, value, mask, public string, hidden bool) {
	t.Helper()
	checkMaskBasics(t, value, mask, hidden)
	body := lengthSuffixRE.ReplaceAllString(mask, "")
	secret := strings.Replace(body, public, "•", 1)
	for _, seg := range strings.FieldsFunc(secret, func(r rune) bool { return !isClear(r) }) {
		if utf8.RuneCountInString(seg) > 1 {
			t.Fatalf("mask %q shows segment %q with more than one clear character", mask, seg)
		}
	}
}

// checkMaskBasics: a bullet, valid UTF-8, at most 64 bytes, never the value,
// clear characters (public literals included) at most a quarter of the
// value's runes, none at all for hidden types.
func checkMaskBasics(t *testing.T, value, mask string, hidden bool) {
	t.Helper()
	if !strings.Contains(mask, "•") || len(mask) > 64 || !utf8.ValidString(mask) {
		t.Fatalf("mask %q: needs a bullet, valid UTF-8 and at most 64 bytes", mask)
	}
	if strings.Contains(mask, value) {
		t.Fatalf("mask %q contains the value", mask)
	}
	clear := countClear(lengthSuffixRE.ReplaceAllString(mask, ""))
	if hidden && clear > 0 {
		t.Fatalf("mask %q shows %d clear characters, want none", mask, clear)
	}
	if n := utf8.RuneCountInString(value); clear*4 > n {
		t.Fatalf("mask %q shows %d clear characters of %d", mask, clear, n)
	}
}

func isClear(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func countClear(s string) int {
	n := 0
	for _, r := range s {
		if isClear(r) {
			n++
		}
	}
	return n
}

// TestMaskInvariantsOnFindings runs Mask over real findings of all built-in
// rules and checks the rules that hold without knowing the public literal.
func TestMaskInvariantsOnFindings(t *testing.T) {
	r := New(Options{Email: true, APIKeys: true, JWT: true, ConnectionURLs: true, IPAddresses: true, PrivateKeys: true})
	auth := "Author" + "ization"
	text := strings.Join([]string{
		"mail " + maskMail, "key " + maskAnthropic, "aws " + maskAWS, "jwt " + maskJWT,
		"db postgres://app:" + "Sup3rGeheim!" + "@db.invalid/app", maskPEM, "ip 203.0.113.7 und 2001:db8::7",
		"pass" + "word=" + "hunter2hunter2", auth + ": " + strings.Repeat("t", 24),
	}, "\n")
	fs := r.Find(text)
	if len(fs) < 9 {
		t.Fatalf("only %d findings", len(fs))
	}
	for _, f := range fs {
		v := text[f.Start:f.End]
		m := Mask(f.Rule, f.Type, v)
		if m != Mask(f.Rule, f.Type, v) {
			t.Fatalf("rule %s: not deterministic", f.Rule)
		}
		hidden := f.Type == TypeConnectionPassword || f.Type == TypePrivateKey || f.Priority == prioGeneric
		checkMaskBasics(t, v, m, hidden)
	}
}
