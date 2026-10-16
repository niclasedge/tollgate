package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/niclasedge/tollgate/internal/auth"
)

// TestCompressSwitchWiring checks that compress.enabled hangs the compress
// stage into the pipeline (#19): with it on, a large go test output of a
// Bash call reaches the upstream shortened and marked, a Read result of the
// same size stays unchanged; by default both pass unchanged.
func TestCompressSwitchWiring(t *testing.T) {
	var out strings.Builder
	for i := range 120 {
		fmt.Fprintf(&out, "ok  \texample.com/app/pkg%03d\t0.%03ds\n", i, i)
	}
	out.WriteString("--- FAIL: TestBroken (0.00s)\n    broken_test.go:12: want 1, got 2\nFAIL\nFAIL\texample.com/app/broken\t0.010s\n")
	raw, _ := json.Marshal(out.String())
	msg := `{"model":"claude-test","messages":[` +
		`{"role":"assistant","content":[{"type":"tool_use","id":"b1","name":"Bash","input":{"command":"go test ./..."}},` +
		`{"type":"tool_use","id":"r1","name":"Read","input":{"file_path":"/src/app/out.txt"}}]},` +
		`{"role":"user","content":[{"type":"tool_result","tool_use_id":"b1","content":` + string(raw) + `},` +
		`{"type":"tool_result","tool_use_id":"r1","content":` + string(raw) + `}]}]}`

	for _, on := range []bool{false, true} {
		t.Run(map[bool]string{false: "default off", true: "on"}[on], func(t *testing.T) {
			var mu sync.Mutex
			var bodies []string
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/messages" {
					b, _ := io.ReadAll(r.Body)
					mu.Lock()
					bodies = append(bodies, string(b))
					mu.Unlock()
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ok"}]}`)
			}))
			t.Cleanup(up.Close)

			conf := writeConf(t, "127.0.0.1:0", up.URL)
			if on {
				f, err := os.OpenFile(conf, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = f.WriteString("compress:\n  enabled: true\n")
				f.Close()
			}
			addr, _ := serveConf(t, conf)
			base := "http://" + addr
			token, _ := os.ReadFile(filepath.Join(filepath.Dir(conf), "admin.token"))
			code, body := call(t, http.MethodPost, base+"/api/clients", strings.TrimSpace(string(token)), `{"name":"laptop"}`)
			var created auth.CreateClientResponse
			if code != 201 || json.Unmarshal(body, &created) != nil {
				t.Fatalf("create client = %d %s", code, body)
			}
			if code, answer := call(t, http.MethodPost, base+"/v1/messages", created.Key, msg); code != 200 {
				t.Fatalf("POST /v1/messages = %d %s", code, answer)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(bodies) != 1 {
				t.Fatalf("upstream bodies = %d", len(bodies))
			}
			var sent struct {
				Messages []struct {
					Content []struct {
						Content string `json:"content"`
					} `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal([]byte(bodies[0]), &sent); err != nil {
				t.Fatal(err)
			}
			bash, read := sent.Messages[1].Content[0].Content, sent.Messages[1].Content[1].Content
			if read != out.String() {
				t.Error("Read result must never be compressed")
			}
			if !on {
				if bash != out.String() {
					t.Error("compress is off by default")
				}
				return
			}
			if len(bash) >= out.Len() || !strings.Contains(bash, "[tollgate: go_test · ") ||
				!strings.Contains(bash, "--- FAIL: TestBroken") || !strings.Contains(bash, "want 1, got 2") {
				t.Errorf("bash result not compressed as expected:\n%s", bash)
			}
		})
	}
}
