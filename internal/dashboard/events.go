package dashboard

import (
	"encoding/json"
	"net/http"
	"time"
)

// events streams trace events as SSE: event "trace" with a TraceEvent as
// data, plus a heartbeat comment so idle connections stay open. It ends when
// the client disconnects.
func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("Cache-Control", "no-store")
	hd.Set("X-Accel-Buffering", "no")
	ch, unsubscribe := h.broker.Subscribe()
	defer unsubscribe()
	// SSE responses run for minutes: lift the write deadline of this response.
	_ = rc.SetWriteDeadline(time.Time{})
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(": connected\n\n")); err != nil || rc.Flush() != nil {
		return
	}
	tick := time.NewTicker(h.heartbeat)
	defer tick.Stop()
	for {
		var frame []byte
		select {
		case <-r.Context().Done():
			return
		case <-h.done: // nil channel when unset: never ready
			return
		case <-tick.C:
			frame = []byte(": heartbeat\n\n")
		case ev := <-ch:
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			frame = append(append([]byte("event: trace\ndata: "), data...), '\n', '\n')
		}
		if _, err := w.Write(frame); err != nil || rc.Flush() != nil {
			return
		}
	}
}
