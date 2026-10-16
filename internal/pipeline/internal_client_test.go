package pipeline

import (
	"context"
	"net/http"
	"testing"
)

const probeBody = `{"model":"m","stream":true,"messages":[{"role":"user","content":"my-secret-value"}]}`

func stageNames(tr *Trace) []string {
	var out []string
	for _, s := range tr.Stages {
		out = append(out, s.Name)
	}
	return out
}

// The internal identity authenticates without a key and may skip only the
// redact stage (#87).
func TestInternalClientSkipsOnlyRedact(t *testing.T) {
	r := NewRunner([]Stage{newIntake(), replaceStage("redact", "my-secret-value", "<redacted:x_1>"), replaceStage("compress", "x", "y")})
	in := Input{Method: http.MethodPost, Path: "/v1/messages", Header: http.Header{}, Body: []byte(probeBody)}

	ctx := WithInternalClient(context.Background(), InternalClient{ID: "speedtest", SkipRedact: true})
	req, tr, err := r.Run(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if req.ClientID != "speedtest" || len(tr.Stages) != 2 || tr.Stages[1].Name != "compress" {
		t.Fatalf("client %q, stages %v", req.ClientID, stageNames(tr))
	}
	if string(req.Body) != probeBody {
		t.Fatalf("redact ran: %s", req.Body)
	}

	ctx = WithInternalClient(context.Background(), InternalClient{ID: "speedtest"})
	req, tr, err = r.Run(ctx, in)
	if err != nil || len(tr.Stages) != 3 || string(req.Body) == probeBody {
		t.Fatalf("with redact: %v %v %s", err, stageNames(tr), req.Body)
	}
	if tr.Stages[1].BytesBefore != len(probeBody) || tr.Stages[1].BytesAfter != len(req.Body) {
		t.Fatalf("stage bytes %d -> %d", tr.Stages[1].BytesBefore, tr.Stages[1].BytesAfter)
	}
}

// Headers or body cannot imitate the internal identity or skip a stage.
func TestInternalClientNotReachableFromHTTP(t *testing.T) {
	r := NewRunner([]Stage{newIntake(), replaceStage("redact", "my-secret-value", "<redacted:x_1>")})
	h := http.Header{}
	h.Set("X-Internal-Client", "speedtest")
	h.Set("X-Skip-Redact", "true")
	body := `{"model":"m","internal_client":"speedtest","skip_redact":true,"messages":[]}`
	if _, _, err := r.Run(context.Background(), Input{Method: http.MethodPost, Path: "/v1/messages", Header: h, Body: []byte(body)}); err == nil {
		t.Fatal("request without key accepted")
	}
	h.Set("Authorization", "Bearer "+testKey)
	req, tr, err := r.Run(context.Background(), Input{Method: http.MethodPost, Path: "/v1/messages", Header: h, Body: []byte(probeBody)})
	if err != nil || req.ClientID != "client-a" || len(tr.Stages) != 2 {
		t.Fatalf("client %q stages %v err %v", req.ClientID, stageNames(tr), err)
	}
}
