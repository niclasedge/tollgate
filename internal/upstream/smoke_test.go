package upstream

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSmokeRealCLIProxyAPI starts the real CLIProxyAPI binary with the
// rendered deploy template on 127.0.0.1 and checks health plus management
// auth. It is the smoke test from the ADR-0014 update procedure and only runs
// when TOLLGATE_CPA_BIN points at a cli-proxy-api binary built by `make cpa`.
func TestSmokeRealCLIProxyAPI(t *testing.T) {
	bin := os.Getenv("TOLLGATE_CPA_BIN")
	if bin == "" {
		t.Skip("TOLLGATE_CPA_BIN not set")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	authDir := filepath.Join(dir, "auth")
	if err := os.Mkdir(authDir, 0o700); err != nil {
		t.Fatal(err)
	}
	apiKey := strings.Repeat("a1B2", 12)
	mgmtKey := strings.Repeat("m9Z8", 12)
	apiFile, mgmtFile := filepath.Join(dir, "api"), filepath.Join(dir, "mgmt")
	must(t, os.WriteFile(apiFile, []byte(apiKey+"\n"), 0o600))
	must(t, os.WriteFile(mgmtFile, []byte(mgmtKey+"\n"), 0o600))

	cfg := filepath.Join(dir, "config.yaml")
	render := exec.Command("sh", filepath.Join(repo, "deploy/cliproxyapi/render-config.sh"),
		filepath.Join(repo, "deploy/cliproxyapi/config.yaml"), cfg, authDir)
	render.Env = append(os.Environ(), "TOLLGATE_API_KEY_FILE="+apiFile, "MANAGEMENT_SECRET_FILE="+mgmtFile)
	if out, err := render.CombinedOutput(); err != nil {
		t.Fatalf("render: %v: %s", err, out)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	raw, err := os.ReadFile(cfg)
	must(t, err)
	must(t, os.WriteFile(cfg, []byte(strings.Replace(string(raw), "port: 8317", fmt.Sprintf("port: %d", port), 1)), 0o600))

	cmd := exec.Command(bin, "-config", cfg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+dir)
	var logs strings.Builder
	cmd.Stdout, cmd.Stderr = &logs, &logs
	must(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	c, err := New(base)
	must(t, err)
	deadline := time.Now().Add(30 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = c.Health(ctx)
		cancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("health never became ok: %v\nlogs:\n%s", err, logs.String())
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Hard rule: only reachable via loopback, not via the host's other addresses.
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.IsLoopback() || ipn.IP.To4() == nil {
			continue
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ipn.IP.String(), fmt.Sprint(port)), time.Second)
		if err == nil {
			conn.Close()
			t.Errorf("CLIProxyAPI reachable on non-loopback address %s", ipn.IP)
		}
		t.Logf("dial %s:%d -> %v", ipn.IP, port, err)
	}

	get := func(key string) int {
		req, _ := http.NewRequest(http.MethodGet, base+"/v8/management/credentials", nil)
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := http.DefaultClient.Do(req)
		must(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		t.Logf("credentials key=%t -> %d %s", key != "", resp.StatusCode, strings.TrimSpace(string(b)))
		return resp.StatusCode
	}
	if got := get(""); got != http.StatusUnauthorized {
		t.Errorf("management without key = %d, want 401", got)
	}
	if got := get(apiKey); got != http.StatusUnauthorized {
		t.Errorf("management with the tollgate API key = %d, want 401", got)
	}
	if got := get(mgmtKey); got != http.StatusOK {
		t.Errorf("management with management key = %d, want 200", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
