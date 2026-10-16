package redact

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Masks (issue #112). A mask lets a human recognise which value a
// placeholder replaced without revealing it: "•" hides characters, at most
// one clear character per secret segment, all clear characters together
// (public literals such as a key prefix or the TLD included) at most a
// quarter of the value's runes, at most maxMaskBytes. Every rule tries its
// candidates from most to least telling and takes the first that keeps the
// limits; the last candidate, "••• (N)", shows nothing but the length.

const (
	hidden       = "•••"
	maxMaskBytes = 64
)

// keyPrefixes are the public prefixes of the vendor key rules, longest
// first per rule. They come from the rule regexes, so they say no more than
// the rule name does.
var keyPrefixes = map[string][]string{
	"anthropic_key":  {"sk-ant-api03-", "sk-ant-"},
	"openai_key":     {"sk-proj-", "sk-"},
	"aws_access_key": {"AKIA"},
	"github_token":   {"ghp_", "gho_", "ghu_", "ghs_", "ghr_"},
	"slack_token":    {"xoxb-", "xoxa-", "xoxp-", "xoxr-", "xoxs-"},
	"stripe_live":    {"sk_live_", "pk_live_", "rk_live_"},
	"jwt":            {"eyJ"},
}

// Mask returns the display mask of value, found by rule with placeholder
// type typ. It is pure and deterministic. Word list entries (all lists), passwords,
// private keys and every other built-in type without its own rule below show
// the length only; custom rules show the first character and the length.
func Mask(rule, typ, value string) string {
	n := utf8.RuneCountInString(value)
	var cands []string
	switch {
	case rule == "email":
		cands = emailMasks(value)
	case rule == "ipv4":
		cands = []string{firstClear(value) + "•.•.•.•", "•.•.•.•"}
	case rule == "ipv6":
		cands = ipv6Masks(value)
	case keyPrefixes[rule] != nil:
		cands = prefixMasks(keyPrefixes[rule], value, n)
	case !isBuiltinType(typ) && !IsListType(typ):
		if c := firstClear(value); c != "" {
			cands = []string{c + lengthMask(n)}
		}
	}
	for _, m := range cands {
		if len(m) <= maxMaskBytes && clearCount(m)*4 <= n {
			return m
		}
	}
	return lengthMask(n)
}

// lengthMask is the mask without any clear character: "••• (N)".
func lengthMask(n int) string { return hidden + " (" + strconv.Itoa(n) + ")" }

// emailMasks: first character of the local part and of the first domain
// label, plus the TLD; then without TLD, then without the domain initial.
func emailMasks(v string) []string {
	at := strings.LastIndexByte(v, '@')
	if at < 0 {
		return nil
	}
	local, domain := v[:at], v[at+1:]
	l := firstClear(local) + hidden
	labels := strings.Split(domain, ".")
	d := firstClear(labels[0]) + hidden
	var out []string
	if len(labels) > 1 {
		tld := labels[len(labels)-1]
		if strings.IndexFunc(tld, func(r rune) bool { return !isClearRune(r) }) < 0 {
			out = append(out, l+"@"+d+"."+tld)
		}
		out = append(out, l+"@"+d+"."+hidden)
	}
	return append(out, l+"@"+d, l+"@"+hidden, hidden+"@"+hidden)
}

// ipv6Masks keeps the group structure ("::" stays), shows the first
// character of the first group and hides every other group.
func ipv6Masks(v string) []string {
	groups := strings.Split(v, ":")
	plain := make([]string, len(groups))
	for i, g := range groups {
		if g != "" {
			plain[i] = "•"
		}
	}
	shown := append([]string(nil), plain...)
	if c := firstClear(groups[0]); c != "" {
		shown[0] = c + "•"
	}
	return []string{strings.Join(shown, ":"), strings.Join(plain, ":")}
}

// prefixMasks: public prefix plus the next character and the length, then
// the prefix and the length.
func prefixMasks(prefixes []string, v string, n int) []string {
	for _, p := range prefixes {
		if !strings.HasPrefix(v, p) {
			continue
		}
		var out []string
		if c := firstClear(v[len(p):]); c != "" {
			out = append(out, p+c+lengthMask(n))
		}
		return append(out, p+lengthMask(n))
	}
	return nil
}

// firstClear returns the first rune of s if it is a letter or digit, else "".
func firstClear(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || !isClearRune(r) {
		return ""
	}
	return s[:size]
}

func isClearRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// clearCount counts letters and digits of m outside the length suffix.
func clearCount(m string) int {
	if i := strings.LastIndex(m, " ("); i >= 0 && strings.HasSuffix(m, ")") {
		m = m[:i]
	}
	c := 0
	for _, r := range m {
		if isClearRune(r) {
			c++
		}
	}
	return c
}

func isBuiltinType(typ string) bool {
	for _, t := range reservedTypes {
		if t == typ {
			return true
		}
	}
	return false
}
