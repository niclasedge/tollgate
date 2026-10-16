// Ported from Headroom crates/headroom-core/src/transforms/log_compressor.rs
// (normalize_for_dedupe).
// Headroom (Apache-2.0, Copyright 2025 Headroom Contributors), Stand 0947a28,
// portiert und verändert: hex addresses are replaced BEFORE decimals (Headroom
// does it the other way round, so its hex pattern never matches).

package logs

import (
	"regexp"
	"strings"
)

var (
	reHex   = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	reDigit = regexp.MustCompile(`\d+`)
	rePath  = regexp.MustCompile(`/[\w/]+/`)
)

// normalizeForDedupe keeps the message prefix (everything before the first
// ':' or '=') verbatim and tokenizes only the variable tail, so distinct
// messages stay distinct while the same warning with other numbers,
// addresses or directories collapses.
func normalizeForDedupe(content string) string {
	split := strings.IndexAny(content, ":=")
	if split < 0 {
		split = len(content)
	}
	prefix, suffix := content[:split], content[split:]
	suffix = reHex.ReplaceAllString(suffix, "ADDR")
	suffix = reDigit.ReplaceAllString(suffix, "N")
	suffix = rePath.ReplaceAllString(suffix, "/PATH/")
	return prefix + suffix
}
