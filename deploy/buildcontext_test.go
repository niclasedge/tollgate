package deploy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const tollgateDockerfile = "deploy/tollgate/Dockerfile"

// copyLine is one COPY instruction of the build stage, destination already
// resolved against WORKDIR.
type copyLine struct {
	from string // stage of --from, empty for the build context
	srcs []string
	dest string
	dir  bool // destination is a directory
}

// buildStage returns WORKDIR and the COPY lines of the stage that runs
// "go build" (issue #107).
func buildStage(content string) (string, []copyLine, error) {
	// Join continuation lines first.
	var lines []string
	var cur strings.Builder
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasSuffix(l, "\\") {
			cur.WriteString(strings.TrimSuffix(l, "\\") + " ")
			continue
		}
		cur.WriteString(l)
		lines = append(lines, cur.String())
		cur.Reset()
	}

	type stage struct {
		workdir string
		copies  []copyLine
		goBuild bool
	}
	var stages []*stage
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) == 0 {
			continue
		}
		instr := strings.ToUpper(f[0])
		if instr == "FROM" {
			stages = append(stages, &stage{workdir: "/"})
			continue
		}
		if len(stages) == 0 {
			continue
		}
		st := stages[len(stages)-1]
		switch instr {
		case "WORKDIR":
			if len(f) != 2 {
				return "", nil, errors.New("unsupported WORKDIR: " + l)
			}
			st.workdir = path.Join(st.workdir, f[1])
		case "RUN":
			if strings.Contains(l, "go build") {
				st.goBuild = true
			}
		case "COPY", "ADD":
			args := f[1:]
			var c copyLine
			for len(args) > 0 && strings.HasPrefix(args[0], "--") {
				if v, ok := strings.CutPrefix(args[0], "--from="); ok {
					c.from = v
				}
				args = args[1:]
			}
			if len(args) < 2 || strings.HasPrefix(args[0], "[") {
				return "", nil, errors.New("unsupported COPY: " + l)
			}
			c.srcs = args[:len(args)-1]
			dest := args[len(args)-1]
			c.dir = strings.HasSuffix(dest, "/") || len(c.srcs) > 1
			for _, s := range c.srcs {
				if strings.ContainsAny(s, "*?[") {
					c.dir = true
				}
			}
			if !path.IsAbs(dest) {
				dest = path.Join(st.workdir, dest)
			}
			c.dest = path.Clean(dest)
			st.copies = append(st.copies, c)
		}
	}
	for _, st := range stages {
		if st.goBuild {
			return st.workdir, st.copies, nil
		}
	}
	return "", nil, errors.New("no stage runs go build")
}

// prefixes returns f and all its parent directories, shortest first.
func prefixes(f string) []string {
	parts := strings.Split(f, "/")
	out := make([]string, len(parts))
	for i := range parts {
		out[i] = strings.Join(parts[:i+1], "/")
	}
	return out
}

// target reports where c puts the context file f, or "" if c does not copy f.
func (c copyLine) target(f string) string {
	for _, src := range c.srcs {
		src = path.Clean(src)
		for _, p := range prefixes(f) {
			ok := src == "." || p == src
			if !ok {
				ok, _ = path.Match(src, p)
			}
			if !ok {
				continue
			}
			switch {
			case src == ".":
				return path.Join(c.dest, f)
			case p == f && c.dir:
				return path.Join(c.dest, path.Base(f))
			case p == f:
				return c.dest
			default: // a directory source copies its contents
				return path.Join(c.dest, f[len(p)+1:])
			}
		}
	}
	return ""
}

// dockerignore holds the patterns of a .dockerignore file in order.
type dockerignore []string

func parseDockerignore(content string) (dockerignore, error) {
	var d dockerignore
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if strings.Contains(l, "**") {
			return nil, errors.New("pattern with ** not supported by this check: " + l)
		}
		d = append(d, l)
	}
	return d, nil
}

// excluded applies Docker's rule: the last matching pattern wins, "!"
// re-includes, and a pattern matching a parent directory matches its files.
func (d dockerignore) excluded(f string) bool {
	ex := false
	for _, p := range d {
		neg := strings.HasPrefix(p, "!")
		p = strings.TrimPrefix(path.Clean(strings.TrimPrefix(p, "!")), "/")
		for _, pre := range prefixes(f) {
			if ok, _ := path.Match(p, pre); ok {
				ex = !neg
				break
			}
		}
	}
	return ex
}

// missingInputs returns every required file that the build stage does not
// receive at its repository path below WORKDIR.
func missingInputs(workdir string, copies []copyLine, ignore dockerignore, files []string) []string {
	var missing []string
	for _, f := range files {
		want := path.Join(workdir, f)
		ok := false
		for _, c := range copies {
			if c.from != "" {
				if want == c.dest || strings.HasPrefix(want, c.dest+"/") {
					ok = true
				}
				continue
			}
			if c.target(f) == want && !ignore.excluded(f) {
				ok = true
			}
		}
		if !ok {
			missing = append(missing, f)
		}
	}
	return missing
}

// buildInputs lists go.mod, go.sum and the Go and embedded files of every
// package of this module that ./cmd/tollgate depends on, relative to root.
func buildInputs(t *testing.T, root string) []string {
	t.Helper()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "list", "-deps", "-json", "./cmd/tollgate")
	cmd.Dir = abs
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, stderr.String())
	}
	files := []string{"go.mod", "go.sum"}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p struct {
			ImportPath string
			Dir        string
			GoFiles    []string
			EmbedFiles []string
			Module     *struct{ Main bool }
		}
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if p.Module == nil || !p.Module.Main {
			continue
		}
		rel, err := filepath.Rel(abs, p.Dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range slices.Concat(p.GoFiles, p.EmbedFiles) {
			files = append(files, path.Join(filepath.ToSlash(rel), f))
		}
	}
	slices.Sort(files)
	return files
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTollgateImageBuildInputs(t *testing.T) {
	root := ".."
	workdir, copies, err := buildStage(readFile(t, filepath.Join(root, tollgateDockerfile)))
	if err != nil {
		t.Fatal(err)
	}
	ignore, err := parseDockerignore(readFile(t, filepath.Join(root, ".dockerignore")))
	if err != nil {
		t.Fatal(err)
	}
	files := buildInputs(t, root)
	for _, want := range []string{"cmd/tollgate/main.go", "fixtures.go", "config/schema/tollgate-config.v1.schema.json"} {
		if !slices.Contains(files, want) {
			t.Errorf("%s not among the build inputs; go list returned %v", want, files)
		}
	}
	for _, f := range missingInputs(workdir, copies, ignore, files) {
		t.Errorf("%s: %s is needed by ./cmd/tollgate but not copied to %s or excluded by .dockerignore",
			tollgateDockerfile, f, path.Join(workdir, f))
	}
}

// Only testdata/fixtures may leave testdata for the build context.
func TestDockerignoreReleasesOnlyFixtures(t *testing.T) {
	root := ".."
	ignore, err := parseDockerignore(readFile(t, filepath.Join(root, ".dockerignore")))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	err = filepath.WalkDir(filepath.Join(root, "testdata"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		seen++
		inFixtures := strings.HasPrefix(rel, "testdata/fixtures/")
		if got := ignore.excluded(rel); got == inFixtures {
			t.Errorf("%s: excluded = %v, want %v", rel, got, !inFixtures)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("no files under testdata; the check would pass without looking at it")
	}
}

func TestMissingInputsRules(t *testing.T) {
	const head = "FROM golang AS build\nWORKDIR /src\n"
	files := []string{"go.mod", "fixtures.go", "cmd/x/main.go", "web/embed.go", "web/dist/index.html", "testdata/fixtures/a.json"}
	cases := []struct {
		name       string
		dockerfile string
		ignore     string
		missing    []string
	}{
		{"all covered", head + "COPY go.mod ./\nCOPY fixtures.go ./\nCOPY cmd ./cmd\nCOPY web/*.go ./web/\nCOPY --from=web /src/web/dist ./web/dist\nCOPY testdata/fixtures ./testdata/fixtures\nRUN go build ./cmd/x\n",
			"testdata\n!testdata/fixtures\n", nil},
		{"missing root file and dir", head + "COPY go.mod ./\nCOPY cmd ./cmd\nCOPY web/*.go ./web/\nCOPY --from=web /src/web/dist ./web/dist\nRUN go build ./cmd/x\n",
			"", []string{"fixtures.go", "testdata/fixtures/a.json"}},
		{"excluded by dockerignore", head + "COPY . .\nRUN go build ./cmd/x\n",
			"testdata\nweb/dist\n", []string{"web/dist/index.html", "testdata/fixtures/a.json"}},
		{"re-included", head + "COPY . .\nRUN go build ./cmd/x\n",
			"testdata\n!testdata/fixtures\n", nil},
		{"wrong destination", head + "COPY . .\nCOPY cmd ./other\nRUN go build \\\n  ./cmd/x\n",
			"cmd\n", []string{"cmd/x/main.go"}},
		{"file to file", head + "COPY . .\nCOPY fixtures.go ./renamed.go\nRUN go build ./cmd/x\n",
			"fixtures.go\n", []string{"fixtures.go"}},
		{"other stage ignored", "FROM node AS web\nWORKDIR /src\nCOPY . .\n" + head + "COPY go.mod ./\nRUN go build ./cmd/x\n",
			"", []string{"cmd/x/main.go", "fixtures.go", "testdata/fixtures/a.json", "web/dist/index.html", "web/embed.go"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			workdir, copies, err := buildStage(c.dockerfile)
			if err != nil {
				t.Fatal(err)
			}
			ignore, err := parseDockerignore(c.ignore)
			if err != nil {
				t.Fatal(err)
			}
			got := missingInputs(workdir, copies, ignore, files)
			slices.Sort(got)
			want := slices.Clone(c.missing)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("missing = %v, want %v", got, want)
			}
		})
	}
	if _, err := parseDockerignore("**/secret\n"); err == nil {
		t.Error("** pattern accepted; the check does not implement it")
	}
	if _, _, err := buildStage("FROM node\nRUN pnpm build\n"); err == nil {
		t.Error("Dockerfile without go build accepted")
	}
}
