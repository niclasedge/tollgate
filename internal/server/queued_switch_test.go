package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/pipeline"
)

func TestProxyQueuedRequestWaitsThenResolvesNewAccount(t *testing.T) {
	gate := new(accounts.Gate)
	end, ok := gate.Exclusive()
	if !ok {
		t.Fatal("exclusive")
	}
	var current atomic.Value
	current.Store("account-a")
	ids := make(chan string, 1)
	up := newUpstream(t, okJSON)
	srv, err := New(Options{Runner: pipeline.NewRunner([]pipeline.Stage{&pipeline.Intake{Keys: fakeKeys{clientKey: "client-1"}}}), UpstreamURL: up.URL, UpstreamKey: cpaKey, AccountGate: gate, AccountID: func(context.Context) (string, error) { id := current.Load().(string); ids <- id; return id, nil }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	completed := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest("POST", ts.URL+"/v1/messages", strings.NewReader(msgBody))
		req.Header.Set("Authorization", "Bearer "+clientKey)
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			if res.StatusCode != 200 {
				err = fmt.Errorf("status %d", res.StatusCode)
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}
		completed <- err
	}()
	select {
	case <-ids:
		t.Fatal("account resolved before switch finished")
	case <-time.After(30 * time.Millisecond):
	}
	current.Store("account-b")
	end()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not resume")
	}
	if id := <-ids; id != "account-b" {
		t.Fatal(id)
	}
	if len(up.seen()) != 1 {
		t.Fatal("queued request not forwarded exactly once")
	}
}

func TestProxyForcedSwitchAbortsRunningRequestWithRetryableError(t *testing.T) {
	gate := new(accounts.Gate)
	st := openStore(t)
	arrived, release := make(chan struct{}), make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		select { // a long answer that only the abort ends
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer up.Close()
	defer close(release)
	srv, err := New(Options{Runner: pipeline.NewRunner([]pipeline.Stage{&pipeline.Intake{Keys: fakeKeys{clientKey: "client-1"}}}), Store: st, UpstreamURL: up.URL, UpstreamKey: cpaKey, AccountGate: gate, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	type answer struct {
		status int
		retry  string
	}
	answered := make(chan answer, 1)
	go func() {
		req, _ := http.NewRequest("POST", ts.URL+"/v1/messages", strings.NewReader(msgBody))
		req.Header.Set("Authorization", "Bearer "+clientKey)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			answered <- answer{}
			return
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		answered <- answer{res.StatusCode, res.Header.Get("Retry-After")}
	}()
	<-arrived
	switched := make(chan func(), 1)
	go func() {
		end, err := gate.WaitExclusive(context.Background())
		if err == nil {
			switched <- end
		}
	}()
	for !gate.Status().Waiting {
		time.Sleep(time.Millisecond)
	}
	if n, err := gate.Force(); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	select {
	case end := <-switched:
		end()
	case <-time.After(2 * time.Second):
		t.Fatal("switch still waits for the aborted request")
	}
	if a := <-answered; a.status != http.StatusServiceUnavailable || a.retry == "" {
		t.Fatalf("client got %+v, want retryable 503", a)
	}
	if tr := waitFinished(t, st); tr.ErrorKind != "account_switch" {
		t.Fatal(tr.ErrorKind)
	}
}
