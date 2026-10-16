// Package deploy holds only tests for the container build files.
package deploy

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// pinnedImage is name:tag@sha256:<index digest> (issue #59).
var pinnedImage = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*(:[0-9]+)?(/[a-z0-9._-]+)*:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}@sha256:[0-9a-f]{64}$`)

// skipDirs are never part of the build: VCS data, local references,
// dependencies and the untracked former CLIProxyAPI copy (ADR-0014).
var skipDirs = []string{".git", "ref", "node_modules", filepath.Join("third_party", "cliproxyapi")}

func findDockerfiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if slices.Contains(skipDirs, rel) || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), "Dockerfile") {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// unpinnedFrom returns one finding per FROM line that neither names an
// earlier stage nor pins tag and digest.
func unpinnedFrom(content string) []string {
	var bad []string
	stages := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(content))
	for n := 1; sc.Scan(); n++ {
		f := strings.Fields(sc.Text())
		if len(f) == 0 || !strings.EqualFold(f[0], "FROM") {
			continue
		}
		args := f[1:]
		for len(args) > 0 && strings.HasPrefix(args[0], "--") {
			args = args[1:]
		}
		if len(args) == 0 {
			bad = append(bad, sc.Text())
			continue
		}
		image := args[0]
		if !stages[strings.ToLower(image)] && !pinnedImage.MatchString(image) {
			bad = append(bad, "line "+strconv.Itoa(n)+": "+image)
		}
		// A stage name becomes usable only after its own FROM line.
		if len(args) >= 3 && strings.EqualFold(args[1], "AS") {
			stages[strings.ToLower(args[2])] = true
		}
	}
	return bad
}

func TestBaseImagesPinnedByDigest(t *testing.T) {
	root := ".."
	files := findDockerfiles(t, root)
	for _, want := range []string{"deploy/tollgate/Dockerfile", "deploy/cliproxyapi/Dockerfile"} {
		if !slices.Contains(files, want) {
			t.Errorf("%s not found; the check would pass without looking at it", want)
		}
	}
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range unpinnedFrom(string(b)) {
			t.Errorf("%s %s: base image needs name:tag@sha256:<index digest>", rel, finding)
		}
	}
}

func TestUnpinnedFromRules(t *testing.T) {
	const digest = "@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251"
	cases := []struct {
		name    string
		content string
		bad     int
	}{
		{"floating tag", "FROM debian:bookworm-slim\n", 1},
		{"digest without tag", "FROM debian" + digest + "\n", 1},
		{"short digest", "FROM debian:bookworm-slim@sha256:3783cc01\n", 1},
		{"tag and digest", "FROM debian:bookworm-slim" + digest + "\n", 0},
		{"platform flag", "FROM --platform=$BUILDPLATFORM golang:1.26-bookworm" + digest + " AS build\n", 0},
		{"registry with port", "FROM registry.example:5000/team/img:1.0" + digest + "\n", 0},
		{"earlier stage", "FROM golang:1.26-bookworm" + digest + " AS build\nFROM build\n", 0},
		{"stage used before defined", "FROM build\nFROM golang:1.26-bookworm" + digest + " AS build\n", 1},
		{"self reference", "FROM base AS base\n", 1},
		{"lowercase from", "from node:22\n", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := unpinnedFrom(c.content); len(got) != c.bad {
				t.Errorf("findings = %v, want %d", got, c.bad)
			}
		})
	}
}
