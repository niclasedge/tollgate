package redactcfg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/store"
)

// draftKeep is agentsExample with an empty word list, as the filter view
// sends it while the word list is taken over from a stored version.
var draftKeep = strings.Replace(agentsExample, `list: ["Projekt Falke", "Muster GmbH"]`, `list: []`, 1)

func wordTypes(hs []Hit) int {
	n := 0
	for _, h := range hs {
		if h.Type == "word" {
			n++
		}
	}
	return n
}

func TestWordsFromVersion(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, true)
	v2 := e.importYAML(t, agentsExample, true)

	// Import takes the stored list over.
	v3, err := e.m.Import(ctx, ImportRequest{YAML: []byte(draftKeep), WordsFrom: v2.ID, Activate: true})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := e.m.Export(ctx, v3.ID, ExportPlainWords)
	if err != nil || !strings.Contains(string(plain), "Projekt Falke") || !strings.Contains(string(plain), "Muster GmbH") {
		t.Fatalf("export v3: %v\n%s", err, plain)
	}
	if strings.Contains(v3.Diff, "Projekt Falke") {
		t.Fatalf("diff leaks a word: %s", v3.Diff)
	}

	// Dry run and live test see the same words.
	dr, err := e.m.DryRunDraft(ctx, Draft{YAML: []byte(draftKeep), WordsFrom: v2.ID})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range dr.Fixtures {
		if wordTypes(f.Candidate) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("dry run without the taken-over words: %+v", dr.Fixtures)
	}
	res, err := e.m.Test(ctx, TestRequest{Text: "Projekt Falke startet", YAML: []byte(draftKeep), WordsFrom: v2.ID})
	if err != nil || wordTypes(res.Hits) != 1 || strings.Contains(res.Redacted, "Falke") {
		t.Fatalf("test: %v %+v", err, res)
	}
	// Without words_from the draft has no words.
	res, err = e.m.Test(ctx, TestRequest{Text: "Projekt Falke startet", YAML: []byte(draftKeep)})
	if err != nil || wordTypes(res.Hits) != 0 {
		t.Fatalf("test without words_from: %v %+v", err, res)
	}
}

func issuePaths(t *testing.T, err error) []string {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	var ps []string
	for _, is := range ve.Issues {
		ps = append(ps, is.Path)
	}
	return ps
}

func TestWordsFromRejections(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, true)
	v2 := e.importYAML(t, agentsExample, true)

	// A list and words_from together.
	_, err := e.m.Import(ctx, ImportRequest{YAML: []byte(agentsExample), WordsFrom: v2.ID})
	if ps := issuePaths(t, err); len(ps) != 1 || ps[0] != "words.list" {
		t.Fatalf("paths = %v", ps)
	}
	// Unknown version.
	if _, err := e.m.Import(ctx, ImportRequest{YAML: []byte(draftKeep), WordsFrom: 99}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown version: %v", err)
	}
	// The masked YAML of a version is not importable, and the issue does not
	// quote the entry.
	vd, err := e.m.Version(ctx, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.m.Import(ctx, ImportRequest{YAML: []byte(vd.YAML)})
	ps := issuePaths(t, err)
	if len(ps) != 2 || ps[0] != "words.list[0]" || ps[1] != "words.list[1]" {
		t.Fatalf("masked paths = %v", ps)
	}
	if strings.Contains(err.Error(), "••••") || strings.Contains(err.Error(), "P•") {
		t.Fatalf("issue quotes the entry: %v", err)
	}
	// Short entries are masked as the filler alone.
	short := strings.Replace(draftKeep, `list: []`, `list: ["••••••••"]`, 1)
	if ps := issuePaths(t, func() error { _, err := Parse([]byte(short)); return err }()); len(ps) != 1 || ps[0] != "words.list[0]" {
		t.Fatalf("short masked paths = %v", ps)
	}
}

func TestWordsFromAPI(t *testing.T) {
	e, srv := newAPI(t, true)
	v2 := e.importYAML(t, agentsExample, true)

	code, _, body := call(t, srv, "POST", "/api/config/dry-run", DryRunRequest{YAML: draftKeep, WordsFrom: v2.ID})
	if code != 200 {
		t.Fatalf("dry-run: %d %s", code, body)
	}
	code, _, body = call(t, srv, "POST", "/api/config/test", TestBody{Text: "Muster GmbH", YAML: draftKeep, WordsFrom: v2.ID})
	var tr TestResult
	if code != 200 || json.Unmarshal([]byte(body), &tr) != nil || wordTypes(tr.Hits) != 1 {
		t.Fatalf("test: %d %s", code, body)
	}
	code, _, body = call(t, srv, "POST", "/api/config/import", ImportBody{YAML: draftKeep, WordsFrom: v2.ID, Activate: true})
	var v VersionInfo
	if code != 201 || json.Unmarshal([]byte(body), &v) != nil || !v.HasWords {
		t.Fatalf("import: %d %s", code, body)
	}
	code, _, body = call(t, srv, "POST", "/api/config/import", ImportBody{YAML: draftKeep, WordsFrom: 99})
	if code != 404 {
		t.Fatalf("unknown words_from: %d %s", code, body)
	}
	for _, b := range []string{body} {
		if strings.Contains(b, "Falke") || strings.Contains(b, "Muster GmbH") {
			t.Fatalf("plaintext word in %s", b)
		}
	}
}
