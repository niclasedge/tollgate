package redactcfg

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/redact"
)

// agentsExample is the schema example from AGENTS.md.
const agentsExample = `version: 1
builtin:
  email: true
  api_keys: true
  jwt: true
  connection_urls: true
  ip_addresses: false
  private_keys: true
custom:
  - name: kundennummer
    pattern: 'KD-\d{6}'
    placeholder: customer_id
    examples: ["KD-123456"]
words:
  case_sensitive: false
  whole_word: true
  list: ["Projekt Falke", "Muster GmbH"]
allowlist: ["noreply@github.com"]
`

const withCompress = agentsExample + `compress:
  min_bytes: 2048
  rules:
    - name: go_test
      match: ["go test"]
      strategy: test_log
      max_errors: 10
      context_lines: 3
    - name: git_diff
      match: ["git diff", "git show"]
      strategy: diff
      max_files: 20
      max_hunks: 10
`

func mustParse(t *testing.T, src string) *Document {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

func TestParseExample(t *testing.T) {
	d := mustParse(t, agentsExample)
	want := &Document{
		Version: 1,
		Builtin: Builtin{Email: true, APIKeys: true, JWT: true, ConnectionURLs: true, PrivateKeys: true},
		Custom:  []CustomRule{{Name: "kundennummer", Pattern: `KD-\d{6}`, Placeholder: "customer_id", Examples: []string{"KD-123456"}}},
		Words: Words{WholeWord: true, List: []string{"Projekt Falke", "Muster GmbH"},
			Customers: []string{}, FirstNames: []string{}, LastNames: []string{}},
		Allowlist: []string{"noreply@github.com"},
	}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("got  %+v\nwant %+v", d, want)
	}
}

func TestRoundtrip(t *testing.T) {
	for name, src := range map[string]string{"example": agentsExample, "compress": withCompress, "minimal": "version: 1\n"} {
		t.Run(name, func(t *testing.T) {
			d := mustParse(t, src)
			out := Export(d, ExportPlainWords)
			if !strings.HasPrefix(string(out), PlainWordsWarning+"\n") {
				t.Fatalf("export does not start with the warning:\n%s", out)
			}
			d2 := mustParse(t, string(out))
			if !reflect.DeepEqual(d, d2) {
				t.Fatalf("roundtrip changed the document:\n%+v\n%+v", d, d2)
			}
			if out2 := Export(d2, ExportPlainWords); string(out2) != string(out) {
				t.Fatalf("second export differs:\n%s\n---\n%s", out, out2)
			}
		})
	}
}

func TestExportModes(t *testing.T) {
	d := mustParse(t, agentsExample)
	omit := string(Export(d, ExportOmitWords))
	if strings.Contains(omit, "Projekt Falke") || !strings.Contains(omit, "list: []") || strings.Contains(omit, "WARNUNG") {
		t.Errorf("omit export:\n%s", omit)
	}
	masked := string(Export(d, ExportMaskedWords))
	if strings.Contains(masked, "Projekt Falke") || strings.Contains(masked, "Muster GmbH") || !strings.Contains(masked, "••") {
		t.Errorf("masked export:\n%s", masked)
	}
	// The omit export imports to the same config without words.
	d2 := mustParse(t, omit)
	d.Words.List = []string{}
	if !reflect.DeepEqual(d, d2) {
		t.Errorf("omit export does not re-import:\n%+v\n%+v", d, d2)
	}
}

func TestDefaults(t *testing.T) {
	d := mustParse(t, "version: 1\nbuiltin:\n  ip_addresses: true\nwords:\n  case_sensitive: true\n")
	if !d.Builtin.Email || !d.Builtin.IPAddresses || !d.Words.WholeWord || !d.Words.CaseSensitive {
		t.Fatalf("defaults not applied: %+v", d)
	}
	if !reflect.DeepEqual(mustParse(t, "version: 1\n"), Default()) {
		t.Fatalf("minimal document is not the default")
	}
	if got, want := Default().RedactConfig().Builtin, redact.DefaultOptions(); got != want {
		t.Fatalf("default builtin = %+v, want %+v", got, want)
	}
}

func TestAnchorsAllowed(t *testing.T) {
	src := "version: 1\ncustom:\n  - name: a\n    pattern: 'KD-\\d{6}'\n    placeholder: customer_id\n    examples: &ex [\"KD-123456\"]\n  - name: b\n    pattern: 'KD-\\d{6}'\n    placeholder: customer_ref\n    examples: *ex\n"
	d := mustParse(t, src)
	if !reflect.DeepEqual(d.Custom[1].Examples, []string{"KD-123456"}) {
		t.Fatalf("alias not resolved: %+v", d.Custom)
	}
}

func issues(t *testing.T, src string) []Issue {
	t.Helper()
	_, err := Parse([]byte(src))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if len(ve.Issues) == 0 {
		t.Fatal("no issues")
	}
	return ve.Issues
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		path       string
		line, col  int
		msgContain string
	}{
		{"typo key", "version: 1\nbuiltin:\n  emial: true\n", "builtin", 3, 3, "emial"},
		{"bad placeholder", "version: 1\ncustom:\n  - name: k\n    pattern: 'KD-\\d{6}'\n    placeholder: Kunde-ID\n    examples: [\"KD-123456\"]\n", "custom[0].placeholder", 5, 18, "^[a-z][a-z0-9_]{0,31}$"},
		{"builtin placeholder", "version: 1\ncustom:\n  - name: k\n    pattern: 'KD-\\d{6}'\n    placeholder: email\n    examples: [\"KD-123456\"]\n", "custom[0].placeholder", 5, 18, "eingebaut"},
		{"example misses", "version: 1\ncustom:\n  - name: k\n    pattern: 'KD-\\d{6}'\n    placeholder: customer_id\n    examples:\n      - KD-123456\n      - KD-12\n", "custom[0].examples[1]", 8, 9, "trifft"},
		{"bad regex", "version: 1\ncustom:\n  - name: k\n    pattern: 'KD-('\n    placeholder: customer_id\n    examples: [\"KD-\"]\n", "custom[0].pattern", 4, 14, "Regex"},
		{"missing field", "version: 1\ncustom:\n  - name: k\n    placeholder: customer_id\n    examples: [\"KD-1\"]\n", "custom[0]", 3, 5, "pattern"},
		{"wrong type", "version: 1\nwords:\n  whole_word: ja\n", "words.whole_word", 3, 15, "boolean"},
		{"wrong version", "version: 2\n", "version", 1, 10, "1"},
		{"missing version", "builtin: {}\n", "", 1, 1, "version"},
		{"blank word", "version: 1\nwords:\n  list: [\"ok\", \"   \"]\n", "words.list[1]", 3, 16, "leer"},
		{"duplicate rule", "version: 1\ncompress:\n  rules:\n    - {name: a, match: [x], strategy: diff}\n    - {name: a, match: [y], strategy: diff}\n", "compress.rules[1].name", 5, 14, "doppelt"},
		{"blank match", "version: 1\ncompress:\n  rules:\n    - {name: a, match: [\" \"], strategy: diff}\n", "compress.rules[0].match[0]", 4, 25, "leer"},
		{"bad strategy", "version: 1\ncompress:\n  rules:\n    - {name: a, match: [x], strategy: zip}\n", "compress.rules[0].strategy", 4, 39, "test_log"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := issues(t, tt.src)
			got := is[0]
			if got.Path != tt.path || got.Line != tt.line || got.Column != tt.col || !strings.Contains(got.Message, tt.msgContain) {
				t.Fatalf("issue = %+v, want path %q line %d col %d containing %q", got, tt.path, tt.line, tt.col, tt.msgContain)
			}
		})
	}
}

func TestParseYAMLSyntax(t *testing.T) {
	for name, src := range map[string]string{
		"broken":        "version: 1\nbuiltin:\n  email: [true\n",
		"tab indent":    "version: 1\nbuiltin:\n\temail: true\n",
		"two documents": "version: 1\n---\nversion: 1\n",
		"duplicate key": "version: 1\nbuiltin:\n  email: true\n  email: false\n",
		"empty":         "",
		"not a mapping": "- a\n- b\n",
		"comments only": "# nothing\n",
	} {
		t.Run(name, func(t *testing.T) {
			is := issues(t, src)
			if is[0].Line == 0 || is[0].Message == "" {
				t.Fatalf("issue without line or message: %+v", is[0])
			}
			if strings.Contains(is[0].Message, "yaml:") && !strings.Contains(is[0].Message, "Zeile") {
				t.Fatalf("raw parser message: %+v", is[0])
			}
		})
	}
}

func TestErrorsNeverQuoteWords(t *testing.T) {
	src := "version: 1\nwords:\n  list: [\"Projekt Falke\", \"\", 42, \"" + strings.Repeat("x", 201) + "\"]\n"
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "Falke") || strings.Contains(err.Error(), "xxxxxxxxxx") {
		t.Fatalf("error quotes a word: %v", err)
	}
}

func TestValidationErrorString(t *testing.T) {
	is := issues(t, "version: 1\ncustom:\n  - name: k\n    pattern: 'KD-\\d{6}'\n    placeholder: Kunde-ID\n    examples: [\"KD-123456\"]\n")
	err := &ValidationError{Issues: is}
	if !strings.Contains(err.Error(), "custom[0].placeholder (Zeile 5, Spalte 18): muss") {
		t.Fatalf("Error() = %q", err.Error())
	}
}
