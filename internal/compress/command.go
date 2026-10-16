package compress

import (
	"regexp"
	"strings"
)

var (
	envAssign = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// Redirections that do not change what ends up in the tool result's
	// shape: stderr into stdout, or a stream into /dev/null.
	redirect = regexp.MustCompile(`^[0-9]*(>&[0-9]|>>?/dev/null)$`)
)

// Segments without output of their own; they are dropped from a chain.
var setupCommands = map[string]bool{"cd": true, "pushd": true, "export": true, "set": true}

// Prefix words that only change how a command runs.
var wrappers = map[string]bool{"time": true, "env": true}

// Options with an argument that only pick a directory or config, per tool.
var dirOptions = map[string]map[string]bool{
	"git":  {"-C": true, "-c": true},
	"make": {"-C": true},
	"pnpm": {"-C": true, "--dir": true},
}

// Normalize reduces a Bash command to the one command whose output the tool
// result holds (ADR-0007): setup segments (cd, export, set) of a chain,
// leading env assignments, time/env, harmless redirections, git --no-pager
// and directory options are dropped, whitespace is collapsed. It reports
// false when the output may mix several commands or has an unknown shape:
// pipes, more than one remaining segment, ||, command substitution, heredocs
// or unbalanced quotes. Quoted text is never split.
func Normalize(cmd string) (string, bool) {
	segs, ok := split(cmd)
	if !ok {
		return "", false
	}
	var kept []string
	for _, s := range segs {
		f := fields(s)
		if len(f) == 0 || setupCommands[f[0]] {
			continue
		}
		kept = append(kept, strings.Join(clean(f), " "))
	}
	if len(kept) != 1 || kept[0] == "" {
		return "", false
	}
	return kept[0], true
}

// split cuts cmd at &&, ; and line breaks outside quotes. It fails on pipes,
// ||, command substitution, heredocs and unbalanced quotes.
func split(cmd string) ([]string, bool) {
	var segs []string
	var quote byte
	start := 0
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			}
			continue
		case c == '\\':
			i++
			continue
		case c == '`', c == '$' && i+1 < len(cmd) && cmd[i+1] == '(':
			return nil, false
		case quote == '"':
			if c == '"' {
				quote = 0
			}
			continue
		case c == '\'' || c == '"':
			quote = c
		case c == '|':
			return nil, false
		case c == '<' && i+1 < len(cmd) && cmd[i+1] == '<':
			return nil, false
		case c == '&' && i+1 < len(cmd) && cmd[i+1] == '&':
			segs = append(segs, cmd[start:i])
			i++
			start = i + 1
		case c == ';' || c == '\n':
			segs = append(segs, cmd[start:i])
			start = i + 1
		}
	}
	if quote != 0 {
		return nil, false
	}
	return append(segs, cmd[start:]), true
}

// fields splits a segment at whitespace outside quotes; quotes stay in the
// words.
func fields(s string) []string {
	var out []string
	var quote byte
	start := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
			continue
		case c == '\\':
			if start < 0 {
				start = i
			}
			i++
			continue
		case c == ' ' || c == '\t':
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
			continue
		case c == '\'' || c == '"':
			quote = c
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// clean drops leading env assignments and wrappers, redirections and the
// directory options of known tools.
func clean(f []string) []string {
	for len(f) > 0 && (envAssign.MatchString(f[0]) || wrappers[f[0]]) {
		f = f[1:]
	}
	if len(f) == 0 {
		return nil
	}
	out := []string{f[0]}
	dirs := dirOptions[f[0]]
	for i := 1; i < len(f); i++ {
		switch {
		case redirect.MatchString(f[i]):
		case f[0] == "git" && f[i] == "--no-pager":
		case dirs[f[i]] && i+1 < len(f):
			i++
		default:
			out = append(out, f[i])
		}
	}
	return out
}

// matchPrefix reports whether the normalized command starts with the words
// of prefix.
func matchPrefix(cmd, prefix string) bool {
	return cmd == prefix || strings.HasPrefix(cmd, prefix+" ")
}
