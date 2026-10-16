package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// checkout copies the files a fresh clone would contain (tracked plus new,
// non-ignored files) into a temporary directory.
func checkout(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	list := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	list.Dir = root
	out, err := list.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	dir := t.TempDir()
	for _, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		info, err := os.Lstat(filepath.Join(root, rel))
		if errors.Is(err, fs.ErrNotExist) {
			continue // deleted in the working tree, not part of the next commit
		}
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func run(t *testing.T, dir, name string, args ...string) (stdout, stderr string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	// Ignore caller overrides that would change what or for which platform is built.
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOOS=", "GOARCH=")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, errOut.String())
	}
	return out.String(), errOut.String()
}

func generated(t *testing.T, dir string) map[string]string {
	t.Helper()
	root := filepath.Join(dir, "web/src/lib/types")
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		files[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// logLine is the subset of a slog JSON line the start contract looks at.
type logLine struct {
	Level   string `json:"level"`
	Message string `json:"msg"`
	Addr    string `json:"addr"`
	Error   string `json:"error"`
}

func TestFreshCheckoutBuildsWithoutCgoAndStartsTheProxy(t *testing.T) {
	dir := checkout(t)
	run(t, dir, "make", "build")
	bin := filepath.Join(dir, "bin/tollgate")

	work := t.TempDir()
	keyFile := filepath.Join(work, "cpa.key")
	if err := os.WriteFile(keyFile, []byte("cpa-secret-key-zyxwvutsrqponmlk\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(work, "tollgate.yaml")
	writeConf := func(listen string) {
		body := "listen: \"" + listen + "\"\nupstream:\n  api_key_file: \"" + keyFile + "\"\ndb:\n  path: \"" + filepath.Join(work, "t.db") + "\"\n"
		if err := os.WriteFile(conf, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("refuses a wildcard binding", func(t *testing.T) {
		writeConf("0.0.0.0:8443")
		cmd := exec.Command(bin, "-config", conf, "--dev-allow-any-listen")
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err == nil {
			t.Fatal("start with a wildcard address must fail")
		}
		if stdout.Len() != 0 {
			t.Fatalf("unexpected stdout: %q", stdout.String())
		}
		lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
		var entry logLine
		if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil || entry.Level != "ERROR" || !strings.Contains(entry.Error, "wildcard") {
			t.Fatalf("expected a JSON error line naming the wildcard, got %q", stderr.String())
		}
	})

	t.Run("serves and stops on SIGTERM", func(t *testing.T) {
		writeConf("127.0.0.1:0")
		cmd := exec.Command(bin, "-config", conf, "--dev-allow-any-listen")
		pipe, err := cmd.StderrPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill() })

		addr := make(chan string, 1)
		go func() {
			sc := bufio.NewScanner(pipe)
			for sc.Scan() {
				var e logLine
				if json.Unmarshal(sc.Bytes(), &e) == nil && e.Message == "tollgate: listening" {
					addr <- e.Addr
				}
			}
		}()
		var a string
		select {
		case a = <-addr:
		case <-time.After(20 * time.Second):
			t.Fatal("no listening log line")
		}
		resp, err := http.Get("http://" + a + "/v1/models")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("request without client key: status %d, want 401", resp.StatusCode)
		}
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("exit after SIGTERM: %v", err)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("no clean exit after SIGTERM")
		}
	})
}

func TestGenerateNeedsNoGlobalInstallAndIsStable(t *testing.T) {
	dir := checkout(t)
	before := generated(t, dir)
	for i := range 2 {
		run(t, dir, "make", "generate")
		after := generated(t, dir)
		if len(after) != len(before) {
			t.Fatalf("run %d changed generated files: %v -> %v", i+1, before, after)
		}
		for name, content := range before {
			if after[name] != content {
				t.Fatalf("run %d changed %s", i+1, name)
			}
		}
	}
	stdout, _ := run(t, dir, "go", "tool", "tygo", "--help")
	if !strings.Contains(stdout, "generate") {
		t.Fatalf("tygo help does not mention generate: %q", stdout)
	}
}
