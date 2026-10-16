package accounts

import (
	"context"
	"errors"
	"sync"
)

// Gate protects the complete proxy stream. A reserved switch drains existing
// requests while new requests wait. The zero value is ready for use.
type Gate struct {
	mu        sync.Mutex
	running   int
	switching bool
	queued    bool
	changed   chan struct{}
	next      int
	cancels   map[int]context.CancelCauseFunc
}

// ErrForcedSwitch is the cancel cause of requests aborted by Force.
var ErrForcedSwitch = errors.New("Anfrage für sofortigen Account-Wechsel abgebrochen")

func (g *Gate) changeLocked() {
	if g.changed != nil {
		close(g.changed)
	}
	g.changed = make(chan struct{})
}
func (g *Gate) notificationLocked() <-chan struct{} {
	if g.changed == nil {
		g.changed = make(chan struct{})
	}
	return g.changed
}
func (g *Gate) requestDone(id int) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			g.running--
			cancel := g.cancels[id]
			delete(g.cancels, id)
			g.changeLocked()
			g.mu.Unlock()
			if cancel != nil {
				cancel(nil)
			}
		})
	}
}
func (g *Gate) switchDone() func() {
	var once sync.Once
	return func() { once.Do(func() { g.mu.Lock(); g.switching = false; g.changeLocked(); g.mu.Unlock() }) }
}

// Begin and Exclusive are nonblocking probes, also used for manual tests.
func (g *Gate) Begin() (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.switching || g.queued {
		return nil, false
	}
	g.running++
	g.changeLocked()
	return g.requestDone(0), true
}
func (g *Gate) Exclusive() (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.switching || g.queued || g.running != 0 {
		return nil, false
	}
	g.switching = true
	g.changeLocked()
	return g.switchDone(), true
}

// WaitBegin delays admission while a switch is reserved or in progress. The
// returned context is canceled with ErrForcedSwitch when Force aborts the
// request; Begin's probe slots cannot be aborted.
func (g *Gate) WaitBegin(ctx context.Context) (context.Context, func(), error) {
	for {
		g.mu.Lock()
		if err := ctx.Err(); err != nil {
			g.mu.Unlock()
			return nil, nil, err
		}
		if !g.switching && !g.queued {
			rctx, cancel := context.WithCancelCause(ctx)
			if g.cancels == nil {
				g.cancels = map[int]context.CancelCauseFunc{}
			}
			g.next++
			g.cancels[g.next] = cancel
			g.running++
			g.changeLocked()
			done := g.requestDone(g.next)
			g.mu.Unlock()
			return rctx, done, nil
		}
		changed := g.notificationLocked()
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-changed:
		}
	}
}

// Force aborts all admitted requests so a reserved switch runs at once. It
// returns the number of aborted requests. Without a reservation it changes
// nothing, so running requests are never aborted by accident.
func (g *Gate) Force() (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.queued {
		return 0, errors.New("Kein Account-Wechsel vorgemerkt")
	}
	for _, cancel := range g.cancels {
		cancel(ErrForcedSwitch)
	}
	return len(g.cancels), nil
}

// WaitExclusive reserves before draining, so incoming requests cannot starve a
// switch. Cancellation releases only this reservation without token mutation.
func (g *Gate) WaitExclusive(ctx context.Context) (func(), error) {
	g.mu.Lock()
	if err := ctx.Err(); err != nil {
		g.mu.Unlock()
		return nil, err
	}
	if g.queued {
		g.mu.Unlock()
		return nil, errors.New("Ein Account-Wechsel ist bereits vorgemerkt")
	}
	g.queued = true
	g.changeLocked()
	for g.running > 0 || g.switching {
		changed := g.notificationLocked()
		g.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-changed:
		}
		g.mu.Lock()
		if err := ctx.Err(); err != nil {
			g.queued = false
			g.changeLocked()
			g.mu.Unlock()
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		g.queued = false
		g.changeLocked()
		g.mu.Unlock()
		return nil, err
	}
	g.queued = false
	g.switching = true
	g.changeLocked()
	g.mu.Unlock()
	return g.switchDone(), nil
}
func (g *Gate) Status() SwitchStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	return SwitchStatus{Switching: g.switching || g.queued, Waiting: g.queued, RunningRequests: g.running}
}
