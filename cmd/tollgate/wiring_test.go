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

// startServe runs serve in-process and returns the listen address and the
// directory holding db and admin token.
func startServe(t *testing.T, upstreamURL string) (addr, dir string) {
	t.Helper()
	conf := writeConf(t, "127.0.0.1:0", upstreamURL)
	dir = filepath.Dir(conf)
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
	addrCh := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			if strings.Contains(sc.Text(), "cpa-secret-key") {
				t.Error("secret in log")
			}
			var e struct {
				Msg  string `json:"msg"`
				Addr string `json:"addr"`
			}
			if json.Unmarshal(sc.Bytes(), &e) == nil && e.Msg == "tollgate: listening" {
				addrCh <- e.Addr
			}
		}
	}()
	select {
	case addr = <-addrCh:
	case <-time.After(10 * time.Second):
		t.Fatal("no listening log line")
	}
	return addr, dir
}

func call(t *testing.T, method, url, bearer, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestWiringFrontendAdminAPIAndProxy(t *testing.T) {
	var mu sync.Mutex
	var upstreamAuth []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") { // skip the startup health check
			mu.Lock()
			upstreamAuth = append(upstreamAuth, r.Header.Get("Authorization"))
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(up.Close)
	addr, dir := startServe(t, up.URL)
	base := "http://" + addr

	// Frontend: the embedded shell (or its placeholder) is served at "/".
	if code, body := call(t, http.MethodGet, base+"/", "", ""); code != 200 || !strings.Contains(string(body), "<html") {
		t.Fatalf("GET / = %d %q", code, body)
	}

	// The admin token was created next to the database with mode 0600.
	tokenFile := filepath.Join(dir, "admin.token")
	info, err := os.Stat(tokenFile)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("admin token file: %v, %v", info, err)
	}
	raw, _ := os.ReadFile(tokenFile)
	admin := strings.TrimSpace(string(raw))

	if code, _ := call(t, http.MethodGet, base+"/api/clients", "", ""); code != 401 {
		t.Fatalf("admin API without token = %d", code)
	}
	code, body := call(t, http.MethodPost, base+"/api/clients", admin, `{"name":"laptop"}`)
	if code != 201 {
		t.Fatalf("create client = %d %q", code, body)
	}
	var created auth.CreateClientResponse
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}

	// Admin token is not a client key, and the client key is not an admin token.
	if code, _ := call(t, http.MethodGet, base+"/v1/models", admin, ""); code != 401 {
		t.Fatalf("admin token as client key = %d", code)
	}
	if code, _ := call(t, http.MethodGet, base+"/api/clients", created.Key, ""); code != 401 {
		t.Fatalf("client key on admin API = %d", code)
	}

	// The client key passes; upstream sees the CPA key, never the client key.
	if code, body := call(t, http.MethodGet, base+"/v1/models", created.Key, ""); code != 200 {
		t.Fatalf("proxied request = %d %q", code, body)
	}
	mu.Lock()
	seen := append([]string(nil), upstreamAuth...)
	mu.Unlock()
	if len(seen) != 1 || seen[0] != "Bearer cpa-secret-key-zyxwvutsrqponmlk" {
		t.Fatalf("upstream Authorization = %q", seen)
	}

	// The dashboard API is wired behind the same admin token.
	if code, _ := call(t, http.MethodGet, base+"/api/traces", created.Key, ""); code != 401 {
		t.Fatalf("client key on dashboard API = %d", code)
	}
	code, body = call(t, http.MethodGet, base+"/api/traces", admin, "")
	var list struct {
		Traces []struct {
			ClientName string `json:"client_name"`
		} `json:"traces"`
	}
	if err := json.Unmarshal(body, &list); err != nil || code != 200 || len(list.Traces) != 1 ||
		list.Traces[0].ClientName != "laptop" {
		t.Fatalf("GET /api/traces = %d %s", code, body)
	}
	if code, body := call(t, http.MethodGet, base+"/api/status", admin, ""); code != 200 || !strings.Contains(string(body), `"tollgate":{"ok":true`) {
		t.Fatalf("GET /api/status = %d %s", code, body)
	}

	// Blocking takes effect for the very next request.
	if code, _ := call(t, http.MethodPost, base+"/api/clients/"+created.Client.ID+"/block", admin, ""); code != 200 {
		t.Fatalf("block = %d", code)
	}
	if code, _ := call(t, http.MethodGet, base+"/v1/models", created.Key, ""); code != 401 {
		t.Fatalf("blocked client = %d", code)
	}
}
