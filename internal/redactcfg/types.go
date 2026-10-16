package redactcfg

import "time"

// API types of the filter view; web/src/lib/types/redactcfg.ts is generated
// from this file and api.go, dryrun.go, parse.go, model.go (make generate).

// VersionInfo describes a stored version. Diff has the word list masked.
type VersionInfo struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Author    string    `json:"author"`
	Comment   string    `json:"comment"`
	Diff      string    `json:"diff"`
	Active    bool      `json:"active"`
	HasWords  bool      `json:"has_words"`
}

// VersionDetail adds the masked YAML of a version.
type VersionDetail struct {
	VersionInfo `tstype:",extends"`
	YAML        string `json:"yaml"`
}

// Hit is one finding, without its value. Offsets are bytes in the tested
// text (or the fixture target named by Path).
type Hit struct {
	Path  string `json:"path,omitempty"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Type  string `json:"type"`
	Rule  string `json:"rule"`
}

// TestResult lists the findings and the text with unnumbered placeholders.
type TestResult struct {
	Version  int64  `json:"version"`
	Hits     []Hit  `json:"hits"`
	Redacted string `json:"redacted"`
}
