package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite" // also registers the "sqlite" driver (pure Go, no cgo)
)

// Options configures Open.
type Options struct {
	// Now supplies timestamps for created_at, rotated_at and session
	// activity. Defaults to time.Now. Tests inject a fixed clock.
	Now func() time.Time
}

// Store is the SQLite-backed persistence layer. It uses one writer connection
// (WAL, ADR-0003) and a separate read-only pool.
type Store struct {
	w   *sql.DB
	r   *sql.DB
	now func() time.Time
}

// Open opens (and creates) the database at path and applies pending
// migrations. Only file paths are supported: an in-memory database would be
// private to each connection and break the writer/reader split.
func Open(ctx context.Context, path string, opts Options) (*Store, error) {
	if path == "" || strings.Contains(path, ":memory:") || strings.Contains(path, "mode=memory") {
		return nil, errors.New("store: a file path is required, in-memory databases are not supported")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("store: resolve path: %w", err)
	}
	migs, err := loadEmbeddedMigrations()
	if err != nil {
		return nil, err
	}

	if err := ensureFile(abs); err != nil {
		return nil, err
	}
	w, err := sql.Open("sqlite", dsn(abs, false))
	if err != nil {
		return nil, fmt.Errorf("store: open writer: %w", err)
	}
	w.SetMaxOpenConns(1)
	if err := pingRetryBusy(ctx, w); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("store: open %s: %w", filepath.Base(abs), err)
	}
	if err := migrate(ctx, w, migs); err != nil {
		_ = w.Close()
		return nil, err
	}

	r, err := sql.Open("sqlite", dsn(abs, true))
	if err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("store: open reader: %w", err)
	}
	r.SetMaxOpenConns(4)
	if err := r.PingContext(ctx); err != nil {
		_ = w.Close()
		_ = r.Close()
		return nil, fmt.Errorf("store: open reader: %w", err)
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Store{w: w, r: r, now: now}, nil
}

// pingRetryBusy opens the first connection. Switching a fresh file to WAL
// (journal_mode pragma) can fail with SQLITE_BUSY immediately, without
// consulting busy_timeout, when several openers race on the same new file.
// Retry with a short backoff until ctx ends or a deadline of 5s passes.
func pingRetryBusy(ctx context.Context, db *sql.DB) error {
	deadline := time.Now().Add(5 * time.Second)
	for delay := 5 * time.Millisecond; ; delay = min(delay*2, 100*time.Millisecond) {
		err := db.PingContext(ctx)
		if err == nil || !isBusy(err) || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func isBusy(err error) bool {
	var se *sqlite.Error
	if errors.As(err, &se) {
		code := se.Code() & 0xff      // primary result code
		return code == 5 || code == 6 // SQLITE_BUSY, SQLITE_LOCKED
	}
	// Errors from connection-time pragmas may arrive without the typed error.
	m := err.Error()
	return strings.Contains(m, "SQLITE_BUSY") || strings.Contains(m, "database is locked")
}

// ensureFile creates the database file with mode 0600 before SQLite does (it
// would use 0644) and tightens an existing file that is group/world accessible.
// SQLite gives new -wal and -shm files the mode of the main file, but sidecars
// left behind by an older 0644 installation keep theirs, so they are tightened
// too.
func ensureFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("store: create %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("store: stat %s: %w", filepath.Base(path), err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		if err := f.Chmod(0o600); err != nil {
			return fmt.Errorf("store: chmod %s: %w", filepath.Base(path), err)
		}
	}
	for _, side := range []string{path + "-wal", path + "-shm"} {
		fi, err := os.Lstat(side)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("store: stat %s: %w", filepath.Base(side), err)
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("store: %s is not a regular file", filepath.Base(side))
		}
		if fi.Mode().Perm()&0o077 != 0 {
			if err := os.Chmod(side, 0o600); err != nil {
				return fmt.Errorf("store: chmod %s: %w", filepath.Base(side), err)
			}
		}
	}
	return nil
}

func dsn(absPath string, readOnly bool) string {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)") // first: applies to the following pragmas, too
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if readOnly {
		q.Add("_pragma", "query_only(1)")
	} else {
		// BEGIN IMMEDIATE: a read-then-write transaction takes the write lock
		// up front and waits (busy_timeout) instead of failing with
		// SQLITE_BUSY when another process or Store holds the lock.
		q.Add("_txlock", "immediate")
	}
	u := url.URL{Scheme: "file", Path: absPath, RawQuery: q.Encode()}
	return u.String()
}

// Close closes both connection pools.
func (s *Store) Close() error {
	return errors.Join(s.r.Close(), s.w.Close())
}

func ms(t time.Time) int64 { return t.UnixMilli() }

// nowMS returns the store clock as UTC truncated to milliseconds, i.e. exactly
// what is persisted, so returned values equal what a later read yields.
func (s *Store) nowMS() time.Time { return fromMS(ms(s.now())) }

func fromMS(v int64) time.Time { return time.UnixMilli(v).UTC() }

// inTx runs fn in a write (BEGIN IMMEDIATE) transaction. fn must only use tx:
// the single writer connection is held, so calling back into the Store from
// fn deadlocks.
func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
