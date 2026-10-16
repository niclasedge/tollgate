package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/sse"
)

// --- stream builders (invented data only) ---

func ev(name, data string) string { return "event: " + name + "\ndata: " + data + "\n\n" }

// jstr encodes s like Anthropic does: no HTML escaping.
func jstr(s string) string { return js(s) }

func blockStart(i int, typ string) string {
	body := `{"type":"` + typ + `","text":""}`
	switch typ {
	case "tool_use":
		body = `{"type":"tool_use","id":"toolu_1","name":"Bash","input":{}}`
	case "thinking":
		body = `{"type":"thinking","thinking":"","signature":""}`
	}
	return ev("content_block_start", `{"type":"content_block_start","index":`+strconv.Itoa(i)+`,"content_block":`+body+`}`)
}

func textDelta(i int, s string) string {
	return ev("content_block_delta", `{"type":"content_block_delta","index":`+strconv.Itoa(i)+`,"delta":{"type":"text_delta","text":`+jstr(s)+`}}`)
}

func jsonDelta(i int, s string) string {
	return ev("content_block_delta", `{"type":"content_block_delta","index":`+strconv.Itoa(i)+`,"delta":{"type":"input_json_delta","partial_json":`+jstr(s)+`}}`)
}

func thinkingDelta(i int, s string) string {
	return ev("content_block_delta", `{"type":"content_block_delta","index":`+strconv.Itoa(i)+`,"delta":{"type":"thinking_delta","thinking":`+jstr(s)+`}}`)
}

func sigDelta(i int, s string) string {
	return ev("content_block_delta", `{"type":"content_block_delta","index":`+strconv.Itoa(i)+`,"delta":{"type":"signature_delta","signature":`+jstr(s)+`}}`)
}

func blockStop(i int) string {
	return ev("content_block_stop", `{"type":"content_block_stop","index":`+strconv.Itoa(i)+`}`)
}

const (
	msgStart = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"usage\":{\"input_tokens\":3}}}\n\n"
	ping     = "event: ping\ndata: {\"type\": \"ping\"}\n\n"
	msgDelta = "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":9}}\n\n"
	msgStop  = "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
)

func rewrite(t *testing.T, in string, mapping map[string]string) string {
	t.Helper()
	var out bytes.Buffer
	if _, err := CopyRehydrated(&out, nil, strings.NewReader(in), mapping, nil); err != nil {
		t.Fatalf("CopyRehydrated: %v", err)
	}
	return out.String()
}

type parsedDelta struct {
	Index int
	Type  string
	Value string
}

// deltas parses all content_block_delta events of out in order.
func deltas(t *testing.T, out string) []parsedDelta {
	t.Helper()
	rd := sse.NewReader(strings.NewReader(out))
	var res []parsedDelta
	for {
		e, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return res
		}
		if err != nil {
			t.Fatal(err)
		}
		data := eventData(e)
		if !bytes.Contains(data, []byte(`"content_block_delta"`)) {
			continue
		}
		var v struct {
			Index int `json:"index"`
			Delta struct {
				Type, Text, Thinking, Signature string
				PartialJSON                     string `json:"partial_json"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("output event is not valid JSON: %v\n%s", err, e)
		}
		d := v.Delta
		val := d.Text + d.PartialJSON + d.Thinking + d.Signature
		res = append(res, parsedDelta{v.Index, d.Type, val})
	}
}

func eventData(e []byte) []byte {
	for line := range bytes.Lines(e) {
		if rest, ok := bytes.CutPrefix(bytes.TrimRight(line, "\r\n"), []byte("data: ")); ok {
			return rest
		}
	}
	return nil
}

// blockText concatenates the delta values of one block and counts them.
func blockText(t *testing.T, out string, idx int) (string, int) {
	t.Helper()
	var b strings.Builder
	n := 0
	for _, d := range deltas(t, out) {
		if d.Index == idx {
			b.WriteString(d.Value)
			n++
		}
	}
	return b.String(), n
}

// --- tests ---

func TestStreamPlaceholderAcrossDeltas(t *testing.T) {
	in := msgStart + blockStart(0, "text") + ping +
		textDelta(0, "Mail an <redac") + textDelta(0, "ted:email") + textDelta(0, "_1> ist raus.") +
		blockStop(0) + msgDelta + msgStop
	out := rewrite(t, in, testMapping)
	text, n := blockText(t, out, 0)
	if want := "Mail an " + plMail + " ist raus."; text != want {
		t.Fatalf("text = %q, want %q", text, want)
	}
	if n != 3 {
		t.Fatalf("delta events = %d, want 3", n)
	}
	for _, keep := range []string{msgStart, blockStart(0, "text"), ping, blockStop(0), msgDelta, msgStop} {
		if !strings.Contains(out, keep) {
			t.Fatalf("event changed or missing: %q", keep)
		}
	}
	// The emptied middle delta is still forwarded, with empty text.
	if !strings.Contains(out, textDelta(0, "")) {
		t.Fatalf("empty delta missing:\n%s", out)
	}
}

func TestStreamInterleavedBlocks(t *testing.T) {
	in := blockStart(0, "text") + blockStart(1, "tool_use") +
		textDelta(0, "a <redacted:em") + jsonDelta(1, `{"command":"echo <redacted:api`) +
		textDelta(0, "ail_1> b") + jsonDelta(1, `_key_2>"}`) +
		blockStop(0) + blockStop(1)
	out := rewrite(t, in, testMapping)
	if text, _ := blockText(t, out, 0); text != "a "+plMail+" b" {
		t.Fatalf("block 0 = %q", text)
	}
	js, _ := blockText(t, out, 1)
	var v struct{ Command string }
	if err := json.Unmarshal([]byte(js), &v); err != nil {
		t.Fatalf("partial_json does not assemble to JSON: %v\n%s", err, js)
	}
	if v.Command != "echo "+plKey {
		t.Fatalf("command = %q, want %q", v.Command, "echo "+plKey)
	}
}

func TestStreamRestAtBlockEnd(t *testing.T) {
	in := blockStart(0, "text") + textDelta(0, "ende <redac") + blockStop(0) + msgStop
	out := rewrite(t, in, testMapping)
	want := blockStart(0, "text") + textDelta(0, "ende ") + textDelta(0, "<redac") + blockStop(0) + msgStop
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
}

func TestStreamRestFlushedAtEOF(t *testing.T) {
	in := blockStart(1, "text") + textDelta(1, "b <r") + blockStart(0, "text") + textDelta(0, "a <")
	out := rewrite(t, in, testMapping)
	want := blockStart(1, "text") + textDelta(1, "b ") + blockStart(0, "text") + textDelta(0, "a ") +
		textDelta(0, "<") + textDelta(1, "<r") // ascending index
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
}

func TestStreamThinkingUnchanged(t *testing.T) {
	in := blockStart(0, "thinking") + thinkingDelta(0, "denke an <redacted:em") + thinkingDelta(0, "ail_1>") +
		sigDelta(0, "c2lnPHJlZGFjdGVkOmVtYWlsXzE+") + blockStop(0)
	if out := rewrite(t, in, testMapping); out != in {
		t.Fatalf("thinking stream changed:\n%s", out)
	}
}

func TestStreamUnknownEventsAndFields(t *testing.T) {
	unknown := "event: future_event\ndata: {\"type\":\"future_event\",\"text\":\"" + phMail + "\"}\n\n"
	comment := ": keepalive " + phMail + "\n\n"
	citation := ev("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"citations_delta","citation":{"cited_text":"`+phMail+`"}}}`)
	extra := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"x_future\":{\"a\":[1,2]},\"delta\":{\"type\":\"text_delta\",\"text\":\"hi " + phMail + "\",\"x\":\"\\u00e9\"}}\n\n"
	in := blockStart(0, "text") + unknown + comment + citation + extra + blockStop(0)
	out := rewrite(t, in, testMapping)
	for _, keep := range []string{unknown, comment, citation} {
		if !strings.Contains(out, keep) {
			t.Fatalf("event changed: %q\n%s", keep, out)
		}
	}
	wantExtra := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"x_future\":{\"a\":[1,2]},\"delta\":{\"type\":\"text_delta\",\"text\":\"hi " + plMail + "\",\"x\":\"\\u00e9\"}}\n\n"
	if !strings.Contains(out, wantExtra) {
		t.Fatalf("only delta.text may change:\n%s", out)
	}
}

func TestStreamCRLFEvent(t *testing.T) {
	in := "event: content_block_delta\r\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + phMail + "\"}}\r\n\r\n"
	want := "event: content_block_delta\r\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + plMail + "\"}}\r\n\r\n"
	if out := rewrite(t, in, testMapping); out != want {
		t.Fatalf("got %q", out)
	}
}

func TestStreamEmptyMappingByteIdentical(t *testing.T) {
	in := msgStart + blockStart(0, "text") + textDelta(0, "x "+phMail+" <redac") + blockStop(0) + msgStop + "trailing"
	for _, m := range []map[string]string{nil, {}} {
		if out := rewrite(t, in, m); out != in {
			t.Fatalf("empty mapping changed stream:\n%s", out)
		}
	}
}

func TestStreamObserveSeesUpstreamEvents(t *testing.T) {
	in := msgStart + blockStart(0, "text") + textDelta(0, phMail) + blockStop(0) + msgDelta
	var seen []string
	var out bytes.Buffer
	flushes := 0
	if _, err := CopyRehydrated(&out, func() { flushes++ }, strings.NewReader(in), testMapping,
		func(e []byte) { seen = append(seen, string(e)) }); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, "") != in {
		t.Fatalf("observe must see the unchanged upstream events")
	}
	if flushes == 0 {
		t.Fatalf("flush never called")
	}
}

func TestStreamDeterministic(t *testing.T) {
	in := blockStart(2, "text") + blockStart(0, "text") + blockStart(1, "text") +
		textDelta(2, "<") + textDelta(0, "<r") + textDelta(1, "<re") + msgDelta
	first := rewrite(t, in, testMapping)
	for range 20 {
		if rewrite(t, in, testMapping) != first {
			t.Fatalf("output not deterministic")
		}
	}
	idx := []int{}
	for _, d := range deltas(t, first) {
		if d.Value != "" {
			idx = append(idx, d.Index)
		}
	}
	if !sort.IntsAreSorted(idx) {
		t.Fatalf("flush order %v not ascending", idx)
	}
}

// TestStreamHTTPChunks sends the same stream from an httptest upstream with
// different chunkings; the rewritten output must be identical.
func TestStreamHTTPChunks(t *testing.T) {
	in := msgStart + blockStart(0, "text") + ping +
		textDelta(0, "Mail an <redac") + textDelta(0, "ted:email") + textDelta(0, "_1> und ") +
		blockStart(1, "tool_use") + jsonDelta(1, `{"command":"curl -u <redacted:api_key_`) + textDelta(0, "fertig") +
		jsonDelta(1, `2> x"}`) + blockStop(0) + blockStop(1) + msgDelta + msgStop
	want := rewrite(t, in, testMapping)

	rng := rand.New(rand.NewPCG(1, 2))
	chunkings := map[string]func() []int{
		"whole": func() []int { return []int{len(in)} },
		"bytes": func() []int {
			s := make([]int, len(in))
			for i := range s {
				s[i] = 1
			}
			return s
		},
		"random": func() []int {
			var s []int
			for n := 0; n < len(in); {
				k := 1 + rng.IntN(40)
				s = append(s, k)
				n += k
			}
			return s
		},
	}
	for _, name := range []string{"whole", "bytes", "random"} {
		t.Run(name, func(t *testing.T) {
			sizes := chunkings[name]()
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				f := w.(http.Flusher)
				pos := 0
				for _, k := range sizes {
					end := min(pos+k, len(in))
					_, _ = io.WriteString(w, in[pos:end])
					f.Flush()
					pos = end
				}
			}))
			defer up.Close()
			resp, err := http.Get(up.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var out bytes.Buffer
			if _, err := CopyRehydrated(&out, nil, resp.Body, testMapping, nil); err != nil {
				t.Fatal(err)
			}
			if out.String() != want {
				t.Fatalf("chunking %s changed output:\n%s\nwant\n%s", name, out.String(), want)
			}
		})
	}
	if text, _ := blockText(t, want, 0); text != "Mail an "+plMail+" und fertig" {
		t.Fatalf("block 0 = %q", text)
	}
	js, _ := blockText(t, want, 1)
	var v struct{ Command string }
	if err := json.Unmarshal([]byte(js), &v); err != nil || v.Command != "curl -u "+plKey+" x" {
		t.Fatalf("block 1 = %q (%v)", js, err)
	}
}

func TestRehydrateBody(t *testing.T) {
	thinking := `{"type":"thinking","thinking":"an ` + phMail + `","signature":"sig` + phMail + `"}`
	body := `{"id":"msg_1","type":"message","x_future":"` + phMail + `","content":[` +
		thinking + `,` +
		`{"text":"Mail ` + phMail + ` <redacted:email_9>","type":"text","citations":null},` +
		`{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"echo ` + phKey + `","list":["` + phMail + `",1,{"k":"` + phMail + `"}]}},` +
		`{"type":"future_block","text":"` + phMail + `"}` +
		`],"usage":{"input_tokens":1}}`
	out := RehydrateBody([]byte(body), testMapping)
	s := string(out)
	for _, keep := range []string{thinking, `"x_future":"` + phMail + `"`, `{"type":"future_block","text":"` + phMail + `"}`, `"usage":{"input_tokens":1}`} {
		if !strings.Contains(s, keep) {
			t.Fatalf("%q changed:\n%s", keep, s)
		}
	}
	var v struct {
		Content []struct {
			Type  string
			Text  string
			Input struct {
				Command string
				List    []any
			}
		}
	}
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
	if v.Content[1].Text != "Mail "+plMail+" <redacted:email_9>" {
		t.Fatalf("text = %q", v.Content[1].Text)
	}
	in := v.Content[2].Input
	if in.Command != "echo "+plKey || in.List[0] != plMail || in.List[2].(map[string]any)["k"] != plMail {
		t.Fatalf("tool_use input = %+v", in)
	}
	for _, bad := range []string{"", "not json", `[1,2]`, `{"content":[`} {
		if got := RehydrateBody([]byte(bad), testMapping); string(got) != bad {
			t.Fatalf("invalid body %q changed to %q", bad, got)
		}
	}
	if got := RehydrateBody([]byte(body), nil); string(got) != body {
		t.Fatalf("empty mapping changed body")
	}
}
