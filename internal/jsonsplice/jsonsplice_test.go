package jsonsplice

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/go-json-experiment/json/jsontext"
)

// sampleBody mixes locked blocks, unknown fields/types, odd escaping and number forms.
const sampleBody = `{ "model":"claude-x", "max_tokens" : 1.0e3 ,"stream":true,
 "metadata":{"user_id":"u_1"},
 "system":[{"type":"text","text":"system ä text"}],
 "unknown_top":{"a":[1,2,{"text":"not a target"}]},
 "messages":[
  {"role":"user","content":"plain ä \"quoted\" text"},
  {"role":"assistant","content":[
    {"type":"thinking","thinking":"secret thought","signature":"SIG=="},
    {"type":"redacted_thinking","data":"AAAA"},
    {"type":"text","text":"hello ä"},
    {"text":"typed later","type":"text"},
    {"type":"tool_use","id":"t1","name":"Bash","input":{"command":"echo hi","n":5,"nested":{"k":["a","b"]}}},
    {"type":"compaction","text":"keep me"},
    {"type":"tool_use","id":"t3","name":"X","text":"tool_use text","content":"tool_use content","input":{}},
    {"type":"text","content":"text content","input":{"k":"text input"}},
    {"type":"future_block","text":"unknown type text","x":{"y":"z"}}
  ]},
  {"role":"user","content":[
    {"type":"tool_result","tool_use_id":"t1","content":"output string"},
    {"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"out A"},{"type":"image","source":{"data":"IMG"}},{"type":"text","text":"out B"}]},
    {"type":"image","source":{"type":"base64","data":"IMG"}},
    {"type":"document","title":"doc","text":"doc text"},
    {"type":"text","text":"user text","cache_control":{"type":"ephemeral"}}
  ]}
 ]}`

func TestScanTargets(t *testing.T) {
	doc, err := Scan([]byte(sampleBody))
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		path  string
		kind  Kind
		role  string
		value string
	}
	wants := []want{
		{"messages[0].content", KindMessageContent, "user", "plain ä \"quoted\" text"},
		{"messages[1].content[2].text", KindText, "assistant", "hello ä"},
		{"messages[1].content[3].text", KindText, "assistant", "typed later"},
		{"messages[1].content[4].input.command", KindToolUseInput, "assistant", "echo hi"},
		{"messages[1].content[4].input.nested.k[0]", KindToolUseInput, "assistant", "a"},
		{"messages[1].content[4].input.nested.k[1]", KindToolUseInput, "assistant", "b"},
		{"messages[2].content[0].content", KindToolResult, "user", "output string"},
		{"messages[2].content[1].content[0].text", KindToolResult, "user", "out A"},
		{"messages[2].content[1].content[2].text", KindToolResult, "user", "out B"},
		{"messages[2].content[4].text", KindText, "user", "user text"},
	}
	if len(doc.Targets) != len(wants) {
		for _, tg := range doc.Targets {
			t.Logf("got %s", tg.Path)
		}
		t.Fatalf("targets = %d, want %d", len(doc.Targets), len(wants))
	}
	for i, w := range wants {
		g := doc.Targets[i]
		if g.Path != w.path || g.Kind != w.kind || g.Role != w.role || g.Value != w.value {
			t.Errorf("target %d = %+v, want %+v", i, g, w)
		}
		raw := []byte(sampleBody)[g.Start:g.End]
		if raw[0] != '"' || raw[len(raw)-1] != '"' {
			t.Errorf("target %d range %q is not a string token", i, raw)
		}
	}
	// Tool linkage for the compress stage (#19): tool_use carries its id and
	// name, tool_result the tool_use_id it answers.
	tools := map[string][2]string{}
	for _, g := range doc.Targets {
		tools[g.Path] = [2]string{g.ToolID, g.ToolName}
	}
	for path, w := range map[string][2]string{
		"messages[1].content[4].input.command":     {"t1", "Bash"},
		"messages[1].content[4].input.nested.k[0]": {"t1", "Bash"},
		"messages[2].content[0].content":           {"t1", ""},
		"messages[2].content[1].content[0].text":   {"t2", ""},
		"messages[2].content[1].content[2].text":   {"t2", ""},
		"messages[1].content[2].text":              {"", ""},
		"messages[0].content":                      {"", ""},
	} {
		if tools[path] != w {
			t.Errorf("%s tool = %v, want %v", path, tools[path], w)
		}
	}
	if doc.Model != "claude-x" || !doc.Stream || doc.MetadataUserID != "u_1" {
		t.Errorf("top-level = %q %v %q", doc.Model, doc.Stream, doc.MetadataUserID)
	}
}

func TestScanDeepInputUsesLinearMemory(t *testing.T) {
	const depth = 9000
	body := `{"messages":[{"role":"assistant","content":[{"type":"tool_use","input":` +
		strings.Repeat(`{"a":`, depth) + `"leaf"` + strings.Repeat("}", depth) + `}]}]}`
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	doc, err := Scan([]byte(body))
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].Value != "leaf" {
		t.Fatalf("targets = %+v", doc.Targets)
	}
	// Quadratic path building allocated about 85 MB here.
	if got := after.TotalAlloc - before.TotalAlloc; got > 16<<20 {
		t.Fatalf("scan allocated %d bytes for a %d byte body", got, len(body))
	}
}

func TestScanInvalid(t *testing.T) {
	for name, body := range map[string]string{
		"empty":           ``,
		"not json":        `hello`,
		"truncated":       `{"messages":[{"role":"user","content":"x"}`,
		"trailing":        `{"messages":[]} x`,
		"top not object":  `[1]`,
		"duplicate names": `{"messages":[],"messages":[]}`,
		"bad utf8":        "{\"messages\":[{\"role\":\"user\",\"content\":\"\xff\"}]}",
		"messages shape":  `{"messages":{"a":1}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Scan([]byte(body)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestScanNoMessages(t *testing.T) {
	doc, err := Scan([]byte(`{"model":"m","other":{"x":"y"}}`))
	if err != nil || len(doc.Targets) != 0 || doc.Model != "m" {
		t.Fatalf("doc=%+v err=%v", doc, err)
	}
}

func TestApplyNoReplacementsIsIdentity(t *testing.T) {
	src := []byte(sampleBody)
	out, err := Apply(src, nil)
	if err != nil || !bytes.Equal(out, src) {
		t.Fatalf("err=%v identical=%v", err, bytes.Equal(out, src))
	}
}

func TestEscape(t *testing.T) {
	cases := map[string]string{
		"plain":      `plain`,
		"ä € 😀":      "ä € 😀",
		`a"b\c`:      `a\"b\\c`,
		"l1\nl2\t\r": `l1\nl2\t\r`,
		"\b\f":       `\b\f`,
		"\x00\x1f":   `\u0000\u001f`,
		"/ <>& ":     "/ <>& ",
		"\x7f":       "\x7f",
	}
	for in, want := range cases {
		if got := string(Escape(nil, in)); got != want {
			t.Errorf("Escape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyReplacesOnlyTargets(t *testing.T) {
	src := []byte(sampleBody)
	doc, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	// Replace two targets, given out of order.
	a, b := doc.Targets[1], doc.Targets[6]
	reps := []Replacement{
		{Start: b.Start, End: b.End, New: "NEW \"B\"\n"},
		{Start: a.Start, End: a.End, New: "<redacted:x_1>"},
	}
	out, err := Apply(src, reps)
	if err != nil {
		t.Fatal(err)
	}
	want := string(src[:a.Start]) + `"<redacted:x_1>"` + string(src[a.End:b.Start]) +
		`"NEW \"B\"\n"` + string(src[b.End:])
	if string(out) != want {
		t.Fatalf("output differs:\n%s\nwant:\n%s", out, want)
	}
	if !jsontext.Value(out).IsValid() {
		t.Fatal("output is not valid JSON")
	}
	// Locked bytes survive.
	for _, locked := range []string{`"thinking":"secret thought","signature":"SIG=="`, `"system":[{"type":"text","text":"system ä text"}]`, `"max_tokens" : 1.0e3`, `{"type":"compaction","text":"keep me"}`} {
		if !strings.Contains(string(out), locked) {
			t.Errorf("locked bytes lost: %s", locked)
		}
	}
	// Input must not be mutated.
	if string(src) != sampleBody {
		t.Fatal("input was mutated")
	}
}

func TestApplyErrors(t *testing.T) {
	src := []byte(`{"a":"xx","b":"yy"}`)
	cases := map[string][]Replacement{
		"out of range":   {{Start: 0, End: 100, New: "x"}},
		"negative":       {{Start: -1, End: 3, New: "x"}},
		"inverted":       {{Start: 6, End: 5, New: "x"}},
		"not a string":   {{Start: 0, End: 1, New: "x"}},
		"overlap":        {{Start: 5, End: 9, New: "x"}, {Start: 5, End: 9, New: "y"}},
		"invalid utf-8":  {{Start: 5, End: 9, New: "\xff"}},
		"inside a token": {{Start: 6, End: 8, New: "x"}},
	}
	for name, reps := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Apply(src, reps); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestScanDeterministic(t *testing.T) {
	first, _ := Scan([]byte(sampleBody))
	for i := 0; i < 100; i++ {
		again, err := Scan([]byte(sampleBody))
		if err != nil || len(again.Targets) != len(first.Targets) {
			t.Fatal("non-deterministic scan")
		}
		for j := range again.Targets {
			if again.Targets[j] != first.Targets[j] {
				t.Fatal("non-deterministic target")
			}
		}
	}
}
