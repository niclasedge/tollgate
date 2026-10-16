// Package alert warns before the 5-hour subscription limit, switches the
// account when Anthropic rejects requests and delivers the messages through a
// spool directory to Slack (ADR-0019).
package alert

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/meter"
)

const (
	// DefaultCooldown separates two switch attempts, so a series of
	// rejections produces one attempt and one message.
	DefaultCooldown = 10 * time.Minute
	// DefaultThreshold is the 5-hour utilization in percent of the first warning.
	DefaultThreshold = 90
	queueSize        = 16
	listTimeout      = 30 * time.Second
	switchTimeout    = 2 * time.Minute
)

// Accounts is the part of accounts.Manager the watcher needs.
type Accounts interface {
	List(ctx context.Context) ([]accounts.Account, error)
	Usage(ctx context.Context, id string) (accounts.UsageSnapshot, error)
	Activate(ctx context.Context, id string) (accounts.Activation, error)
}

type Options struct {
	Accounts Accounts
	// Notify delivers one message, e.g. Spool.Write. Nil: no messages.
	Notify     func(text string) error
	AutoSwitch bool
	// Threshold is the 5-hour utilization in percent from which every higher
	// full point produces one warning.
	Threshold int
	Cooldown  time.Duration
	Now       func() time.Time
	Logger    *slog.Logger
}

type jobKind int

const (
	jobWarn jobKind = iota
	jobReject
)

type job struct {
	kind    jobKind
	account string
	pct     int
	reset   *time.Time
	window  string
}

type accountState struct {
	reset time.Time // 5-hour reset of the last warning
	pct   int       // point of the last warning
	near  bool      // last observation rejected or a window at the threshold
}

// Watcher observes the limit headers of every response. Observe never
// blocks; one worker (Run) handles warnings and switches in order.
type Watcher struct {
	o    Options
	jobs chan job

	mu         sync.Mutex
	state      map[string]*accountState
	lastReject time.Time
	handling   bool
}

func NewWatcher(o Options) *Watcher {
	if o.Threshold <= 0 {
		o.Threshold = DefaultThreshold
	}
	if o.Cooldown <= 0 {
		o.Cooldown = DefaultCooldown
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.DiscardHandler)
	}
	return &Watcher{o: o, jobs: make(chan job, queueSize), state: map[string]*accountState{}}
}

func rejected(l meter.Limits) bool {
	return l.Status == "rejected" || l.FiveHour.Status == "rejected" || l.SevenDay.Status == "rejected"
}

func full(w meter.Window) bool { return w.Utilization != nil && *w.Utilization >= 1 }

// point is the full percentage point; the epsilon keeps 0.29 at 29.
func point(share float64) int { return int(math.Floor(share*100 + 1e-9)) }

// account returns the state of id; w.mu is held.
func (w *Watcher) account(id string) *accountState {
	st := w.state[id]
	if st == nil {
		st = &accountState{}
		w.state[id] = st
	}
	return st
}

// Observe is called with the limits of every upstream response of account
// id (accounts.Manager.OnLimits). It must stay cheap: it only queues.
func (w *Watcher) Observe(id string, l meter.Limits) {
	if id == "" {
		return
	}
	now := w.o.Now()
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.account(id)
	switch {
	case rejected(l):
		st.near = true
		window, reset := "Limit", l.Reset
		switch {
		case l.FiveHour.Status == "rejected" || full(l.FiveHour):
			window, reset = "5-h-Limit", l.FiveHour.Reset
		case l.SevenDay.Status == "rejected" || full(l.SevenDay):
			window, reset = "Wochenlimit", l.SevenDay.Reset
		}
		w.reject(job{kind: jobReject, account: id, window: window, reset: reset}, now)
	default:
		st.near = w.atThreshold(l.FiveHour) || w.atThreshold(l.SevenDay)
		if l.FiveHour.Utilization == nil {
			return
		}
		p := point(*l.FiveHour.Utilization)
		if p < w.o.Threshold {
			return
		}
		var reset time.Time
		if l.FiveHour.Reset != nil {
			reset = *l.FiveHour.Reset
		}
		if reset.Equal(st.reset) && p <= st.pct {
			return
		}
		st.reset, st.pct = reset, p
		w.enqueue(job{kind: jobWarn, account: id, pct: p, reset: l.FiveHour.Reset})
	}
}

func (w *Watcher) atThreshold(win meter.Window) bool {
	return win.Utilization != nil && point(*win.Utilization) >= w.o.Threshold
}

// ObserveStatus is called with the final HTTP status of every traced request
// of account id. CLIProxyAPI turns Anthropic's 429 into its own error without
// the limit headers and answers 429 itself while it cools the account down.
// So a 429 counts as a rejection when the last observation of the account was
// at the warning threshold; without that history it is a short rate limit.
func (w *Watcher) ObserveStatus(id string, status int) {
	if id == "" || status != http.StatusTooManyRequests {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.account(id).near {
		w.reject(job{kind: jobReject, account: id, window: "Limit"}, w.o.Now())
	}
}

// reject queues one rejection per cooldown; w.mu is held.
func (w *Watcher) reject(j job, now time.Time) {
	if w.handling || (!w.lastReject.IsZero() && now.Sub(w.lastReject) < w.o.Cooldown) {
		return
	}
	w.lastReject, w.handling = now, true
	if !w.enqueue(j) {
		w.handling = false
	}
}

func (w *Watcher) enqueue(j job) bool {
	select {
	case w.jobs <- j:
		return true
	default:
		w.o.Logger.Warn("alert: queue full, message dropped", "account_id", j.account)
		return false
	}
}

// Run handles queued jobs until ctx is done.
func (w *Watcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-w.jobs:
			w.handle(ctx, j)
		}
	}
}

// drain handles every queued job synchronously (tests).
func (w *Watcher) drain(ctx context.Context) {
	for {
		select {
		case j := <-w.jobs:
			w.handle(ctx, j)
		default:
			return
		}
	}
}

func (w *Watcher) notify(text string) {
	if w.o.Notify == nil {
		return
	}
	if err := w.o.Notify(text); err != nil {
		w.o.Logger.Warn("alert: message not queued", "error", err.Error())
	}
}

func (w *Watcher) handle(ctx context.Context, j job) {
	if j.kind == jobReject {
		defer func() {
			w.mu.Lock()
			w.handling = false
			w.mu.Unlock()
		}()
	}
	now := w.o.Now()
	lctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	list, err := w.o.Accounts.List(lctx)
	if err != nil {
		w.o.Logger.Warn("alert: accounts not readable", "error", err.Error())
		return
	}
	name, active := j.account, ""
	for _, a := range list {
		if a.ID == j.account {
			name = a.Name
		}
		if a.Active {
			active = a.ID
		}
	}
	cands := pickCandidates(lctx, w.o.Accounts, list, j.account, now)
	if j.kind == jobWarn {
		w.notify("⚠️ 5-h-Limit von *" + name + "* bei " + itoa(j.pct) + " %" + resetText(j.reset, now) + ". " + alternative(cands))
		return
	}
	head := "*" + name + "* abgelehnt (" + j.window + "). "
	switch {
	case active != j.account:
		// Someone switched already; the rejection belongs to the old account.
		w.o.Logger.Info("alert: rejected account is no longer active", "account_id", j.account)
	case !w.o.AutoSwitch:
		w.notify("⛔ " + head + "Automatischer Wechsel ist aus. " + alternative(cands))
	case len(cands) == 0:
		w.notify("⛔ " + head + "Kein anderes Abo frei.")
	default:
		c := cands[0]
		sctx, cancel := context.WithTimeout(ctx, switchTimeout)
		defer cancel()
		if _, err := w.o.Accounts.Activate(sctx, c.account.ID); err != nil {
			w.o.Logger.Warn("alert: automatic switch failed", "from", j.account, "to", c.account.ID)
			w.notify("❌ " + head + "Wechsel auf *" + c.account.Name + "* fehlgeschlagen: " + err.Error())
			return
		}
		w.o.Logger.Info("alert: switched account", "from", j.account, "to", c.account.ID)
		w.notify("🔁 " + head + "Gewechselt auf *" + c.account.Name + "* (" + c.describe() + ").")
	}
}
