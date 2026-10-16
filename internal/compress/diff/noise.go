// DiffNoise: drop lockfile hunks and whitespace-only hunks.
//
// Source: Headroom crates/headroom-core/src/transforms/pipeline/offloads/diff_noise.rs
// (is_lockfile, body_is_whitespace_only, strip_ws) and the lockfile list of
// crates/headroom-core/config/pipeline.toml. Headroom (Apache-2.0, Copyright
// 2025 Headroom Contributors), Stand 0947a28, portiert und verändert.
//
// Deviations: no CCR store; the summary names the files instead. The
// whitespace test is per hunk (Headroom: per file), and added or removed
// blank lines count as whitespace-only (Headroom documents this but its
// pairing rejects it). The lockfile list is extended.

package diff

import "strings"

// lockfileNames are matched case-sensitively as a path suffix at a path
// segment boundary, so "Cargo.lock" matches "crates/x/Cargo.lock" but not
// "MyCargo.lock". The first nine are Headroom's list.
var lockfileNames = []string{
	"Cargo.lock", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
	"poetry.lock", "Pipfile.lock", "Gemfile.lock", "go.sum", "composer.lock",
	"uv.lock", "bun.lock", "bun.lockb", "npm-shrinkwrap.json", "go.work.sum",
	"Podfile.lock", "Package.resolved", "flake.lock", "mix.lock",
	"pubspec.lock", "deno.lock", "packages.lock.json", "gradle.lockfile",
}

func isLockfile(path string) bool {
	if path == "" {
		return false
	}
	for _, name := range lockfileNames {
		if !strings.HasSuffix(path, name) {
			continue
		}
		prefix := len(path) - len(name)
		if prefix == 0 || path[prefix-1] == '/' || path[prefix-1] == '\\' {
			return true
		}
	}
	return false
}

func stripWS(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\r', '\n', '\v', '\f':
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// isWhitespaceOnly is true when the added and removed lines are equal in
// order once all ASCII whitespace is removed and empty lines are ignored.
// Combined-diff hunks are never treated as whitespace-only.
func (h *hunk) isWhitespaceOnly() bool {
	if h.width != 1 {
		return false
	}
	var adds, subs []string
	changed := false
	for _, l := range h.lines {
		if l == "" {
			continue
		}
		switch l[0] {
		case '+':
			changed = true
			if s := stripWS(l[1:]); s != "" {
				adds = append(adds, s)
			}
		case '-':
			changed = true
			if s := stripWS(l[1:]); s != "" {
				subs = append(subs, s)
			}
		}
	}
	if !changed || len(adds) != len(subs) {
		return false
	}
	for i := range adds {
		if adds[i] != subs[i] {
			return false
		}
	}
	return true
}

// noiseFacts are what the noise step removed.
type noiseFacts struct {
	lockfiles []string
	wsHunks   int
}

// dropNoise marks lockfile hunks and whitespace-only hunks as dropped.
func dropNoise(d *document) noiseFacts {
	var nf noiseFacts
	for _, b := range d.blocks {
		f := b.f
		if f == nil {
			continue
		}
		lock := isLockfile(f.path)
		if lock && len(f.hunks) > 0 {
			nf.lockfiles = appendUnique(nf.lockfiles, f.path)
		}
		for _, h := range f.hunks {
			switch {
			case lock:
				h.drop = dropLockfile
			case h.isWhitespaceOnly():
				h.drop = dropWhitespace
				nf.wsHunks++
			}
		}
	}
	return nf
}

func appendUnique(list []string, s string) []string {
	for _, e := range list {
		if e == s {
			return list
		}
	}
	return append(list, s)
}
