package upstream

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// CLIProxyAPI is built from the pinned upstream commit, never from a copy in
// the repository (ADR-0014). These tests guard the pin and its documentation.

var pinLine = regexp.MustCompile(`(?m)^ARG CPA_VERSION=([0-9a-f]{40})$`)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func cpaPin(t *testing.T) string {
	t.Helper()
	docker := readRepoFile(t, "deploy/cliproxyapi/Dockerfile")
	m := pinLine.FindAllStringSubmatch(docker, -1)
	if len(m) != 1 {
		t.Fatalf("Dockerfile needs exactly one 'ARG CPA_VERSION=<40 hex>', found %d", len(m))
	}
	return m[0][1]
}

func TestCPANoVendoredCopy(t *testing.T) {
	// Untracked local leftovers (e.g. logs) are not part of the repository;
	// tracked files or module sources are.
	out, err := exec.Command("git", "-C", filepath.Join("..", ".."), "ls-files", "third_party/cliproxyapi").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		t.Error("third_party/cliproxyapi still has tracked files")
	}
	for _, f := range []string{"go.mod", "LICENSE", "cmd"} {
		if _, err := os.Stat(filepath.Join("..", "..", "third_party", "cliproxyapi", f)); !os.IsNotExist(err) {
			t.Errorf("third_party/cliproxyapi/%s must not exist (err=%v)", f, err)
		}
	}
	compose := readRepoFile(t, "docker-compose.yml")
	for _, s := range []string{"cpa-src", "third_party/cliproxyapi"} {
		if strings.Contains(compose, s) {
			t.Errorf("docker-compose.yml still references %q", s)
		}
	}
}

func TestCPAPinnedBuild(t *testing.T) {
	pin := cpaPin(t)
	docker := readRepoFile(t, "deploy/cliproxyapi/Dockerfile")
	if !strings.Contains(docker, "go mod download -json github.com/router-for-me/CLIProxyAPI/v8@${CPA_VERSION}") ||
		!strings.Contains(docker, "/bridge/apply.sh") || !strings.Contains(docker, "go test -race") {
		t.Error("Dockerfile must download pinned sources, apply the documented barrier and test it")
	}
	if !strings.Contains(docker, "CGO_ENABLED=0") {
		t.Error("Dockerfile must build with CGO_ENABLED=0")
	}
	if strings.Contains(docker, "cpa-src") {
		t.Error("Dockerfile still copies from the cpa-src context")
	}
	if strings.Count(docker, pin) != 1 {
		t.Error("the commit must appear only in the ARG line of the Dockerfile")
	}

	makefile := readRepoFile(t, "Makefile")
	if regexp.MustCompile(`[0-9a-f]{40}`).MatchString(makefile) {
		t.Error("Makefile must read the pin from the Dockerfile, not repeat a commit")
	}
	if !strings.Contains(makefile, "ARG CPA_VERSION") || !strings.Contains(makefile, "cpa:") {
		t.Error("Makefile needs a cpa target that reads ARG CPA_VERSION")
	}
}

func TestCPAProvenanceDocumented(t *testing.T) {
	pin := cpaPin(t)
	upstream := readRepoFile(t, "deploy/cliproxyapi/UPSTREAM.md")
	notice := readRepoFile(t, "third_party/NOTICE.md")
	for name, doc := range map[string]string{"UPSTREAM.md": upstream, "NOTICE.md": notice} {
		for _, want := range []string{
			pin,
			"MIT",
			"Copyright (c) 2025-2005.9 Luis Pater",
			"Copyright (c) 2025.9-present Router-For.ME",
		} {
			if !strings.Contains(doc, want) {
				t.Errorf("%s does not mention %q", name, want)
			}
		}
	}
	for _, want := range []string{"ref/cliproxyapi", "Update"} {
		if !strings.Contains(upstream, want) {
			t.Errorf("UPSTREAM.md does not mention %q", want)
		}
	}
}
