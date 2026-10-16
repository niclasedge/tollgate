package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/alert"
)

const notifyToken = "fake-slack-token-for-tests"

// runNotify starts the subcommand against a fake Slack and returns once the
// spool is empty (or fails after a timeout).
func runNotify(t *testing.T, token string) (texts []string, log string) {
	t.Helper()
	var mu sync.Mutex
	slack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m struct{ Channel, Text string }
		_ = json.NewDecoder(r.Body).Decode(&m)
		mu.Lock()
		texts = append(texts, m.Channel+":"+m.Text)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer slack.Close()

	dir := t.TempDir()
	spool := filepath.Join(dir, "spool")
	if err := os.Mkdir(spool, 0o700); err != nil {
		t.Fatal(err)
	}
	s := &alert.Spool{Dir: spool}
	_ = s.Write("eins")
	_ = s.Write("zwei")
	tokenFile, channelFile := filepath.Join(dir, "token"), filepath.Join(dir, "channel")
	_ = os.WriteFile(tokenFile, []byte(token+"\n"), 0o600)
	_ = os.WriteFile(channelFile, []byte("C42\n"), 0o600)

	ctx, cancel := context.WithCancel(context.Background())
	var stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- notify(ctx, []string{"--spool", spool, "--token-file", tokenFile, "--channel-file", channelFile,
			"--interval", "20ms", "--slack-url", slack.URL}, &stderr, nil)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if entries, _ := os.ReadDir(spool); len(entries) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("spool not drained")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	return texts, stderr.String()
}

func TestNotifyDeliversSpool(t *testing.T) {
	texts, log := runNotify(t, notifyToken)
	if strings.Join(texts, ",") != "C42:eins,C42:zwei" {
		t.Fatalf("sent = %v", texts)
	}
	if strings.Contains(log, notifyToken) {
		t.Fatal("log contains the token")
	}
}

func TestNotifyWithoutTokenDiscards(t *testing.T) {
	texts, _ := runNotify(t, "")
	if len(texts) != 0 {
		t.Fatalf("sent without token: %v", texts)
	}
}

func TestNotifyNeedsSpool(t *testing.T) {
	if err := notify(context.Background(), nil, &bytes.Buffer{}, nil); err == nil {
		t.Fatal("notify without --spool succeeded")
	}
}
