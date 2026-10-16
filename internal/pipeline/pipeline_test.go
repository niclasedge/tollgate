package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

const testKey = "tg_test-key-0123456789"

// fakeKeys is the test double for the key verifier (real one: #8).
type fakeKeys map[string]string // key -> client id

func (f fakeKeys) Verify(_ context.Context, key string) (string, error) {
	if id, ok := f[key]; ok {
		return id, nil
	}
	return "", ErrInvalidKey
}

type brokenKeys struct{}

func (brokenKeys) Verify(context.Context, string) (string, error) {
	return "", errors.New("database unreachable: secret-dsn")
}

type fakePinner struct{ version int64 }

func (p fakePinner) Pin(_ context.Context, _, _ string) (int64, error) { return p.version, nil }

// funcStage adapts a function to Stage.
type funcStage struct {
	name string
	fn   func(context.Context, *Request) ([]jsonsplice.Replacement, error)
}

func (s funcStage) Name() string { return s.name }
func (s funcStage) Process(ctx context.Context, r *Request) ([]jsonsplice.Replacement, error) {
	return s.fn(ctx, r)
}

// replaceStage rewrites every target whose value equals from.
func replaceStage(name, from, to string) Stage {
	return funcStage{name, func(_ context.Context, r *Request) ([]jsonsplice.Replacement, error) {
		doc, err := jsonsplice.Scan(r.Body)
		if err != nil {
			return nil, err
		}
		var reps []jsonsplice.Replacement
		for _, t := range doc.Targets {
			if t.Value == from {
				reps = append(reps, jsonsplice.Replacement{Start: t.Start, End: t.End, New: to})
			}
		}
		return reps, nil
	}}
}

func noopStage(name string) Stage {
	return funcStage{name, func(context.Context, *Request) ([]jsonsplice.Replacement, error) { return nil, nil }}
}

func newIntake() *Intake {
	return &Intake{Keys: fakeKeys{testKey: "client-a", "tg_other": "client-b"}}
}

func authHeader() http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+testKey)
	h.Set("Anthropic-Version", "2023-06-01")
	return h
}

func input(path string, body string) Input {
	return Input{Method: http.MethodPost, Path: path, Header: authHeader(), Body: []byte(body)}
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// roundtripBodies cover whitespace, escaping, number forms and unknown parts.
var roundtripBodies = map[string]string{
	"compact":                `{"model":"m","messages":[{"role":"user","content":"hi"}]}`,
	"whitespace and escapes": "{ \"model\" : \"m\",\n\t\"messages\":[ {\"role\":\"user\",\"content\":\"\\u00e4 ä \\/ \\\"x\\\"\"} ] }",
	"numbers":                `{"max_tokens":1.0E3,"temperature":0.10,"big":12345678901234567890123,"messages":[]}`,
	"unknown fields and block types": `{"zz":{"a":null,"b":[true,{"text":"t"}]},"messages":[{"role":"user","extra":1,"content":[
		{"type":"future","text":"stay","nested":{"k":"v"}},{"type":"text","text":"x","citations":[{"a":1}]}]}],"tools":[{"name":"t"}]}`,
	"locked blocks": `{"system":[{"type":"text","text":"sys"}],"messages":[{"role":"assistant","content":[
		{"type":"thinking","thinking":"t","signature":"s=="},{"type":"redacted_thinking","data":"d"},{"type":"compaction","text":"c"}]}]}`,
}

func TestRoundtripKeepsEveryByte(t *testing.T) {
	for name, body := range roundtripBodies {
		t.Run(name, func(t *testing.T) {
			r := NewRunner([]Stage{newIntake(), noopStage("noop")})
			req, trace, err := r.Run(context.Background(), input("/v1/messages", body))
			if err != nil {
				t.Fatal(err)
			}
			if string(req.Body) != body || string(req.OriginalBody) != body {
				t.Fatalf("bytes changed:\n%s", req.Body)
			}
			for _, st := range trace.Stages {
				if st.InHash != sum([]byte(body)) || st.OutHash != st.InHash {
					t.Errorf("stage %s hashes differ for an unchanged body", st.Name)
				}
			}
		})
	}
}

func TestTraceRecordsPerStageData(t *testing.T) {
	body := `{"model":"claude-x","messages":[{"role":"user","content":"my-secret-value"}]}`
	var tick time.Duration
	base := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { tick += 5 * time.Millisecond; return base.Add(tick) }
	est := func(b []byte) int { return len(b) }
	r := NewRunner(
		[]Stage{&Intake{Keys: fakeKeys{testKey: "client-a"}, Pinner: fakePinner{7}}, replaceStage("redact", "my-secret-value", "<redacted:x_1>")},
		WithClock(clock), WithIDGenerator(func() string { return "trace-1" }), WithEstimator(est),
	)
	req, trace, err := r.Run(context.Background(), input("/v1/messages", body))
	if err != nil {
		t.Fatal(err)
	}
	if trace.TraceID != "trace-1" || req.TraceID != "trace-1" {
		t.Errorf("trace id = %q / %q", trace.TraceID, req.TraceID)
	}
	if len(trace.Stages) != 2 || trace.Stages[0].Name != "intake" || trace.Stages[1].Name != "redact" {
		t.Fatalf("stages = %+v", trace.Stages)
	}
	in, red := trace.Stages[0], trace.Stages[1]
	if in.Duration <= 0 || red.Duration <= 0 {
		t.Errorf("durations = %v, %v", in.Duration, red.Duration)
	}
	if in.TokensBefore != len(body) || in.TokensAfter != len(body) {
		t.Errorf("intake tokens = %d/%d", in.TokensBefore, in.TokensAfter)
	}
	if red.TokensBefore != len(body) || red.TokensAfter != len(req.Body) || red.TokensAfter >= red.TokensBefore {
		t.Errorf("redact tokens = %d/%d", red.TokensBefore, red.TokensAfter)
	}
	if red.InHash != in.OutHash || red.InHash != sum([]byte(body)) || red.OutHash != sum(req.Body) {
		t.Errorf("hash chain broken: %+v", trace.Stages)
	}
	if in.ConfigVersion != 7 || red.ConfigVersion != 7 || trace.ConfigVersion != 7 {
		t.Errorf("config version = %d/%d/%d", in.ConfigVersion, red.ConfigVersion, trace.ConfigVersion)
	}
	if len(red.Replacements) != 1 || red.Replacements[0].New != "<redacted:x_1>" {
		t.Errorf("replacements = %+v", red.Replacements)
	}
	if !strings.Contains(string(req.Body), `"<redacted:x_1>"`) {
		t.Errorf("body not rewritten: %s", req.Body)
	}
	if trace.ClientID != "client-a" || trace.StartedAt.IsZero() {
		t.Errorf("trace header = %+v", trace)
	}
}

func TestTraceNeverContainsPlaintext(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"my-secret-value"}]}`
	r := NewRunner([]Stage{newIntake(), replaceStage("redact", "my-secret-value", "<redacted:x_1>")})
	_, trace, err := r.Run(context.Background(), input("/v1/messages", body))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(trace)
	for _, leak := range []string{"my-secret-value", testKey} {
		if bytes.Contains(raw, []byte(leak)) {
			t.Fatalf("trace leaks %q: %s", leak, raw)
		}
	}
}

func TestTraceIDsAreUniqueHex(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{32}$`)
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		id := NewTraceID()
		if !re.MatchString(id) || seen[id] {
			t.Fatalf("bad or duplicate trace id %q", id)
		}
		seen[id] = true
	}
}

func TestFailingStageStopsPipeline(t *testing.T) {
	ran := false
	boom := funcStage{"boom", func(context.Context, *Request) ([]jsonsplice.Replacement, error) {
		return nil, errors.New("contains secret-value")
	}}
	after := funcStage{"after", func(context.Context, *Request) ([]jsonsplice.Replacement, error) { ran = true; return nil, nil }}
	r := NewRunner([]Stage{newIntake(), boom, after})
	_, trace, err := r.Run(context.Background(), input("/v1/messages", `{"messages":[]}`))
	var se *StageError
	if !errors.As(err, &se) || se.Status != 500 || se.Type != "api_error" || se.Stage != "boom" {
		t.Fatalf("err = %#v", err)
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Errorf("error text leaks stage error: %v", err)
	}
	if ran {
		t.Error("later stage ran after failure")
	}
	if len(trace.Stages) != 2 || !trace.Stages[1].Failed || trace.Stages[0].Failed {
		t.Errorf("trace = %+v", trace.Stages)
	}
	status, body := ErrorResponse(err)
	if status != 500 || string(body) != `{"type":"error","error":{"type":"api_error","message":"internal error in stage boom"}}` {
		t.Errorf("response = %d %s", status, body)
	}
}

func TestInvalidReplacementAbortsInsteadOfForwarding(t *testing.T) {
	bad := funcStage{"bad", func(context.Context, *Request) ([]jsonsplice.Replacement, error) {
		return []jsonsplice.Replacement{{Start: 0, End: 1, New: "x"}}, nil
	}}
	r := NewRunner([]Stage{newIntake(), bad})
	_, trace, err := r.Run(context.Background(), input("/v1/messages", `{"messages":[]}`))
	var se *StageError
	if !errors.As(err, &se) || se.Status != 500 || !trace.Stages[1].Failed {
		t.Fatalf("err = %v, trace = %+v", err, trace.Stages)
	}
}

func TestReplacementsOutsideTargetsAbort(t *testing.T) {
	body := `{"messages":[{"role":"assistant","content":[` +
		`{"type":"thinking","thinking":"thought","signature":"SIG=="},` +
		`{"type":"text","text":"say \"hi\" now"}]}]}`
	tokenOf := func(s string) [2]int {
		i := strings.Index(body, s)
		return [2]int{i, i + len(s)}
	}
	cases := map[string][2]int{
		"signature":     tokenOf(`"SIG=="`),
		"thinking":      tokenOf(`"thought"`),
		"member name":   tokenOf(`"text"`),
		"inside string": {strings.Index(body, `\"hi`) + 1, strings.Index(body, `now"`) + 4},
	}
	for name, rng := range cases {
		t.Run(name, func(t *testing.T) {
			bad := funcStage{"bad", func(context.Context, *Request) ([]jsonsplice.Replacement, error) {
				return []jsonsplice.Replacement{{Start: rng[0], End: rng[1], New: "X"}}, nil
			}}
			req, trace, err := NewRunner([]Stage{newIntake(), bad}).Run(context.Background(), input("/v1/messages", body))
			var se *StageError
			if !errors.As(err, &se) || se.Status != 500 || !trace.Stages[1].Failed {
				t.Fatalf("err = %v, trace = %+v", err, trace.Stages)
			}
			if string(req.Body) != body {
				t.Fatalf("body changed: %s", req.Body)
			}
		})
	}
}

func TestRunIsDeterministicAndRaceFree(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"a-secret"},{"role":"user","content":[{"type":"text","text":"a-secret"}]}]}`
	r := NewRunner([]Stage{newIntake(), replaceStage("redact", "a-secret", "<redacted:x_1>")})
	want := ""
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _, err := r.Run(context.Background(), input("/v1/messages", body))
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if h := sum(req.Body); want == "" {
				want = h
			} else if h != want {
				t.Error("non-deterministic output")
			}
		}()
	}
	wg.Wait()
}

func TestRunDoesNotMutateInput(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"a-secret"}]}`)
	orig := string(body)
	h := authHeader()
	r := NewRunner([]Stage{newIntake(), replaceStage("redact", "a-secret", "<redacted:x_1>")})
	if _, _, err := r.Run(context.Background(), Input{Method: "POST", Path: "/v1/messages", Header: h, Body: body}); err != nil {
		t.Fatal(err)
	}
	if string(body) != orig {
		t.Fatal("input body mutated")
	}
}

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens(nil) != 0 || EstimateTokens([]byte("abcd")) != 1 || EstimateTokens([]byte("abcde")) != 2 {
		t.Fatal("unexpected default estimate")
	}
}

func TestRunnerAuthenticateUsesHeadersOnly(t *testing.T) {
	ctx := context.Background()
	r := NewRunner([]Stage{newIntake(), noopStage("redact")})
	if err := r.Authenticate(ctx, authHeader()); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	var se *StageError
	err := r.Authenticate(ctx, http.Header{"Authorization": {"Bearer tg_nope"}})
	if !errors.As(err, &se) || se.Status != http.StatusUnauthorized || se.Stage != StageIntake {
		t.Fatalf("unknown key: %v", err)
	}
	err = NewRunner([]Stage{&Intake{Keys: brokenKeys{}}}).Authenticate(ctx, authHeader())
	if !errors.As(err, &se) || se.Status != http.StatusInternalServerError {
		t.Fatalf("broken verifier: %v", err)
	}
	if err := NewRunner([]Stage{noopStage("redact")}).Authenticate(ctx, nil); err != nil {
		t.Fatalf("runner without authenticator: %v", err)
	}
}
