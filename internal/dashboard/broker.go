package dashboard

import (
	"sync"
)

// subscriberBuffer is how many events a subscriber may lag behind before it
// starts to lose events.
const subscriberBuffer = 64

// Broker fans trace events out to the SSE subscribers. Publish never blocks:
// a subscriber whose buffer is full loses the event, so a slow or hung
// frontend cannot delay the proxy.
type Broker struct {
	mu   sync.RWMutex
	subs map[chan TraceEvent]struct{}
}

// NewBroker returns an empty Broker.
func NewBroker() *Broker {
	return &Broker{subs: make(map[chan TraceEvent]struct{})}
}

// Subscribe registers a subscriber. The returned function unregisters it and
// must be called when the subscriber is done.
func (b *Broker) Subscribe() (<-chan TraceEvent, func()) {
	ch := make(chan TraceEvent, subscriberBuffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}

// Subscribers returns the number of registered subscribers.
func (b *Broker) Subscribers() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// Publish hands ev to every subscriber that has room for it.
func (b *Broker) Publish(ev TraceEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default: // slow subscriber: drop the event
		}
	}
}
