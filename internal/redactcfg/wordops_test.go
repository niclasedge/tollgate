package redactcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/config/schema"
)

func TestApplyWordOps(t *testing.T) {
	base := []string{"Projekt Falke", "Muster GmbH"}
	tests := []struct {
		name          string
		base          []string
		ops           WordOps
		caseSensitive bool
		want          []string
	}{
		{"no ops", base, WordOps{}, false, base},
		{"add", base, WordOps{WordsAdd: []string{"Neu AG"}}, false, []string{"Projekt Falke", "Muster GmbH", "Neu AG"}},
		{"add keeps input order", base, WordOps{WordsAdd: []string{"Zweite KG", "Erste AG"}}, false,
			[]string{"Projekt Falke", "Muster GmbH", "Zweite KG", "Erste AG"}},
		{"add trims", base, WordOps{WordsAdd: []string{"  Neu AG\t"}}, false, []string{"Projekt Falke", "Muster GmbH", "Neu AG"}},
		{"add drops empty", base, WordOps{WordsAdd: []string{"", " ", "\n"}}, false, base},
		{"remove first", base, WordOps{WordsRemove: []int{0}}, false, []string{"Muster GmbH"}},
		{"remove all in any order", base, WordOps{WordsRemove: []int{1, 0}}, false, []string{}},
		{"duplicates case-insensitive", base, WordOps{WordsAdd: []string{"muster gmbh", "Neu AG", "NEU AG", "neu ag"}}, false,
			[]string{"Projekt Falke", "Muster GmbH", "Neu AG"}},
		{"duplicates case-sensitive", base, WordOps{WordsAdd: []string{"muster gmbh", "Muster GmbH", "muster gmbh"}}, true,
			[]string{"Projekt Falke", "Muster GmbH", "muster gmbh"}},
		{"unicode folding", []string{"Straße"}, WordOps{WordsAdd: []string{"STRAßE", "Ǆemal", "ǆemal", "ǅemal"}}, false,
			[]string{"Straße", "Ǆemal"}},
		{"change = remove and add", base, WordOps{WordsRemove: []int{1}, WordsAdd: []string{"Muster GmbH"}}, false,
			[]string{"Projekt Falke", "Muster GmbH"}},
		{"change moves to the end", base, WordOps{WordsRemove: []int{0}, WordsAdd: []string{"projekt falke"}}, false,
			[]string{"Muster GmbH", "projekt falke"}},
		{"existing base duplicates stay", []string{"A-Wort", "a-wort"}, WordOps{WordsAdd: []string{"A-WORT"}}, false,
			[]string{"A-Wort", "a-wort"}},
		{"empty base", nil, WordOps{WordsAdd: []string{"Neu AG"}}, false, []string{"Neu AG"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := append([]string(nil), tt.base...)
			got, err := applyWordOps(in, tt.ops, tt.caseSensitive)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			if !reflect.DeepEqual(in, append([]string(nil), tt.base...)) {
				t.Fatalf("base modified: %q", in)
			}
			// Deterministic: the same input gives the same output.
			again, _ := applyWordOps(in, tt.ops, tt.caseSensitive)
			if !reflect.DeepEqual(got, again) {
				t.Fatalf("not deterministic: %q vs %q", got, again)
			}
		})
	}
}

func TestApplyWordOpsErrors(t *testing.T) {
	base := []string{"Projekt Falke", "Muster GmbH"}
	long := strings.Repeat("x", maxWordRunes+1)
	tests := []struct {
		name     string
		ops      WordOps
		op       string // substring of a *WordOpError message
		issue    string // path of a *ValidationError issue
		baseSize int
	}{
		{name: "index out of range", ops: WordOps{WordsRemove: []int{2}}, op: "words_remove[0]: Index 2"},
		{name: "negative index", ops: WordOps{WordsRemove: []int{0, -1}}, op: "words_remove[1]: Index -1"},
		{name: "duplicate index", ops: WordOps{WordsRemove: []int{1, 1}}, op: "words_remove[1]: Index 1 ist doppelt"},
		{name: "too long", ops: WordOps{WordsAdd: []string{"ok", long}}, issue: "words_add[1]"},
		{name: "masked filler", ops: WordOps{WordsAdd: []string{"P" + maskDots + "e"}}, issue: "words_add[0]"},
		{name: "too many", ops: WordOps{WordsAdd: []string{"Neu AG"}}, issue: "words_add", baseSize: maxWords},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := base
			if tt.baseSize > 0 {
				b = make([]string, tt.baseSize)
				for i := range b {
					b[i] = fmt.Sprintf("w%d", i)
				}
			}
			_, err := applyWordOps(b, tt.ops, false)
			var oe *WordOpError
			var ve *ValidationError
			switch {
			case tt.op != "":
				if !errors.As(err, &oe) || !strings.Contains(oe.Error(), tt.op) {
					t.Fatalf("err = %v, want WordOpError containing %q", err, tt.op)
				}
			default:
				if !errors.As(err, &ve) || len(ve.Issues) != 1 || ve.Issues[0].Path != tt.issue {
					t.Fatalf("err = %v, want issue at %q", err, tt.issue)
				}
			}
			for _, w := range append(append([]string{}, base...), tt.ops.WordsAdd...) {
				if len(w) > 3 && strings.Contains(err.Error(), w) {
					t.Fatalf("error quotes an entry: %v", err)
				}
			}
			if strings.Contains(err.Error(), maskDots) {
				t.Fatalf("error quotes the masked entry: %v", err)
			}
		})
	}
}

// TestWordLimitsMatchSchema keeps the limits of words_add equal to the
// words.list limits of the YAML schema.
func TestWordLimitsMatchSchema(t *testing.T) {
	var s struct {
		Properties struct {
			Words struct {
				Properties struct {
					List struct {
						MaxItems int `json:"maxItems"`
						Items    struct {
							MaxLength int `json:"maxLength"`
						} `json:"items"`
					} `json:"list"`
				} `json:"properties"`
			} `json:"words"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema.ConfigV1, &s); err != nil {
		t.Fatal(err)
	}
	l := s.Properties.Words.Properties.List
	if l.MaxItems != maxWords || l.Items.MaxLength != maxWordRunes {
		t.Fatalf("schema maxItems=%d maxLength=%d, code %d/%d", l.MaxItems, l.Items.MaxLength, maxWords, maxWordRunes)
	}
}

func exportWords(t *testing.T, e *env, id int64) []string {
	t.Helper()
	plain, err := e.m.Export(context.Background(), id, ExportPlainWords)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(plain)
	if err != nil {
		t.Fatal(err)
	}
	return d.Words.List
}

func TestWordOpsImport(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, true)
	v2 := e.importYAML(t, agentsExample, true)

	// Add and remove on the list taken over from version 2.
	v3, err := e.m.Import(ctx, ImportRequest{YAML: []byte(draftKeep), WordsFrom: v2.ID, Activate: true,
		WordOps: WordOps{WordsRemove: []int{0}, WordsAdd: []string{"Neu AG", "muster gmbh"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := exportWords(t, e, v3.ID); !reflect.DeepEqual(got, []string{"Muster GmbH", "Neu AG"}) {
		t.Fatalf("v3 words = %q", got)
	}
	// Version 2 is unchanged.
	if got := exportWords(t, e, v2.ID); !reflect.DeepEqual(got, []string{"Projekt Falke", "Muster GmbH"}) {
		t.Fatalf("v2 words = %q", got)
	}
	// Without words_from the operations apply to the draft list.
	v4, err := e.m.Import(ctx, ImportRequest{YAML: []byte(agentsExample), WordOps: WordOps{WordsRemove: []int{1}, WordsAdd: []string{"Zweite KG"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := exportWords(t, e, v4.ID); !reflect.DeepEqual(got, []string{"Projekt Falke", "Zweite KG"}) {
		t.Fatalf("v4 words = %q", got)
	}
	// Dry run and live test see the result.
	res, err := e.m.Test(ctx, TestRequest{Text: "Neu AG und Projekt Falke", YAML: []byte(draftKeep), WordsFrom: v2.ID,
		WordOps: WordOps{WordsRemove: []int{0}, WordsAdd: []string{"Neu AG"}}})
	if err != nil || res.Redacted != "<redacted:word> und Projekt Falke" {
		t.Fatalf("test: %v %+v", err, res)
	}
	dr, err := e.m.DryRunDraft(ctx, Draft{YAML: []byte(draftKeep), WordsFrom: v2.ID, WordOps: WordOps{WordsRemove: []int{0, 1}}})
	if err != nil || dr.NeedsSealer {
		t.Fatalf("dry run: %v %+v", err, dr)
	}
	// Removing every entry gives a version without words.
	v5, err := e.m.Import(ctx, ImportRequest{YAML: []byte(draftKeep), WordsFrom: v2.ID, WordOps: WordOps{WordsRemove: []int{1, 0}}})
	if err != nil || v5.HasWords {
		t.Fatalf("v5: %v %+v", err, v5)
	}
}

func TestWordOpsAPI(t *testing.T) {
	e, srv := newAPI(t, true)
	v2 := e.importYAML(t, agentsExample, true)
	secrets := []string{"Projekt Falke", "Muster GmbH", "Neu AG", "Zweite KG"}
	var outputs []string

	ops := WordOps{WordsRemove: []int{1}, WordsAdd: []string{"Neu AG", "Zweite KG"}}
	code, _, body := call(t, srv, "POST", "/api/config/dry-run", DryRunRequest{YAML: draftKeep, WordsFrom: v2.ID, WordOps: ops})
	outputs = append(outputs, body)
	if code != 200 {
		t.Fatalf("dry-run: %d %s", code, body)
	}
	code, _, body = call(t, srv, "POST", "/api/config/test", TestBody{Text: "Zweite KG", YAML: draftKeep, WordsFrom: v2.ID, WordOps: ops})
	outputs = append(outputs, body)
	var tr TestResult
	if code != 200 || json.Unmarshal([]byte(body), &tr) != nil || wordTypes(tr.Hits) != 1 {
		t.Fatalf("test: %d %s", code, body)
	}
	code, _, body = call(t, srv, "POST", "/api/config/import", ImportBody{YAML: draftKeep, WordsFrom: v2.ID, Activate: true, WordOps: ops})
	outputs = append(outputs, body)
	var v VersionInfo
	if code != 201 || json.Unmarshal([]byte(body), &v) != nil || !v.HasWords || !strings.Contains(v.Diff, "N••••••••G") {
		t.Fatalf("import: %d %s", code, body)
	}
	if got := exportWords(t, e, v.ID); !reflect.DeepEqual(got, []string{"Projekt Falke", "Neu AG", "Zweite KG"}) {
		t.Fatalf("words = %q", got)
	}
	code, _, body = call(t, srv, "GET", "/api/config/versions", nil)
	outputs = append(outputs, body)
	code2, _, body2 := call(t, srv, "GET", "/api/config/versions/3", nil)
	outputs = append(outputs, body2)
	if code != 200 || code2 != 200 {
		t.Fatalf("versions: %d %d", code, code2)
	}

	errs := []struct {
		name     string
		path     string
		body     any
		code     int
		contains string
	}{
		{"index out of range", "/api/config/dry-run", DryRunRequest{YAML: draftKeep, WordsFrom: v2.ID, WordOps: WordOps{WordsRemove: []int{5}}}, 400, "Index 5"},
		{"duplicate index", "/api/config/import", ImportBody{YAML: draftKeep, WordsFrom: v2.ID, WordOps: WordOps{WordsRemove: []int{0, 0}}}, 400, "doppelt"},
		{"index without base", "/api/config/test", TestBody{Text: "x", YAML: draftKeep, WordOps: WordOps{WordsRemove: []int{0}}}, 400, "Index 0"},
		{"masked word", "/api/config/import", ImportBody{YAML: draftKeep, WordsFrom: v2.ID, WordOps: WordOps{WordsAdd: []string{"N" + maskDots + "G"}}}, 422, "words_add[0]"},
		{"bad type", "/api/config/import", map[string]any{"yaml": draftKeep, "words_remove": []string{"a"}}, 400, "ungültig"},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			code, _, body := call(t, srv, "POST", tt.path, tt.body)
			outputs = append(outputs, body)
			if code != tt.code || !strings.Contains(body, tt.contains) {
				t.Fatalf("got %d %s, want %d containing %q", code, body, tt.code, tt.contains)
			}
		})
	}

	db := readDBFiles(t, e.dbPath)
	for _, w := range secrets {
		if strings.Contains(e.logs.String(), w) {
			t.Errorf("word %q in logs", w)
		}
		if strings.Contains(string(db), w) {
			t.Errorf("word %q in database files", w)
		}
		for i, o := range outputs {
			if strings.Contains(o, w) {
				t.Errorf("word %q in response %d: %s", w, i, o)
			}
		}
	}
}
