package sse

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func readAll(t *testing.T, r io.Reader) []string {
	t.Helper()
	rd := NewReader(r)
	var out []string
	for {
		ev, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		out = append(out, string(ev))
	}
}

func TestReaderFramesEvents(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"one LF event", "event: a\ndata: 1\n\n", []string{"event: a\ndata: 1\n\n"}},
		{"two events", "data: 1\n\ndata: 2\n\n", []string{"data: 1\n\n", "data: 2\n\n"}},
		{"CRLF", "data: 1\r\n\r\ndata: 2\r\n\r\n", []string{"data: 1\r\n\r\n", "data: 2\r\n\r\n"}},
		{"comment keepalive", ": ping\n\ndata: 1\n\n", []string{": ping\n\n", "data: 1\n\n"}},
		{"multi data lines", "data: a\ndata: b\n\n", []string{"data: a\ndata: b\n\n"}},
		{"extra blank lines", "data: 1\n\n\n\ndata: 2\n\n", []string{"data: 1\n\n", "\n", "\n", "data: 2\n\n"}},
		{"partial tail", "data: 1\n\ndata: 2", []string{"data: 1\n\n", "data: 2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readAll(t, strings.NewReader(tt.in))
			if strings.Join(got, "|") != strings.Join(tt.want, "|") || len(got) != len(tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			// Same result when the source delivers one byte per Read (chunk boundaries anywhere).
			slow := readAll(t, iotest.OneByteReader(strings.NewReader(tt.in)))
			if strings.Join(slow, "|") != strings.Join(got, "|") || len(slow) != len(got) {
				t.Fatalf("one-byte reads: got %q, want %q", slow, got)
			}
		})
	}
}

func TestReaderRejectsOversizedEvent(t *testing.T) {
	rd := NewReaderSize(strings.NewReader("data: "+strings.Repeat("x", 100)+"\n\n"), 32)
	if _, err := rd.Next(); !errors.Is(err, ErrEventTooLarge) {
		t.Fatalf("got %v, want ErrEventTooLarge", err)
	}
}

type flushRecorder struct {
	bytes.Buffer
	flushes []int // buffer length at each flush
}

func (f *flushRecorder) Flush() { f.flushes = append(f.flushes, f.Len()) }

func TestCopyFlushesPerEventAndKeepsBytes(t *testing.T) {
	in := "event: a\ndata: 1\n\n: ping\n\nevent: b\ndata: 2\n\ndata: tail"
	var dst flushRecorder
	n, err := Copy(&dst, dst.Flush, iotest.OneByteReader(strings.NewReader(in)))
	if err != nil {
		t.Fatal(err)
	}
	if dst.String() != in || n != int64(len(in)) {
		t.Fatalf("copy altered bytes: %q (n=%d)", dst.String(), n)
	}
	want := []int{len("event: a\ndata: 1\n\n"), len("event: a\ndata: 1\n\n: ping\n\n"), len("event: a\ndata: 1\n\n: ping\n\nevent: b\ndata: 2\n\n"), len(in)}
	if len(dst.flushes) != len(want) {
		t.Fatalf("flushes %v, want %v", dst.flushes, want)
	}
	for i := range want {
		if dst.flushes[i] != want[i] {
			t.Fatalf("flushes %v, want %v", dst.flushes, want)
		}
	}
}

func FuzzReaderRoundTrip(f *testing.F) {
	f.Add("event: x\ndata: {}\n\n")
	f.Add("data: 1\r\n\r\ndata")
	f.Add("\n\n\n")
	f.Fuzz(func(t *testing.T, in string) {
		var out strings.Builder
		rd := NewReader(iotest.OneByteReader(strings.NewReader(in)))
		for {
			ev, err := rd.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			out.Write(ev)
		}
		if out.String() != in {
			t.Fatalf("round trip changed bytes: %q -> %q", in, out.String())
		}
	})
}

func TestCopyObserveSeesEveryEventUnchanged(t *testing.T) {
	in := "event: a\ndata: 1\n\nevent: b\r\ndata: 2\r\n\r\ntail"
	var seen []string
	var out strings.Builder
	if _, err := CopyObserve(&out, nil, strings.NewReader(in), func(ev []byte) { seen = append(seen, string(ev)) }); err != nil {
		t.Fatal(err)
	}
	if out.String() != in || strings.Join(seen, "") != in || len(seen) != 3 {
		t.Fatalf("out %q, seen %q", out.String(), seen)
	}
}
