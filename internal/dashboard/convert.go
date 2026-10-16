package dashboard

import (
	"context"
	"time"

	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/perf"
	"github.com/niclasedge/tollgate/internal/store"
)

// timeFormat is RFC 3339 in UTC with millisecond precision, as the store
// keeps timestamps.
const timeFormat = "2006-01-02T15:04:05.000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeFormat) }

// summary converts a stored trace into its API form. Usage and cost stay nil
// until the trace is finished and reported usage.
func summary(t store.Trace, clientName string, saved int64) TraceSummary {
	s := TraceSummary{
		ID: t.ID, ClientID: t.ClientID, AccountID: t.AccountID, ClientName: clientName, StartedAt: formatTime(t.StartedAt),
		Endpoint: t.Endpoint, Model: t.Model, Stream: t.Stream, Status: t.Status,
		DurationMs: t.DurationMicros / 1000, EstimatedInputTokens: t.EstimatedInputTokens,
		SavedTokens: saved, ErrorKind: t.ErrorKind,
	}
	if t.Usage != nil {
		u := toUsage(*t.Usage)
		s.Usage = &u
		if c, ok := meter.Cost(t.Model, *t.Usage); ok {
			s.CostNanoUSD = &c
		}
	}
	var stages int64
	for _, st := range t.Stages {
		stages += st.DurationMicros
	}
	setPerf(&s, t, stages)
	return s
}

// setPerf fills the performance fields (#87, #116). stageMicros is the sum
// of the stage durations (the list reads it without the stages).
func setPerf(s *TraceSummary, t store.Trace, stageMicros int64) {
	s.TTFTMs, s.OutputTPS, s.TollgateMs, s.Perf, s.TPS = nil, nil, nil, nil, nil
	if t.TTFTMicros != nil {
		v := msOf(*t.TTFTMicros)
		s.TTFTMs = &v
	}
	if t.Usage != nil && t.GenStartMicros != nil {
		if tps, ok := perf.OutputTPS(t.Usage.Output, t.DurationMicros, *t.GenStartMicros, true, t.Stream); ok {
			s.OutputTPS = &tps
		}
	}
	if t.Status != 0 {
		ret := int64(0)
		if t.ReturnMicros != nil {
			ret = *t.ReturnMicros
		}
		v := msOf(perf.TollgateMicros([]int64{stageMicros}, ret))
		s.TollgateMs = &v
	}
	if len(t.Perf) > 0 {
		tp := &TracePerf{}
		for _, p := range t.Perf {
			m := &PerfMetric{Verdict: p.Verdict, Class: p.Class, Samples: p.Samples, ValueMs: msOf(p.ValueMicros),
				MedianMs: msPtr(p.MedianMicros), P90Ms: msPtr(p.P90Micros)}
			switch p.Metric {
			case perf.MetricTTFT:
				tp.TTFT = m
			case perf.MetricDuration:
				tp.Duration = m
			}
			tp.Slow = tp.Slow || p.Verdict == string(perf.Slow)
		}
		s.Perf = tp
	}
	if t.TPS != nil {
		s.TPS = &TPSJudgement{Verdict: t.TPS.Verdict, Class: t.TPS.Class, Samples: t.TPS.Samples, TPS: tpsOf(t.TPS.TPSMilli),
			P10: tpsPtr(t.TPS.P10Milli), Median: tpsPtr(t.TPS.MedianMilli), P90: tpsPtr(t.TPS.P90Milli),
			DeviationPermille: t.TPS.DeviationPermille}
	}
}

// msOf converts microseconds to milliseconds with one decimal, rounded half
// up in integers so equal inputs give equal output.
func msOf(us int64) float64 { return float64((us+50)/100) / 10 }

func msPtr(us *int64) *float64 {
	if us == nil {
		return nil
	}
	v := msOf(*us)
	return &v
}

// tpsOf converts milli-tokens/s to tokens/s with one decimal.
func tpsOf(milli int64) float64 { return float64((milli+50)/100) / 10 }

func tpsPtr(milli *int64) *float64 {
	if milli == nil {
		return nil
	}
	v := tpsOf(*milli)
	return &v
}

func toUsage(u store.Usage) Usage {
	return Usage{Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheCreation: u.CacheCreation}
}

// savedTokens is the estimate before the first stage minus after the last.
func savedTokens(stages []store.Stage) int64 {
	if len(stages) == 0 {
		return 0
	}
	return stages[0].TokensIn - stages[len(stages)-1].TokensOut
}

// ClientLister is the part of the store the Publisher needs.
type ClientLister interface {
	ListClients(ctx context.Context) ([]store.Client, error)
}

const (
	nameLookupTimeout = 2 * time.Second
	// publisherQueue is how many traces may wait for the publisher goroutine;
	// further ones are dropped like events of a slow subscriber.
	publisherQueue = 256
)

// Publisher turns traces observed by the proxy into broker events. Observe
// only enqueues; a single goroutine resolves client names (store lookups) and
// publishes in order, so a slow or failing store never delays a request.
type Publisher struct {
	broker  *Broker
	clients ClientLister
	queue   chan store.Trace
	stop    chan struct{}
	done    chan struct{}
	names   map[string]string // owned by the goroutine
}

// NewPublisher starts a Publisher that publishes to b and resolves client
// names through clients. Call Close to stop it.
func NewPublisher(b *Broker, clients ClientLister) *Publisher {
	p := &Publisher{
		broker: b, clients: clients, names: make(map[string]string),
		queue: make(chan store.Trace, publisherQueue), stop: make(chan struct{}), done: make(chan struct{}),
	}
	go p.run()
	return p
}

// Close stops the goroutine; traces still queued are dropped.
func (p *Publisher) Close() {
	close(p.stop)
	<-p.done
}

// Observe queues t for publishing. It never blocks and does nothing while
// nobody listens. It fits server.Options.TraceObserver.
func (p *Publisher) Observe(t store.Trace) {
	if p.broker.Subscribers() == 0 {
		return
	}
	select {
	case p.queue <- t:
	default: // overloaded: drop
	}
}

func (p *Publisher) run() {
	defer close(p.done)
	for {
		select {
		case <-p.stop:
			return
		case t := <-p.queue:
			p.broker.Publish(TraceEvent{Trace: summary(t, p.clientName(t.ClientID), savedTokens(t.Stages))})
		}
	}
}

// clientName returns the name of a client. Names never change, so a miss
// reloads the client list and hits are served from memory. On a store error
// the name stays empty for this event only.
func (p *Publisher) clientName(id string) string {
	if n, ok := p.names[id]; ok {
		return n
	}
	ctx, cancel := context.WithTimeout(context.Background(), nameLookupTimeout)
	defer cancel()
	cs, err := p.clients.ListClients(ctx)
	if err != nil {
		return ""
	}
	for _, c := range cs {
		p.names[c.ID] = c.Name
	}
	return p.names[id]
}
