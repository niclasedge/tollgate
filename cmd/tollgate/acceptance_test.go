package main

// Phase-2 acceptance (#17): cache-stable prefixes over several turns with
// return path and re-redaction, a leak test over DB files, logs and admin
// API, and an end-to-end run over the cleaned fixtures. All three drive the
// real binary wiring (serve) with the redact stage on, the real store and the
// versioned configuration; only CLIProxyAPI is replaced by a mock model.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/auth"
)

// Invented secrets (reserved .invalid domain, canary keys). They are secrets
// under every configuration version the tests activate, so none of them may
// ever reach the upstream, the DB, the logs or the admin API.
const (
	accMail     = "jane.roe@muster-gmbh.invalid"
	accMail2    = "max.mustermann@kunde.invalid"
	accAPIKey   = "sk-ant-api03-ACCEPTCANARY0123456789abcdefghijklmnop"
	accPassword = "fakepw-0000"
	accJWT      = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0b2xsZ2F0ZS10ZXN0In0.ZmFrZS1zaWduYXR1cmUtMDAwMQ"
	accWord     = "Projekt Falke"
	accWord2    = "Muster GmbH"
)

// canaries are searched in every leak check; KD numbers are excluded on
// purpose: a pinned session keeps redacting them with its older version.
var canaries = []string{accMail, accMail2, accAPIKey, "ACCEPTCANARY", accPassword, accJWT, "ZmFrZS1zaWduYXR1cmUtMDAwMQ",
	accWord, "Falke", accWord2, "jane.roe", "max.mustermann"}

// v2Config adds a custom rule and a word list (activated mid-session).
const v2Config = `version: 1
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
    examples: ["KD-000000"]
words:
  case_sensitive: false
  whole_word: true
  list: ["Projekt Falke", "Muster GmbH"]
allowlist: ["noreply@github.com"]
`

var accPlaceholderRE = regexp.MustCompile(`<redacted:[a-z][a-z0-9_]{0,31}_[1-9][0-9]{0,5}>`)

// syncBuffer collects the JSON log lines of serve.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// modelTurn is what the mock model answered to one /v1/messages request,
// in the form the model produced it (with placeholders).
type modelTurn struct {
	Thinking, Signature, Text, ToolID, ToolInput string
}

// mockModel stands in for CLIProxyAPI. It records every /v1/messages body
// and answers with an SSE stream that quotes the (redacted) last user text,
// so the answer carries placeholders in text_delta and input_json_delta,
// split over event and chunk boundaries.
type mockModel struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []string
	turns  []modelTurn
}

func newMockModel(t *testing.T) *mockModel {
	t.Helper()
	m := &mockModel{}
	m.Server = httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(m.Close)
	return m
}

func (m *mockModel) recorded() ([]string, []modelTurn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.bodies...), append([]modelTurn(nil), m.turns...)
}

func (m *mockModel) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/messages" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(body, &req)
	quote := lastUserText(req.Messages[len(req.Messages)-1].Content)
	var phs []string
	seen := map[string]bool{}
	for _, p := range accPlaceholderRE.FindAllString(string(body), -1) {
		if !seen[p] {
			seen[p] = true
			phs = append(phs, p)
		}
	}
	input, _ := marshalNoEscape(map[string]string{"command": "printf '%s\\n' " + quote})
	turn := modelTurn{
		Thinking:  "Ich sehe " + strings.Join(phs, " "),
		Signature: fmt.Sprintf("fake-signature-%02d", len(req.Messages)),
		Text:      "Zitat: " + quote,
		ToolID:    fmt.Sprintf("toolu_%026d", len(req.Messages)),
		ToolInput: string(input),
	}
	m.mu.Lock()
	m.bodies = append(m.bodies, string(body))
	m.turns = append(m.turns, turn)
	m.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	fl := w.(http.Flusher)
	send := func(event string, data any) {
		b, _ := marshalNoEscape(data)
		frame := "event: " + event + "\ndata: " + string(b) + "\n\n"
		// Two writes per event: the SSE parser must reassemble frames.
		half := len(frame) / 2
		_, _ = io.WriteString(w, frame[:half])
		fl.Flush()
		_, _ = io.WriteString(w, frame[half:])
		fl.Flush()
	}
	delta := func(i int, typ, field, s string) {
		for _, part := range chunks(s, 7) { // 7 bytes: every placeholder straddles deltas
			send("content_block_delta", map[string]any{"type": "content_block_delta", "index": i,
				"delta": map[string]string{"type": typ, field: part}})
		}
	}
	send("message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": "msg_test", "type": "message", "role": "assistant", "model": "claude-opus-5-5", "content": []any{},
		"usage": map[string]int{"input_tokens": 100, "output_tokens": 1, "cache_read_input_tokens": 50, "cache_creation_input_tokens": 10}}})
	send("content_block_start", map[string]any{"type": "content_block_start", "index": 0,
		"content_block": map[string]string{"type": "thinking", "thinking": "", "signature": ""}})
	delta(0, "thinking_delta", "thinking", turn.Thinking)
	send("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0,
		"delta": map[string]string{"type": "signature_delta", "signature": turn.Signature}})
	send("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
	send("ping", map[string]string{"type": "ping"})
	send("content_block_start", map[string]any{"type": "content_block_start", "index": 1,
		"content_block": map[string]string{"type": "text", "text": ""}})
	delta(1, "text_delta", "text", turn.Text)
	send("content_block_stop", map[string]any{"type": "content_block_stop", "index": 1})
	send("content_block_start", map[string]any{"type": "content_block_start", "index": 2,
		"content_block": map[string]any{"type": "tool_use", "id": turn.ToolID, "name": "Bash", "input": map[string]any{}}})
	delta(2, "input_json_delta", "partial_json", turn.ToolInput)
	send("content_block_stop", map[string]any{"type": "content_block_stop", "index": 2})
	send("message_delta", map[string]any{"type": "message_delta", "delta": map[string]string{"stop_reason": "tool_use"},
		"usage": map[string]int{"output_tokens": 42}})
	send("message_stop", map[string]string{"type": "message_stop"})
}

// lastUserText returns the text of a user message: the string content, or
// the text blocks and string tool_result contents joined by " ".
func lastUserText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type    string          `json:"type"`
		Text    string          `json:"text"`
		Content json.RawMessage `json:"content"`
	}
	_ = json.Unmarshal(raw, &blocks)
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "tool_result":
			if json.Unmarshal(b.Content, &s) == nil {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, " ")
}

func chunks(s string, n int) []string {
	var out []string
	for len(s) > n {
		out = append(out, s[:n])
		s = s[n:]
	}
	return append(out, s)
}

// marshalNoEscape is json.Marshal without HTML escaping (like JSON.stringify
// in Claude Code), so "<redacted:…>" stays literal.
func marshalNoEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// accEnv is one running tollgate (serve) with redact on and a mock model.
type accEnv struct {
	base, dir, admin, key string
	logs                  *syncBuffer
	up                    *mockModel
}

func startAcceptance(t *testing.T) *accEnv {
	t.Helper()
	e := &accEnv{up: newMockModel(t), logs: &syncBuffer{}}
	conf := writeConf(t, "127.0.0.1:0", e.up.URL)
	e.dir = filepath.Dir(conf)
	f, err := os.OpenFile(conf, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Debug level: the leak check covers every log line tollgate can write.
	_, _ = f.WriteString("redact:\n  enabled: true\nlog:\n  level: debug\n")
	f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, []string{"-config", conf, "--dev-allow-any-listen"}, e.logs, noEnv) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("serve: %v", err)
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for e.base == "" {
		for _, line := range strings.Split(e.logs.String(), "\n") {
			var l struct{ Msg, Addr string }
			if json.Unmarshal([]byte(line), &l) == nil && l.Msg == "tollgate: listening" {
				e.base = "http://" + l.Addr
			}
		}
		if e.base == "" {
			if time.Now().After(deadline) {
				t.Fatalf("no listening log line:\n%s", e.logs.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	raw, err := os.ReadFile(filepath.Join(e.dir, "admin.token"))
	if err != nil {
		t.Fatal(err)
	}
	e.admin = strings.TrimSpace(string(raw))
	code, body := call(t, http.MethodPost, e.base+"/api/clients", e.admin, `{"name":"abnahme"}`)
	var created auth.CreateClientResponse
	if code != 201 || json.Unmarshal(body, &created) != nil {
		t.Fatalf("create client = %d %s", code, body)
	}
	e.key = created.Key
	return e
}

// importConfig imports and activates a configuration version and returns its ID.
func (e *accEnv) importConfig(t *testing.T, yamlDoc string) int64 {
	t.Helper()
	req, _ := marshalNoEscape(map[string]any{"yaml": yamlDoc, "comment": "Abnahme #17", "activate": true})
	code, body := call(t, http.MethodPost, e.base+"/api/config/import", e.admin, string(req))
	var v struct {
		ID int64 `json:"id"`
	}
	if code != 201 || json.Unmarshal(body, &v) != nil || v.ID == 0 {
		t.Fatalf("import = %d %s", code, body)
	}
	return v.ID
}

// lastTraceConfigVersion returns the configuration version the redact stage
// of the newest trace used.
func (e *accEnv) lastTraceConfigVersion(t *testing.T) int64 {
	t.Helper()
	_, body := call(t, http.MethodGet, e.base+"/api/traces?limit=1", e.admin, "")
	var list struct {
		Traces []struct {
			ID string `json:"id"`
		} `json:"traces"`
	}
	if json.Unmarshal(body, &list) != nil || len(list.Traces) != 1 {
		t.Fatalf("traces: %s", body)
	}
	_, body = call(t, http.MethodGet, e.base+"/api/traces/"+list.Traces[0].ID, e.admin, "")
	var d struct {
		Stages []struct {
			Name          string `json:"name"`
			ConfigVersion int64  `json:"config_version"`
		} `json:"stages"`
	}
	if json.Unmarshal(body, &d) != nil {
		t.Fatalf("detail: %s", body)
	}
	for _, s := range d.Stages {
		if s.Name == "redact" {
			return s.ConfigVersion
		}
	}
	t.Fatalf("no redact stage: %s", body)
	return 0
}

// sseBlock is one content block as the client reassembles it.
type sseBlock struct {
	Type, ID, Name, Text, Thinking, Signature, Input string
}

// parseSSE reassembles the content blocks of a /v1/messages stream as Claude
// Code does: text and thinking deltas are appended, partial_json is
// concatenated into the tool input.
func parseSSE(t *testing.T, raw []byte) []sseBlock {
	t.Helper()
	var blocks []sseBlock
	for _, frame := range strings.Split(string(raw), "\n\n") {
		var data string
		for _, line := range strings.Split(frame, "\n") {
			if rest, ok := strings.CutPrefix(line, "data: "); ok {
				data = rest
			}
		}
		if data == "" {
			continue
		}
		var ev struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Block struct {
				Type, ID, Name string
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				Signature   string `json:"signature"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			t.Fatalf("client got a broken SSE event %q: %v", data, err)
		}
		switch ev.Type {
		case "content_block_start":
			for len(blocks) <= ev.Index {
				blocks = append(blocks, sseBlock{})
			}
			blocks[ev.Index] = sseBlock{Type: ev.Block.Type, ID: ev.Block.ID, Name: ev.Block.Name}
		case "content_block_delta":
			b := &blocks[ev.Index]
			b.Text += ev.Delta.Text
			b.Thinking += ev.Delta.Thinking
			b.Signature += ev.Delta.Signature
			b.Input += ev.Delta.PartialJSON
		}
	}
	return blocks
}

// assistantMessage renders the reassembled blocks as the assistant message
// Claude Code sends back in the next turn (thinking unchanged, text and tool
// input as the client received them, i.e. in plaintext).
func assistantMessage(t *testing.T, blocks []sseBlock) string {
	t.Helper()
	var content []any
	for _, b := range blocks {
		switch b.Type {
		case "thinking":
			content = append(content, map[string]string{"type": "thinking", "thinking": b.Thinking, "signature": b.Signature})
		case "text":
			content = append(content, map[string]string{"type": "text", "text": b.Text})
		case "tool_use":
			if !json.Valid([]byte(b.Input)) {
				t.Fatalf("client got invalid tool input %q", b.Input)
			}
			content = append(content, map[string]any{"type": "tool_use", "id": b.ID, "name": b.Name, "input": json.RawMessage(b.Input)})
		}
	}
	out, err := marshalNoEscape(map[string]any{"role": "assistant", "content": content})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// session plays one Claude Code session: every turn sends the whole history,
// including the rehydrated assistant answers of the turns before.
type session struct {
	e        *accEnv
	id       string
	messages []string
	lastTool string
}

// bodyPrefix is everything before the messages array; messages come last so
// that the end of the previous turn's messages is a byte offset.
const bodyPrefix = `{"model":"claude-opus-5-5","max_tokens":1024,"stream":true,"system":[{"type":"text","text":"Du bist ein Testsystem."}],"messages":[`

func (s *session) body() string { return bodyPrefix + strings.Join(s.messages, ",") + "]}" }

// turn appends a user message (text plus the result of the previous tool
// call, if any), sends the request and appends the rehydrated answer.
func (s *session) turn(t *testing.T, userText, toolOutput string) []sseBlock {
	t.Helper()
	var content []any
	if s.lastTool != "" {
		content = append(content, map[string]any{"type": "tool_result", "tool_use_id": s.lastTool, "content": toolOutput})
	}
	content = append(content, map[string]string{"type": "text", "text": userText})
	msg, _ := marshalNoEscape(map[string]any{"role": "user", "content": content})
	s.messages = append(s.messages, string(msg))

	req, _ := http.NewRequest(http.MethodPost, s.e.base+"/v1/messages", strings.NewReader(s.body()))
	req.Header.Set("Authorization", "Bearer "+s.e.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req.Header.Set("X-Claude-Code-Session-Id", s.id)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("turn %d = %d %s", len(s.messages), resp.StatusCode, raw)
	}
	blocks := parseSSE(t, raw)
	if len(blocks) != 3 {
		t.Fatalf("client got %d blocks: %s", len(blocks), raw)
	}
	s.messages = append(s.messages, assistantMessage(t, blocks))
	s.lastTool = blocks[2].ID
	return blocks
}

// runCacheSession plays four turns of session A with a configuration change
// after turn 2 and one turn of a new session B, and returns what it needs
// for the assertions.
type cacheRun struct {
	upA     []string    // upstream bodies of session A, in order
	turnsA  []modelTurn // model answers of session A
	clientA [][]sseBlock
	upB     string
	v1, v2  int64
	cfgA    []int64 // configuration version per turn of session A
	cfgB    int64
}

func runCacheSession(t *testing.T, e *accEnv) cacheRun {
	t.Helper()
	var run cacheRun
	a := &session{e: e, id: "00000000-0000-4000-8000-00000000000a"}
	record := func(blocks []sseBlock) {
		run.clientA = append(run.clientA, blocks)
		run.cfgA = append(run.cfgA, e.lastTraceConfigVersion(t))
	}
	record(a.turn(t, "Bitte prüfe die Mail an "+accMail+" und den Key "+accAPIKey+".", ""))
	record(a.turn(t, "Und die Datenbank?", "psql postgres://app:"+accPassword+"@db.invalid:5432/app\nSession-Token "+accJWT+" für "+accMail))
	run.v1 = run.cfgA[0]
	run.v2 = e.importConfig(t, v2Config) // mid-session: new rules for new sessions only
	record(a.turn(t, "Ticket KD-111111 betrifft "+accMail2+".", "ok "+accMail))
	record(a.turn(t, "Danke, "+accMail+".", "fertig"))

	bodies, turns := e.up.recorded()
	run.upA, run.turnsA = bodies[len(bodies)-4:], turns[len(turns)-4:]

	b := &session{e: e, id: "00000000-0000-4000-8000-00000000000b"}
	b.turn(t, "Neue Sitzung: Ticket KD-222222 zu "+accWord+" bei der "+accWord2+", Mail "+accMail+".", "")
	run.cfgB = e.lastTraceConfigVersion(t)
	bodies, _ = e.up.recorded()
	run.upB = bodies[len(bodies)-1]
	return run
}

// upstreamMessages decodes the messages array of an upstream body.
func upstreamMessages(t *testing.T, body string) []json.RawMessage {
	t.Helper()
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("upstream body is no JSON: %v", err)
	}
	return req.Messages
}

// TestCacheStablePrefixAcrossTurns is the cache test of ADR-0006: four turns
// of one session, each sending the rehydrated answers back in plaintext,
// produce upstream bodies whose earlier turns are byte-identical prefixes,
// also across a configuration change in the middle of the session.
func TestCacheStablePrefixAcrossTurns(t *testing.T) {
	e := startAcceptance(t)
	run := runCacheSession(t, e)

	for k := 1; k < len(run.upA); k++ {
		prev, cur := run.upA[k-1], run.upA[k]
		if !strings.HasSuffix(prev, "]}") {
			t.Fatalf("turn %d: upstream body does not end with the messages array", k)
		}
		if p := prev[:len(prev)-2] + ","; !strings.HasPrefix(cur, p) {
			i := 0
			for i < len(p) && i < len(cur) && p[i] == cur[i] {
				i++
			}
			t.Fatalf("turn %d is no byte prefix of turn %d; first difference at byte %d:\nprev …%s\ncur  …%s",
				k, k+1, i, p[max(0, i-60):min(len(p), i+60)], cur[max(0, i-60):min(len(cur), i+60)])
		}
	}

	// The model sees its own answer again: the re-redacted assistant message
	// equals what the model produced, placeholders and numbers included.
	for k := 1; k < len(run.upA); k++ {
		msgs := upstreamMessages(t, run.upA[k])
		var got struct {
			Content []struct {
				Type, Text, Thinking, Signature, ID string
				Input                               json.RawMessage
			}
		}
		if err := json.Unmarshal(msgs[2*k-1], &got); err != nil || len(got.Content) != 3 {
			t.Fatalf("turn %d: assistant message %s: %v", k+1, msgs[2*k-1], err)
		}
		want := run.turnsA[k-1]
		c := got.Content
		if c[0].Thinking != want.Thinking || c[0].Signature != want.Signature {
			t.Errorf("turn %d: thinking changed: %q / %q, want %q / %q", k+1, c[0].Thinking, c[0].Signature, want.Thinking, want.Signature)
		}
		if c[1].Text != want.Text {
			t.Errorf("turn %d: assistant text upstream = %q, model wrote %q", k+1, c[1].Text, want.Text)
		}
		if string(c[2].Input) != want.ToolInput || c[2].ID != want.ToolID {
			t.Errorf("turn %d: tool input upstream = %s, model wrote %s", k+1, c[2].Input, want.ToolInput)
		}
	}

	// Numbering: the first values keep their numbers in every turn; new
	// values get higher ones.
	for k, body := range run.upA {
		for _, ph := range []string{"<redacted:email_1>", "<redacted:api_key_1>"} {
			if !strings.Contains(body, ph) {
				t.Errorf("turn %d: %s missing", k+1, ph)
			}
		}
	}
	if !strings.Contains(run.upA[2], "<redacted:email_2>") || strings.Contains(run.upA[1], "<redacted:email_2>") {
		t.Error("the new mail of turn 3 must be email_2 and must not exist before")
	}

	// The client sees plaintext in text and tool input, placeholders only in
	// the (signed) thinking.
	for k, blocks := range run.clientA {
		if !strings.Contains(blocks[0].Thinking, "<redacted:") {
			t.Errorf("turn %d: thinking was rehydrated: %q", k+1, blocks[0].Thinking)
		}
		for _, s := range []string{blocks[1].Text, blocks[2].Input} {
			if accPlaceholderRE.MatchString(s) {
				t.Errorf("turn %d: client sees a placeholder: %q", k+1, s)
			}
		}
	}
	if !strings.Contains(run.clientA[0][1].Text, accMail) || !strings.Contains(run.clientA[0][2].Input, accAPIKey) {
		t.Errorf("turn 1: client lacks the plaintext: %+v", run.clientA[0])
	}

	// Pinning: session A stays on version 1 after the change, session B
	// starts on version 2.
	if run.v2 <= run.v1 || run.cfgA[0] != run.v1 || run.cfgA[1] != run.v1 || run.cfgA[2] != run.v1 || run.cfgA[3] != run.v1 {
		t.Fatalf("config versions of session A = %v, v1 %d, v2 %d", run.cfgA, run.v1, run.v2)
	}
	if run.cfgB != run.v2 {
		t.Fatalf("session B uses version %d, want %d", run.cfgB, run.v2)
	}
	if !strings.Contains(run.upA[2], "KD-111111") || strings.Contains(run.upA[2], "customer_id") {
		t.Error("session A must keep version 1 (KD numbers not redacted) after the change")
	}
	for _, want := range []string{"<redacted:customer_id_1>", "<redacted:word_1>", "<redacted:word_2>"} {
		if !strings.Contains(run.upB, want) {
			t.Errorf("session B lacks %s: %s", want, run.upB)
		}
	}
	if strings.Contains(run.upB, "KD-222222") {
		t.Error("session B must redact KD numbers with version 2")
	}

	for _, body := range append(run.upA, run.upB) {
		for _, c := range canaries {
			if strings.Contains(body, c) {
				t.Errorf("upstream saw %q", c)
			}
		}
	}
}

// fixtureFile is the format of testdata/fixtures/*.json.
type fixtureFile struct {
	Method string            `json:"method"`
	Path   string            `json:"path"`
	Header map[string]string `json:"header"`
	Body   json.RawMessage   `json:"body"`
}

// fixtureSecrets lists, per fixture, the invented secrets that must not
// reach the upstream but must reach the client again, and the values that
// must pass unchanged (allowlist). Every fixture must be listed here.
var fixtureSecrets = map[string]struct{ secrets, keep []string }{
	"claude-code-2.1.289-messages.json": {},
	"redact-dry-run-messages.json": {
		secrets: []string{accWord, accWord2, "KD-123456", "KD-654321", accMail, accPassword, "Projekt\nFalke"},
		keep:    []string{"noreply@github.com"},
	},
	"multi-turn-session.json": {
		secrets: []string{accMail, accMail2, accAPIKey, accPassword, accJWT, accWord, "KD-314159"},
		keep:    []string{"noreply@github.com"},
	},
}

type fixtureRun struct {
	name     string
	fx       fixtureFile
	upstream string
	blocks   []sseBlock
}

// runFixtures activates v2Config and sends every fixture through tollgate.
func runFixtures(t *testing.T, e *accEnv) []fixtureRun {
	t.Helper()
	e.importConfig(t, v2Config)
	names, err := filepath.Glob("../../testdata/fixtures/*.json")
	if err != nil || len(names) == 0 {
		t.Fatalf("fixtures: %v", err)
	}
	var runs []fixtureRun
	for _, path := range names {
		name := filepath.Base(path)
		if _, ok := fixtureSecrets[name]; !ok {
			t.Fatalf("fixture %s has no entry in fixtureSecrets", name)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fx fixtureFile
		if err := json.Unmarshal(raw, &fx); err != nil || fx.Method != http.MethodPost || fx.Path != "/v1/messages" {
			t.Fatalf("%s: %v", name, err)
		}
		req, _ := http.NewRequest(fx.Method, e.base+fx.Path, bytes.NewReader(fx.Body))
		for k, v := range fx.Header {
			req.Header.Set(k, v)
		}
		req.Header.Set("Authorization", "Bearer "+e.key)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d %s", name, resp.StatusCode, out)
		}
		bodies, _ := e.up.recorded()
		runs = append(runs, fixtureRun{name: name, fx: fx, upstream: bodies[len(bodies)-1], blocks: parseSSE(t, out)})
	}
	if len(runs) != len(fixtureSecrets) {
		t.Fatalf("ran %d fixtures, fixtureSecrets lists %d", len(runs), len(fixtureSecrets))
	}
	return runs
}

// TestEndToEndFixtures sends every cleaned fixture through the whole binary:
// the upstream sees only placeholders, the SSE answer carries them split over
// chunk boundaries, the client gets the plaintext back.
func TestEndToEndFixtures(t *testing.T) {
	e := startAcceptance(t)
	for _, r := range runFixtures(t, e) {
		t.Run(r.name, func(t *testing.T) {
			exp := fixtureSecrets[r.name]
			if len(exp.secrets) == 0 && r.upstream != string(r.fx.Body) {
				t.Errorf("fixture without secrets changed on the way upstream:\n%s\n%s", r.fx.Body, r.upstream)
			}
			for _, s := range exp.secrets {
				if strings.Contains(r.upstream, s) || strings.Contains(r.upstream, strings.ReplaceAll(s, "\n", `\n`)) {
					t.Errorf("upstream saw %q", s)
				}
			}
			for _, s := range exp.keep {
				if !strings.Contains(r.upstream, s) {
					t.Errorf("allowlisted %q did not pass", s)
				}
			}
			if len(r.blocks) != 3 {
				t.Fatalf("blocks = %+v", r.blocks)
			}
			// The model quoted the last user text with its placeholders; the
			// client gets that text back in plaintext.
			var body struct {
				Messages []struct {
					Content json.RawMessage `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(r.fx.Body, &body); err != nil {
				t.Fatal(err)
			}
			want := "Zitat: " + lastUserText(body.Messages[len(body.Messages)-1].Content)
			if r.blocks[1].Text != want {
				t.Errorf("client text = %q, want %q", r.blocks[1].Text, want)
			}
			var input struct{ Command string }
			if err := json.Unmarshal([]byte(r.blocks[2].Input), &input); err != nil || input.Command != "printf '%s\\n' "+strings.TrimPrefix(want, "Zitat: ") {
				t.Errorf("client tool input = %s (%v)", r.blocks[2].Input, err)
			}
			if accPlaceholderRE.MatchString(r.blocks[1].Text + r.blocks[2].Input) {
				t.Errorf("client sees placeholders: %+v", r.blocks)
			}
			// Thinking and signatures of the fixture pass byte for byte.
			for _, sig := range regexp.MustCompile(`"signature":"[^"]*"`).FindAllString(string(r.fx.Body), -1) {
				if !strings.Contains(r.upstream, sig) {
					t.Errorf("signature changed: %s", sig)
				}
			}
			for _, th := range regexp.MustCompile(`"thinking":"[^"]*"`).FindAllString(string(r.fx.Body), -1) {
				if !strings.Contains(r.upstream, th) {
					t.Errorf("thinking changed: %s", th)
				}
			}
		})
	}
}

// TestNoPlaintextInDBLogsAndAdminAPI is the leak test of ADR-0009: after the
// cache session and all fixtures, no invented secret appears in the SQLite
// files (including WAL and SHM), the logs, the live event stream or any
// admin API answer.
func TestNoPlaintextInDBLogsAndAdminAPI(t *testing.T) {
	e := startAcceptance(t)

	// Live events of the dashboard, captured during the whole traffic.
	evReq, _ := http.NewRequest(http.MethodGet, e.base+"/api/traces/events", nil)
	evCtx, stopEvents := context.WithCancel(context.Background())
	evReq = evReq.WithContext(evCtx)
	evReq.Header.Set("Authorization", "Bearer "+e.admin)
	evResp, err := http.DefaultClient.Do(evReq)
	if err != nil || evResp.StatusCode != 200 {
		t.Fatalf("events: %v", err)
	}
	var events syncBuffer
	evDone := make(chan struct{})
	go func() { _, _ = io.Copy(&events, evResp.Body); close(evDone) }()

	run := runCacheSession(t, e)
	runFixtures(t, e)
	time.Sleep(200 * time.Millisecond) // let the last trace events arrive
	stopEvents()
	<-evDone
	evResp.Body.Close()

	type source struct {
		name string
		data []byte
	}
	var sources []source
	add := func(name string, data []byte) { sources = append(sources, source{name, data}) }
	get := func(path string) []byte {
		code, body := call(t, http.MethodGet, e.base+path, e.admin, "")
		if code != 200 {
			t.Fatalf("GET %s = %d %s", path, code, body)
		}
		add("GET "+path, body)
		return body
	}
	post := func(path, body string) {
		code, out := call(t, http.MethodPost, e.base+path, e.admin, body)
		if code != 200 {
			t.Fatalf("POST %s = %d %s", path, code, out)
		}
		add("POST "+path, out)
	}

	var list struct {
		Traces []struct {
			ID string `json:"id"`
		} `json:"traces"`
	}
	if err := json.Unmarshal(get("/api/traces?limit=200"), &list); err != nil || len(list.Traces) < 8 {
		t.Fatalf("traces: %v %d", err, len(list.Traces))
	}
	// Masks (issue #112): every stored hit carries one, and no mask holds a
	// canary or, for the confidential types, any clear character.
	type hit struct {
		Type        string `json:"type"`
		Placeholder string `json:"placeholder"`
		Mask        string `json:"mask"`
	}
	var masks []hit
	for _, tr := range list.Traces {
		var d struct {
			Stages []struct {
				Matches []hit `json:"matches"`
			} `json:"stages"`
		}
		if err := json.Unmarshal(get("/api/traces/"+tr.ID), &d); err != nil {
			t.Fatal(err)
		}
		for _, s := range d.Stages {
			masks = append(masks, s.Matches...)
		}
	}
	var recent struct {
		Redactions []hit `json:"redactions"`
	}
	if err := json.Unmarshal(get("/api/redactions?limit=50"), &recent); err != nil || len(recent.Redactions) == 0 {
		t.Fatalf("redactions: %v %d", err, len(recent.Redactions))
	}
	masks = append(masks, recent.Redactions...)
	types := map[string]bool{}
	for _, m := range masks {
		types[m.Type] = true
		if m.Placeholder == "" || !strings.Contains(m.Mask, "•") {
			t.Errorf("hit %s (%s) without mask: %q", m.Placeholder, m.Type, m.Mask)
		}
		for _, c := range canaries {
			if strings.Contains(m.Mask, c) {
				t.Errorf("mask %q of %s contains the canary %q", m.Mask, m.Placeholder, c)
			}
		}
		switch m.Type {
		case "word", "connection_password", "private_key":
			if !hiddenMaskRE.MatchString(m.Mask) {
				t.Errorf("mask %q of %s shows clear characters", m.Mask, m.Placeholder)
			}
		}
	}
	for _, typ := range []string{"email", "api_key", "word", "connection_password"} {
		if !types[typ] {
			t.Errorf("no masked hit of type %s (types %v)", typ, types)
		}
	}
	get("/api/costs")
	get("/api/status")
	get("/api/clients")
	var versions struct {
		Versions []struct {
			ID int64 `json:"id"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(get("/api/config/versions"), &versions); err != nil || len(versions.Versions) < 3 {
		t.Fatalf("versions: %v %+v", err, versions)
	}
	for _, v := range versions.Versions {
		get(fmt.Sprintf("/api/config/versions/%d", v.ID))
	}
	get("/api/config/export")
	get(fmt.Sprintf("/api/config/export?version=%d", run.v2))
	post("/api/config/dry-run", `{"yaml":`+jsonString(v2Config)+`}`)
	post("/api/config/test", `{"text":`+jsonString("Mail "+accMail+" Key "+accAPIKey+" "+accWord)+`}`)

	add("logs", []byte(e.logs.String()))
	add("GET /api/traces/events", []byte(events.String()))
	dbFiles, _ := filepath.Glob(filepath.Join(e.dir, "t.db*"))
	if len(dbFiles) < 2 { // t.db and at least the WAL
		t.Fatalf("db files = %v", dbFiles)
	}
	for _, f := range dbFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		add(filepath.Base(f), data)
	}

	if !strings.Contains(events.String(), "abnahme") {
		t.Errorf("event stream captured nothing: %q", events.String())
	}
	for _, s := range sources {
		for _, c := range canaries {
			if bytes.Contains(s.data, []byte(c)) {
				t.Errorf("%s contains the plaintext %q", s.name, c)
			}
		}
	}
}

func jsonString(s string) string {
	b, _ := marshalNoEscape(s)
	return string(b)
}

// hiddenMaskRE is the mask of confidential types: bullets and the length only.
var hiddenMaskRE = regexp.MustCompile(`^••• \([0-9]+\)$`)
