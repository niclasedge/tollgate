package redact

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
	"github.com/niclasedge/tollgate/internal/pipeline"
)

const stageKey = "tg_stage-test-key-0123456789"

type stageKeys struct{}

func (stageKeys) Verify(_ context.Context, key string) (string, error) {
	if key == stageKey {
		return "client-a", nil
	}
	return "", pipeline.ErrInvalidKey
}

type fixedPinner int64

func (p fixedPinner) Pin(context.Context, string, string) (int64, error) { return int64(p), nil }

// js returns s as a JSON string literal without HTML escaping, so that
// placeholder text stays readable in the body.
func js(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// Test values. Key-shaped values are joined at runtime (secret scanners).
var (
	mailA   = "jane.roe@mailhost.de"
	mailB   = "max.muster@mailhost.de"
	antKey  = fakeKey("sk-ant-api03-", "abcdefghijklmnopqrstuvwxyz0123")
	connURL = "postgres://app:s3cretpw@db.prod.io:5432/x"
)

// runStage sends body through intake and the redact stage and returns the
// request, the trace and the error of the run.
func runStage(t *testing.T, path, body string, res Resolver, pin int64) (*pipeline.Request, *pipeline.Trace, error) {
	t.Helper()
	if res == nil {
		res = Builtin()
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+stageKey)
	h.Set("X-Claude-Code-Session-Id", "00000000-0000-4000-8000-000000000001")
	r := pipeline.NewRunner([]pipeline.Stage{
		&pipeline.Intake{Keys: stageKeys{}, Pinner: fixedPinner(pin)},
		NewStage(res),
	})
	return r.Run(context.Background(), pipeline.Input{Method: http.MethodPost, Path: path, Header: h, Body: []byte(body)})
}

func mustRun(t *testing.T, body string) (*pipeline.Request, *pipeline.Trace) {
	t.Helper()
	req, tr, err := runStage(t, "/v1/messages", body, nil, 0)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return req, tr
}

func stageTrace(t *testing.T, tr *pipeline.Trace) pipeline.StageTrace {
	t.Helper()
	for _, s := range tr.Stages {
		if s.Name == StageName {
			return s
		}
	}
	t.Fatalf("no %s stage in trace", StageName)
	return pipeline.StageTrace{}
}

func msgs(m ...string) string {
	return `{"model":"claude-test","stream":true,"messages":[` + strings.Join(m, ",") + `]}`
}

func TestStageNumbering(t *testing.T) {
	cases := []struct {
		name, in, want string
		mapping        map[string]string
	}{
		{
			name: "same value same number",
			in:   msgs(`{"role":"user","content":` + js("a "+mailA+" b "+mailB+" c "+mailA) + `}`),
			want: msgs(`{"role":"user","content":"a <redacted:email_1> b <redacted:email_2> c <redacted:email_1>"}`),
			mapping: map[string]string{
				"<redacted:email_1>": mailA, "<redacted:email_2>": mailB,
			},
		},
		{
			name: "numbering per type across messages and blocks",
			in: msgs(
				`{"role":"user","content":[{"type":"text","text":`+js("key "+antKey)+`}]}`,
				`{"role":"assistant","content":[{"type":"text","text":`+js("mail "+mailA)+`},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":`+js("psql "+connURL)+`,"env":{"list":[`+js(mailB)+`,1,true]}}}]}`,
				`{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":`+js("out "+mailA)+`},{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":`+js(antKey)+`},{"type":"image","source":{}}]}]}`,
			),
			want: msgs(
				`{"role":"user","content":[{"type":"text","text":"key <redacted:api_key_1>"}]}`,
				`{"role":"assistant","content":[{"type":"text","text":"mail <redacted:email_1>"},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"psql postgres://app:<redacted:connection_password_1>@db.prod.io:5432/x","env":{"list":["<redacted:email_2>",1,true]}}}]}`,
				`{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"out <redacted:email_1>"},{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"<redacted:api_key_1>"},{"type":"image","source":{}}]}]}`,
			),
			mapping: map[string]string{
				"<redacted:api_key_1>": antKey, "<redacted:email_1>": mailA, "<redacted:email_2>": mailB,
				"<redacted:connection_password_1>": "s3cretpw",
			},
		},
		{
			name: "reserved numbers are skipped and never mapped",
			in: msgs(
				`{"role":"user","content":`+js("seen <redacted:email_1> and <redacted:email_3>")+`}`,
				`{"role":"user","content":`+js(mailA+" "+mailB)+`}`,
			),
			want: msgs(
				`{"role":"user","content":"seen <redacted:email_1> and <redacted:email_3>"}`,
				`{"role":"user","content":"<redacted:email_2> <redacted:email_4>"}`,
			),
			mapping: map[string]string{"<redacted:email_2>": mailA, "<redacted:email_4>": mailB},
		},
		{
			name: "reservation by a later message",
			in: msgs(
				`{"role":"user","content":`+js(mailA)+`}`,
				`{"role":"user","content":"typed <redacted:email_1>"}`,
			),
			want: msgs(
				`{"role":"user","content":"<redacted:email_2>"}`,
				`{"role":"user","content":"typed <redacted:email_1>"}`,
			),
			mapping: map[string]string{"<redacted:email_2>": mailA},
		},
		{
			name:    "escaped input is rewritten with the fixed escaper",
			in:      msgs(`{"role":"user","content":"line\u0021\n` + mailA + `\/x"}`),
			want:    msgs(`{"role":"user","content":"line!\n<redacted:email_1>/x"}`),
			mapping: map[string]string{"<redacted:email_1>": mailA},
		},
		{
			name: "no secret leaves the body untouched",
			in:   msgs(`{"role":"user","content":"hello\u0021 world"}`),
			want: msgs(`{"role":"user","content":"hello\u0021 world"}`),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, _ := mustRun(t, c.in)
			if got := string(req.Body); got != c.want {
				t.Errorf("body\n got %s\nwant %s", got, c.want)
			}
			if len(c.mapping) == 0 && len(req.Placeholders) != 0 {
				t.Errorf("mapping = %v, want empty", keys(req.Placeholders))
			}
			if len(c.mapping) > 0 && !reflect.DeepEqual(req.Placeholders, c.mapping) {
				t.Errorf("mapping keys %v, want %v", keys(req.Placeholders), keys(c.mapping))
			}
		})
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestStageLeavesProtectedBlocksByteIdentical(t *testing.T) {
	// Every protected place carries the same e-mail address; only the two
	// allowed targets may change.
	m := js(mailA)
	in := `{"model":"claude-test","system":[{"type":"text","text":` + m + `}],` +
		`"tools":[{"name":"x","description":` + m + `,"input_schema":{"type":"object"}}],` +
		`"metadata":{"user_id":` + m + `},"future_top":{"text":` + m + `},` +
		`"messages":[` +
		`{"role":"assistant","content":[` +
		`{"type":"thinking","thinking":` + js("think "+mailA+" <redacted:email_1>") + `,"signature":"c2lnbmF0dXJl\u002b+/="},` +
		`{"type":"redacted_thinking","data":` + m + `},` +
		`{"type":"text","text":` + m + `,"citations":[{"cited_text":` + m + `}],"x_unknown":` + m + `},` +
		`{"type":"future_block","text":` + m + `,"content":` + m + `,"input":{"a":` + m + `}}]},` +
		`{"role":"user","content":[` +
		`{"type":"image","source":{"type":"base64","data":` + m + `}},` +
		`{"type":"document","source":{"type":"text","data":` + m + `},"title":` + m + `},` +
		`{"type":"tool_result","tool_use_id":"t","content":[{"type":"text","text":` + m + `},{"type":"future","text":` + m + `}],"is_error":false},` +
		`{"type":"compaction","content":` + m + `}],"x_msg":` + m + `}]}`
	want := strings.Replace(in, `{"type":"text","text":`+m+`,"citations"`, `{"type":"text","text":"<redacted:email_1>","citations"`, 1)
	want = strings.Replace(want, `"content":[{"type":"text","text":`+m+`},{"type":"future"`, `"content":[{"type":"text","text":"<redacted:email_1>"},{"type":"future"`, 1)
	if want == in {
		t.Fatal("test setup: replacements did not apply")
	}
	req, _ := mustRun(t, in)
	if got := string(req.Body); got != want {
		t.Errorf("body\n got %s\nwant %s", got, want)
	}
	// The placeholder in the thinking block must not reserve email_1.
	if req.Placeholders["<redacted:email_1>"] != mailA {
		t.Errorf("thinking placeholder reserved a number: %v", keys(req.Placeholders))
	}
}

func TestStageEndpoints(t *testing.T) {
	body := msgs(`{"role":"user","content":` + js("hi "+mailA) + `}`)
	msg, _ := mustRun(t, body)
	ct, _, err := runStage(t, "/v1/messages/count_tokens", body, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(ct.Body) != string(msg.Body) || string(msg.Body) == body {
		t.Errorf("count_tokens body %s, messages body %s", ct.Body, msg.Body)
	}
	for _, path := range []string{"/v1/models", "/v1/messages/batches"} {
		req, tr, err := runStage(t, path, body, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if string(req.Body) != body || len(req.Placeholders) != 0 || len(stageTrace(t, tr).Replacements) != 0 {
			t.Errorf("%s: body changed to %s", path, req.Body)
		}
	}
}

func TestStageDeterministic(t *testing.T) {
	body := msgs(
		`{"role":"user","content":`+js(strings.Repeat(mailA+" "+antKey+" "+mailB+" "+connURL+"\n", 5))+`}`,
		`{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"Bash","input":{"b":`+js(mailB)+`,"a":`+js(antKey)+`}}]}`,
	)
	req1, tr1 := mustRun(t, body)
	for i := 0; i < 20; i++ {
		req2, tr2 := mustRun(t, body)
		s1, s2 := stageTrace(t, tr1), stageTrace(t, tr2)
		if string(req1.Body) != string(req2.Body) || !reflect.DeepEqual(req1.Placeholders, req2.Placeholders) ||
			s1.Diff != s2.Diff || !reflect.DeepEqual(s1.Matches, s2.Matches) || s1.OutHash != s2.OutHash {
			t.Fatalf("run %d differs", i)
		}
	}
}

// TestStageCachePrefix is the cache test: three turns of one session that
// only grow at the end yield byte-identical prefixes, and repeated secrets
// keep their numbers.
func TestStageCachePrefix(t *testing.T) {
	turns := []string{
		`{"role":"user","content":` + js("connect "+connURL+" as "+mailA) + `}`,
		`{"role":"assistant","content":[{"type":"text","text":` + js("ok "+mailA) + `},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":` + js("psql "+connURL) + `}}]}`,
		`{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":` + js("ok "+mailB+" "+antKey) + `},{"type":"text","text":` + js("again "+mailA) + `}]}`,
		`{"role":"assistant","content":[{"type":"thinking","thinking":"use <redacted:email_1>","signature":"sig"},{"type":"text","text":` + js("done "+mailB) + `}]}`,
		`{"role":"user","content":` + js("thanks <redacted:email_9> "+mailA) + `}`,
	}
	var prev string
	for n := 1; n <= len(turns); n += 2 {
		req, _ := mustRun(t, msgs(turns[:n]...))
		out := string(req.Body)
		if prev != "" {
			prefix := strings.TrimSuffix(prev, "]}")
			if !strings.HasPrefix(out, prefix) {
				t.Fatalf("turn %d: prefix changed\nprev %s\n now %s", n, prefix, out)
			}
		}
		if req.Placeholders["<redacted:email_1>"] != mailA || req.Placeholders["<redacted:connection_password_1>"] != "s3cretpw" {
			t.Fatalf("turn %d: numbers of repeated secrets changed: %v", n, keys(req.Placeholders))
		}
		prev = out
	}
}

var placeholderOut = regexp.MustCompile(`<redacted:([a-z][a-z0-9_]{0,31})_[1-9][0-9]{0,5}>`)

func TestStageTraceWithoutPlaintext(t *testing.T) {
	body := msgs(
		`{"role":"user","content":`+js("mail "+mailA+"\nkey "+antKey)+`}`,
		`{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"Bash","input":{"command":`+js("psql "+connURL)+`}}]}`,
	)
	req, tr := mustRun(t, body)
	st := stageTrace(t, tr)
	raw, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{mailA, antKey, "s3cretpw"} {
		if strings.Contains(string(raw), secret) || strings.Contains(st.Diff, secret) || strings.Contains(string(req.Body), secret) {
			t.Errorf("plaintext %q in trace or diff", secret)
		}
	}
	if len(st.Matches) != 3 {
		t.Fatalf("matches = %+v, want 3", st.Matches)
	}
	wantTypes := []string{"email", "api_key", "connection_password"}
	for i, m := range st.Matches {
		ph := string(req.Body[m.Offset : m.Offset+m.Length])
		sm := placeholderOut.FindStringSubmatch(ph)
		if sm == nil || sm[0] != ph || sm[1] != m.Type || m.Type != wantTypes[i] || m.Rule == "" {
			t.Errorf("match %d %+v points at %q", i, m, ph)
		}
	}
	for _, ph := range []string{"<redacted:email_1>", "<redacted:api_key_1>", "<redacted:connection_password_1>"} {
		if !strings.Contains(st.Diff, ph) {
			t.Errorf("diff lacks %s:\n%s", ph, st.Diff)
		}
	}
	if !strings.Contains(st.Diff, "messages[1].content[0].input.command") {
		t.Errorf("diff lacks the target path:\n%s", st.Diff)
	}
	if st.InHash == st.OutHash || len(st.Replacements) != 2 {
		t.Errorf("hashes or replacements not recorded: %+v", st)
	}
}

func TestStageMatchOffsetsWithEscapes(t *testing.T) {
	// Escaped characters before a placeholder shift its byte offset.
	body := msgs(`{"role":"user","content":` + js("a\"b\\c\n\t\x01ü "+mailA+" \"x\" "+mailB) + `}`)
	req, tr := mustRun(t, body)
	st := stageTrace(t, tr)
	if len(st.Matches) != 2 {
		t.Fatalf("matches = %+v", st.Matches)
	}
	for _, m := range st.Matches {
		if ph := string(req.Body[m.Offset : m.Offset+m.Length]); !placeholderOut.MatchString(ph) || !strings.HasPrefix(ph, "<") || !strings.HasSuffix(ph, ">") {
			t.Errorf("match %+v points at %q", m, ph)
		}
	}
}

func TestStageDiffBounded(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 3000; i++ {
		b.WriteString("user")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString(itoa(i))
		b.WriteString("@mailhost.de, ")
	}
	req, tr := mustRun(t, msgs(`{"role":"user","content":`+js(b.String())+`}`))
	st := stageTrace(t, tr)
	if len(st.Diff) > maxDiffBytes+256 || len(st.Matches) != maxMatches {
		t.Errorf("diff %d bytes, %d matches", len(st.Diff), len(st.Matches))
	}
	if !strings.Contains(st.Diff, "more") {
		t.Errorf("diff does not say it was cut")
	}
	if len(req.Placeholders) != 3000 {
		t.Errorf("mapping has %d entries, want 3000", len(req.Placeholders))
	}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func TestStageFingerprintNote(t *testing.T) {
	cases := []struct {
		name string
		body string
		note bool
	}{
		{"secret at start of first user message", msgs(`{"role":"user","content":[{"type":"text","text":` + js(mailA+" please") + `}]}`), true},
		{"secret late in first user message", msgs(`{"role":"user","content":` + js("please look at this address: "+mailA) + `}`), false},
		{"secret at start of a later user message", msgs(
			`{"role":"user","content":"please look at this one"}`,
			`{"role":"assistant","content":"ok"}`,
			`{"role":"user","content":`+js(mailA)+`}`), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, tr := mustRun(t, c.body)
			got := strings.HasPrefix(stageTrace(t, tr).Diff, FingerprintNote+"\n")
			if got != c.note {
				t.Errorf("note = %v, want %v; diff:\n%s", got, c.note, stageTrace(t, tr).Diff)
			}
		})
	}
}

type recordingResolver struct{ got []int64 }

func (r *recordingResolver) Redactor(_ context.Context, v int64) (*Redactor, error) {
	r.got = append(r.got, v)
	return New(DefaultOptions()), nil
}

type failingResolver struct{}

func (failingResolver) Redactor(context.Context, int64) (*Redactor, error) {
	return nil, errors.New("load version: " + mailA)
}

func TestStageResolver(t *testing.T) {
	rr := &recordingResolver{}
	if _, _, err := runStage(t, "/v1/messages", msgs(`{"role":"user","content":"x"}`), rr, 7); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rr.got, []int64{7}) {
		t.Errorf("resolver versions = %v, want [7]", rr.got)
	}
	_, _, err := runStage(t, "/v1/messages", msgs(`{"role":"user","content":"x"}`), failingResolver{}, 0)
	var se *pipeline.StageError
	if !errors.As(err, &se) || se.Status != http.StatusInternalServerError || strings.Contains(se.Error(), mailA) {
		t.Errorf("err = %v", err)
	}
	if status, b := pipeline.ErrorResponse(err); status != 500 || strings.Contains(string(b), mailA) {
		t.Errorf("error response %d %s", status, b)
	}
}

func TestStageNumberOverflowAborts(t *testing.T) {
	old := numberLimit
	numberLimit = 2
	defer func() { numberLimit = old }()
	body := msgs(`{"role":"user","content":` + js("<redacted:email_2> "+mailA+" "+mailB) + `}`)
	_, _, err := runStage(t, "/v1/messages", body, nil, 0)
	var se *pipeline.StageError
	if !errors.As(err, &se) || se.Stage != StageName || se.Status != http.StatusInternalServerError {
		t.Fatalf("err = %v, want 500 from %s", err, StageName)
	}
	if strings.Contains(err.Error(), mailB) {
		t.Errorf("error leaks plaintext: %v", err)
	}
}

func TestNumberIsPure(t *testing.T) {
	body := []byte(msgs(`{"role":"user","content":` + js(mailA+" "+mailB) + `}`))
	doc, err := jsonsplice.Scan(body)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Number(New(DefaultOptions()), doc.Targets)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Number(New(DefaultOptions()), doc.Targets)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Number not deterministic")
	}
	out, err := jsonsplice.Apply(body, a.Replacements)
	if err != nil {
		t.Fatal(err)
	}
	if want := msgs(`{"role":"user","content":"<redacted:email_1> <redacted:email_2>"}`); string(out) != want {
		t.Errorf("got %s", out)
	}
}

// TestStageMasks: every match carries the mask of its value (issue #112);
// the same value gets the same mask, and the mask never reaches the body.
func TestStageMasks(t *testing.T) {
	body := msgs(`{"role":"user","content":` + js("mail "+mailA+", nochmal "+mailA+"\nkey "+antKey) + `}`)
	req, tr := mustRun(t, body)
	st := stageTrace(t, tr)
	want := []string{Mask("email", TypeEmail, mailA), Mask("email", TypeEmail, mailA), Mask("anthropic_key", TypeAPIKey, antKey)}
	if len(st.Matches) != len(want) {
		t.Fatalf("matches = %+v", st.Matches)
	}
	for i, m := range st.Matches {
		if m.Mask == "" || m.Mask != want[i] {
			t.Errorf("match %d mask = %q, want %q", i, m.Mask, want[i])
		}
	}
	wantBody := msgs(`{"role":"user","content":` + js("mail <redacted:email_1>, nochmal <redacted:email_1>\nkey <redacted:api_key_1>") + `}`)
	if string(req.Body) != wantBody {
		t.Errorf("body = %s, want %s", req.Body, wantBody)
	}
	raw, _ := json.Marshal(tr)
	for _, secret := range []string{mailA, antKey} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("plaintext %q in trace", secret)
		}
	}
}
