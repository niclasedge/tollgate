package accounts

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGateKeepsStreamsAndSwitchExclusive(t *testing.T) {
	g := new(Gate)
	done, ok := g.Begin()
	if !ok {
		t.Fatal("request rejected")
	}
	if _, ok := g.Exclusive(); ok {
		t.Fatal("switch during stream")
	}
	done()
	end, ok := g.Exclusive()
	if !ok {
		t.Fatal("switch rejected")
	}
	if _, ok := g.Begin(); ok {
		t.Fatal("new request during switch")
	}
	end()
	done, ok = g.Begin()
	if !ok {
		t.Fatal("gate not reopened")
	}
	done()
}

func TestGateQueuedSwitchHoldsIncomingRequestsAndHandlesCanceledWaiter(t *testing.T) {
	g := new(Gate)
	done, _ := g.Begin()
	exclusive := make(chan func(), 1)
	go func() {
		end, err := g.WaitExclusive(context.Background())
		if err == nil {
			exclusive <- end
		}
	}()
	waitSwitch(t, g)
	if _, err := g.WaitExclusive(context.Background()); err == nil {
		t.Fatal("second switch replaced reservation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := g.WaitBegin(ctx); err == nil {
		t.Fatal("canceled request admitted")
	}
	if got := g.Status(); got.RunningRequests != 1 || !got.Switching {
		t.Fatal(got)
	}
	admitted := make(chan func(), 1)
	go func() {
		_, end, err := g.WaitBegin(context.Background())
		if err == nil {
			admitted <- end
		}
	}()
	select {
	case <-exclusive:
		t.Fatal("switched before stream ended")
	default:
	}
	done()
	end := <-exclusive
	select {
	case <-admitted:
		t.Fatal("admitted before switch finished")
	default:
	}
	end()
	release := <-admitted
	release()
	if got := g.Status(); got.Switching || got.RunningRequests != 0 {
		t.Fatal(got)
	}
}

func TestGateQueuedSwitchAlsoWaitsForExistingExclusiveOperation(t *testing.T) {
	g := new(Gate)
	finish, ok := g.Exclusive()
	if !ok {
		t.Fatal("existing test")
	}
	next := make(chan func(), 1)
	go func() {
		end, err := g.WaitExclusive(context.Background())
		if err == nil {
			next <- end
		}
	}()
	deadline := time.Now().Add(time.Second)
	for !g.Status().Waiting && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !g.Status().Waiting {
		t.Fatal("switch not queued behind test")
	}
	select {
	case <-next:
		t.Fatal("switch ran during test")
	default:
	}
	finish()
	end := <-next
	end()
	if g.Status().Switching {
		t.Fatal("gate remained closed")
	}
}

func TestGateForceAbortsAdmittedRequestsOnlyForReservedSwitch(t *testing.T) {
	g := new(Gate)
	ctx, done, err := g.WaitBegin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if _, err := g.Force(); err == nil || ctx.Err() != nil {
		t.Fatal("force without reservation aborted a request")
	}
	exclusive := make(chan func(), 1)
	go func() {
		end, err := g.WaitExclusive(context.Background())
		if err == nil {
			exclusive <- end
		}
	}()
	waitSwitch(t, g)
	if n, err := g.Force(); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	<-ctx.Done()
	if !errors.Is(context.Cause(ctx), ErrForcedSwitch) {
		t.Fatal(context.Cause(ctx))
	}
	done()
	select {
	case end := <-exclusive:
		end()
	case <-time.After(time.Second):
		t.Fatal("switch did not start after forced abort")
	}
}
