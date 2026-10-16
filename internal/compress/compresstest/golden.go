// Package compresstest runs the golden files of a compress strategy.
package compresstest

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/compress/strat"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// Golden runs every testdata/<rule>/<case>.input through s with default
// limits and compares strat.Render(rule, result) with <case>.golden. Optional
// <case>.cmd holds the command the output came from (ADR-0007).
// go test -update rewrites the golden files.
func Golden(t *testing.T, rule string, s strat.Strategy) {
	t.Helper()
	inputs, err := filepath.Glob(filepath.Join("testdata", rule, "*.input"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatalf("no golden inputs in testdata/%s", rule)
	}
	for _, in := range inputs {
		name := strings.TrimSuffix(in, ".input")
		t.Run(filepath.Base(name), func(t *testing.T) {
			raw, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			got := strat.Render(rule, s(string(raw), strat.Params{}, nil))
			if again := strat.Render(rule, s(string(raw), strat.Params{}, nil)); again != got {
				t.Fatal("strategy is not deterministic")
			}
			golden := name + ".golden"
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run go test -update)", err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s (go test -update after review)\n--- got ---\n%s", golden, got)
			}
		})
	}
}
