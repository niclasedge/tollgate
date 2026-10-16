package redact

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// knownDeviations are the deliberate differences between the Rust
// hook-redactor and this port, see docs/adr/0005-redactor-port.md,
// section "Paritätstest". A corpus case that differs from Rust must name one
// of them in its deviation section; a case that names one must differ.
var knownDeviations = map[string]string{
	// ADR-0005 Paritätstest, item 1.
	// The rest of a displaced generic finding becomes its own finding (#83,
	// ADR-0005 Nachtrag).
	"overlap": "Go resolves overlaps once and keeps the specific label, replacing the uncovered rest of a displaced generic finding separately; Rust lets later generic rules (generic_assign, auth_header) overwrite earlier results",
	// ADR-0005 Paritätstest, item 2.
	"connection-url": "Go replaces only the password of a connection URL and leaves URLs without password alone; Rust replaces the whole URL",
	// ADR-0005 Paritätstest, item 3.
	"prefix": "Go replaces only the secret group, so separators, spaces and quotes of the prefix stay; Rust templates rewrite them (e.g. 'TenantId: x' becomes 'TenantId=...')",
	// ADR-0005 Paritätstest, item 4.
	"unicode": "Go regexp has ASCII-only \\b, \\d and \\s; Rust uses Unicode classes",
}

// portedRustRules are the Rust rule names that the parity run keeps
// (ADR-0005, "Zuordnung zu den eingebauten Schaltern"). conn_string runs
// with its original Rust pattern; the difference is deviation
// "connection-url".
var portedRustRules = []string{
	"ps_secret_field", "ipv6", "anthropic_key", "openai_key", "aws_access_key", "github_token",
	"slack_token", "stripe_live", "jwt", "pem_private", "email", "ipv4", "conn_string",
	"azure_cred_guid", "cert_thumbprint", "client_secret", "auth_header", "bearer_token",
	"generic_assign",
}

// rustRuleRE extracts ("name", r"regex" or r#"regex"#, "replacement") tuples
// from defaults() in src/patterns.rs.
var rustRuleRE = regexp.MustCompile(`(?s)\(\s*"([a-z0-9_]+)",\s*(?:r#"(.*?)"#|r"(.*?)"),\s*"([^"]*)",?\s*\)`)

var normalizeRE = regexp.MustCompile(`<redacted(?::[^<>\s]*)?>`)

type rustRule struct {
	Name string `json:"name"`
	Re   string `json:"re"`
	Sub  string `json:"sub"`
}

// TestParityWithRust runs every corpus case through the Rust binary and the
// Go port and compares the normalised outputs (ADR-0005, Paritätstest).
//
// Neutralising the rules that are not ported: Rust runs with --no-defaults
// and --patterns pointing to a JSON file that holds exactly the ported rules,
// read from src/patterns.rs of the reference repo (regex and template
// unchanged, Rust order). That is "Rust without the non-ported rules"
// without bending the corpus around them, and the false-positive filters
// still apply because Rust selects them by rule name. --no-config, an empty
// working directory and non-existent REDACTOR_CONFIG_DIR and
// REDACTOR_IGNOREFILE_PATH keep .redactor.toml, keywords and patterns.json
// out.
func TestParityWithRust(t *testing.T) {
	repo := redactorRepo(t)
	bin := filepath.Join(repo, "target", "release", "redactor")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("no Rust redactor binary at %s (set TOLLGATE_REDACTOR_REPO): %v", bin, err)
	}
	src, err := os.ReadFile(filepath.Join(repo, "src", "patterns.rs"))
	if err != nil {
		t.Skipf("no Rust redactor sources in %s: %v", repo, err)
	}

	tmp := t.TempDir()
	patterns := filepath.Join(tmp, "ported-rules.json")
	writePortedRules(t, string(src), patterns)
	workdir := filepath.Join(tmp, "work")
	if err := os.Mkdir(workdir, 0o700); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"REDACTOR_CONFIG_DIR="+filepath.Join(tmp, "no-such-config-dir"),
		"REDACTOR_IGNOREFILE_PATH="+filepath.Join(tmp, "no-such-ignorefile"),
		"REDACTOR_LOG_DIR="+filepath.Join(tmp, "no-such-log-dir"),
	)

	r := New(corpusOptions())
	cases := loadCorpus(t)
	differing := 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(bin, "-t", "pattern", "--no-log", "--no-defaults", "--no-config", "--patterns", patterns, "-")
			cmd.Dir = workdir
			cmd.Env = env
			cmd.Stdin = strings.NewReader(c.input)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("rust redactor: %v\n%s", err, stderr.String())
			}
			rust := normalize(strings.TrimSuffix(string(out), "\n"))
			goOut := normalize(strings.TrimSuffix(r.Redact(c.input), "\n"))

			for _, d := range c.deviations {
				if _, ok := knownDeviations[d]; !ok {
					t.Errorf("unknown deviation %q (allowed: see knownDeviations, ADR-0005)", d)
				}
			}
			switch {
			case rust != goOut && len(c.deviations) == 0:
				t.Errorf("unexpected deviation from Rust\nrust: %q\n  go: %q", rust, goOut)
			case rust == goOut && len(c.deviations) > 0:
				t.Errorf("marked as deviation %q but Go and Rust agree: %q", c.deviations, goOut)
			case rust != goOut:
				differing++
				t.Logf("deliberate deviation %q\nrust: %q\n  go: %q", c.deviations, rust, goOut)
			}
		})
	}
	t.Logf("%d corpus cases compared, %d deliberate deviations", len(cases), differing)
}

// redactorRepo resolves TOLLGATE_REDACTOR_REPO (default ref/hook-redactor);
// relative paths are taken from the repository root.
func redactorRepo(t *testing.T) string {
	t.Helper()
	repo := os.Getenv("TOLLGATE_REDACTOR_REPO")
	if repo == "" {
		repo = filepath.Join("ref", "hook-redactor")
	}
	if !filepath.IsAbs(repo) {
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		repo = filepath.Join(root, repo)
	}
	if fi, err := os.Stat(repo); err != nil || !fi.IsDir() {
		t.Skipf("no Rust redactor repo at %s (set TOLLGATE_REDACTOR_REPO)", repo)
	}
	return repo
}

// writePortedRules extracts the ported rules from patterns.rs, keeps their
// Rust order and writes them in the --patterns JSON format.
func writePortedRules(t *testing.T, src, path string) {
	t.Helper()
	_, body, ok := strings.Cut(src, "pub fn defaults()")
	if !ok {
		t.Fatal("defaults() not found in patterns.rs")
	}
	body, _, _ = strings.Cut(body, "raw.iter()")
	var rules []rustRule
	var names []string
	for _, m := range rustRuleRE.FindAllStringSubmatch(body, -1) {
		names = append(names, m[1])
		if !slices.Contains(portedRustRules, m[1]) {
			continue
		}
		re := m[2]
		if re == "" {
			re = m[3]
		}
		rules = append(rules, rustRule{Name: m[1], Re: re, Sub: m[4]})
	}
	for _, n := range portedRustRules {
		if !slices.Contains(names, n) {
			t.Fatalf("rule %q not found in patterns.rs defaults() (found %q)", n, names)
		}
	}
	data, err := json.Marshal(rules)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func normalize(s string) string { return normalizeRE.ReplaceAllString(s, "<redacted>") }
