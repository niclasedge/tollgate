package server

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// idleBody bounds reading the request body by silence instead of total time
// (#130): every read moves the connection's read deadline, so a slow link
// (mobile network, DERP relay) can send a long history as long as bytes keep
// coming. The first failed read is logged with sizes and timing, never with
// content.
type idleBody struct {
	io.ReadCloser
	rc     *http.ResponseController
	idle   time.Duration
	start  time.Time
	n      int64
	want   int64
	remote string
	log    *slog.Logger
	failed bool
}

func newIdleBody(w http.ResponseWriter, r *http.Request, idle time.Duration, log *slog.Logger) *idleBody {
	return &idleBody{ReadCloser: r.Body, rc: http.NewResponseController(w), idle: idle, start: time.Now(),
		want: r.ContentLength, remote: r.RemoteAddr, log: log}
}

func (b *idleBody) Read(p []byte) (int, error) {
	_ = b.rc.SetReadDeadline(time.Now().Add(b.idle))
	n, err := b.ReadCloser.Read(p)
	b.n += int64(n)
	if err != nil && !errors.Is(err, io.EOF) && !b.failed {
		b.failed = true
		cause := "connection"
		if errors.Is(err, os.ErrDeadlineExceeded) {
			cause = "idle_timeout"
		}
		b.log.Warn("request body read failed", "cause", cause, "read_bytes", b.n, "content_length", b.want,
			"duration_ms", time.Since(b.start).Milliseconds(), "remote", b.remote)
	}
	return n, err
}
