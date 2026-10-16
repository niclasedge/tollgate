package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/auth"
)

// TestRedactSwitchWiring checks that redact.enabled hangs the redact stage
// into the pipeline, that the return path restores placeholders in the
// answer (#14) and that both stay out by default (#13).
func TestRedactSwitchWiring(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run(map[bool]string{false: "default off", true: "on"}[on], func(t *testing.T) {
			var mu sync.Mutex
			var bodies []string
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/messages" {
					b, _ := io.ReadAll(r.Body)
					mu.Lock()
					bodies = append(bodies, string(b))
					mu.Unlock()
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"sent to <redacted:email_1>"}]}`)
			}))
			t.Cleanup(up.Close)

			conf := writeConf(t, "127.0.0.1:0", up.URL)
			if on {
				f, err := os.OpenFile(conf, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = f.WriteString("redact:\n  enabled: true\n")
				f.Close()
			}
			addr, warned := serveConf(t, conf)
			if warned != on {
				t.Errorf("start warning = %v, want %v", warned, on)
			}
			base := "http://" + addr
			raw, _ := os.ReadFile(filepath.Join(filepath.Dir(conf), "admin.token"))
			code, body := call(t, http.MethodPost, base+"/api/clients", strings.TrimSpace(string(raw)), `{"name":"laptop"}`)
			var created auth.CreateClientResponse
			if code != 201 || json.Unmarshal(body, &created) != nil {
				t.Fatalf("create client = %d %s", code, body)
			}
			msg := `{"model":"claude-test","messages":[{"role":"user","content":"mail jane.roe@mailhost.de"}]}`
			code, answer := call(t, http.MethodPost, base+"/v1/messages", created.Key, msg)
			if code != 200 {
				t.Fatalf("POST /v1/messages = %d %s", code, answer)
			}
			// With the stage on, the return path (#14) restores the value for the
			// client; with it off the upstream text passes unchanged.
			wantAnswer := "sent to <redacted:email_1>"
			if on {
				wantAnswer = "sent to jane.roe@mailhost.de"
			}
			if !strings.Contains(string(answer), wantAnswer) {
				t.Errorf("client answer = %s, want %q", answer, wantAnswer)
			}
			mu.Lock()
			defer mu.Unlock()
			want := msg
			if on {
				want = strings.Replace(msg, "jane.roe@mailhost.de", "<redacted:email_1>", 1)
			}
			if len(bodies) != 1 || bodies[0] != want {
				t.Fatalf("upstream body = %q, want %q", bodies, want)
			}
		})
	}
}

// serveConf runs serve with the given config file and reports the listen
// address and whether the redact warning was logged.
func serveConf(t *testing.T, conf string) (addr string, warned bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, []string{"-config", conf, "--dev-allow-any-listen"}, pw, noEnv)
		_ = pw.Close()
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("serve: %v", err)
		}
	})
	type line struct {
		addr   string
		warned bool
	}
	ch := make(chan line, 1)
	go func() {
		w := false
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			var e struct {
				Msg  string `json:"msg"`
				Addr string `json:"addr"`
			}
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue
			}
			if strings.Contains(e.Msg, "redact stage is on") {
				w = true
			}
			if e.Msg == "tollgate: listening" {
				ch <- line{e.Addr, w}
			}
		}
	}()
	select {
	case l := <-ch:
		return l.addr, l.warned
	case <-time.After(10 * time.Second):
		t.Fatal("no listening log line")
	}
	return "", false
}
