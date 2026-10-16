package main

import (
	"context"
	"log/slog"

	"github.com/niclasedge/tollgate/internal/accounts"
	"github.com/niclasedge/tollgate/internal/alert"
	"github.com/niclasedge/tollgate/internal/config"
	"github.com/niclasedge/tollgate/internal/dashboard"
)

// startAlerts starts the limit watcher (#122). It sees the limits of every
// response through the account manager and the final status of every
// request as a subscriber of the trace broker. Messages go to the spool for
// the notifier (ADR-0019).
func startAlerts(ctx context.Context, cfg config.Alerts, m *accounts.Manager, broker *dashboard.Broker, logger *slog.Logger) {
	var notify func(string) error
	if cfg.SpoolDir != "" {
		notify = (&alert.Spool{Dir: cfg.SpoolDir}).Write
	}
	w := alert.NewWatcher(alert.Options{
		Accounts: m, Notify: notify, AutoSwitch: cfg.AutoSwitch, Threshold: cfg.FiveHourThreshold, Logger: logger,
	})
	m.OnLimits = w.Observe
	events, unsubscribe := broker.Subscribe()
	go func() {
		defer unsubscribe()
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-events:
				if ev.Trace.Status != 0 {
					w.ObserveStatus(ev.Trace.AccountID, ev.Trace.Status)
				}
			}
		}
	}()
	go w.Run(ctx)
	logger.Info("tollgate: limit alerts on", "spool", cfg.SpoolDir != "", "auto_switch", cfg.AutoSwitch, "five_hour_threshold", cfg.FiveHourThreshold)
}
