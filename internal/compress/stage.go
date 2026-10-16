package compress

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
	"github.com/niclasedge/tollgate/internal/pipeline"
)

const (
	// StageName is the name of the compress stage in traces.
	StageName = "compress"
	// bashTool is the name of Claude Code's shell tool.
	bashTool = "Bash"
	// maxReportLines caps the trace report; the store accepts 64 KiB per stage.
	maxReportLines = 500
)

// Resolver returns the compress configuration of a pinned configuration
// version.
type Resolver interface {
	Compressor(ctx context.Context, version int64) (*Config, error)
}

type builtinResolver struct{}

func (builtinResolver) Compressor(context.Context, int64) (*Config, error) { return Default(), nil }

// Builtin returns a resolver with the built-in rules for every version.
func Builtin() Resolver { return builtinResolver{} }

// Stage is the compress stage (ADR-0007). It runs after redact, so it only
// sees redacted text, and rewrites every Bash tool result of the request,
// the whole history included: the same input gives the same bytes in every
// turn, without session state.
type Stage struct {
	Resolver Resolver
}

// NewStage returns the compress stage for the given resolver.
func NewStage(r Resolver) *Stage { return &Stage{Resolver: r} }

// Name implements pipeline.Stage.
func (*Stage) Name() string { return StageName }

// Process implements pipeline.Stage. Only /v1/messages and
// /v1/messages/count_tokens are compressed, so the count matches what is sent.
func (s *Stage) Process(ctx context.Context, req *pipeline.Request) ([]jsonsplice.Replacement, error) {
	if req.Endpoint != pipeline.EndpointMessages && req.Endpoint != pipeline.EndpointCountTokens {
		return nil, nil
	}
	doc, err := jsonsplice.Scan(req.Body)
	if err != nil {
		return nil, &pipeline.StageError{Status: http.StatusBadRequest, Type: pipeline.ErrTypeInvalidRequest,
			Message: "request body is not valid JSON", Cause: err}
	}
	c, err := s.Resolver.Compressor(ctx, req.ConfigVersion)
	if err != nil {
		return nil, err
	}
	reps, diff := Rewrite(c, doc.Targets)
	req.Report(diff, nil)
	return reps, nil
}

// Rewrite compresses the tool results (and their text blocks) that answer a
// Bash tool_use, using the command of that tool_use. Read, Edit and every
// other tool stay untouched. The report has one line per compressed output
// with its path, rule and sizes; it never contains output text.
func Rewrite(c *Config, targets []jsonsplice.Target) ([]jsonsplice.Replacement, string) {
	commands := map[string]string{} // tool_use id → command; lookups only
	for _, t := range targets {
		if t.Kind == jsonsplice.KindToolUseInput && t.ToolName == bashTool && t.ToolID != "" &&
			t.Path == blockPath(t.Path, ".input.command") {
			commands[t.ToolID] = t.Value
		}
	}
	var reps []jsonsplice.Replacement
	var report strings.Builder
	lines := 0
	for _, t := range targets {
		if t.Kind != jsonsplice.KindToolResult || t.ToolID == "" {
			continue
		}
		cmd, ok := commands[t.ToolID]
		if !ok {
			continue
		}
		out, a, ok := c.Compress(cmd, t.Value)
		if !ok {
			continue
		}
		reps = append(reps, jsonsplice.Replacement{Start: t.Start, End: t.End, New: out})
		if lines < maxReportLines {
			if lines > 0 {
				report.WriteByte('\n')
			}
			report.WriteString(t.Path + " · " + a.Rule + " · " + strconv.Itoa(a.BytesBefore) + " → " +
				strconv.Itoa(a.BytesAfter) + " Bytes · " + strconv.Itoa(a.Omitted) + " Zeilen ausgelassen")
		}
		lines++
	}
	if lines > maxReportLines {
		report.WriteString("\n… " + strconv.Itoa(lines-maxReportLines) + " weitere")
	}
	return reps, report.String()
}

// blockPath returns path when it is "<block>"+suffix for a content block
// path "messages[i].content[j]", else "".
func blockPath(path, suffix string) string {
	base, ok := strings.CutSuffix(path, suffix)
	if !ok || !strings.HasSuffix(base, "]") || strings.Contains(base, ".input") {
		return ""
	}
	return path
}
