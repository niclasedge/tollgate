package redact

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strconv"
	"time"

	"github.com/go-json-experiment/json/jsontext"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
	"github.com/niclasedge/tollgate/internal/sse"
)

const (
	deltaText = "text_delta"
	deltaJSON = "input_json_delta"
)

// CopyRehydrated is sse.CopyObserve with the return path of ADR-0006: in
// text_delta and input_json_delta events, placeholders of mapping are
// replaced by their plaintext (JSON-escaped in partial_json), with one
// holdback buffer per content block. Only the value of delta.text or
// delta.partial_json is spliced; every other event and byte passes
// unchanged, thinking and signature deltas included. A rest that is still a
// placeholder prefix when its block stops is emitted as one extra delta
// before content_block_stop.
//
// observe (if not nil) sees every upstream event unchanged, before it is
// rewritten. With an empty mapping the stream is copied byte for byte.
func CopyRehydrated(dst io.Writer, flush func(), src io.Reader, mapping map[string]string, observe func(ev []byte)) (int64, error) {
	return CopyRehydratedStats(dst, flush, src, mapping, observe, nil)
}

// ReturnStats are the measurements of the return path (#87): the time spent
// translating placeholders back (without waiting for the upstream or the
// client) and the number of placeholders translated.
type ReturnStats struct {
	Duration     time.Duration
	Replacements int
}

// CopyRehydratedStats is CopyRehydrated that also fills st (if not nil). With
// an empty mapping no return path runs and st stays zero.
func CopyRehydratedStats(dst io.Writer, flush func(), src io.Reader, mapping map[string]string, observe func(ev []byte), st *ReturnStats) (int64, error) {
	if len(mapping) == 0 {
		return sse.CopyObserve(dst, flush, src, observe)
	}
	rw := &streamRewriter{dst: dst, mapping: mapping, blocks: map[int]*blockBuf{}}
	defer func() {
		if st != nil {
			st.Duration, st.Replacements = rw.work-rw.writing, rw.replaced
		}
	}()
	rd := sse.NewReader(src)
	for {
		ev, err := rd.Next()
		if errors.Is(err, io.EOF) {
			rw.timed(rw.flushAll)
			if rw.err == nil && flush != nil {
				flush()
			}
			return rw.total, rw.err
		}
		if err != nil {
			return rw.total, err
		}
		if observe != nil {
			observe(ev)
		}
		rw.timed(func() { rw.event(ev) })
		if rw.err != nil {
			return rw.total, rw.err
		}
		if flush != nil {
			flush()
		}
	}
}

type blockBuf struct {
	hb        *Holdback
	deltaType string
}

type streamRewriter struct {
	dst     io.Writer
	mapping map[string]string
	blocks  map[int]*blockBuf // lookup only, never iterated unsorted
	total   int64
	err     error
	buf     []byte
	// work is the time spent in event handling, writing the part of it
	// spent in dst.Write; replaced counts translated placeholders.
	work, writing time.Duration
	replaced      int
}

// timed runs fn and adds its duration to work.
func (w *streamRewriter) timed(fn func()) {
	start := time.Now()
	fn()
	w.work += time.Since(start)
}

func (w *streamRewriter) write(p []byte) {
	if w.err != nil {
		return
	}
	start := time.Now()
	n, err := w.dst.Write(p)
	w.writing += time.Since(start)
	w.total += int64(n)
	w.err = err
}

// event processes one raw upstream event.
func (w *streamRewriter) event(ev []byte) {
	dataStart, dataEnd, ok := singleDataLine(ev)
	if !ok {
		w.write(ev) // comment, keep-alive or multi-line data: unchanged
		return
	}
	e, err := parseEvent(ev[dataStart:dataEnd])
	if err != nil {
		w.flushAll()
		w.write(ev)
		return
	}
	switch e.typ {
	case "content_block_delta":
		if !e.hasIndex {
			w.flushAll()
			w.write(ev)
			return
		}
		if (e.deltaType != deltaText && e.deltaType != deltaJSON) || !e.hasValue {
			w.flushBlock(e.index)
			w.write(ev)
			return
		}
		w.delta(ev, dataStart, e)
	case "content_block_stop":
		if e.hasIndex {
			w.flushBlock(e.index)
			delete(w.blocks, e.index)
		} else {
			w.flushAll()
		}
		w.write(ev)
	case "message_delta", "message_stop", "error":
		w.flushAll()
		w.write(ev)
	default:
		w.write(ev)
	}
}

func (w *streamRewriter) delta(ev []byte, dataStart int, e parsedEvent) {
	b := w.blocks[e.index]
	if b != nil && b.deltaType != e.deltaType {
		w.flushBlock(e.index)
		b = nil
	}
	if b == nil {
		b = &blockBuf{hb: &Holdback{mapping: w.mapping, json: e.deltaType == deltaJSON, replaced: &w.replaced}, deltaType: e.deltaType}
		w.blocks[e.index] = b
	}
	out := b.hb.Write(e.value)
	if out == e.value {
		w.write(ev)
		return
	}
	w.buf = append(w.buf[:0], ev[:dataStart+e.valStart]...)
	w.buf = append(w.buf, '"')
	w.buf = jsonsplice.Escape(w.buf, out)
	w.buf = append(w.buf, '"')
	w.buf = append(w.buf, ev[dataStart+e.valEnd:]...)
	w.write(w.buf)
}

// flushBlock emits the held-back rest of one block as a synthetic delta.
func (w *streamRewriter) flushBlock(index int) {
	b := w.blocks[index]
	if b == nil {
		return
	}
	rest := b.hb.Flush()
	if rest == "" {
		return
	}
	field := "text"
	if b.deltaType == deltaJSON {
		field = "partial_json"
	}
	w.buf = append(w.buf[:0], "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":"...)
	w.buf = strconv.AppendInt(w.buf, int64(index), 10)
	w.buf = append(w.buf, `,"delta":{"type":"`...)
	w.buf = append(w.buf, b.deltaType...)
	w.buf = append(w.buf, `","`...)
	w.buf = append(w.buf, field...)
	w.buf = append(w.buf, `":"`...)
	w.buf = jsonsplice.Escape(w.buf, rest)
	w.buf = append(w.buf, "\"}}\n\n"...)
	w.write(w.buf)
}

// flushAll flushes every block in ascending index order.
func (w *streamRewriter) flushAll() {
	idx := make([]int, 0, len(w.blocks))
	for i := range w.blocks {
		idx = append(idx, i)
	}
	slices.Sort(idx)
	for _, i := range idx {
		w.flushBlock(i)
	}
}

// singleDataLine returns the byte range of the payload of the only "data:"
// line of ev (after an optional single space, without the line ending).
func singleDataLine(ev []byte) (start, end int, ok bool) {
	pos, found := 0, false
	for line := range bytes.Lines(ev) {
		if rest, isData := bytes.CutPrefix(line, []byte("data:")); isData {
			if found {
				return 0, 0, false
			}
			found = true
			start = pos + len(line) - len(rest)
			if len(rest) > 0 && rest[0] == ' ' {
				start++
			}
			end = pos + len(bytes.TrimRight(line, "\r\n"))
		}
		pos += len(line)
	}
	return start, end, found && start <= end
}

type parsedEvent struct {
	typ       string
	index     int
	hasIndex  bool
	deltaType string
	value     string
	hasValue  bool
	// valStart and valEnd delimit the string token of the delta value
	// (including quotes) in the data payload.
	valStart, valEnd int
}

// parseEvent reads the fields of a stream event that the return path needs
// and the position of delta.text or delta.partial_json. Unknown members are
// skipped; duplicate names are rejected by the decoder.
func parseEvent(data []byte) (parsedEvent, error) {
	var e parsedEvent
	dec := jsontext.NewDecoder(bytes.NewReader(data))
	type strTok struct {
		start, end int
		val        string
		ok         bool
	}
	var text, partial strTok
	err := object(dec, func(name string) error {
		switch name {
		case "type":
			s, _, _, err := readString(dec)
			e.typ = s
			return err
		case "index":
			raw, err := dec.ReadValue()
			if err != nil {
				return err
			}
			if n, err := strconv.Atoi(string(raw)); err == nil {
				e.index, e.hasIndex = n, true
			}
			return nil
		case "delta":
			if dec.PeekKind() != '{' {
				return dec.SkipValue()
			}
			return object(dec, func(name string) error {
				switch name {
				case "type":
					s, _, _, err := readString(dec)
					e.deltaType = s
					return err
				case "text", "partial_json":
					if dec.PeekKind() != '"' {
						return dec.SkipValue()
					}
					s, start, end, err := readString(dec)
					if name == "text" {
						text = strTok{start, end, s, true}
					} else {
						partial = strTok{start, end, s, true}
					}
					return err
				}
				return dec.SkipValue()
			})
		}
		return dec.SkipValue()
	})
	if err != nil {
		return e, err
	}
	if _, err := dec.ReadToken(); !errors.Is(err, io.EOF) {
		return e, errors.New("redact: trailing data in event")
	}
	tok := text
	if e.deltaType == deltaJSON {
		tok = partial
	}
	if tok.ok {
		e.value, e.valStart, e.valEnd, e.hasValue = tok.val, tok.start, tok.end, true
	}
	return e, nil
}

// object iterates the members of the next value, which must be an object.
// fn must consume the member value.
func object(dec *jsontext.Decoder, fn func(name string) error) error {
	t, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if t.Kind() != '{' {
		return errors.New("redact: not an object")
	}
	for dec.PeekKind() != '}' {
		nt, err := dec.ReadToken()
		if err != nil {
			return err
		}
		if err := fn(nt.String()); err != nil {
			return err
		}
	}
	_, err = dec.ReadToken()
	return err
}

// readString reads a string value and returns it with its token range.
func readString(dec *jsontext.Decoder) (val string, start, end int, err error) {
	if dec.PeekKind() != '"' {
		return "", 0, 0, errors.New("redact: not a string")
	}
	raw, err := dec.ReadValue()
	if err != nil {
		return "", 0, 0, err
	}
	end = int(dec.InputOffset())
	start = end - len(raw)
	v, err := jsontext.AppendUnquote(nil, raw)
	return string(v), start, end, err
}

// RehydrateBody is the return path for a non-streaming Messages response:
// content[].text of text blocks and every string value in content[].input of
// tool_use blocks are rehydrated by byte splicing. Thinking blocks,
// signatures, unknown blocks and fields stay byte-identical. A body that is
// not a valid JSON object is returned unchanged.
func RehydrateBody(body []byte, mapping map[string]string) []byte {
	out, _ := RehydrateBodyCount(body, mapping)
	return out
}

// RehydrateBodyCount is RehydrateBody that also returns the number of
// placeholders translated back.
func RehydrateBodyCount(body []byte, mapping map[string]string) ([]byte, int) {
	n := 0
	if out, ok := rehydrateBody(body, mapping, &n); ok {
		return out, n
	}
	return body, 0 // unchanged: fallback paths count nothing
}

// rehydrateBody returns the rewritten body and true, or false if the body
// stays unchanged.
func rehydrateBody(body []byte, mapping map[string]string, n *int) ([]byte, bool) {
	if len(mapping) == 0 || !bytes.Contains(body, []byte(placeholderPrefix)) {
		return nil, false
	}
	var reps []jsonsplice.Replacement
	add := func(val string, start, end int) {
		if nv := rehydrateCount(val, mapping, false, n); nv != val {
			reps = append(reps, jsonsplice.Replacement{Start: start, End: end, New: nv})
		}
	}
	dec := jsontext.NewDecoder(bytes.NewReader(body))
	err := object(dec, func(name string) error {
		if name != "content" || dec.PeekKind() != '[' {
			return dec.SkipValue()
		}
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		for dec.PeekKind() != ']' {
			raw, err := dec.ReadValue()
			if err != nil {
				return err
			}
			base := int(dec.InputOffset()) - len(raw)
			if err := contentBlock(raw, base, add); err != nil {
				return err
			}
		}
		_, err := dec.ReadToken()
		return err
	})
	if err == nil {
		if _, err2 := dec.ReadToken(); !errors.Is(err2, io.EOF) {
			err = errors.New("redact: trailing data")
		}
	}
	if err != nil || len(reps) == 0 {
		return nil, false
	}
	out, err := jsonsplice.Apply(body, reps)
	if err != nil {
		return nil, false
	}
	return out, true
}

// contentBlock collects the rewritable strings of one content block. raw
// starts at offset base of the body. The block type may follow the fields,
// so candidates are collected first and kept by type afterwards.
func contentBlock(raw []byte, base int, add func(val string, start, end int)) error {
	if len(raw) == 0 || raw[0] != '{' {
		return nil
	}
	type cand struct {
		val        string
		start, end int
	}
	var typ string
	var text, input []cand
	dec := jsontext.NewDecoder(bytes.NewReader(raw))
	err := object(dec, func(name string) error {
		switch name {
		case "type":
			if dec.PeekKind() != '"' {
				return dec.SkipValue()
			}
			s, _, _, err := readString(dec)
			typ = s
			return err
		case "text":
			if dec.PeekKind() != '"' {
				return dec.SkipValue()
			}
			s, start, end, err := readString(dec)
			text = append(text, cand{s, start, end})
			return err
		case "input":
			return walkStrings(dec, func(s string, start, end int) {
				input = append(input, cand{s, start, end})
			})
		}
		return dec.SkipValue()
	})
	if err != nil {
		return err
	}
	var keep []cand
	switch typ {
	case "text":
		keep = text
	case "tool_use":
		keep = input
	}
	for _, c := range keep {
		add(c.val, base+c.start, base+c.end)
	}
	return nil
}

// walkStrings calls fn for every string value (not member names) of the
// next value, recursively, in document order.
func walkStrings(dec *jsontext.Decoder, fn func(s string, start, end int)) error {
	switch dec.PeekKind() {
	case '"':
		s, start, end, err := readString(dec)
		if err == nil {
			fn(s, start, end)
		}
		return err
	case '{':
		return object(dec, func(string) error { return walkStrings(dec, fn) })
	case '[':
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		for dec.PeekKind() != ']' {
			if err := walkStrings(dec, fn); err != nil {
				return err
			}
		}
		_, err := dec.ReadToken()
		return err
	}
	return dec.SkipValue()
}
