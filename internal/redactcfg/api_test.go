package redactcfg

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/config/schema"
)

func newAPI(t *testing.T, withSealer bool) (*env, *httptest.Server) {
	t.Helper()
	e := newEnv(t, withSealer)
	mux := http.NewServeMux()
	e.m.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return e, srv
}

func call(t *testing.T, srv *httptest.Server, method, path string, body any) (int, http.Header, string) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(method, srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func TestAPIFlow(t *testing.T) {
	e, srv := newAPI(t, true)
	var all []string

	code, hdr, body := call(t, srv, "GET", "/api/config/schema", nil)
	if code != 200 || body != string(schema.ConfigV1) || hdr.Get("Content-Type") != "application/schema+json" {
		t.Fatalf("schema: %d %s", code, hdr)
	}

	code, _, body = call(t, srv, "POST", "/api/config/dry-run", DryRunRequest{YAML: agentsExample})
	all = append(all, body)
	var dr DryRunResult
	if code != 200 || json.Unmarshal([]byte(body), &dr) != nil || dr.ActiveVersion != 1 || len(dr.Fixtures) == 0 {
		t.Fatalf("dry-run: %d %s", code, body)
	}

	code, _, body = call(t, srv, "POST", "/api/config/import", ImportBody{YAML: agentsExample, Comment: "Kunden", Activate: true})
	all = append(all, body)
	var v VersionInfo
	if code != 201 || json.Unmarshal([]byte(body), &v) != nil || v.ID != 2 || !v.Active || v.Author != AuthorImport || v.Comment != "Kunden" {
		t.Fatalf("import: %d %s", code, body)
	}

	code, _, body = call(t, srv, "GET", "/api/config/versions", nil)
	all = append(all, body)
	var vl VersionList
	if code != 200 || json.Unmarshal([]byte(body), &vl) != nil || vl.Active != 2 || len(vl.Versions) != 2 {
		t.Fatalf("versions: %d %s", code, body)
	}

	code, _, body = call(t, srv, "GET", "/api/config/versions/2", nil)
	all = append(all, body)
	var vd VersionDetail
	if code != 200 || json.Unmarshal([]byte(body), &vd) != nil || !strings.Contains(vd.YAML, "P••••••••e") {
		t.Fatalf("version: %d %s", code, body)
	}

	code, hdr, body = call(t, srv, "GET", "/api/config/export", nil)
	all = append(all, body)
	if code != 200 || !strings.Contains(body, "list: []") || hdr.Get(ExportWarningHeader) != "" ||
		!strings.Contains(hdr.Get("Content-Disposition"), "tollgate-config-v2.yaml") || hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("export: %d %v %s", code, hdr, body)
	}
	code, hdr, body = call(t, srv, "GET", "/api/config/export?version=2&words=plain", nil)
	if code != 200 || !strings.HasPrefix(body, PlainWordsWarning+"\n") || hdr.Get(ExportWarningHeader) == "" || !strings.Contains(body, "Projekt Falke") {
		t.Fatalf("plain export: %d %v %s", code, hdr, body)
	}
	// The plain export re-imports to the same configuration.
	d1, _ := Parse([]byte(agentsExample))
	d2, err := Parse([]byte(body))
	if err != nil || string(Export(d1, ExportPlainWords)) != string(Export(d2, ExportPlainWords)) {
		t.Fatalf("plain export does not round-trip: %v", err)
	}

	code, _, body = call(t, srv, "POST", "/api/config/test", TestBody{Text: "KD-123456 und Projekt Falke"})
	all = append(all, body)
	var tr TestResult
	if code != 200 || json.Unmarshal([]byte(body), &tr) != nil || tr.Redacted != "<redacted:customer_id> und <redacted:word>" || len(tr.Hits) != 2 {
		t.Fatalf("test: %d %s", code, body)
	}

	code, _, body = call(t, srv, "POST", "/api/config/versions/1/activate", nil)
	if code != 200 || e.m.Active().Version != 1 {
		t.Fatalf("activate: %d %s", code, body)
	}
	code, _, body = call(t, srv, "POST", "/api/config/versions/2/rollback", nil)
	all = append(all, body)
	if code != 201 || e.m.Active().Version != 3 {
		t.Fatalf("rollback: %d %s", code, body)
	}

	for i, b := range all {
		if strings.Contains(b, "Projekt Falke") || strings.Contains(b, "Muster GmbH") {
			t.Errorf("response %d leaks a word: %s", i, b)
		}
	}
}

func TestAPIErrors(t *testing.T) {
	_, srv := newAPI(t, false)
	tests := []struct {
		name, method, path string
		body               any
		code               int
		contains           string
	}{
		{"invalid yaml", "POST", "/api/config/dry-run", DryRunRequest{YAML: "version: 1\nbuiltin:\n  emial: true\n"}, 422, `"line":3`},
		{"import invalid", "POST", "/api/config/import", ImportBody{YAML: "version: 1\ncustom:\n  - {name: k, pattern: 'KD-\\d{6}', placeholder: customer_id, examples: [KD-1]}\n"}, 422, "custom[0].examples[0]"},
		{"no sealer", "POST", "/api/config/import", ImportBody{YAML: wordsV2}, 422, "Verschlüsselung"},
		{"unknown field", "POST", "/api/config/import", map[string]any{"yaml": "version: 1\n", "x": 1}, 400, "ungültig"},
		{"bad id", "GET", "/api/config/versions/abc", nil, 400, "Versions-ID"},
		{"missing version", "GET", "/api/config/versions/42", nil, 404, "nicht gefunden"},
		{"activate missing", "POST", "/api/config/versions/42/activate", nil, 404, "nicht gefunden"},
		{"rollback missing", "POST", "/api/config/versions/42/rollback", nil, 404, "nicht gefunden"},
		{"bad words mode", "GET", "/api/config/export?words=all", nil, 400, "words"},
		{"bad export version", "GET", "/api/config/export?version=-1", nil, 400, "version"},
		{"comment too long", "POST", "/api/config/import", ImportBody{YAML: "version: 1\n", Comment: strings.Repeat("x", 2000)}, 400, "Kommentar"},
		{"test bad draft", "POST", "/api/config/test", TestBody{Text: "x", YAML: "version: 3\n"}, 422, "version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, body := call(t, srv, tt.method, tt.path, tt.body)
			if code != tt.code || !strings.Contains(body, tt.contains) {
				t.Fatalf("got %d %s, want %d containing %q", code, body, tt.code, tt.contains)
			}
		})
	}
}
