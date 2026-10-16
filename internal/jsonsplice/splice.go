package jsonsplice

import (
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/go-json-experiment/json/jsontext"
)

// Replacement swaps one JSON string token of the source for a new value.
// [Start, End) is the byte range of the complete token including both quotes;
// New is the decoded replacement value and is escaped by Apply.
type Replacement struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	New   string `json:"new"`
}

// Apply returns src with all replacements spliced in. It never modifies src.
// Without replacements it returns src itself. The result is built in one
// forward pass, which is equivalent to the back-to-front splicing of ADR-0004
// and keeps every offset relative to the original body.
//
// Replacements may be given in any order. Apply rejects ranges that are not
// exactly one JSON string token, overlapping ranges and values that are not
// valid UTF-8, so a successful Apply always yields valid JSON.
func Apply(src []byte, reps []Replacement) ([]byte, error) {
	if len(reps) == 0 {
		return src, nil
	}
	sorted := slices.Clone(reps)
	slices.SortStableFunc(sorted, func(a, b Replacement) int {
		if a.Start != b.Start {
			return a.Start - b.Start
		}
		return a.End - b.End
	})
	size := len(src)
	prevEnd := 0
	for i, r := range sorted {
		if r.Start < 0 || r.End > len(src) || r.End-r.Start < 2 {
			return nil, fmt.Errorf("jsonsplice: replacement %d: range [%d,%d) out of bounds", i, r.Start, r.End)
		}
		if r.Start < prevEnd {
			return nil, fmt.Errorf("jsonsplice: replacement %d: range [%d,%d) overlaps previous", i, r.Start, r.End)
		}
		tok := jsontext.Value(src[r.Start:r.End])
		if tok.Kind() != '"' || !tok.IsValid() {
			return nil, fmt.Errorf("jsonsplice: replacement %d: range [%d,%d) is not a single JSON string", i, r.Start, r.End)
		}
		if !utf8.ValidString(r.New) {
			return nil, fmt.Errorf("jsonsplice: replacement %d: new value is not valid UTF-8", i)
		}
		size += len(r.New) + 2 - (r.End - r.Start)
		prevEnd = r.End
	}
	out := make([]byte, 0, max(size, 0)+16)
	pos := 0
	for _, r := range sorted {
		out = append(out, src[pos:r.Start]...)
		out = append(out, '"')
		out = Escape(out, r.New)
		out = append(out, '"')
		pos = r.End
	}
	out = append(out, src[pos:]...)
	return out, nil
}

const hexDigits = "0123456789abcdef"

// Escape appends s to dst as the content of a JSON string (without quotes).
// Only '"', '\' and the control characters U+0000-U+001F are escaped; all
// other bytes, including non-ASCII UTF-8, stay raw (ADR-0004, point 4).
// The function is fixed on purpose: it never depends on library versions.
func Escape(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			dst = append(dst, '\\', '"')
		case c == '\\':
			dst = append(dst, '\\', '\\')
		case c == '\n':
			dst = append(dst, '\\', 'n')
		case c == '\t':
			dst = append(dst, '\\', 't')
		case c == '\r':
			dst = append(dst, '\\', 'r')
		case c == '\b':
			dst = append(dst, '\\', 'b')
		case c == '\f':
			dst = append(dst, '\\', 'f')
		case c < 0x20:
			dst = append(dst, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
		default:
			dst = append(dst, c)
		}
	}
	return dst
}

var errNotObject = errors.New("jsonsplice: request body is not a JSON object")
