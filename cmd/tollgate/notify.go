package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/niclasedge/tollgate/internal/alert"
)

const maxSecretBytes = 4 << 10

// readOptional returns the trimmed content of a small file, empty if it is
// missing. Errors never contain the content.
func readOptional(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if len(b) > maxSecretBytes {
		return "", fmt.Errorf("%s is larger than %d bytes", path, maxSecretBytes)
	}
	return strings.TrimSpace(string(b)), nil
}

// notify is the subcommand of the notifier container (ADR-0019): it drains
// the alert spool to Slack until ctx is done. Token and channel are read on
// every run so new secrets apply without a restart; both empty: Slack is off
// and messages are discarded.
func notify(ctx context.Context, args []string, stderr io.Writer, _ func(string) (string, bool)) error {
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	fail := func(err error) error {
		logger.Error("tollgate notify: start failed", "error", err.Error())
		return err
	}
	fs := flag.NewFlagSet("tollgate notify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	spoolDir := fs.String("spool", "", "alert spool directory")
	tokenFile := fs.String("token-file", "", "file with the Slack bot token")
	channelFile := fs.String("channel-file", "", "file with the Slack channel ID")
	interval := fs.Duration("interval", 10*time.Second, "poll interval")
	slackURL := fs.String("slack-url", alert.SlackPostMessage, "Slack chat.postMessage URL")
	if err := fs.Parse(args); err != nil {
		return fail(fmt.Errorf("arguments: %w", err))
	}
	if *spoolDir == "" || *interval <= 0 {
		return fail(errors.New("--spool and a positive --interval are required"))
	}
	spool := &alert.Spool{Dir: *spoolDir}
	slack := alert.Slack{URL: *slackURL, Client: &http.Client{Timeout: 15 * time.Second}}
	logger.Info("tollgate notify: started", "spool", *spoolDir, "interval", interval.String())
	enabled := -1 // unknown; logs only changes
	for {
		token, err := readOptional(*tokenFile)
		channel, cerr := readOptional(*channelFile)
		if err = errors.Join(err, cerr); err != nil {
			logger.Warn("tollgate notify: secrets not readable", "error", err.Error())
		} else {
			var send func(context.Context, string) error
			on := 0
			if token != "" && channel != "" {
				on = 1
				send = func(ctx context.Context, text string) error { return slack.Send(ctx, token, channel, text) }
			}
			if on != enabled {
				enabled = on
				logger.Info("tollgate notify: slack", "enabled", on == 1)
			}
			if n, err := spool.Drain(ctx, send); err != nil && ctx.Err() == nil {
				logger.Warn("tollgate notify: delivery failed, retrying", "sent", n, "error", err.Error())
			} else if n > 0 {
				logger.Info("tollgate notify: delivered", "sent", n)
			}
		}
		select {
		case <-ctx.Done():
			logger.Info("tollgate notify: stopped")
			return nil
		case <-time.After(*interval):
		}
	}
}
