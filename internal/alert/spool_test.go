package alert

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSpoolWriteDrainInOrder(t *testing.T) {
	dir := t.TempDir()
	now := t0
	s := &Spool{Dir: dir, Now: func() time.Time { return now }}
	for _, text := range []string{"eins", "zwei", "drei"} {
		if err := s.Write(text); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp") {
			t.Fatalf("temporary file left: %s", e.Name())
		}
		if info, _ := e.Info(); info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, want 0600", e.Name(), info.Mode().Perm())
		}
	}
	var got []string
	n, err := s.Drain(context.Background(), func(_ context.Context, text string) error {
		got = append(got, text)
		return nil
	})
	if err != nil || n != 3 {
		t.Fatalf("Drain = %d, %v", n, err)
	}
	if strings.Join(got, ",") != "eins,zwei,drei" {
		t.Fatalf("order = %v", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("files left: %d", len(entries))
	}
}

func TestSpoolDrainStopsOnError(t *testing.T) {
	s := &Spool{Dir: t.TempDir(), Now: func() time.Time { return t0 }}
	_ = s.Write("eins")
	_ = s.Write("zwei")
	calls := 0
	_, err := s.Drain(context.Background(), func(context.Context, string) error {
		calls++
		return errors.New("slack: channel_not_found")
	})
	if err == nil || calls != 1 {
		t.Fatalf("Drain err = %v, calls = %d", err, calls)
	}
	if entries, _ := os.ReadDir(s.Dir); len(entries) != 2 {
		t.Fatalf("files = %d, want 2 kept for retry", len(entries))
	}
}

func TestSpoolDropsOldBrokenAndDisabled(t *testing.T) {
	now := t0
	s := &Spool{Dir: t.TempDir(), Now: func() time.Time { return now }}
	_ = s.Write("alt")
	if err := os.WriteFile(filepath.Join(s.Dir, "00000000000000000001-000000.json"), []byte("{kaputt"), 0o600); err != nil {
		t.Fatal(err)
	}
	now = t0.Add(25 * time.Hour)
	_ = s.Write("neu")
	var got []string
	if _, err := s.Drain(context.Background(), func(_ context.Context, text string) error {
		got = append(got, text)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "neu" {
		t.Fatalf("sent = %v, want only the fresh message", got)
	}

	// send == nil: Slack is off, files are discarded without sending.
	_ = s.Write("aus")
	if n, err := s.Drain(context.Background(), nil); err != nil || n != 0 {
		t.Fatalf("Drain(nil) = %d, %v", n, err)
	}
	if entries, _ := os.ReadDir(s.Dir); len(entries) != 0 {
		t.Fatalf("files left: %d", len(entries))
	}
}

func TestSpoolFull(t *testing.T) {
	s := &Spool{Dir: t.TempDir(), MaxFiles: 2, Now: func() time.Time { return t0 }}
	_ = s.Write("1")
	_ = s.Write("2")
	if err := s.Write("3"); err == nil {
		t.Fatal("Write into a full spool succeeded")
	}
}
