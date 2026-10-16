package logs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/niclasedge/tollgate/internal/compress/compresstest"
	"github.com/niclasedge/tollgate/internal/compress/strat"
)

func TestGoldenTestLog(t *testing.T)  { compresstest.Golden(t, "test_log", TestLog) }
func TestGoldenBuildLog(t *testing.T) { compresstest.Golden(t, "build_log", BuildLog) }

// Every strategy must give byte-identical results on repeated runs and
// must never fail on its own golden inputs.
func TestDeterministicOnGoldenInputs(t *testing.T) {
	for rule, s := range map[string]strat.Strategy{"test_log": TestLog, "build_log": BuildLog} {
		files, _ := filepath.Glob(filepath.Join("testdata", rule, "*.input"))
		if len(files) == 0 {
			t.Fatalf("no inputs for %s", rule)
		}
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) < 2048 {
				t.Errorf("%s: input has %d bytes, want >= 2048", f, len(raw))
			}
			a, b := s(string(raw), strat.Params{}, nil), s(string(raw), strat.Params{}, nil)
			if a.Text != b.Text || a.Omitted != b.Omitted || len(a.Details) != len(b.Details) {
				t.Errorf("%s: not deterministic", f)
			}
			if len(a.Text) >= len(raw) {
				t.Errorf("%s: not smaller (%d >= %d)", f, len(a.Text), len(raw))
			}
			for _, d := range a.Details {
				if containsAny(d, "]\n") {
					t.Errorf("%s: detail %q contains ] or newline", f, d)
				}
			}
		}
	}
}

func containsAny(s, chars string) bool {
	for _, c := range chars {
		for _, r := range s {
			if r == c {
				return true
			}
		}
	}
	return false
}
