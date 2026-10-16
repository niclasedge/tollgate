package redactcfg

import (
	"context"
	"testing"

	"github.com/niclasedge/tollgate/internal/compress"
)

const compressV2 = `version: 1
compress:
  min_bytes: 4096
  rules:
    - name: go_test_off
      match: ["go test -run Slow"]
      strategy: passthrough
    - name: own_diff
      match: ["git diff"]
      strategy: diff
      context_lines: 1
      max_files: 5
`

func TestCompressorPerVersion(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()

	v1, err := e.m.Compressor(ctx, e.m.Active().Version)
	if err != nil {
		t.Fatal(err)
	}
	if v1.MinBytes != compress.DefaultMinBytes || len(v1.Rules) != len(compress.Default().Rules) {
		t.Fatalf("version without compress section = %+v", v1)
	}

	v2 := e.importYAML(t, compressV2, true)
	c, err := e.m.Compressor(ctx, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c.MinBytes != 4096 || c.Rules[0].Name != "go_test_off" || c.Rules[1].Name != "own_diff" {
		t.Fatalf("own rules must come first: %+v", c.Rules[:2])
	}
	if p := c.Rules[1].Params; p.ContextLines != 1 || p.MaxFiles != 5 || p.MaxHunks != 0 {
		t.Fatalf("params = %+v", p)
	}
	if len(c.Rules) != 2+len(compress.Default().Rules) {
		t.Fatalf("built-in rules must follow, got %d rules", len(c.Rules))
	}
	if r, _ := c.Match("go test -run Slow ./..."); r.Strategy != "passthrough" {
		t.Errorf("own passthrough rule must win, got %+v", r)
	}
	// The pinned old version keeps its rules.
	if old, _ := e.m.Compressor(ctx, 1); old.MinBytes != compress.DefaultMinBytes {
		t.Errorf("version 1 changed: %+v", old)
	}
}
