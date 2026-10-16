package server

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/redact"
)

// TestReturnPathRehydratesThroughServer checks the wiring of #14: the proxy
// redacts the request, the upstream answers with the placeholder split over
// chunk boundaries (SSE) or in a JSON body, and the client sees the original
// value in both, also inside input_json_delta.
func TestReturnPathRehydratesThroughServer(t *testing.T) {
	const mail = "jane.roe@mailhost.de"
	sse := []string{
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"mail <reda\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"cted:email_1> ok\"}}\n\n",
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"t\",\"name\":\"Bash\",\"input\":{}}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"command\\\":\\\"echo <redacted:em\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"ail_1>\\\"}\"}}\n\n",
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
	}
	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    []string
	}{
		{"sse", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, chunk := range sse {
				_, _ = io.WriteString(w, chunk)
				w.(http.Flusher).Flush()
			}
		}, []string{"mail " + mail + " ok", `echo ` + mail}},
		{"json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"mail <redacted:email_1>"}]}`)
		}, []string{"mail " + mail}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := newUpstream(t, tc.handler)
			ts := newProxy(t, up, proxyOpts{}, redact.NewStage(redact.Builtin()))
			body := `{"model":"claude-test","stream":true,"messages":[{"role":"user","content":"mail ` + mail + `"}]}`
			resp, got := doReq(t, http.MethodPost, ts.URL+"/v1/messages", body, auth(nil))
			if resp.StatusCode != 200 {
				t.Fatalf("status %d: %s", resp.StatusCode, got)
			}
			if seen := up.seen(); len(seen) != 1 || strings.Contains(string(seen[0].Body), mail) {
				t.Fatalf("upstream saw plaintext or wrong count: %d", len(seen))
			}
			joined := joinDeltas(string(got))
			for _, w := range tc.want {
				if !strings.Contains(joined, w) {
					t.Errorf("client response lacks %q:\n%s", w, got)
				}
			}
			if strings.Contains(string(got), "<redacted:") {
				t.Errorf("client response still has a placeholder:\n%s", got)
			}
		})
	}
}

// joinDeltas concatenates all text and partial_json delta strings of an SSE
// body (unescaped) so assertions do not depend on event boundaries.
func joinDeltas(body string) string {
	var b strings.Builder
	for _, key := range []string{`"text":"`, `"partial_json":"`} {
		rest := body
		for {
			i := strings.Index(rest, key)
			if i < 0 {
				break
			}
			rest = rest[i+len(key):]
			end := 0
			for end < len(rest) && (rest[end] != '"' || (end > 0 && rest[end-1] == '\\')) {
				end++
			}
			b.WriteString(strings.ReplaceAll(rest[:end], `\"`, `"`))
			rest = rest[end:]
		}
	}
	return b.String()
}
