package compress

import (
	"strings"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

// DefaultMinBytes is the size gate of ADR-0007: smaller outputs stay as they
// are.
const DefaultMinBytes = 2048

// Rule picks a strategy for Bash commands that start with one of Match
// (whole words, after Normalize).
type Rule struct {
	Name     string
	Match    []string
	Strategy string
	Params   strat.Params
	// Builtin marks the rules of defaultRules, for the compress view (#21).
	Builtin bool
}

// Config is the compress section of one configuration version.
type Config struct {
	MinBytes int
	// Rules are tried in order; the first match wins.
	Rules []Rule
}

// defaultRules is the built-in rule set. It follows the token analysis of
// #18 (docs/token-analyse.md); configured rules come first and can switch a
// command off with the strategy passthrough.
var defaultRules = []Rule{
	{Name: "go_test", Match: []string{"go test"}, Strategy: "test_log"},
	{Name: "cargo_test", Match: []string{"cargo test", "cargo nextest"}, Strategy: "test_log"},
	{Name: "pytest", Match: []string{"pytest", "python -m pytest", "python3 -m pytest", "uv run pytest"}, Strategy: "test_log"},
	{Name: "js_test", Match: []string{"pnpm test", "pnpm run test", "pnpm vitest", "pnpm exec vitest", "pnpm jest",
		"npm test", "npm run test", "yarn test", "vitest", "jest"}, Strategy: "test_log"},
	{Name: "make_test", Match: []string{"make test", "just test"}, Strategy: "test_log"},
	{Name: "git_diff", Match: []string{"git diff", "git show", "git log -p", "git log --patch", "git stash show -p"}, Strategy: "diff"},
	// Not find: with -exec it prints arbitrary file contents.
	{Name: "search", Match: []string{"grep", "egrep", "fgrep", "rg", "git grep"}, Strategy: "search"},
	{Name: "js_install", Match: []string{"pnpm install", "pnpm i", "pnpm add", "npm install", "npm i", "npm ci",
		"yarn install", "yarn add", "bun install"}, Strategy: "build_log"},
	{Name: "build", Match: []string{"cargo build", "cargo check", "cargo clippy", "go build", "go vet", "docker build",
		"docker buildx build", "docker compose build", "pip install", "uv pip install", "uv sync", "make"}, Strategy: "build_log"},
}

// Default is the built-in configuration.
func Default() *Config { return WithDefaults(nil, nil) }

// WithDefaults builds the configuration of a version: minBytes (nil: the
// default), the configured rules, then the built-in ones.
func WithDefaults(minBytes *int, rules []Rule) *Config {
	c := &Config{MinBytes: DefaultMinBytes}
	if minBytes != nil {
		c.MinBytes = *minBytes
	}
	c.Rules = append(c.Rules, rules...)
	for _, r := range defaultRules {
		r.Builtin = true
		c.Rules = append(c.Rules, r)
	}
	return c
}

// Match returns the first rule for a command, or false when none applies or
// the command is not a single command (see Normalize).
func (c *Config) Match(cmd string) (Rule, bool) {
	n, ok := Normalize(cmd)
	if !ok {
		return Rule{}, false
	}
	for _, r := range c.Rules {
		for _, m := range r.Match {
			if matchPrefix(n, strings.Join(strings.Fields(m), " ")) {
				return r, true
			}
		}
	}
	return Rule{}, false
}
