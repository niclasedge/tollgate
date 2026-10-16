package compress

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
	"github.com/niclasedge/tollgate/internal/pipeline"
)

type fixed struct{ c *Config }

func (f fixed) Compressor(context.Context, int64) (*Config, error) { return f.c, nil }

func js(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func testConfig(t *testing.T) *Config {
	withStrategy(t, "keep_first", keepFirst)
	return &Config{MinBytes: 2048, Rules: []Rule{{Name: "go_test", Match: []string{"go test"}, Strategy: "keep_first"}}}
}

// turn builds a request with a Bash call, a Read call and their results.
// extra appends further messages, as a later turn does.
func turn(big string, extra ...string) string {
	msgs := []string{
		`{"role":"user","content":"run the tests"}`,
		`{"role":"assistant","content":[` +
			`{"type":"thinking","thinking":"go test ./...","signature":"SIG"},` +
			`{"type":"tool_use","id":"bash1","name":"Bash","input":{"command":"go test ./...","description":"tests"}},` +
			`{"type":"tool_use","id":"read1","name":"Read","input":{"file_path":"/src/app/main_test.go"}},` +
			`{"type":"tool_use","id":"bash2","name":"Bash","input":{"command":"go test ./... | tail"}},` +
			`{"type":"tool_use","id":"bash3","name":"Bash","input":{"command":"go test ./internal/..."}}]}`,
		`{"role":"user","content":[` +
			`{"type":"tool_result","tool_use_id":"bash1","is_error":true,"content":` + js(big) + `,"x_future":{"k":1}},` +
			`{"type":"tool_result","tool_use_id":"read1","content":` + js(big) + `},` +
			`{"type":"tool_result","tool_use_id":"bash2","content":` + js(big) + `},` +
			`{"type":"tool_result","tool_use_id":"bash3","content":[{"type":"text","text":` + js(big) + `},{"type":"image","source":{"type":"base64","data":"AAAA"}}]}]}`,
	}
	return `{"model":"claude-opus-5-5","max_tokens":10,"messages":[` + strings.Join(append(msgs, extra...), ",") + `]}`
}

func process(t *testing.T, s *Stage, body string) (string, *pipeline.Request) {
	t.Helper()
	req := &pipeline.Request{Endpoint: pipeline.EndpointMessages, Body: []byte(body)}
	reps, err := s.Process(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jsonsplice.Apply(req.Body, reps)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), req
}

func TestStageCompressesOnlyBashResults(t *testing.T) {
	s := NewStage(fixed{testConfig(t)})
	big := bigOutput(200)
	out, _ := process(t, s, turn(big))

	var doc struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	var results []map[string]any
	if err := json.Unmarshal(doc.Messages[2].Content, &results); err != nil {
		t.Fatal(err)
	}
	short := "=== RUN   TestSomething/case_\n[tollgate: go_test · 200 Zeilen ausgelassen · Test]"
	if results[0]["content"] != short {
		t.Errorf("bash result = %q", results[0]["content"])
	}
	if results[0]["is_error"] != true || results[0]["tool_use_id"] != "bash1" || results[0]["x_future"] == nil {
		t.Errorf("structure of the bash result changed: %v", results[0])
	}
	if results[1]["content"] != big {
		t.Error("Read result must never be compressed")
	}
	if results[2]["content"] != big {
		t.Error("pipeline output must stay unchanged")
	}
	blocks := results[3]["content"].([]any)
	if blocks[0].(map[string]any)["text"] != short {
		t.Errorf("text block in content array = %q", blocks[0].(map[string]any)["text"])
	}
	if blocks[1].(map[string]any)["type"] != "image" {
		t.Error("image block must stay")
	}
	// Thinking and tool_use stay byte-identical.
	for _, keep := range []string{`{"type":"thinking","thinking":"go test ./...","signature":"SIG"}`, `"command":"go test ./..."`} {
		if !strings.Contains(out, keep) {
			t.Errorf("lost %s", keep)
		}
	}
}

func TestRewriteReport(t *testing.T) {
	big := bigOutput(200)
	doc, err := jsonsplice.Scan([]byte(turn(big)))
	if err != nil {
		t.Fatal(err)
	}
	reps, diff := Rewrite(testConfig(t), doc.Targets)
	if len(reps) != 2 {
		t.Fatalf("replacements = %d", len(reps))
	}
	short := "=== RUN   TestSomething/case_\n[tollgate: go_test · 200 Zeilen ausgelassen · Test]"
	sizes := strconv.Itoa(len(big)) + " → " + strconv.Itoa(len(short)) + " Bytes"
	want := "messages[2].content[0].content · go_test · " + sizes + " · 200 Zeilen ausgelassen\n" +
		"messages[2].content[3].content[0].text · go_test · " + sizes + " · 200 Zeilen ausgelassen"
	if diff != want {
		t.Errorf("report:\n%s\nwant:\n%s", diff, want)
	}
}

func TestStageDeterministicAndCacheStable(t *testing.T) {
	s := NewStage(fixed{testConfig(t)})
	big := bigOutput(300)
	first, _ := process(t, s, turn(big))
	again, _ := process(t, s, turn(big))
	if first != again {
		t.Fatal("same input must give the same bytes")
	}
	// The next turn carries the history plus new messages: the compressed
	// prefix must stay byte-identical, so the prompt cache keeps hitting.
	next, _ := process(t, s, turn(big, `{"role":"assistant","content":"done"}`, `{"role":"user","content":"thanks"}`))
	prefix := first[:strings.LastIndex(first, "]}")]
	if !strings.HasPrefix(next, prefix) {
		t.Fatal("history of the next turn differs from the previous turn")
	}
}

func TestStageLeavesOtherEndpointsAndInvalidJSON(t *testing.T) {
	s := NewStage(fixed{testConfig(t)})
	req := &pipeline.Request{Endpoint: pipeline.EndpointPassthrough, Body: []byte("not json")}
	if reps, err := s.Process(context.Background(), req); err != nil || reps != nil {
		t.Fatalf("passthrough: %v %v", reps, err)
	}
	req = &pipeline.Request{Endpoint: pipeline.EndpointMessages, Body: []byte("{")}
	if _, err := s.Process(context.Background(), req); err == nil {
		t.Fatal("invalid JSON must fail")
	}
	// Without Bash calls nothing changes.
	body := `{"model":"m","messages":[{"role":"user","content":"hi"}]}`
	out, _ := process(t, s, body)
	if !bytes.Equal([]byte(out), []byte(body)) {
		t.Fatal("body changed")
	}
}
