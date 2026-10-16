package redact

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// contextRunes is the number of characters of context on each side that the
// false-positive filter sees, as in hook-redactor's redact_pattern.
const contextRunes = 16

// whitelistSubstrings is WHITELIST_SUBSTRINGS from hook-redactor
// src/redactor.rs: exact matches for every rule, substrings for e-mails.
var whitelistSubstrings = []string{
	"example.com",
	"example.test",
	"example.org",
	"localhost",
	"noreply@",
	"no-reply@",
}

// versionVocab are tokens that mark a dotted quad as a version string.
var versionVocab = []string{"version", "ver", "v", "build", "rev", "release", "semver", "tag"}

// isFalsePositive ports is_false_positive: matched is the whole regex match,
// before and after are up to contextRunes characters of surrounding text.
func isFalsePositive(matched, ruleName, before, after string) bool {
	lower := asciiLower(matched)
	for _, w := range whitelistSubstrings {
		if lower == w {
			return true
		}
	}
	switch ruleName {
	case "ipv4":
		return isPrivateIPv4(matched) || looksLikeVersionString(before, after)
	case "ipv6":
		return isPrivateIPv6(matched)
	case "email":
		for _, w := range whitelistSubstrings {
			if strings.Contains(lower, w) {
				return true
			}
		}
	}
	return false
}

// isPrivateIPv4 reports private, loopback and zero addresses. Like the Rust
// original, octets that do not parse as u8 are dropped, and anything that is
// not four octets afterwards counts as private (safer to leave alone).
func isPrivateIPv4(s string) bool {
	var parts []uint64
	for _, p := range strings.Split(s, ".") {
		if v, err := strconv.ParseUint(p, 10, 8); err == nil {
			parts = append(parts, v)
		}
	}
	if len(parts) != 4 {
		return true
	}
	switch {
	case parts[0] == 10, parts[0] == 127:
		return true
	case parts[0] == 0 && parts[1] == 0 && parts[2] == 0 && parts[3] == 0:
		return true
	case parts[0] == 192 && parts[1] == 168:
		return true
	case parts[0] == 172 && parts[1] >= 16 && parts[1] <= 31:
		return true
	}
	return false
}

// isPrivateIPv6 reports loopback, unspecified, link-local (fe80::/10) and
// unique-local (fc00::/7) addresses, and shapes too short to be IPv6.
func isPrivateIPv6(s string) bool {
	lower := asciiLower(s)
	if lower == "::" || lower == "::1" {
		return true
	}
	prefix, _, _ := strings.Cut(lower, ":")
	if first, err := strconv.ParseUint(prefix, 16, 16); err == nil {
		if first >= 0xfe80 && first <= 0xfebf {
			return true
		}
		if first >= 0xfc00 && first <= 0xfdff {
			return true
		}
	}
	if !strings.Contains(s, "::") && strings.Count(s, ":") < 7 {
		return true
	}
	return false
}

// looksLikeVersionString reports a dotted quad enclosed in matching ASCII
// quotes or preceded by a version token such as "version" or "v".
func looksLikeVersionString(before, after string) bool {
	last, _ := utf8.DecodeLastRuneInString(before)
	first, _ := utf8.DecodeRuneInString(after)
	if before != "" && after != "" && last == first && (last == '\'' || last == '"') {
		return true
	}
	tokens := strings.FieldsFunc(before, func(r rune) bool {
		return unicode.IsSpace(r) || r == '=' || r == ':' || r == '('
	})
	if len(tokens) == 0 {
		return false
	}
	prev := strings.TrimRight(tokens[len(tokens)-1], ".,;")
	if prev == "" {
		return false
	}
	prev = asciiLower(prev)
	for _, w := range versionVocab {
		if prev == w {
			return true
		}
	}
	return false
}

// asciiLower mirrors Rust's to_ascii_lowercase: only A-Z change.
func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, s)
}

// contextAround returns up to contextRunes characters before start and after
// end.
func contextAround(text string, start, end int) (string, string) {
	b := start
	for i := 0; i < contextRunes && b > 0; i++ {
		_, size := utf8.DecodeLastRuneInString(text[:b])
		b -= size
	}
	a := end
	for i := 0; i < contextRunes && a < len(text); i++ {
		_, size := utf8.DecodeRuneInString(text[a:])
		a += size
	}
	return text[b:start], text[end:a]
}
