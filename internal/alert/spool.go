package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultMaxFiles = 500
	defaultMaxAge   = 24 * time.Hour
)

// Spool is a directory of pending messages. tollgate writes, the notifier
// drains (ADR-0019). Names sort in write order.
type Spool struct {
	Dir      string
	MaxFiles int           // default 500; further messages are dropped
	MaxAge   time.Duration // default 24 h; older messages are dropped
	Now      func() time.Time
	seq      atomic.Uint64
}

type message struct {
	Text    string    `json:"text"`
	Created time.Time `json:"created"`
}

func (s *Spool) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// pending lists the message files in name order.
func (s *Spool) pending() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

// Write stores one message atomically (temporary file, then rename).
func (s *Spool) Write(text string) error {
	names, err := s.pending()
	if err != nil {
		return fmt.Errorf("alert spool: %w", err)
	}
	if limit := orDefault(s.MaxFiles, defaultMaxFiles); len(names) >= limit {
		return errors.New("alert spool: full, is the notifier running?")
	}
	now := s.now()
	data, err := json.Marshal(message{Text: text, Created: now.UTC()})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("alert spool: %w", err)
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		name := fmt.Sprintf("%020d-%06d.json", now.UnixNano(), s.seq.Add(1)%1_000_000)
		err = os.Rename(tmp, filepath.Join(s.Dir, name))
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("alert spool: %w", err)
	}
	return nil
}

func orDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// Drain sends the pending messages in order and deletes each after success.
// It stops at the first send error and keeps that message for the next run.
// Old and unreadable files are dropped. send == nil discards every message.
func (s *Spool) Drain(ctx context.Context, send func(ctx context.Context, text string) error) (int, error) {
	names, err := s.pending()
	if err != nil {
		return 0, fmt.Errorf("alert spool: %w", err)
	}
	maxAge := s.MaxAge
	if maxAge <= 0 {
		maxAge = defaultMaxAge
	}
	now, sent := s.now(), 0
	for _, name := range names {
		if ctx.Err() != nil {
			return sent, ctx.Err()
		}
		path := filepath.Join(s.Dir, name)
		var m message
		data, err := os.ReadFile(path)
		if err == nil {
			err = json.Unmarshal(data, &m)
		}
		if err != nil || send == nil || now.Sub(m.Created) > maxAge {
			_ = os.Remove(path)
			continue
		}
		if err := send(ctx, m.Text); err != nil {
			return sent, err
		}
		if err := os.Remove(path); err != nil {
			return sent, fmt.Errorf("alert spool: %w", err)
		}
		sent++
	}
	return sent, nil
}
