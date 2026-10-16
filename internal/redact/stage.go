package redact

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
	"github.com/niclasedge/tollgate/internal/pipeline"
)

const (
	// StageName is the name of the redact stage in traces.
	StageName = "redact"
	// BuiltinConfig names the fixed configuration DefaultOptions() that
	// applies until the versioned configuration (#15) exists. In traces it
	// appears as configuration version 0.
	BuiltinConfig = "builtin-v1"
	// FingerprintNote is the first diff line when the stage changed one of
	// the characters Claude Code hashes into the cc_version fingerprint
	// (spike of #13, see the change design).
	FingerprintNote = "note: cc_version fingerprint input changed"

	maxDiffBytes = 32 << 10 // the store accepts 64 KiB per stage
	maxMatches   = 1000
	diffContext  = 24 // bytes of redacted context on each side of a placeholder
)

// numberLimit is the largest N of the placeholder grammar (ADR-0006). A
// variable so tests can provoke the overflow.
var numberLimit = 999999

var (
	typeRE            = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	errNumberOverflow = errors.New("redact: placeholder numbers exhausted for a type")
	errPlaceholderTyp = errors.New("redact: placeholder type outside the ADR-0006 grammar")
)

// Resolver returns the Redactor for a pinned configuration version.
type Resolver interface {
	Redactor(ctx context.Context, version int64) (*Redactor, error)
}

type builtinResolver struct{ r *Redactor }

func (b builtinResolver) Redactor(context.Context, int64) (*Redactor, error) { return b.r, nil }

// Builtin returns the resolver for BuiltinConfig. It ignores the version;
// issue #15 replaces it with a resolver that loads the pinned version.
func Builtin() Resolver { return builtinResolver{r: New(DefaultOptions())} }

// Stage is the redact stage (ADR-0006). It replaces secrets in the targets
// of jsonsplice.Scan with numbered placeholders and leaves the mapping in
// Request.Placeholders for the return path.
type Stage struct {
	Resolver Resolver
}

// NewStage returns the redact stage for the given resolver.
func NewStage(r Resolver) *Stage { return &Stage{Resolver: r} }

// Name implements pipeline.Stage.
func (*Stage) Name() string { return StageName }

// Process implements pipeline.Stage. Only /v1/messages and
// /v1/messages/count_tokens are redacted; every other endpoint passes.
func (s *Stage) Process(ctx context.Context, req *pipeline.Request) ([]jsonsplice.Replacement, error) {
	if req.Endpoint != pipeline.EndpointMessages && req.Endpoint != pipeline.EndpointCountTokens {
		return nil, nil
	}
	doc, err := jsonsplice.Scan(req.Body)
	if err != nil {
		return nil, &pipeline.StageError{Status: http.StatusBadRequest, Type: pipeline.ErrTypeInvalidRequest,
			Message: "request body is not valid JSON", Cause: err}
	}
	r, err := s.Resolver.Redactor(ctx, req.ConfigVersion)
	if err != nil {
		return nil, err
	}
	res, err := Number(r, doc.Targets)
	if err != nil {
		return nil, err
	}
	req.Placeholders = res.Placeholders
	req.Report(res.Diff, res.Matches)
	return res.Replacements, nil
}

// Result is the outcome of Number.
type Result struct {
	// Replacements rewrite every target with at least one finding.
	Replacements []jsonsplice.Replacement
	// Placeholders maps placeholder to plaintext. RAM only, never logged.
	Placeholders map[string]string
	// Matches locate the placeholders in the body after Replacements are
	// applied; at most maxMatches.
	Matches []pipeline.Match
	// Diff lists every placeholder with redacted context, bounded in size.
	Diff string
}

type valueKey struct{ typ, val string }

// Number redacts the targets (in document order, as returned by
// jsonsplice.Scan) with numbered placeholders following ADR-0006:
//
//   - every placeholder already present in a target reserves its number;
//   - per type, each new value gets the next free number on first sight,
//     the same value always the same number;
//   - targets without findings stay untouched.
//
// Number is pure: maps are used for lookups only, never iterated.
func Number(r *Redactor, targets []jsonsplice.Target) (Result, error) {
	reserved := map[string]map[int]bool{}
	for _, t := range targets {
		for _, m := range placeholderRE.FindAllStringSubmatch(t.Value, -1) {
			n, _ := strconv.Atoi(m[2])
			if reserved[m[1]] == nil {
				reserved[m[1]] = map[int]bool{}
			}
			reserved[m[1]][n] = true
		}
	}

	var (
		res      Result
		assigned = map[valueKey]string{}
		last     = map[string]int{}
		diff     diffBuilder
		escBuf   []byte
		shift    int // output offset minus input offset after the previous replacement
		firstMsg = firstUserMessage(targets)
	)
	for _, t := range targets {
		fs := r.Find(t.Value)
		if len(fs) == 0 {
			continue
		}
		var b strings.Builder
		b.Grow(len(t.Value))
		escLen, pos := 0, 0
		var spans [][2]int // placeholder ranges in the new value
		for _, f := range fs {
			seg := t.Value[pos:f.Start]
			b.WriteString(seg)
			escBuf = jsonsplice.Escape(escBuf[:0], seg)
			escLen += len(escBuf)

			k := valueKey{f.Type, t.Value[f.Start:f.End]}
			ph, ok := assigned[k]
			if !ok {
				if !typeRE.MatchString(f.Type) {
					return Result{}, errPlaceholderTyp
				}
				n := last[f.Type] + 1
				for reserved[f.Type][n] {
					n++
				}
				if n > numberLimit {
					return Result{}, errNumberOverflow
				}
				last[f.Type] = n
				ph = "<redacted:" + f.Type + "_" + strconv.Itoa(n) + ">"
				assigned[k] = ph
				if res.Placeholders == nil {
					res.Placeholders = map[string]string{}
				}
				res.Placeholders[ph] = k.val
			}
			if len(res.Matches) < maxMatches {
				// Placeholders contain no byte the escaper changes. The mask is
				// the only trace of the value (issue #112).
				res.Matches = append(res.Matches, pipeline.Match{
					Offset: t.Start + shift + 1 + escLen, Length: len(ph), Type: f.Type, Rule: f.Rule, Placeholder: ph,
					Mask: Mask(f.Rule, f.Type, k.val),
				})
			}
			spans = append(spans, [2]int{b.Len(), b.Len() + len(ph)})
			b.WriteString(ph)
			escLen += len(ph)
			pos = f.End
		}
		tail := t.Value[pos:]
		b.WriteString(tail)
		escBuf = jsonsplice.Escape(escBuf[:0], tail)
		escLen += len(escBuf)

		nv := b.String()
		res.Replacements = append(res.Replacements, jsonsplice.Replacement{Start: t.Start, End: t.End, New: nv})
		shift += 2 + escLen - (t.End - t.Start)
		if t.MessageIndex == firstMsg && t.Kind != jsonsplice.KindToolResult && fingerprintChanged(t.Value, nv) {
			diff.note = true
		}
		for _, sp := range spans {
			diff.add(t.Path, nv, sp[0], sp[1])
		}
	}
	res.Diff = diff.String()
	return res, nil
}

// firstUserMessage returns the message index of the first user target, or -1.
func firstUserMessage(targets []jsonsplice.Target) int {
	for _, t := range targets {
		if t.Role == "user" {
			return t.MessageIndex
		}
	}
	return -1
}

// fingerprintChanged reports whether the UTF-16 code units at index 4, 7 and
// 20 differ between before and after. Claude Code hashes exactly these into
// the cc_version fingerprint of its billing block (CLIProxyAPI
// computeFingerprint); missing positions count as '0'.
func fingerprintChanged(before, after string) bool {
	a, b := utf16.Encode([]rune(before)), utf16.Encode([]rune(after))
	at := func(u []uint16, i int) uint16 {
		if i < len(u) {
			return u[i]
		}
		return '0'
	}
	for _, i := range [...]int{4, 7, 20} {
		if at(a, i) != at(b, i) {
			return true
		}
	}
	return false
}

// diffBuilder renders one line per placeholder: the target path and up to
// diffContext bytes of redacted context on each side, JSON-escaped so that
// a line never breaks. The total is capped at maxDiffBytes.
type diffBuilder struct {
	note    bool
	lines   strings.Builder
	omitted int
	buf     []byte
}

func (d *diffBuilder) add(path, val string, start, end int) {
	from, to := start-diffContext, end+diffContext
	pre, post := "…", "…"
	if from <= 0 {
		from, pre = 0, ""
	}
	if to >= len(val) {
		to, post = len(val), ""
	}
	for from > 0 && !utf8.RuneStart(val[from]) {
		from++
	}
	for to < len(val) && !utf8.RuneStart(val[to]) {
		to--
	}
	d.buf = append(d.buf[:0], path...)
	d.buf = append(d.buf, ": "...)
	d.buf = append(d.buf, pre...)
	d.buf = jsonsplice.Escape(d.buf, val[from:to])
	d.buf = append(d.buf, post...)
	d.buf = append(d.buf, '\n')
	if d.omitted > 0 || d.lines.Len()+len(d.buf) > maxDiffBytes {
		d.omitted++
		return
	}
	d.lines.Write(d.buf)
}

func (d *diffBuilder) String() string {
	var b strings.Builder
	if d.note {
		b.WriteString(FingerprintNote)
		b.WriteByte('\n')
	}
	b.WriteString(d.lines.String())
	if d.omitted > 0 {
		b.WriteString("… " + strconv.Itoa(d.omitted) + " more placeholders\n")
	}
	return b.String()
}
