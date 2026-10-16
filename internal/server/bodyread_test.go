package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// slowPost sends chunks with a pause before each one, so the whole body takes
// longer than the idle limit while no single pause exceeds it.
func slowPost(t *testing.T, url string, chunks int, pause time.Duration) (int, error) {
	t.Helper()
	pr, pw := io.Pipe()
	go func() {
		for range chunks {
			time.Sleep(pause)
			if _, err := pw.Write([]byte(strings.Repeat("x", 1024))); err != nil {
				return
			}
		}
		_ = pw.Close()
	}()
	req, err := http.NewRequest("POST", url, pr)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = int64(chunks * 1024)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func idleServer(t *testing.T, idle time.Duration, logs io.Writer) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewJSONHandler(logs, nil))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = newIdleBody(w, r, idle, log)
		// Like proxy: a total deadline first, which the reads must extend.
		_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(idle))
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte{byte(len(body) / 1024)})
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestIdleBodyAllowsSlowSenders(t *testing.T) {
	logs := &lockedBuf{}
	ts := idleServer(t, 300*time.Millisecond, logs)
	// 8 × 100 ms = 800 ms in total, far over the 300 ms limit, but never idle that long.
	code, err := slowPost(t, ts.URL, 8, 100*time.Millisecond)
	if err != nil || code != http.StatusOK {
		t.Fatalf("slow but steady sender: %d %v", code, err)
	}
	if logs.String() != "" {
		t.Errorf("unexpected log: %s", logs.String())
	}
}

func TestIdleBodyStopsSilentSenders(t *testing.T) {
	logs := &lockedBuf{}
	ts := idleServer(t, 150*time.Millisecond, logs)
	code, _ := slowPost(t, ts.URL, 3, 400*time.Millisecond)
	if code == http.StatusOK {
		t.Fatal("a sender silent for longer than the limit must fail")
	}
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(logs.String(), `"cause":"idle_timeout"`) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	got := logs.String()
	for _, want := range []string{`"msg":"request body read failed"`, `"cause":"idle_timeout"`, `"content_length":3072`, `"read_bytes":`} {
		if !strings.Contains(got, want) {
			t.Errorf("log %s misses %s", got, want)
		}
	}
	if strings.Contains(got, "xxx") {
		t.Error("body content must never be logged")
	}
}
