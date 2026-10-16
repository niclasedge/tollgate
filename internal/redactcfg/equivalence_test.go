package redactcfg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// handWritten is the configuration the form test in web/src/filters.test.ts
// builds by clicking: ip_addresses switched on, one custom rule, one
// allowlist entry, everything else the default.
const handWritten = `version: 1
builtin: {email: true, api_keys: true, jwt: true, connection_urls: true, ip_addresses: true, private_keys: true}
custom:
  - name: kundennummer
    pattern: 'KD-\d{6}'
    placeholder: customer_id
    examples: ["KD-123456"]
allowlist: ["noreply@github.com"]
`

// TestFormAndYAMLGiveTheSameConfig is the acceptance test of #16: the YAML
// the filter form sends (golden file shared with the frontend test) and the
// hand-written YAML import to byte-identical exports.
func TestFormAndYAMLGiveTheSameConfig(t *testing.T) {
	form, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "testdata", "form-config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := Parse(form)
	if err != nil {
		t.Fatalf("form YAML: %v", err)
	}
	b, err := Parse([]byte(handWritten))
	if err != nil {
		t.Fatalf("hand-written YAML: %v", err)
	}
	ea, eb := Export(a, ExportPlainWords), Export(b, ExportPlainWords)
	if !bytes.Equal(ea, eb) {
		t.Fatalf("exports differ\nform:\n%s\nhand:\n%s", ea, eb)
	}
}
