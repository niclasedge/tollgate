package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

func reportStage(name, diff string, matches []Match, fail bool) Stage {
	return funcStage{name, func(_ context.Context, r *Request) ([]jsonsplice.Replacement, error) {
		r.Report(diff, matches)
		if fail {
			return nil, errors.New("boom")
		}
		return nil, nil
	}}
}

func TestStageReportInTrace(t *testing.T) {
	m := []Match{{Offset: 3, Length: 18, Type: "email", Rule: "email"}}
	r := NewRunner([]Stage{newIntake(), reportStage("first", "d1", m, false), noopStage("second")})
	_, trace, err := r.Run(context.Background(), input("/v1/messages", `{"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.Stages) != 3 {
		t.Fatalf("stages = %d", len(trace.Stages))
	}
	if s := trace.Stages[1]; s.Diff != "d1" || !reflect.DeepEqual(s.Matches, m) {
		t.Errorf("first stage = %+v", s)
	}
	for _, i := range []int{0, 2} {
		if s := trace.Stages[i]; s.Diff != "" || s.Matches != nil {
			t.Errorf("stage %s carries a foreign report: %+v", s.Name, s)
		}
	}
}

func TestFailedStageReportIsDropped(t *testing.T) {
	r := NewRunner([]Stage{newIntake(), reportStage("bad", "partial", []Match{{Type: "x"}}, true)})
	_, trace, err := r.Run(context.Background(), input("/v1/messages", `{"messages":[]}`))
	if err == nil {
		t.Fatal("want error")
	}
	if s := trace.Stages[1]; !s.Failed || s.Diff != "" || s.Matches != nil {
		t.Errorf("failed stage = %+v", s)
	}
}

func TestPlaceholdersAreNotSerialized(t *testing.T) {
	req := &Request{TraceID: "t", Placeholders: map[string]string{"<redacted:email_1>": "plain-secret-value"}}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "plain-secret-value") || strings.Contains(string(b), "redacted:email_1") {
		t.Errorf("mapping serialized: %s", b)
	}
}
