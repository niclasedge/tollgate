package redactcfg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/compress"
)

func TestCompressRulesAPI(t *testing.T) {
	e, srv := newAPI(t, false)
	code, _, body := call(t, srv, "GET", "/api/compress/rules", nil)
	var l compress.RuleList
	if code != 200 || json.Unmarshal([]byte(body), &l) != nil {
		t.Fatalf("rules: %d %s", code, body)
	}
	if l.Version != e.m.Active().Version || l.Enabled || l.MinBytes != compress.DefaultMinBytes || len(l.Rules) != len(compress.Default().Rules) {
		t.Errorf("list = %+v", l)
	}

	v2 := e.importYAML(t, compressV2, true)
	_, _, body = call(t, srv, "GET", "/api/compress/rules", nil)
	l = compress.RuleList{}
	if json.Unmarshal([]byte(body), &l) != nil || l.Version != v2.ID || l.MinBytes != 4096 || l.Rules[0].Name != "go_test_off" ||
		l.Rules[0].Builtin || l.Rules[1].ContextLines != 1 || !l.Rules[2].Builtin {
		t.Errorf("list after import = %s", body)
	}
}

func TestCompressPlaygroundAPI(t *testing.T) {
	e, srv := newAPI(t, false)
	v1 := e.m.Active().Version
	out := strings.Repeat("=== RUN   TestX\n--- PASS: TestX (0.00s)\n", 120) + "ok  \tex/pkg\t0.1s\n"

	explain := func(req compress.PlaygroundRequest) compress.Explanation {
		t.Helper()
		code, _, body := call(t, srv, "POST", "/api/compress/playground", req)
		var ex compress.Explanation
		if code != 200 || json.Unmarshal([]byte(body), &ex) != nil {
			t.Fatalf("playground: %d %s", code, body)
		}
		return ex
	}
	ex := explain(compress.PlaygroundRequest{Command: "go test -v ./...", Output: out})
	if !ex.Applied || ex.Version != v1 || ex.Rule != "go_test" || ex.BytesAfter >= ex.BytesBefore || len(ex.Steps) < 3 {
		t.Errorf("active version: %+v", ex)
	}

	// The new active version switches go test off; version v1 still compresses.
	e.importYAML(t, compressV2+"    - name: go_off\n      match: [\"go test\"]\n      strategy: passthrough\n", true)
	if ex := explain(compress.PlaygroundRequest{Command: "go test -v ./...", Output: out}); ex.Applied || ex.Rule != "go_off" {
		t.Errorf("new active version: %+v", ex.Rule)
	}
	if ex := explain(compress.PlaygroundRequest{Command: "go test -v ./...", Output: out, Version: v1}); !ex.Applied || ex.Version != v1 {
		t.Errorf("pinned version: %+v", ex.Rule)
	}

	if code, _, _ := call(t, srv, "POST", "/api/compress/playground", compress.PlaygroundRequest{Command: "x", Version: 99}); code != 404 {
		t.Errorf("unknown version: %d", code)
	}
	if code, _, _ := call(t, srv, "POST", "/api/compress/playground", map[string]string{"cmd": "x"}); code != 400 {
		t.Errorf("unknown field: %d", code)
	}
	if strings.Contains(e.logs.String(), "TestX") {
		t.Error("playground text must not be logged")
	}
}
