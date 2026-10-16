package compress

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"go test ./...", "go test ./...", true},
		{"  go   test\t./... ", "go test ./...", true},
		{"cd /src/app && go test ./...", "go test ./...", true},
		{"cd web; pnpm test", "pnpm test", true},
		{"CGO_ENABLED=0 GOFLAGS=-count=1 go test ./...", "go test ./...", true},
		{`FOO="a b" go test ./...`, "go test ./...", true},
		{"env FOO=1 go test ./...", "go test ./...", true},
		{"time cargo test", "cargo test", true},
		{"go test ./... 2>&1", "go test ./...", true},
		{"cargo build 2>/dev/null", "cargo build", true},
		{"git --no-pager diff HEAD~1", "git diff HEAD~1", true},
		{"git -C /src/app log -p -3", "git log -p -3", true},
		{"pnpm -C web install", "pnpm install", true},
		{"make -C deploy test", "make test", true},
		{"export GOFLAGS=-race && go test ./...", "go test ./...", true},
		{`git commit -m "fix: a; b && c | d"`, `git commit -m "fix: a; b && c | d"`, true},
		{`echo 'x | y'`, `echo 'x | y'`, true},
		// Mixed output or unknown shape: never compressed.
		{"go test ./... | tail -20", "", false},
		{"go build ./... && go test ./...", "", false},
		{"go test ./... || true", "", false},
		{"go test $(go list ./...)", "", false},
		{"go test `go list ./...`", "", false},
		{"cat <<EOF\nx\nEOF", "", false},
		{"cd /src/app", "", false},
		{"", "", false},
		{`echo "unterminated`, "", false},
	}
	for _, c := range cases {
		got, ok := Normalize(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Normalize(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMatchPrefix(t *testing.T) {
	cases := []struct {
		cmd, prefix string
		want        bool
	}{
		{"go test ./...", "go test", true},
		{"go test", "go test", true},
		{"go testx", "go test", false},
		{"go vet ./...", "go test", false},
		{"make", "make", true},
		{"make build", "make", true},
		{"makefile", "make", false},
		{"git log -p -3", "git log -p", true},
		{"git log --oneline", "git log -p", false},
	}
	for _, c := range cases {
		if got := matchPrefix(c.cmd, c.prefix); got != c.want {
			t.Errorf("matchPrefix(%q, %q) = %v", c.cmd, c.prefix, got)
		}
	}
}
