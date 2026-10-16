package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/store"
)

func TestAccountSnapshotHeldThroughStreamAndUsedByNextRequest(t *testing.T) {
	st := openStore(t)
	gate := new(accounts.Gate)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Anthropic-Ratelimit-Unified-7d-Utilization", "0.2")
		if calls.Load() == 1 {
			close(started)
			<-release
		}
		_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":11}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":3}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	})
	current := "account-a"
	var eventMu sync.Mutex
	completed := make(chan struct{}, 2)
	var events []store.Trace
	var observed []string
	srv, err := New(Options{Runner: pipeline.NewRunner([]pipeline.Stage{&pipeline.Intake{Keys: fakeKeys{clientKey: "client-1"}}}), Store: st, UpstreamURL: up.URL, UpstreamKey: cpaKey, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), AccountGate: gate, AccountID: func(context.Context) (string, error) { calls.Add(1); return current, nil }, TraceObserver: func(tr store.Trace) {
		eventMu.Lock()
		events = append(events, tr)
		eventMu.Unlock()
		if tr.Status != 0 {
			completed <- struct{}{}
		}
	}, AccountResponseObserver: func(id string, _ http.Header) { eventMu.Lock(); observed = append(observed, id); eventMu.Unlock() }})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	finished := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest("POST", ts.URL+"/v1/messages", nil)
		req.Body = io.NopCloser(strings.NewReader(msgBody))
		req.Header.Set("Authorization", "Bearer "+clientKey)
		req.Header.Set("X-Account-ID", "forged-account")
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			_, err = io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}
		finished <- err
	}()
	<-started
	if end, ok := gate.Exclusive(); ok {
		end()
		t.Fatal("switch allowed during stream")
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	<-completed
	tr := waitFinished(t, st)
	if tr.AccountID != "account-a" || calls.Load() != 1 {
		t.Fatal(tr.AccountID, calls.Load())
	}
	end, ok := gate.Exclusive()
	if !ok {
		t.Fatal("gate not released")
	}
	current = "account-b"
	end()
	resp, _ := doReq(t, "POST", ts.URL+"/v1/messages", msgBody, auth(nil))
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	<-completed
	eventMu.Lock()
	defer eventMu.Unlock()
	if calls.Load() != 2 || len(events) != 4 || events[0].AccountID != "account-a" || events[1].AccountID != "account-a" || events[2].AccountID != "account-b" || events[3].AccountID != "account-b" || len(observed) != 2 || observed[0] != "account-a" || observed[1] != "account-b" {
		t.Fatal("unstable attribution", events, observed)
	}
}
func TestAccountResolutionFailureStopsForwarding(t *testing.T) {
	up := newUpstream(t, okJSON)
	srv, err := New(Options{Runner: pipeline.NewRunner([]pipeline.Stage{&pipeline.Intake{Keys: fakeKeys{clientKey: "client-1"}}}), UpstreamURL: up.URL, UpstreamKey: cpaKey, AccountID: func(context.Context) (string, error) { return "", errors.New("private-token") }})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	res, body := doReq(t, "POST", ts.URL+"/v1/messages", msgBody, auth(nil))
	if res.StatusCode != 503 || len(up.seen()) != 0 || strings.Contains(string(body), "private-token") {
		t.Fatal("failed lookup forwarded or leaked")
	}
}
