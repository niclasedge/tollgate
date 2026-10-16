package deploy

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestComposeNotifier checks ADR-0019: only the notifier holds the Slack
// secrets and leaves the shared namespace; tollgate writes to the spool.
func TestComposeNotifier(t *testing.T) {
	type service struct {
		User        string            `yaml:"user"`
		NetworkMode string            `yaml:"network_mode"`
		Entrypoint  []string          `yaml:"entrypoint"`
		Environment map[string]string `yaml:"environment"`
		Secrets     []string          `yaml:"secrets"`
		Volumes     []string          `yaml:"volumes"`
		Ports       []string          `yaml:"ports"`
		CapAdd      []string          `yaml:"cap_add"`
	}
	var doc struct {
		Services map[string]service `yaml:"services"`
		Volumes  map[string]any     `yaml:"volumes"`
		Secrets  map[string]any     `yaml:"secrets"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, "../docker-compose.yml")), &doc); err != nil {
		t.Fatal(err)
	}
	n, ok := doc.Services["notifier"]
	if !ok {
		t.Fatal("service notifier missing")
	}
	if n.User != "10002:10002" || n.NetworkMode != "" || len(n.Ports) > 0 || len(n.CapAdd) > 0 {
		t.Errorf("notifier user=%q network_mode=%q ports=%v cap_add=%v", n.User, n.NetworkMode, n.Ports, n.CapAdd)
	}
	if len(n.Entrypoint) < 2 || n.Entrypoint[1] != "notify" || !slices.Contains(n.Entrypoint, "--spool=/alerts") {
		t.Errorf("notifier entrypoint = %v", n.Entrypoint)
	}
	if !slices.Equal(n.Secrets, []string{"slack_bot_token", "slack_channel"}) {
		t.Errorf("notifier secrets = %v", n.Secrets)
	}
	for name, s := range doc.Services {
		if name != "notifier" && slices.ContainsFunc(s.Secrets, func(v string) bool { return strings.HasPrefix(v, "slack_") }) {
			t.Errorf("%s must not get the Slack secrets", name)
		}
	}
	tg := doc.Services["tollgate"]
	if tg.Environment["TOLLGATE_ALERTS_SPOOL_DIR"] != "/alerts" {
		t.Errorf("tollgate TOLLGATE_ALERTS_SPOOL_DIR = %q", tg.Environment["TOLLGATE_ALERTS_SPOOL_DIR"])
	}
	for name, s := range map[string]service{"tollgate": tg, "notifier": n} {
		if !slices.Contains(s.Volumes, "tollgate-alerts:/alerts") {
			t.Errorf("%s volumes = %v, want tollgate-alerts:/alerts", name, s.Volumes)
		}
	}
	if _, ok := doc.Volumes["tollgate-alerts"]; !ok {
		t.Error("volume tollgate-alerts missing")
	}
	for _, s := range []string{"slack_bot_token", "slack_channel"} {
		if _, ok := doc.Secrets[s]; !ok {
			t.Errorf("secret %s missing", s)
		}
	}
	// The named volume takes owner and mode of the image directory.
	if docker := readFile(t, "tollgate/Dockerfile"); !strings.Contains(docker, "chown tollgate:tollgate /data /run/tollgate /alerts") {
		t.Error("image must create /alerts for the tollgate user")
	}
}
