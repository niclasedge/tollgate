package compress

// API types of the compress view (#21); web/src/lib/types/compress.ts is
// generated from this file (make generate).

// RuleInfo is one rule as the compress view lists it. Zero limits mean the
// strategy default.
type RuleInfo struct {
	Name         string   `json:"name"`
	Match        []string `json:"match"`
	Strategy     string   `json:"strategy"`
	MaxErrors    int      `json:"max_errors"`
	ContextLines int      `json:"context_lines"`
	MaxFiles     int      `json:"max_files"`
	MaxHunks     int      `json:"max_hunks"`
	// Builtin is false for the rules of the configuration, which come first.
	Builtin bool `json:"builtin"`
}

// RuleList is the body of GET /api/compress/rules: the rules of the active
// configuration version in match order.
type RuleList struct {
	Version int64 `json:"version"`
	// Enabled tells whether the compress stage runs (TOLLGATE_COMPRESS_ENABLED).
	Enabled  bool       `json:"enabled"`
	MinBytes int        `json:"min_bytes"`
	Rules    []RuleInfo `json:"rules"`
}

// PlaygroundRequest is the body of POST /api/compress/playground. Version 0
// is the active version.
type PlaygroundRequest struct {
	Command string `json:"command"`
	Output  string `json:"output"`
	Version int64  `json:"version"`
}

// Step is the text after one step: "input", the steps of the strategy, then
// "result" with the marker line.
type Step struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Bytes int    `json:"bytes"`
	Lines int    `json:"lines"`
}

// Explanation shows what the compress stage does with one Bash output. Its
// Output and Applied match Config.Compress exactly; the steps also run when a
// gate keeps the output unchanged, so the strategy stays visible.
type Explanation struct {
	Version int64 `json:"version"`
	// Normalized is the command the rules match against; "" when it is not a
	// single command.
	Normalized string `json:"normalized"`
	Rule       string `json:"rule"`
	Strategy   string `json:"strategy"`
	Steps      []Step `json:"steps"`
	// Output is what goes upstream: the result when Applied, else the input.
	Output      string `json:"output"`
	BytesBefore int    `json:"bytes_before"`
	BytesAfter  int    `json:"bytes_after"`
	Omitted     int    `json:"omitted"`
	Applied     bool   `json:"applied"`
	// Reason says why the output stays unchanged; "" when Applied.
	Reason string `json:"reason"`
}
