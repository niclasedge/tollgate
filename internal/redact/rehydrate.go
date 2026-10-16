package redact

import (
	"strings"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

const (
	placeholderPrefix = "<redacted:"
	// maxHold is the longest viable placeholder prefix: "<redacted:" + 32
	// type bytes + "_" + 6 digits (ADR-0006, a full placeholder has 50 bytes).
	maxHold = len(placeholderPrefix) + 32 + 1 + 6
)

// Rehydrate replaces every placeholder that has an entry in mapping with its
// plaintext. Unknown placeholders, e.g. reserved ones typed by the user,
// stay unchanged (ADR-0006).
func Rehydrate(s string, mapping map[string]string) string {
	return rehydrateCount(s, mapping, false, nil)
}

// rehydrateCount replaces known placeholders (JSON-escaped with json set) and adds
// the number of replacements to n if n is not nil.
func rehydrateCount(s string, mapping map[string]string, json bool, n *int) string {
	if len(mapping) == 0 || !strings.Contains(s, placeholderPrefix) {
		return s
	}
	return placeholderRE.ReplaceAllStringFunc(s, func(ph string) string {
		v, ok := mapping[ph]
		if !ok {
			return ph
		}
		if n != nil {
			*n++
		}
		if json {
			return string(jsonsplice.Escape(nil, v))
		}
		return v
	})
}

// RehydrateJSON is Rehydrate for raw JSON text such as partial_json: the
// plaintext is inserted JSON-escaped (ADR-0004), because a placeholder the
// model wrote always sits inside a JSON string.
func RehydrateJSON(s string, mapping map[string]string) string {
	return rehydrateCount(s, mapping, true, nil)
}

// Holdback is the return-path buffer of one content block. It holds back a
// trailing text that may still become a placeholder (at most maxHold bytes)
// until the next chunk completes or refutes it.
type Holdback struct {
	mapping map[string]string
	json    bool
	pending string
	// replaced counts the placeholders translated back (#87); it may point
	// to a counter shared by all blocks of a stream.
	replaced *int
}

// NewHoldback returns a buffer for one content block. With json set, the
// plaintext is inserted JSON-escaped (input_json_delta).
func NewHoldback(mapping map[string]string, json bool) *Holdback {
	return &Holdback{mapping: mapping, json: json, replaced: new(int)}
}

// Replaced is the number of placeholders translated back so far.
func (h *Holdback) Replaced() int { return *h.replaced }

// Write appends chunk and returns the rehydrated text that can be emitted
// now; it may be empty.
func (h *Holdback) Write(chunk string) string {
	s := h.pending + chunk
	cut := holdFrom(s)
	h.pending = s[cut:]
	return rehydrateCount(s[:cut], h.mapping, h.json, h.replaced)
}

// Flush returns the held-back rest unchanged and empties the buffer.
func (h *Holdback) Flush() string {
	s := h.pending
	h.pending = ""
	return s
}

// holdFrom returns the start of the trailing viable placeholder prefix of s,
// or len(s). A viable prefix contains exactly one '<' (at its start) and no
// '>', so the cut never splits a complete placeholder and always lies on a
// UTF-8 boundary.
func holdFrom(s string) int {
	i := strings.LastIndexByte(s, '<')
	if i < 0 || len(s)-i > maxHold || !viablePrefix(s[i:]) {
		return len(s)
	}
	return i
}

// viablePrefix reports whether p (starting with '<') can still be extended to
// a placeholder of the ADR-0006 grammar <redacted:TYPE_N>.
func viablePrefix(p string) bool {
	if len(p) <= len(placeholderPrefix) {
		return strings.HasPrefix(placeholderPrefix, p)
	}
	if !strings.HasPrefix(p, placeholderPrefix) {
		return false
	}
	r := p[len(placeholderPrefix):]
	if !isLower(r[0]) {
		return false
	}
	for i := 0; i < len(r); i++ {
		if !isLower(r[i]) && !isDigit(r[i]) && r[i] != '_' {
			return false
		}
	}
	if len(r) <= 32 {
		return true // r may still be (a prefix of) the type
	}
	// Otherwise r must be TYPE "_" digits-prefix for some split.
	for k := 1; k <= 32 && k < len(r); k++ {
		if r[k] == '_' && digitsPrefix(r[k+1:]) {
			return true
		}
	}
	return false
}

// digitsPrefix reports whether d is a prefix of [1-9][0-9]{0,5}.
func digitsPrefix(d string) bool {
	if len(d) > 6 || (len(d) > 0 && d[0] == '0') {
		return false
	}
	for i := 0; i < len(d); i++ {
		if !isDigit(d[i]) {
			return false
		}
	}
	return true
}

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
