package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/niclasedge/tollgate/internal/upstream"
	"go.yaml.in/yaml/v3"
)

// Config is the bootstrap configuration (ADR-0008, level 1).
type Config struct {
	// Listen is a concrete Tailnet IP and port, e.g. "100.101.102.103:8443".
	Listen string `yaml:"listen"`
	// Redact switches the redact stage (#13); off by default.
	Redact Redact `yaml:"redact"`
	// Compress switches the compress stage (#19); off by default.
	Compress Compress `yaml:"compress"`
	// Published is the host address Docker publishes Listen on, e.g.
	// "100.101.102.103:8443" (ADR-0013). Optional; when set it must be a
	// Tailnet address and Listen a private container address.
	Published string   `yaml:"published"`
	Upstream  Upstream `yaml:"upstream"`
	DB        DB       `yaml:"db"`
	Admin     Admin    `yaml:"admin"`
	Crypto    Crypto   `yaml:"crypto"`
	Log       Log      `yaml:"log"`
	Alerts    Alerts   `yaml:"alerts"`
}

// Alerts configures the limit warnings and the automatic account switch
// (#122, ADR-0019).
type Alerts struct {
	// SpoolDir receives the messages for the notifier. Empty: no messages;
	// the automatic switch still works.
	SpoolDir string `yaml:"spool_dir"`
	// AutoSwitch switches to a free account when Anthropic rejects requests.
	// Default: false; the overview offers the switch as a button instead,
	// so the subscription can be adjusted first (#126).
	AutoSwitch bool `yaml:"auto_switch"`
	// FiveHourThreshold is the 5-hour utilization in percent (1..100) from
	// which every higher full point produces one warning. 0 or absent: 90.
	FiveHourThreshold int `yaml:"five_hour_threshold"`
}

// Crypto locates the persistent AES-256-GCM vault key.
type Crypto struct {
	KeyFile string `yaml:"key_file"`
}

// Admin configures access to the admin API and frontend.
type Admin struct {
	// TokenFile holds the admin token (mode 0600). A missing file is created
	// with a random token on first start. Default: admin.token next to db.path.
	TokenFile string `yaml:"token_file"`
}

// Upstream describes CLIProxyAPI.
type Upstream struct {
	// URL must point at loopback. Default: http://127.0.0.1:8317.
	URL string `yaml:"url"`
	// APIKeyFile holds the tollgate -> CLIProxyAPI key (mode 0600).
	APIKeyFile string `yaml:"api_key_file"`
	// ManagementKeyFile holds the CLIProxyAPI management key (mode 0600).
	// Optional: without it the dashboard cannot list the accounts.
	ManagementKeyFile string `yaml:"management_key_file"`
}

// DB locates the SQLite file.
type DB struct {
	Path string `yaml:"path"`
}

// Log configures logging.
type Log struct {
	// Level is debug, info, warn or error. Default: info.
	Level string `yaml:"level"`
}

// Redact configures the redact stage. Enabled is off by default until the
// return path (#14) translates placeholders back for the client; until then
// the client would see placeholders in answers and tool calls.
type Redact struct {
	Enabled bool `yaml:"enabled"`
}

// Compress configures the compress stage. Enabled is off by default: the
// first request after switching it on changes the history of running
// sessions once, which costs one prompt-cache miss per session.
type Compress struct {
	Enabled bool `yaml:"enabled"`
}

// DefaultUpstreamURL is the CLIProxyAPI address from the deploy template.
const DefaultUpstreamURL = "http://127.0.0.1:8317"

// envKeys maps every overridable value to its TOLLGATE_ variable (ADR-0008:
// "TOLLGATE_" plus the path in upper case with "_"). An explicit table keeps
// the mapping deterministic and greppable.
var envKeys = []struct {
	name string
	set  func(*Config, string)
}{
	{"TOLLGATE_LISTEN", func(c *Config, v string) { c.Listen = v }},
	{"TOLLGATE_PUBLISHED", func(c *Config, v string) { c.Published = v }},
	{"TOLLGATE_UPSTREAM_URL", func(c *Config, v string) { c.Upstream.URL = v }},
	{"TOLLGATE_UPSTREAM_API_KEY_FILE", func(c *Config, v string) { c.Upstream.APIKeyFile = v }},
	{"TOLLGATE_UPSTREAM_MANAGEMENT_KEY_FILE", func(c *Config, v string) { c.Upstream.ManagementKeyFile = v }},
	{"TOLLGATE_DB_PATH", func(c *Config, v string) { c.DB.Path = v }},
	{"TOLLGATE_ADMIN_TOKEN_FILE", func(c *Config, v string) { c.Admin.TokenFile = v }},
	{"TOLLGATE_CRYPTO_KEY_FILE", func(c *Config, v string) { c.Crypto.KeyFile = v }},
	{"TOLLGATE_LOG_LEVEL", func(c *Config, v string) { c.Log.Level = v }},
	{"TOLLGATE_ALERTS_SPOOL_DIR", func(c *Config, v string) { c.Alerts.SpoolDir = v }},
}

// Load reads the YAML file at path (empty path: no file), applies defaults and
// then environment overrides via getenv. Unknown sections are ignored because
// later issues own them (#10, #15); strict checking arrives with the full
// schema. Errors never contain file contents.
func Load(path string, getenv func(string) (string, bool)) (*Config, error) {
	c := &Config{Alerts: Alerts{FiveHourThreshold: 90}}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config: read %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, c); err != nil {
			return nil, errors.New("config: " + path + " is not valid YAML")
		}
	}
	for _, e := range envKeys {
		if v, ok := getenv(e.name); ok {
			e.set(c, v)
		}
	}
	// Booleans are parsed here instead of in envKeys so that a typo fails the
	// start instead of silently switching the stage off.
	if v, ok := getenv("TOLLGATE_REDACT_ENABLED"); ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil, errors.New("config: TOLLGATE_REDACT_ENABLED must be true or false")
		}
		c.Redact.Enabled = b
	}
	if v, ok := getenv("TOLLGATE_COMPRESS_ENABLED"); ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil, errors.New("config: TOLLGATE_COMPRESS_ENABLED must be true or false")
		}
		c.Compress.Enabled = b
	}
	if v, ok := getenv("TOLLGATE_ALERTS_AUTO_SWITCH"); ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil, errors.New("config: TOLLGATE_ALERTS_AUTO_SWITCH must be true or false")
		}
		c.Alerts.AutoSwitch = b
	}
	if v, ok := getenv("TOLLGATE_ALERTS_FIVE_HOUR_THRESHOLD"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return nil, errors.New("config: TOLLGATE_ALERTS_FIVE_HOUR_THRESHOLD must be a whole number")
		}
		c.Alerts.FiveHourThreshold = n
	}
	if c.Upstream.URL == "" {
		c.Upstream.URL = DefaultUpstreamURL
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Admin.TokenFile == "" && c.DB.Path != "" {
		c.Admin.TokenFile = filepath.Join(filepath.Dir(c.DB.Path), "admin.token")
	}
	if c.Crypto.KeyFile == "" && c.DB.Path != "" {
		c.Crypto.KeyFile = filepath.Join(filepath.Dir(c.DB.Path), "crypto.key")
	}
	return c, nil
}

// Validate checks required values and the startup rules of ADR-0008. With
// devAllowAny the Tailnet check is lifted, the wildcard check is not.
func (c *Config) Validate(devAllowAny bool) error {
	if c.Listen == "" {
		return errors.New("config: listen is required")
	}
	if err := CheckBinding(c.Listen, c.Published, devAllowAny); err != nil {
		return err
	}
	if c.Upstream.APIKeyFile == "" {
		return errors.New("config: upstream.api_key_file is required")
	}
	if c.DB.Path == "" {
		return errors.New("config: db.path is required")
	}
	if _, err := upstream.New(c.Upstream.URL); err != nil {
		return fmt.Errorf("config: upstream.url: %w", err)
	}
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: log.level %q must be debug, info, warn or error", c.Log.Level)
	}
	if c.Alerts.FiveHourThreshold < 0 || c.Alerts.FiveHourThreshold > 100 {
		return errors.New("config: alerts.five_hour_threshold must be between 1 and 100 (0: default 90)")
	}
	return nil
}

var tailnetPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("fd7a:115c:a1e0::/48"),
}

// CheckListen enforces the binding rule: a concrete IP (no wildcard, no host
// name) inside the Tailscale ranges 100.64.0.0/10 or fd7a:115c:a1e0::/48.
// devAllowAny lifts only the range check (tests, local development).
func CheckListen(addr string, devAllowAny bool) error {
	if addr == "" {
		return errors.New("config: listen is empty (wildcard address refused)")
	}
	ap, err := netip.ParseAddrPort(addr)
	if err != nil {
		host := addr
		if i := strings.LastIndexByte(addr, ':'); i >= 0 {
			host = addr[:i]
		}
		if host == "" {
			return fmt.Errorf("config: listen %q has no host (wildcard address refused)", addr)
		}
		if !strings.Contains(addr, ":") {
			return fmt.Errorf("config: listen %q must be host:port with a concrete IP", addr)
		}
		return fmt.Errorf("config: listen %q must be a concrete IP and port, not a host name", addr)
	}
	ip := ap.Addr().Unmap()
	if ip.IsUnspecified() {
		return fmt.Errorf("config: listen %q is a wildcard address; bind to the Tailnet IP", addr)
	}
	if devAllowAny {
		return nil
	}
	for _, p := range tailnetPrefixes {
		if p.Contains(ip) {
			return nil
		}
	}
	return fmt.Errorf("config: listen %q is outside the Tailnet ranges 100.64.0.0/10 and fd7a:115c:a1e0::/48", addr)
}

// CheckBinding applies the binding rule to a listen address and, if set, the
// host address Docker publishes it on (ADR-0013). Without published it is
// CheckListen. With published, that address must pass CheckListen without the
// dev flag, and listen must be a concrete private container address: the only
// way in from outside is then the published Tailnet socket.
func CheckBinding(listen, published string, devAllowAny bool) error {
	if published == "" {
		return CheckListen(listen, devAllowAny)
	}
	if err := CheckListen(published, false); err != nil {
		return fmt.Errorf("published: %w", err)
	}
	if err := CheckListen(listen, true); err != nil {
		return err
	}
	ip := netip.MustParseAddrPort(listen).Addr().Unmap()
	if !ip.IsPrivate() || ip.Zone() != "" {
		return fmt.Errorf("config: listen %q must be a private container address when published is set", listen)
	}
	return nil
}

// ReadSecretFile returns the trimmed content of a secret file. The file must
// not be accessible to group or others (mode 0600, ADR-0008) and must not be
// empty. Errors never contain the secret.
func ReadSecretFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("config: secret file %s: %w", path, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("config: secret file %s must have mode 0600 (is %04o)", path, info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("config: secret file %s: %w", path, err)
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "", fmt.Errorf("config: secret file %s is empty", path)
	}
	return v, nil
}
