package redact

import (
	"fmt"
	"strings"
	"testing"
)

// benchBashOutput builds about 100 KB of deterministic, Bash-like tool
// output (go test, git log, npm, ls) with a few secrets mixed in (#84).
func benchBashOutput() string {
	var b strings.Builder
	for i := 0; b.Len() < 100_000; i++ {
		switch i % 8 {
		case 0:
			fmt.Fprintf(&b, "=== RUN   TestHandler%d\n--- PASS: TestHandler%d (0.%02ds)\n", i, i, i%100)
		case 1:
			fmt.Fprintf(&b, "ok  \tgithub.com/example/project/internal/pkg%d\t0.%03ds\tcoverage: %d.%d%% of statements\n", i, i%1000, i%100, i%10)
		case 2:
			fmt.Fprintf(&b, "commit %040x\nAuthor: Dev %d <dev%d@mailhost.de>\nDate:   Mon Oct 5 12:%02d:00 2026 +0200\n\n    fix: handle edge case %d in parser\n\n", i*7919, i, i, i%60, i)
		case 3:
			fmt.Fprintf(&b, "added %d packages, and audited %d packages in %ds\nfound 0 vulnerabilities\n", i%300, i%900, i%30)
		case 4:
			fmt.Fprintf(&b, "-rw-r--r--  1 dev  staff  %6d Oct  5 12:%02d file_%d.go\n", i*13, i%60, i)
		case 5:
			fmt.Fprintf(&b, "    main_test.go:%d: got %d, want %d (context: request id req-%d)\n", i%500, i, i+1, i)
		case 6:
			fmt.Fprintf(&b, "{\"level\":\"info\",\"msg\":\"request done\",\"path\":\"/v1/items/%d\",\"status\":200,\"duration_ms\":%d}\n", i, i%250)
		case 7:
			if i%400 == 7 {
				fmt.Fprintf(&b, "DEBUG curl -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.sig%d'\npassword=Sup3rS3cret%d\n", i, i)
			} else {
				fmt.Fprintf(&b, "    at Object.<anonymous> (/srv/app/src/module%d.js:%d:%d)\n", i, i%400, i%80)
			}
		}
	}
	return b.String()
}

// allKeywords holds a literal of every built-in rule, so the prefilter
// skips nothing (worst case: only the folding pass is extra).
const allKeywords = "sk-ant- AKIA ghp_ xox _live_ eyJ -----BEGIN @ :// . : clientsecret tenantid thumbprint authorization bearer password\n"

// BenchmarkFind measures Find on 100 KB of Bash-like output, with and
// without prefilter (#84): default switches (redact stage), IP addresses on
// (corpus), and the worst case where every keyword occurs. Numbers:
// issue-84 design.md.
func BenchmarkFind(b *testing.B) {
	bash := benchBashOutput()
	for _, bc := range []struct {
		name string
		opts Options
		text string
	}{
		{"default", DefaultOptions(), bash},
		{"with-ip", corpusOptions(), bash},
		{"all-keywords", DefaultOptions(), allKeywords + bash},
	} {
		for _, pf := range []string{"prefilter", "raw"} {
			b.Run(bc.name+"/"+pf, func(b *testing.B) {
				r := New(bc.opts)
				if pf == "raw" {
					r = withoutPrefilter(r)
				}
				b.SetBytes(int64(len(bc.text)))
				b.ReportAllocs()
				for b.Loop() {
					r.Find(bc.text)
				}
			})
		}
	}
}
