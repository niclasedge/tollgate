package redactcfg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/config/schema"
	"github.com/niclasedge/tollgate/internal/store"
)

// Invented names only.
const namesYAML = `version: 1
words:
  case_sensitive: false
  whole_word: true
  list: ["Projekt Falke"]
  customers: ["Muster GmbH", "Zander AG"]
  first_names: ["Anna", "Bernd"]
  last_names: ["Müller", "Schmidt"]
`

// namesKeep is a draft without any list, as the filter view sends it while
// the lists are taken over from a stored version.
const namesKeep = "version: 1\n"

// sl returns a pointer to a non-nil list, as lists_set carries it.
func sl(s ...string) *[]string {
	if s == nil {
		s = []string{}
	}
	return &s
}

var nameCanaries = []string{"Projekt Falke", "Muster GmbH", "Zander AG", "Anna", "Bernd", "Müller", "Schmidt", "Neu KG", "Carla"}

func TestNameListsYAMLRoundTrip(t *testing.T) {
	d, err := Parse([]byte(namesYAML))
	if err != nil {
		t.Fatal(err)
	}
	w := d.Words
	if !reflect.DeepEqual(w.Customers, []string{"Muster GmbH", "Zander AG"}) ||
		!reflect.DeepEqual(w.FirstNames, []string{"Anna", "Bernd"}) ||
		!reflect.DeepEqual(w.LastNames, []string{"Müller", "Schmidt"}) ||
		!reflect.DeepEqual(w.List, []string{"Projekt Falke"}) {
		t.Fatalf("words = %+v", w)
	}
	plain := Export(d, ExportPlainWords)
	back, err := Parse(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, d) {
		t.Fatalf("round trip differs:\n%s", plain)
	}
	if !bytes.Equal(Export(back, ExportPlainWords), plain) {
		t.Fatal("export is not deterministic")
	}

	omit := string(Export(d, ExportOmitWords))
	for _, want := range []string{"  list: []\n", "  customers: []\n", "  first_names: []\n", "  last_names: []\n"} {
		if !strings.Contains(omit, want) {
			t.Errorf("omit export lacks %q:\n%s", want, omit)
		}
	}
	masked := string(Export(d, ExportMaskedWords))
	for _, w := range nameCanaries[:7] {
		if strings.Contains(omit, w) || strings.Contains(masked, w) {
			t.Errorf("%q in omit or masked export", w)
		}
	}
	if !strings.Contains(masked, "M••••••••H") {
		t.Errorf("masked export lacks the mask of the first customer:\n%s", masked)
	}
}

func TestOldYAMLWithoutNameListsStaysValid(t *testing.T) {
	d, err := Parse([]byte(draftKeep))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Words.Customers)+len(d.Words.FirstNames)+len(d.Words.LastNames) != 0 {
		t.Fatalf("words = %+v", d.Words)
	}
	// An empty or missing words block works as before.
	for _, src := range []string{"version: 1\n", "version: 1\nwords: {}\n", "version: 1\nwords:\n  customers: []\n"} {
		if _, err := Parse([]byte(src)); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

func TestNameListSchemaErrors(t *testing.T) {
	tests := []struct{ name, src, path string }{
		{"blank entry", "version: 1\nwords:\n  customers: [\"\"]\n", "words.customers[0]"},
		{"too long", "version: 1\nwords:\n  first_names: [\"" + strings.Repeat("x", 201) + "\"]\n", "words.first_names[0]"},
		{"not a list", "version: 1\nwords:\n  last_names: Müller\n", "words.last_names"},
		{"unknown key", "version: 1\nwords:\n  kunden: [\"a\"]\n", "words"},
		{"masked entry", "version: 1\nwords:\n  customers: [\"M" + maskDots + "H\"]\n", "words.customers[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.src))
			ps := issuePaths(t, err)
			if len(ps) != 1 || ps[0] != tt.path {
				t.Fatalf("paths = %v, want %s (%v)", ps, tt.path, err)
			}
			if strings.Contains(err.Error(), "Müller") {
				t.Fatalf("error quotes an entry: %v", err)
			}
		})
	}
}

// TestNameListLimitsMatchSchema keeps the limits of lists_set equal to the
// limits of the YAML schema for all three lists.
func TestNameListLimitsMatchSchema(t *testing.T) {
	var s struct {
		Properties struct {
			Words struct {
				Properties map[string]struct {
					MaxItems int `json:"maxItems"`
					Items    struct {
						MinLength int `json:"minLength"`
						MaxLength int `json:"maxLength"`
					} `json:"items"`
				} `json:"properties"`
			} `json:"words"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema.ConfigV1, &s); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"customers", "first_names", "last_names"} {
		l, ok := s.Properties.Words.Properties[k]
		if !ok || l.MaxItems != maxWords || l.Items.MaxLength != maxWordRunes || l.Items.MinLength != 1 {
			t.Errorf("%s: schema %+v, code %d/%d", k, l, maxWords, maxWordRunes)
		}
	}
}

func TestApplyListsSet(t *testing.T) {
	tests := []struct {
		name          string
		in            ListsSet
		caseSensitive bool
		want          ListsSet
	}{
		{"keeps order", ListsSet{Customers: sl("B", "A")}, false, ListsSet{Customers: sl("B", "A")}},
		{"absent lists stay absent", ListsSet{LastNames: sl("Müller")}, false, ListsSet{LastNames: sl("Müller")}},
		{"trims and drops empty lines", ListsSet{FirstNames: sl("  Anna ", "", "\t", "Bernd\n")}, false, ListsSet{FirstNames: sl("Anna", "Bernd")}},
		{"duplicates fold case", ListsSet{LastNames: sl("Müller", "MÜLLER", "müller", "Schmidt")}, false, ListsSet{LastNames: sl("Müller", "Schmidt")}},
		{"duplicates case-sensitive", ListsSet{LastNames: sl("Müller", "MÜLLER", "Müller")}, true, ListsSet{LastNames: sl("Müller", "MÜLLER")}},
		{"lists are independent", ListsSet{Customers: sl("Falke"), LastNames: sl("Falke")}, false, ListsSet{Customers: sl("Falke"), LastNames: sl("Falke")}},
		{"empty clears", ListsSet{Customers: sl(), FirstNames: sl("", " ")}, false, ListsSet{Customers: sl(), FirstNames: sl()}},
		{"nothing set", ListsSet{}, false, ListsSet{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyListsSet(tt.in, tt.caseSensitive)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %s (%v), want %s", dump(got), err, dump(tt.want))
			}
			again, _ := applyListsSet(tt.in, tt.caseSensitive)
			if !reflect.DeepEqual(got, again) {
				t.Fatal("not deterministic")
			}
		})
	}
}

func dump(l ListsSet) string {
	b, _ := json.Marshal(l)
	return string(b)
}

func TestApplyListsSetErrors(t *testing.T) {
	long := strings.Repeat("x", maxWordRunes+1)
	many := make([]string, maxWords+1)
	for i := range many {
		many[i] = "n" + strings.Repeat("a", i%40) + string(rune('A'+i%26)) + strings.Repeat("b", i/40)
	}
	tests := []struct {
		name string
		in   ListsSet
		path string
	}{
		{"too long", ListsSet{Customers: sl("ok", long)}, "lists_set.customers[1]"},
		{"masked", ListsSet{FirstNames: sl("A" + maskDots + "e")}, "lists_set.first_names[0]"},
		{"too many", ListsSet{LastNames: &many}, "lists_set.last_names"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := applyListsSet(tt.in, false)
			var ve *ValidationError
			if !errors.As(err, &ve) || len(ve.Issues) != 1 || ve.Issues[0].Path != tt.path {
				t.Fatalf("err = %v, want issue at %s", err, tt.path)
			}
			if strings.Contains(err.Error(), long) || strings.Contains(err.Error(), "ok") {
				t.Fatalf("error quotes an entry")
			}
		})
	}
}

func listsOf(t *testing.T, e *env, id int64) Words {
	t.Helper()
	plain, err := e.m.Export(context.Background(), id, ExportPlainWords)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(plain)
	if err != nil {
		t.Fatal(err)
	}
	return d.Words
}

func TestListsFromVersionAndListsSet(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, true)
	v2 := e.importYAML(t, namesYAML, true)

	// words_from alone takes over all four lists.
	v3, err := e.m.Import(ctx, ImportRequest{YAML: []byte(namesKeep), WordsFrom: v2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if w := listsOf(t, e, v3.ID); !reflect.DeepEqual(w, listsOf(t, e, v2.ID)) {
		t.Fatalf("v3 = %+v", w)
	}

	// lists_set replaces the lists that are present completely; words.list stays.
	set := WordOps{ListsSet: &ListsSet{Customers: sl(" Neu KG ", "", "neu kg"), FirstNames: sl(), LastNames: sl("Schmidt")}}
	v4, err := e.m.Import(ctx, ImportRequest{YAML: []byte(namesKeep), WordsFrom: v2.ID, WordOps: set, Activate: true})
	if err != nil {
		t.Fatal(err)
	}
	w := listsOf(t, e, v4.ID)
	if !reflect.DeepEqual(w.Customers, []string{"Neu KG"}) || len(w.FirstNames) != 0 || !reflect.DeepEqual(w.LastNames, []string{"Schmidt"}) ||
		!reflect.DeepEqual(w.List, []string{"Projekt Falke"}) {
		t.Fatalf("v4 = %+v", w)
	}
	// Version 2 is unchanged.
	if w := listsOf(t, e, v2.ID); len(w.Customers) != 2 || len(w.FirstNames) != 2 {
		t.Fatalf("v2 = %+v", w)
	}
	// Together with words_add/words_remove on words.list.
	both := WordOps{WordsRemove: []int{0}, WordsAdd: []string{"Zweit"}, ListsSet: &ListsSet{FirstNames: sl("Carla"), Customers: sl()}}
	v5, err := e.m.Import(ctx, ImportRequest{YAML: []byte(namesKeep), WordsFrom: v2.ID, WordOps: both})
	if err != nil {
		t.Fatal(err)
	}
	if w := listsOf(t, e, v5.ID); !reflect.DeepEqual(w.List, []string{"Zweit"}) || !reflect.DeepEqual(w.FirstNames, []string{"Carla"}) || len(w.Customers) != 0 || !reflect.DeepEqual(w.LastNames, []string{"Müller", "Schmidt"}) {
		t.Fatalf("v5 = %+v", w)
	}
	// Empty lists clear the three lists but keep words.list.
	v6, err := e.m.Import(ctx, ImportRequest{YAML: []byte(namesKeep), WordsFrom: v2.ID, WordOps: WordOps{ListsSet: &ListsSet{Customers: sl(), FirstNames: sl(), LastNames: sl()}}})
	if err != nil {
		t.Fatal(err)
	}
	if w := listsOf(t, e, v6.ID); !reflect.DeepEqual(w.List, []string{"Projekt Falke"}) || len(w.Customers)+len(w.FirstNames)+len(w.LastNames) != 0 {
		t.Fatalf("v6 = %+v", w)
	}
	// Without words_from lists_set replaces the lists of the draft.
	v7, err := e.m.Import(ctx, ImportRequest{YAML: []byte(namesYAML), WordOps: WordOps{ListsSet: &ListsSet{Customers: sl("Neu KG"), FirstNames: sl()}}})
	if err != nil {
		t.Fatal(err)
	}
	if w := listsOf(t, e, v7.ID); !reflect.DeepEqual(w.Customers, []string{"Neu KG"}) || len(w.FirstNames) != 0 || !reflect.DeepEqual(w.LastNames, []string{"Müller", "Schmidt"}) || !reflect.DeepEqual(w.List, []string{"Projekt Falke"}) {
		t.Fatalf("v7 = %+v", w)
	}
}

func TestListsSetHonoursCaseSensitive(t *testing.T) {
	e := newEnv(t, true)
	yml := "version: 1\nwords:\n  case_sensitive: true\n"
	v, err := e.m.Import(context.Background(), ImportRequest{YAML: []byte(yml), WordOps: WordOps{ListsSet: &ListsSet{LastNames: sl("Müller", "MÜLLER")}}})
	if err != nil {
		t.Fatal(err)
	}
	if w := listsOf(t, e, v.ID); !reflect.DeepEqual(w.LastNames, []string{"Müller", "MÜLLER"}) {
		t.Fatalf("last names = %q", w.LastNames)
	}
}

func TestWordsFromRejectsDraftNameLists(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, true)
	v2 := e.importYAML(t, namesYAML, true)
	for key, path := range map[string]string{"customers": "words.customers", "first_names": "words.first_names", "last_names": "words.last_names"} {
		src := "version: 1\nwords:\n  " + key + ": [\"Neu KG\"]\n"
		_, err := e.m.Import(ctx, ImportRequest{YAML: []byte(src), WordsFrom: v2.ID})
		if ps := issuePaths(t, err); len(ps) != 1 || ps[0] != path {
			t.Errorf("%s: paths = %v", key, ps)
		}
	}
}

func TestNameListsRedactAndSealedOnly(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, true)
	v := e.importYAML(t, namesYAML, true)
	r, err := e.m.Redactor(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Redact("Anna Müller von Muster GmbH und Projekt Falke")
	if want := "<redacted:vorname> <redacted:nachname> von <redacted:kunde> und <redacted:word>"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// The stored content holds no entry; the sealed blob holds all four lists.
	sv, err := e.st.ConfigVersionByID(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range nameCanaries[:7] {
		if strings.Contains(string(sv.Content), w) {
			t.Errorf("%q in stored content", w)
		}
	}
	if !v.HasWords {
		t.Error("version with name lists has HasWords = false")
	}
	// Names only (no words.list) also count as words.
	nv := e.importYAML(t, "version: 1\nwords:\n  first_names: [\"Anna\"]\n", false)
	if !nv.HasWords {
		t.Error("only first_names: HasWords = false")
	}
	if dr, err := e.m.DryRunDraft(ctx, Draft{YAML: []byte("version: 1\nwords:\n  last_names: [\"Müller\"]\n")}); err != nil || dr.NeedsSealer {
		t.Fatalf("dry run: %v %+v", err, dr)
	}
}

func TestNameListsNeedSealer(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	_, err := e.m.Import(ctx, ImportRequest{YAML: []byte("version: 1\nwords:\n  customers: [\"Muster GmbH\"]\n")})
	if !errors.Is(err, ErrNoSealer) {
		t.Fatalf("err = %v, want ErrNoSealer", err)
	}
	dr, err := e.m.DryRunDraft(ctx, Draft{YAML: []byte("version: 1\nwords:\n  customers: [\"Muster GmbH\"]\n")})
	if err != nil || !dr.NeedsSealer {
		t.Fatalf("dry run: %v %+v", err, dr)
	}
}

// TestSealedBlobOldArrayFormat keeps versions readable that were sealed
// before #143: a JSON array holds only words.list.
func TestSealedBlobOldArrayFormat(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, true)
	v, err := e.st.CreateConfigVersion(ctx, store.NewConfigVersion{
		Author: "test", Comment: "old format", Content: Export(Default(), ExportOmitWords),
		Seal: func(id int64) ([]byte, error) {
			return e.sealer.Seal(aad(id), []byte(`["Projekt Falke","Muster GmbH"]`))
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.m.load(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Words.List, []string{"Projekt Falke", "Muster GmbH"}) || len(d.Words.Customers)+len(d.Words.FirstNames)+len(d.Words.LastNames) != 0 {
		t.Fatalf("words = %+v", d.Words)
	}
	// Taking the old list over and adding names stores the new format.
	nv, err := e.m.Import(ctx, ImportRequest{YAML: []byte(namesKeep), WordsFrom: v.ID, WordOps: WordOps{ListsSet: &ListsSet{Customers: sl("Zander AG")}}})
	if err != nil {
		t.Fatal(err)
	}
	if w := listsOf(t, e, nv.ID); !reflect.DeepEqual(w.List, []string{"Projekt Falke", "Muster GmbH"}) || !reflect.DeepEqual(w.Customers, []string{"Zander AG"}) {
		t.Fatalf("words = %+v", w)
	}
	// An unreadable blob is an error that names no entry.
	bad, err := e.st.CreateConfigVersion(ctx, store.NewConfigVersion{
		Author: "test", Content: Export(Default(), ExportOmitWords),
		Seal: func(id int64) ([]byte, error) { return e.sealer.Seal(aad(id), []byte(`"Projekt Falke"`)) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.load(ctx, bad.ID); err == nil || strings.Contains(err.Error(), "Falke") {
		t.Fatalf("err = %v", err)
	}
}

// TestNewNameListsOnlyForNewSessions: a session pinned to the version before
// the activation keeps its rules and so its prefix (prompt cache).
func TestNewNameListsOnlyForNewSessions(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	if _, err := e.st.CreateClient(ctx, store.Client{ID: "c1", Name: "laptop", KeyHash: sum("k"), KeyHint: "tg_abcd", Status: store.ClientActive}); err != nil {
		t.Fatal(err)
	}
	key := sum("c1|names")
	if _, err := e.st.PinSession(ctx, store.Session{KeyHash: key, ClientID: "c1", ConfigVersionID: e.m.Active().Version}); err != nil {
		t.Fatal(err)
	}
	v2 := e.importYAML(t, namesYAML, true)
	s, err := e.st.PinSession(ctx, store.Session{KeyHash: key, ClientID: "c1", ConfigVersionID: e.m.Active().Version})
	if err != nil || s.ConfigVersionID != 1 {
		t.Fatalf("pinned = %+v, %v", s, err)
	}
	text := "Muster GmbH meldet sich"
	old, err := e.m.Redactor(ctx, s.ConfigVersionID)
	if err != nil {
		t.Fatal(err)
	}
	if got := old.Redact(text); got != text {
		t.Fatalf("pinned session sees the new customer list: %q", got)
	}
	cur, _ := e.m.Redactor(ctx, v2.ID)
	if got := cur.Redact(text); got != "<redacted:kunde> meldet sich" {
		t.Fatalf("new session: %q", got)
	}
}

func TestNameListsAPI(t *testing.T) {
	e, srv := newAPI(t, true)
	v2 := e.importYAML(t, namesYAML, true)
	var outputs []string

	// The plaintext endpoint: the three lists of the requested or active version.
	code, hdr, body := call(t, srv, "GET", "/api/config/lists?version=2", nil)
	var got NameLists
	if code != 200 || json.Unmarshal([]byte(body), &got) != nil || hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("lists: %d %s %v", code, body, hdr)
	}
	want := NameLists{Customers: []string{"Muster GmbH", "Zander AG"}, FirstNames: []string{"Anna", "Bernd"}, LastNames: []string{"Müller", "Schmidt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lists = %+v", got)
	}
	if strings.Contains(body, "Falke") {
		t.Fatalf("general word list in the response: %s", body)
	}
	if code, _, b := call(t, srv, "GET", "/api/config/lists", nil); code != 200 || b != body {
		t.Fatalf("active version: %d %s", code, b)
	}
	// An older version without names gives empty lists, not null.
	code, _, body = call(t, srv, "GET", "/api/config/lists?version=1", nil)
	if code != 200 || !strings.Contains(body, `"customers":[]`) || !strings.Contains(body, `"first_names":[]`) || !strings.Contains(body, `"last_names":[]`) {
		t.Fatalf("empty lists: %d %s", code, body)
	}
	for _, tt := range []struct {
		path string
		code int
	}{{"/api/config/lists?version=99", 404}, {"/api/config/lists?version=x", 400}, {"/api/config/lists?version=-1", 400}} {
		code, _, b := call(t, srv, "GET", tt.path, nil)
		outputs = append(outputs, b)
		if code != tt.code {
			t.Errorf("%s: %d %s, want %d", tt.path, code, b, tt.code)
		}
	}

	// lists_set through dry run, test and import.
	set := WordOps{ListsSet: &ListsSet{FirstNames: sl("Carla"), Customers: sl("Neu KG"), LastNames: sl()}}
	code, _, body = call(t, srv, "POST", "/api/config/dry-run", DryRunRequest{YAML: namesKeep, WordsFrom: v2.ID, WordOps: set})
	outputs = append(outputs, body)
	if code != 200 {
		t.Fatalf("dry-run: %d %s", code, body)
	}
	code, _, body = call(t, srv, "POST", "/api/config/test", TestBody{Text: "Carla von Neu KG, Zed", YAML: namesKeep, WordsFrom: v2.ID, WordOps: set})
	outputs = append(outputs, body)
	var tr TestResult
	if code != 200 || json.Unmarshal([]byte(body), &tr) != nil || tr.Redacted != "<redacted:vorname> von <redacted:kunde>, Zed" {
		t.Fatalf("test: %d %s", code, body)
	}
	code, _, body = call(t, srv, "POST", "/api/config/import", ImportBody{YAML: namesKeep, WordsFrom: v2.ID, Activate: true, WordOps: set})
	outputs = append(outputs, body)
	var v VersionInfo
	if code != 201 || json.Unmarshal([]byte(body), &v) != nil || !v.HasWords {
		t.Fatalf("import: %d %s", code, body)
	}
	if strings.Contains(v.Diff, "Carla") || strings.Contains(v.Diff, "Neu KG") {
		t.Fatalf("diff leaks an entry: %s", v.Diff)
	}
	_, _, body = call(t, srv, "GET", "/api/config/lists", nil)
	if err := json.Unmarshal([]byte(body), &got); err != nil || !reflect.DeepEqual(got, NameLists{Customers: []string{"Neu KG"}, FirstNames: []string{"Carla"}, LastNames: []string{}}) {
		t.Fatalf("after import: %s", body)
	}

	// Errors name no entry.
	long := strings.Repeat("Z", maxWordRunes+1)
	for _, tt := range []struct {
		name, path string
		body       any
		code       int
		contains   string
	}{
		{"too long", "/api/config/import", ImportBody{YAML: namesKeep, WordsFrom: v2.ID, WordOps: WordOps{ListsSet: &ListsSet{LastNames: sl("ok", long)}}}, 422, "lists_set.last_names[1]"},
		{"masked", "/api/config/dry-run", DryRunRequest{YAML: namesKeep, WordsFrom: v2.ID, WordOps: WordOps{ListsSet: &ListsSet{Customers: sl("M" + maskDots + "H")}}}, 422, "lists_set.customers[0]"},
		{"draft lists with words_from", "/api/config/dry-run", DryRunRequest{YAML: namesYAML, WordsFrom: v2.ID}, 422, "words.list"},
		{"bad type", "/api/config/import", map[string]any{"yaml": namesKeep, "lists_set": map[string]any{"customers": "Muster"}}, 400, "ungültig"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code, _, body := call(t, srv, "POST", tt.path, tt.body)
			outputs = append(outputs, body)
			if code != tt.code || !strings.Contains(body, tt.contains) {
				t.Fatalf("got %d %s, want %d containing %q", code, body, tt.code, tt.contains)
			}
			if strings.Contains(body, long) {
				t.Fatalf("response quotes an entry")
			}
		})
	}

	// The leak test: no entry in the responses, the logs or the database
	// files, the plaintext endpoint aside.
	db := readDBFiles(t, e.dbPath)
	for _, w := range nameCanaries {
		if strings.Contains(e.logs.String(), w) {
			t.Errorf("%q in logs", w)
		}
		if strings.Contains(string(db), w) {
			t.Errorf("%q in database files", w)
		}
		for i, o := range outputs {
			if strings.Contains(o, w) {
				t.Errorf("%q in response %d: %s", w, i, o)
			}
		}
	}
	if !strings.Contains(e.logs.String(), "name lists read") {
		t.Error("the plaintext access is not logged")
	}
}

func TestNameListsEndpointWithoutSealer(t *testing.T) {
	_, srv := newAPI(t, false)
	// Without a sealer no version holds words, so the lists are empty.
	code, _, body := call(t, srv, "GET", "/api/config/lists", nil)
	if code != 200 || !strings.Contains(body, `"customers":[]`) {
		t.Fatalf("lists: %d %s", code, body)
	}
}

// TestNameListsLeakAcrossReadPaths imports names and uses every read path:
// versions, masked and omit exports, rollback, live test.
func TestNameListsLeakAcrossReadPaths(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	v := e.importYAML(t, namesYAML, true)
	var outputs []string
	vs, _ := e.m.Versions(ctx, 0)
	for _, x := range vs {
		outputs = append(outputs, x.Diff+x.Comment)
	}
	d, err := e.m.Version(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	outputs = append(outputs, d.YAML+d.Diff)
	for _, m := range []ExportMode{ExportOmitWords, ExportMaskedWords} {
		exp, _ := e.m.Export(ctx, v.ID, m)
		outputs = append(outputs, string(exp))
	}
	if _, err := e.m.Rollback(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	rb, err := e.m.Rollback(ctx, v.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	outputs = append(outputs, rb.Diff)
	res, _ := e.m.Test(ctx, TestRequest{Text: "Anna Müller, Muster GmbH"})
	outputs = append(outputs, res.Redacted)
	_, err = e.m.Import(ctx, ImportRequest{YAML: []byte("version: 1\nwords:\n  customers: [\"Muster GmbH\", 7]\n")})
	if err == nil {
		t.Fatal("invalid import accepted")
	}
	outputs = append(outputs, err.Error())

	db := readDBFiles(t, e.dbPath)
	for _, w := range nameCanaries[:7] {
		if bytes.Contains(db, []byte(w)) {
			t.Errorf("%q in database files", w)
		}
		if strings.Contains(e.logs.String(), w) {
			t.Errorf("%q in logs", w)
		}
		for i, o := range outputs {
			if strings.Contains(o, w) {
				t.Errorf("%q in output %d: %s", w, i, o)
			}
		}
	}
}
