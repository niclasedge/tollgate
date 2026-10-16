// Package cliproxyapi holds only tests for the CLIProxyAPI container files.
package cliproxyapi

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// The CLIProxyAPI entrypoint limits egress of the shared network namespace to
// Anthropic before CLIProxyAPI starts (ADR-0017). These tests run the real
// script with stand-ins for iptables, setpriv and CLIProxyAPI on PATH.

const fakeLogger = "#!/bin/sh\necho \"$(basename \"$0\") $*\" >> \"$FAKE_LOG\"\n"

// fakeFailing logs like fakeLogger and exits 1 when the arguments contain sub.
func fakeFailing(sub string) string {
	return fakeLogger + "case \"$*\" in *'" + sub + "'*) exit 1 ;; esac\n"
}

const fakeSetpriv = `#!/bin/sh
opts=""
while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do opts="$opts $1"; shift; done
shift
echo "setpriv$opts --" >> "$FAKE_LOG"
exec "$@"
`

func ruleLines(cmd, cidr string) []string {
	lines := []string{
		cmd + " -w -S OUTPUT",
		cmd + " -w -P OUTPUT DROP",
		cmd + " -w -F OUTPUT",
		cmd + " -w -A OUTPUT -o lo -j ACCEPT",
		cmd + " -w -A OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT",
		cmd + " -w -A OUTPUT -p tcp -d " + cidr + " --dport 443 -j ACCEPT",
		cmd + " -w -A OUTPUT -p udp --dport 53 -m owner --uid-owner 0 -j ACCEPT",
		cmd + " -w -A OUTPUT -p tcp --dport 53 -m owner --uid-owner 0 -j ACCEPT",
	}
	if strings.HasPrefix(cmd, "ip6") {
		for _, t := range []string{"router-solicitation", "neighbour-solicitation", "neighbour-advertisement"} {
			lines = append(lines, cmd+" -w -A OUTPUT -p ipv6-icmp --icmpv6-type "+t+" -j ACCEPT")
		}
	}
	return lines
}

const setprivCall = "setpriv --reuid=cpa --regid=cpa --init-groups --no-new-privs --bounding-set=-all --inh-caps=-all --"

// runEntrypoint runs deploy/cliproxyapi/entrypoint.sh with the given fakes
// (name -> script) and returns the logged calls, the output and the exit error.
func runEntrypoint(t *testing.T, fakes map[string]string, args ...string) ([]string, string, error) {
	t.Helper()
	script, err := filepath.Abs("entrypoint.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("entrypoint missing: %v", err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	all := map[string]string{
		"setpriv":          fakeSetpriv,
		"render-config.sh": fakeLogger,
		"cli-proxy-api":    fakeLogger,
	}
	for k, v := range fakes {
		all[k] = v
	}
	for name, body := range all {
		if body == "" {
			continue // absent command
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	logPath := filepath.Join(dir, "calls.log")
	cmd := exec.Command("/bin/sh", append([]string{script}, args...)...)
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "FAKE_LOG=" + logPath}
	out, runErr := cmd.CombinedOutput()
	b, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var calls []string
	for l := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
		if l != "" {
			calls = append(calls, l)
		}
	}
	return calls, string(out), runErr
}

func nftFakes() map[string]string {
	return map[string]string{"iptables": fakeLogger, "ip6tables": fakeLogger}
}

func TestCPAEntrypointLocksEgressBeforeStart(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		cpa  string
	}{
		{"server", nil, "cli-proxy-api -config /run/cpa/config.yaml -local-model"},
		{"login", []string{"-claude-login", "-no-browser"}, "cli-proxy-api -config /run/cpa/config.yaml -local-model -claude-login -no-browser"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, out, err := runEntrypoint(t, nftFakes(), tc.args...)
			if err != nil {
				t.Fatalf("entrypoint failed: %v\n%s", err, out)
			}
			want := append(ruleLines("iptables", "160.79.104.0/23"), ruleLines("ip6tables", "2607:6bc0::/48")...)
			if len(calls) != len(want)+3 {
				t.Fatalf("calls:\n%s", strings.Join(calls, "\n"))
			}
			if !slices.Equal(calls[:len(want)], want) {
				t.Errorf("rules:\n got %q\nwant %q", calls[:len(want)], want)
			}
			if calls[len(want)] != setprivCall {
				t.Errorf("privileges not dropped before start: %q", calls[len(want)])
			}
			if got := calls[len(want)+1]; got != "render-config.sh /etc/cliproxyapi/config.template.yaml /run/cpa/config.yaml /data/auth" {
				t.Errorf("render call %q", got)
			}
			if got := calls[len(want)+2]; got != tc.cpa {
				t.Errorf("start call %q, want %q", got, tc.cpa)
			}
		})
	}
}

func TestCPAEntrypointFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fakes map[string]string
	}{
		{"ipv4 rule fails", map[string]string{"iptables": fakeFailing("--dport 443"), "ip6tables": fakeLogger}},
		{"ipv4 policy fails", map[string]string{"iptables": fakeFailing("-P OUTPUT"), "ip6tables": fakeLogger}},
		{"ipv6 rule fails", map[string]string{"iptables": fakeLogger, "ip6tables": fakeFailing("ipv6-icmp")}},
		{"no ipv4 backend", map[string]string{
			"iptables": fakeFailing("-S OUTPUT"), "iptables-legacy": fakeFailing("-S OUTPUT"), "ip6tables": fakeLogger,
		}},
		{"no ipv6 backend", map[string]string{"iptables": fakeLogger, "ip6tables": ""}},
		{"no iptables at all", map[string]string{"iptables": "", "ip6tables": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, out, err := runEntrypoint(t, tc.fakes)
			if err == nil {
				t.Fatalf("entrypoint succeeded; calls:\n%s", strings.Join(calls, "\n"))
			}
			if !strings.Contains(out, "cpa-entrypoint:") {
				t.Errorf("no error message: %q", out)
			}
			for _, c := range calls {
				if strings.HasPrefix(c, "setpriv") || strings.HasPrefix(c, "render-config.sh") || strings.HasPrefix(c, "cli-proxy-api") {
					t.Errorf("started despite failed egress rules: %q", c)
				}
			}
		})
	}
}

func TestCPAEntrypointLegacyBackend(t *testing.T) {
	calls, out, err := runEntrypoint(t, map[string]string{
		"iptables": fakeFailing("-S OUTPUT"), "iptables-legacy": fakeLogger, "ip6tables": fakeLogger,
	})
	if err != nil {
		t.Fatalf("entrypoint failed: %v\n%s", err, out)
	}
	var v4 []string
	for _, c := range calls {
		if strings.HasPrefix(c, "iptables-legacy ") {
			v4 = append(v4, c)
		} else if strings.HasPrefix(c, "iptables ") && !strings.Contains(c, "-S OUTPUT") {
			t.Errorf("nft backend used although unusable: %q", c)
		}
	}
	if want := ruleLines("iptables-legacy", "160.79.104.0/23"); !slices.Equal(v4, want) {
		t.Errorf("legacy rules:\n got %q\nwant %q", v4, want)
	}
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type composeService struct {
	CapAdd     []string `yaml:"cap_add"`
	Privileged bool     `yaml:"privileged"`
	Entrypoint any      `yaml:"entrypoint"`
	Command    []string `yaml:"command"`
	Build      any      `yaml:"build"`
}

func TestCPAComposeEgressCapabilities(t *testing.T) {
	var doc struct {
		Services map[string]composeService `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "docker-compose.yml")), &doc); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cliproxyapi", "cpa-login", "tollgate"} {
		if _, ok := doc.Services[name]; !ok {
			t.Fatalf("service %s missing", name)
		}
	}
	for name, s := range doc.Services {
		if s.Privileged {
			t.Errorf("%s must not be privileged", name)
		}
		switch name {
		case "cliproxyapi", "cpa-login":
			if !slices.Equal(s.CapAdd, []string{"NET_ADMIN"}) {
				t.Errorf("%s cap_add = %v, want [NET_ADMIN]", name, s.CapAdd)
			}
			if s.Entrypoint != nil {
				t.Errorf("%s overrides the entrypoint and skips the egress rules", name)
			}
		default:
			if len(s.CapAdd) > 0 {
				t.Errorf("%s must not get extra capabilities: %v", name, s.CapAdd)
			}
		}
	}
	if got := doc.Services["cpa-login"].Command; !slices.Equal(got, []string{"-claude-login", "-no-browser"}) {
		t.Errorf("cpa-login command = %v", got)
	}

	docker := readRepoFile(t, "deploy/cliproxyapi/Dockerfile")
	if !regexp.MustCompile(`apt-get install[^\n]*\biptables\b`).MatchString(docker) {
		t.Error("CPA image must install iptables")
	}
	if !strings.Contains(docker, `ENTRYPOINT ["/usr/local/bin/cpa-entrypoint"]`) ||
		!strings.Contains(docker, "COPY entrypoint.sh /usr/local/bin/cpa-entrypoint") {
		t.Error("CPA image must start through the egress entrypoint")
	}
	if regexp.MustCompile(`(?m)^USER `).MatchString(docker) {
		t.Error("CPA image must not set USER; the entrypoint drops privileges after the rules")
	}
}
