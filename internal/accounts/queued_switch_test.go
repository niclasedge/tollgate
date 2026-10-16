package accounts

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func waitSwitch(t *testing.T, g *Gate) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !g.Status().Switching && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !g.Status().Switching {
		t.Fatal("switch not reserved")
	}
}
func TestQueuedSwitchLetsAdmittedRequestResolveIDAndAutomaticallyActivates(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx := context.Background()
	old, _ := m.List(ctx)
	b, _ := m.Import(ctx, "Second", tokenB)
	done, _ := m.gate.Begin()
	result := make(chan error, 1)
	go func() { _, err := m.Activate(ctx, b.ID); result <- err }()
	waitSwitch(t, m.gate)
	resolved := make(chan string, 1)
	go func() { id, _ := m.ActiveID(ctx); resolved <- id }()
	select {
	case id := <-resolved:
		if id != old[0].ID {
			t.Fatal("wrong old account")
		}
	case <-time.After(time.Second):
		t.Fatal("waiting switch blocked manager ID read")
	}
	if !bytes.Equal(f.files["claude-a.json"], tokenA) {
		t.Fatal("token changed before drain")
	}
	next := make(chan struct{})
	go func() {
		_, end, err := m.gate.WaitBegin(ctx)
		if err == nil {
			end()
		}
		close(next)
	}()
	select {
	case <-next:
		t.Fatal("new request was not held")
	default:
	}
	done()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queued switch did not run")
	}
	<-next
	id, err := m.ActiveID(ctx)
	if err != nil || id != b.ID {
		t.Fatal(id, err)
	}
}
func TestCanceledQueuedSwitchPreservesOldAccountAndReopensGate(t *testing.T) {
	m, f, _ := managerEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	m.List(ctx)
	b, _ := m.Import(ctx, "Second", tokenB)
	done, _ := m.gate.Begin()
	result := make(chan error, 1)
	go func() { _, err := m.Activate(ctx, b.ID); result <- err }()
	waitSwitch(t, m.gate)
	cancel()
	if err := <-result; err == nil {
		t.Fatal("canceled wait activated account")
	}
	if m.gate.Status().Switching || !bytes.Equal(f.files["claude-a.json"], tokenA) {
		t.Fatal("cancellation changed token or left gate closed")
	}
	done()
	_, end, err := m.gate.WaitBegin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	end()
}

func TestForceSwitchAbortsRunningRequestAndActivatesAtOnce(t *testing.T) {
	m, _, _ := managerEnv(t)
	ctx := context.Background()
	m.List(ctx)
	b, _ := m.Import(ctx, "Second", tokenB)
	if _, err := m.ForceSwitch(); err == nil {
		t.Fatal("force without reserved switch")
	}
	reqCtx, done, err := m.gate.WaitBegin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() { <-reqCtx.Done(); done() }() // the proxy ends an aborted request
	result := make(chan error, 1)
	go func() { _, err := m.Activate(ctx, b.ID); result <- err }()
	waitSwitch(t, m.gate)
	if _, err := m.ForceSwitch(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("forced switch did not run")
	}
	if id, err := m.ActiveID(ctx); err != nil || id != b.ID {
		t.Fatal(id, err)
	}
}
