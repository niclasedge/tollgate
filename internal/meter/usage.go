package meter

import (
	"bytes"
	"encoding/json"

	"github.com/niclasedge/tollgate/internal/store"
)

// apiUsage is the usage object of the Messages API. Pointers tell a field
// that is absent from one that is zero (message_delta only carries what
// changed).
type apiUsage struct {
	Input         *int64 `json:"input_tokens"`
	Output        *int64 `json:"output_tokens"`
	CacheRead     *int64 `json:"cache_read_input_tokens"`
	CacheCreation *int64 `json:"cache_creation_input_tokens"`
}

// maxTokens bounds a single usage value. Real counts are far below it; the
// bound keeps a garbage value from overflowing cost sums (1<<40 tokens times
// the highest price stays well inside int64).
const maxTokens = 1 << 40

// apply overwrites the fields present in u. The API reports cumulative
// values, so the latest value wins. Out-of-range values are ignored.
func (u *apiUsage) apply(dst *store.Usage) {
	set := func(d *int64, s *int64) {
		if s != nil && *s >= 0 && *s <= maxTokens {
			*d = *s
		}
	}
	set(&dst.Input, u.Input)
	set(&dst.Output, u.Output)
	set(&dst.CacheRead, u.CacheRead)
	set(&dst.CacheCreation, u.CacheCreation)
}

// FromJSON reads the top-level usage of a non-streaming Messages response.
// It returns nil if the body carries no usage.
func FromJSON(body []byte) *store.Usage {
	var v struct {
		Usage *apiUsage `json:"usage"`
	}
	if json.Unmarshal(body, &v) != nil || v.Usage == nil {
		return nil
	}
	var u store.Usage
	v.Usage.apply(&u)
	return &u
}

// Stream collects usage from the raw events of a Messages SSE stream:
// message_start carries the input side (message.usage), message_delta the
// cumulative output and, in newer API versions, updated input counts.
type Stream struct {
	u    store.Usage
	seen bool
}

// Observe inspects one raw event (as returned by sse.Reader). Events other
// than message_start and message_delta are ignored; the event bytes are
// never modified.
func (s *Stream) Observe(ev []byte) {
	data := eventData(ev)
	// Cheap prefilter: most events are content deltas and need no decoding.
	if data == nil || !(bytes.Contains(data, []byte(`"message_start"`)) || bytes.Contains(data, []byte(`"message_delta"`))) {
		return
	}
	var v struct {
		Type    string `json:"type"`
		Message *struct {
			Usage *apiUsage `json:"usage"`
		} `json:"message"`
		Usage *apiUsage `json:"usage"`
	}
	if json.Unmarshal(data, &v) != nil {
		return // e.g. a non-integer count; Anthropic always sends integers
	}
	switch {
	case v.Type == "message_start" && v.Message != nil && v.Message.Usage != nil:
		v.Message.Usage.apply(&s.u)
		s.seen = true
	case v.Type == "message_delta" && v.Usage != nil:
		v.Usage.apply(&s.u)
		s.seen = true
	}
}

// Usage returns the collected usage, or nil if no usage event was seen.
func (s *Stream) Usage() *store.Usage {
	if !s.seen {
		return nil
	}
	u := s.u
	return &u
}

// eventData joins the data lines of one SSE event ("data:" with an optional
// single space, lines joined with "\n"). It returns nil without data lines.
func eventData(ev []byte) []byte {
	var out []byte
	found := false
	for line := range bytes.Lines(ev) {
		line = bytes.TrimRight(line, "\r\n")
		rest, ok := bytes.CutPrefix(line, []byte("data:"))
		if !ok {
			continue
		}
		rest, _ = bytes.CutPrefix(rest, []byte(" "))
		if found {
			out = append(out, '\n')
		}
		out = append(out, rest...)
		found = true
	}
	if !found {
		return nil
	}
	return out
}
