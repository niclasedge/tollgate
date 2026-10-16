package sse

import (
	"bufio"
	"errors"
	"io"
)

// DefaultMaxEventSize caps one event. tool_use input deltas can be large, but
// a single event beyond this size is treated as a broken stream.
const DefaultMaxEventSize = 8 << 20

// ErrEventTooLarge is returned when one event exceeds the reader's limit.
var ErrEventTooLarge = errors.New("sse: event too large")

// Reader splits a stream into raw events. An event ends with an empty line
// ("\n\n" or "\r\n\r\n"); lines end with "\n" (a preceding "\r" is part of the
// line). A lone "\r" is not a line terminator, which matches what Anthropic and
// CLIProxyAPI emit.
type Reader struct {
	br  *bufio.Reader
	max int
}

// NewReader returns a Reader with DefaultMaxEventSize.
func NewReader(r io.Reader) *Reader { return NewReaderSize(r, DefaultMaxEventSize) }

// NewReaderSize returns a Reader that rejects events larger than maxEvent bytes.
func NewReaderSize(r io.Reader, maxEvent int) *Reader {
	return &Reader{br: bufio.NewReaderSize(r, 4096), max: maxEvent}
}

// Next returns the raw bytes of the next event including its terminating empty
// line. At the end of the stream, trailing bytes without a terminator are
// returned as a last event; after that Next returns io.EOF. An empty line
// directly after an event is returned as an event of its own so that no byte is
// dropped. The returned slice is only valid until the next call.
func (r *Reader) Next() ([]byte, error) {
	var ev []byte
	for {
		line, err := r.br.ReadSlice('\n')
		if len(ev)+len(line) > r.max {
			return nil, ErrEventTooLarge
		}
		ev = append(ev, line...)
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			continue // long line, keep reading
		case err != nil:
			if len(ev) > 0 && errors.Is(err, io.EOF) {
				return ev, nil
			}
			return nil, err
		}
		if isBlank(line) {
			return ev, nil
		}
	}
}

func isBlank(line []byte) bool {
	return len(line) == 1 || (len(line) == 2 && line[0] == '\r')
}

// Copy reads events from src and writes each complete event to dst, calling
// flush (if not nil) after every event. It returns the number of bytes written.
func Copy(dst io.Writer, flush func(), src io.Reader) (int64, error) {
	return CopyObserve(dst, flush, src, nil)
}

// CopyObserve is Copy that also hands every event to observe (if not nil)
// before writing it. observe must not keep or modify the slice.
func CopyObserve(dst io.Writer, flush func(), src io.Reader, observe func(ev []byte)) (int64, error) {
	rd := NewReader(src)
	var total int64
	for {
		ev, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		if observe != nil {
			observe(ev)
		}
		n, err := dst.Write(ev)
		total += int64(n)
		if err != nil {
			return total, err
		}
		if flush != nil {
			flush()
		}
	}
}
