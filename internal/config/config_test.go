package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, name, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func noEnv(string) (string, bool) { return "", false }

const minimal = `
listen: "100.101.102.103:8443"
upstream:
  api_key_file: "/etc/tollgate/cpa.key"
db:
  path: "/var/lib/tollgate/tollgate.db"
`

func TestLoadDefaultsAndValues(t *testing.T) {
	p := writeFile(t, "tollgate.yaml", minimal, 0o600)
	c, err := Load(p, noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "100.101.102.103:8443" || c.Upstream.URL != "http://127.0.0.1:8317" ||
		c.DB.Path != "/var/lib/tollgate/tollgate.db" || c.Log.Level != "info" ||
		c.Admin.TokenFile != "/var/lib/tollgate/admin.token" {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestLoadIgnoresADRSectionsOwnedByLaterIssues(t *testing.T) {
	p := writeFile(t, "t.yaml", minimal+"management:\n  url: http://127.0.0.1:8317/v8/management\ncrypto:\n  key_file: /x\nadmin:\n  token_file: /y\n", 0o600)
	c, err := Load(p, noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if c.Admin.TokenFile != "/y" {
		t.Fatalf("admin.token_file = %q", c.Admin.TokenFile)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	p := writeFile(t, "t.yaml", minimal, 0o600)
	env := map[string]string{
		"TOLLGATE_LISTEN":                "100.64.0.9:9000",
		"TOLLGATE_PUBLISHED":             "100.64.0.10:8443",
		"TOLLGATE_UPSTREAM_URL":          "http://127.0.0.1:9999",
		"TOLLGATE_UPSTREAM_API_KEY_FILE": "/k",
		"TOLLGATE_DB_PATH":               "/d.db",
		"TOLLGATE_LOG_LEVEL":             "debug",
		"TOLLGATE_ADMIN_TOKEN_FILE":      "/a.token",
	}
	c, err := Load(p, func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "100.64.0.9:9000" || c.Published != "100.64.0.10:8443" || c.Upstream.URL != "http://127.0.0.1:9999" ||
		c.Upstream.APIKeyFile != "/k" || c.DB.Path != "/d.db" || c.Log.Level != "debug" ||
		c.Admin.TokenFile != "/a.token" {
		t.Fatalf("env not applied: %+v", c)
	}
}

func TestLoadWithoutFileUsesEnvOnly(t *testing.T) {
	env := map[string]string{"TOLLGATE_LISTEN": "100.64.0.9:9000", "TOLLGATE_UPSTREAM_API_KEY_FILE": "/k", "TOLLGATE_DB_PATH": "/d"}
	c, err := Load("", func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "100.64.0.9:9000" {
		t.Fatalf("%+v", c)
	}
}

func TestValidate(t *testing.T) {
	base := func() *Config {
		return &Config{Listen: "100.101.102.103:8443", Upstream: Upstream{URL: "http://127.0.0.1:8317", APIKeyFile: "/k"}, DB: DB{Path: "/d"}, Log: Log{Level: "info"}}
	}
	tests := []struct {
		name    string
		mut     func(*Config)
		dev     bool
		wantErr string
	}{
		{"ok", func(*Config) {}, false, ""},
		{"missing listen", func(c *Config) { c.Listen = "" }, false, "listen"},
		{"missing key file", func(c *Config) { c.Upstream.APIKeyFile = "" }, false, "api_key_file"},
		{"missing db path", func(c *Config) { c.DB.Path = "" }, false, "db.path"},
		{"upstream not loopback", func(c *Config) { c.Upstream.URL = "http://100.101.102.103:8317" }, false, "loopback"},
		{"bad log level", func(c *Config) { c.Log.Level = "loud" }, false, "log.level"},
		{"wildcard listen", func(c *Config) { c.Listen = "0.0.0.0:8443" }, false, "wildcard"},
		{"loopback listen needs dev flag", func(c *Config) { c.Listen = "127.0.0.1:8443" }, false, "Tailnet"},
		{"loopback listen with dev flag", func(c *Config) { c.Listen = "127.0.0.1:8443" }, true, ""},
		{"published container listen", func(c *Config) { c.Listen, c.Published = "172.18.0.2:8443", "100.64.0.1:8443" }, false, ""},
		{"published outside Tailnet", func(c *Config) { c.Listen, c.Published = "172.18.0.2:8443", "192.168.1.5:8443" }, true, "published"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			tt.mut(c)
			err := c.Validate(tt.dev)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCheckListen(t *testing.T) {
	tests := []struct {
		addr    string
		dev     bool
		wantErr string
	}{
		{"100.64.0.1:8443", false, ""},
		{"100.127.255.254:8443", false, ""},
		{"[fd7a:115c:a1e0::1]:8443", false, ""},
		{"100.128.0.1:8443", false, "Tailnet"},
		{"100.63.255.255:8443", false, "Tailnet"},
		{"192.168.1.5:8443", false, "Tailnet"},
		{"[fd7a:115c:a1e1::1]:8443", false, "Tailnet"},
		{"127.0.0.1:8443", false, "Tailnet"},
		{"[::1]:8443", false, "Tailnet"},
		{"0.0.0.0:8443", false, "wildcard"},
		{"[::]:8443", false, "wildcard"},
		{":8443", false, "wildcard"},
		{"", false, "wildcard"},
		{"0.0.0.0:8443", true, "wildcard"},
		{"[::]:8443", true, "wildcard"},
		{"192.168.1.5:8443", true, ""},
		{"127.0.0.1:0", true, ""},
		{"localhost:8443", false, "IP"},
		{"localhost:8443", true, "IP"},
		{"100.64.0.1", false, "host:port"},
		{"[::ffff:0.0.0.0]:8443", true, "wildcard"},
		{"[::ffff:100.64.0.1]:8443", false, ""},
		{"[fe80::1%en0]:8443", false, "Tailnet"},
		{"[fd7a:115c:a1e0::1%tailscale0]:8443", false, "Tailnet"}, // zones never match a prefix
	}
	for _, tt := range tests {
		err := CheckListen(tt.addr, tt.dev)
		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("CheckListen(%q, %v): %v", tt.addr, tt.dev, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("CheckListen(%q, %v) = %v, want error containing %q", tt.addr, tt.dev, err, tt.wantErr)
		}
	}
}

func TestCheckBinding(t *testing.T) {
	tests := []struct {
		listen, published string
		dev               bool
		wantErr           string
	}{
		{"100.64.0.1:8443", "", false, ""},                   // no published: CheckListen
		{"127.0.0.1:8443", "", false, "Tailnet"},             // no published: CheckListen
		{"172.18.0.2:8443", "100.64.0.1:8443", false, ""},    // Docker bridge
		{"10.0.0.5:8443", "100.64.0.1:8443", false, ""},      // other private range
		{"192.168.215.2:8443", "100.64.0.1:8443", false, ""}, // OrbStack bridge
		{"[fd00::2]:8443", "[fd7a:115c:a1e0::1]:8443", false, ""},
		{"172.18.0.2:8443", "192.168.1.5:8443", false, "published"},
		{"172.18.0.2:8443", "192.168.1.5:8443", true, "published"}, // dev flag never lifts published
		{"172.18.0.2:8443", "127.0.0.1:8443", true, "published"},
		{"172.18.0.2:8443", "0.0.0.0:8443", false, "wildcard"},
		{"172.18.0.2:8443", "tollgate:8443", false, "published"},
		{"127.0.0.1:8443", "100.64.0.1:8443", false, "private"},
		{"0.0.0.0:8443", "100.64.0.1:8443", false, "wildcard"},
		{":8443", "100.64.0.1:8443", false, "wildcard"},
		{"8.8.8.8:8443", "100.64.0.1:8443", false, "private"},
		{"100.64.0.2:8443", "100.64.0.1:8443", false, "private"},
		{"localhost:8443", "100.64.0.1:8443", false, "IP"},
		{"[fd00::2%eth0]:8443", "100.64.0.1:8443", false, "private"},
	}
	for _, tt := range tests {
		err := CheckBinding(tt.listen, tt.published, tt.dev)
		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("CheckBinding(%q, %q, %v): %v", tt.listen, tt.published, tt.dev, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("CheckBinding(%q, %q, %v) = %v, want error containing %q", tt.listen, tt.published, tt.dev, err, tt.wantErr)
		}
	}
}

func TestReadSecretFile(t *testing.T) {
	ok := writeFile(t, "k", "  secret-value\n", 0o600)
	if v, err := ReadSecretFile(ok); err != nil || v != "secret-value" {
		t.Fatalf("got %q, %v", v, err)
	}
	loose := writeFile(t, "k", "secret-value", 0o644)
	_, err := ReadSecretFile(loose)
	if err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("group/world readable file must be rejected, got %v", err)
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatal("error leaks secret")
	}
	empty := writeFile(t, "k", "\n", 0o600)
	if _, err := ReadSecretFile(empty); err == nil {
		t.Fatal("empty secret must be rejected")
	}
	if _, err := ReadSecretFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file must be rejected")
	}
}

func TestManagementKeyFileFromYAMLAndEnv(t *testing.T) {
	p := writeFile(t, "t.yaml", minimal+"", 0o600)
	c, err := Load(p, noEnv)
	if err != nil || c.Upstream.ManagementKeyFile != "" {
		t.Fatalf("default: %q, %v", c.Upstream.ManagementKeyFile, err)
	}
	p = writeFile(t, "t2.yaml", strings.Replace(minimal, "upstream:\n", "upstream:\n  management_key_file: \"/y.key\"\n", 1), 0o600)
	if c, err = Load(p, noEnv); err != nil || c.Upstream.ManagementKeyFile != "/y.key" {
		t.Fatalf("yaml: %q, %v", c.Upstream.ManagementKeyFile, err)
	}
	env := func(k string) (string, bool) { return "/env.key", k == "TOLLGATE_UPSTREAM_MANAGEMENT_KEY_FILE" }
	if c, err = Load(p, env); err != nil || c.Upstream.ManagementKeyFile != "/env.key" {
		t.Fatalf("env: %q, %v", c.Upstream.ManagementKeyFile, err)
	}
}
