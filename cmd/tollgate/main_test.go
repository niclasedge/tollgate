package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The root module must never import CLIProxyAPI (ADR-0001, ADR-0014):
// tollgate talks to it over HTTP only.
func TestBinaryDoesNotDependOnCLIProxyAPI(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for pkg := range strings.FieldsSeq(string(out)) {
		if strings.Contains(pkg, "third_party/") || strings.Contains(pkg, "github.com/router-for-me/CLIProxyAPI") {
			t.Fatalf("binary depends on %s", pkg)
		}
	}
}

func writeConf(t *testing.T, listen, upstreamURL string) string {
	t.Helper()
	dir := t.TempDir()
	key := filepath.Join(dir, "cpa.key")
	if err := os.WriteFile(key, []byte("cpa-secret-key-zyxwvutsrqponmlk\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	conf := "listen: \"" + listen + "\"\nupstream:\n  url: \"" + upstreamURL + "\"\n  api_key_file: \"" + key + "\"\ndb:\n  path: \"" + filepath.Join(dir, "t.db") + "\"\n"
	p := filepath.Join(dir, "tollgate.yaml")
	if err := os.WriteFile(p, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func noEnv(string) (string, bool) { return "", false }

func TestRunRefusesUnsafeSetups(t *testing.T) {
	tests := []struct {
		name    string
		listen  string
		up      string
		args    []string
		wantMsg string
	}{
		{"wildcard", "0.0.0.0:8443", "http://127.0.0.1:8317", nil, "wildcard"},
		{"wildcard even with dev flag", "0.0.0.0:8443", "http://127.0.0.1:8317", []string{"--dev-allow-any-listen"}, "wildcard"},
		{"outside tailnet", "192.168.1.5:8443", "http://127.0.0.1:8317", nil, "Tailnet"},
		{"loopback needs dev flag", "127.0.0.1:8443", "http://127.0.0.1:8317", nil, "Tailnet"},
		{"upstream not loopback", "100.64.0.1:8443", "http://100.64.0.2:8317", nil, "loopback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			args := append([]string{"-config", writeConf(t, tt.listen, tt.up)}, tt.args...)
			err := serve(context.Background(), args, &stderr, noEnv)
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("serve = %v, want error containing %q", err, tt.wantMsg)
			}
			lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			var entry struct{ Level, Msg string }
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil || entry.Level != "ERROR" {
				t.Fatalf("last log line is not a JSON error: %q", lines[len(lines)-1])
			}
			if strings.Contains(stderr.String(), "cpa-secret-key") {
				t.Fatal("secret in log")
			}
		})
	}
}

func TestRunRequiresConfig(t *testing.T) {
	var stderr bytes.Buffer
	if err := serve(context.Background(), nil, &stderr, noEnv); err == nil {
		t.Fatal("expected an error without any configuration")
	}
}
