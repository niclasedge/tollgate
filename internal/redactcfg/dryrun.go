package redactcfg

import (
	"context"
	"encoding/json"
	"io/fs"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
	"github.com/niclasedge/tollgate/internal/redact"
)

// FixtureResult compares the findings of the active and the candidate
// configuration on one fixture. Hits carry no values.
type FixtureResult struct {
	Name      string `json:"name"`
	Targets   int    `json:"targets"`
	Active    []Hit  `json:"active"`
	Candidate []Hit  `json:"candidate"`
	Error     string `json:"error,omitempty"`
}

// DryRunResult is the preview of an import (ADR-0008 step 3).
type DryRunResult struct {
	ActiveVersion int64           `json:"active_version"`
	Fixtures      []FixtureResult `json:"fixtures"`
	// NeedsSealer reports that the candidate has a word list that cannot be
	// stored yet because no encryption key is connected.
	NeedsSealer bool `json:"needs_sealer"`
}

// DryRun validates the YAML and runs every fixture through the active and
// the candidate configuration. Nothing is stored. Validation errors are
// *ValidationError.
func (m *Manager) DryRun(ctx context.Context, yamlDoc []byte) (*DryRunResult, error) {
	return m.DryRunDraft(ctx, Draft{YAML: yamlDoc})
}

// DryRunDraft is DryRun for a draft whose word list may come from a stored
// version (see Draft).
func (m *Manager) DryRunDraft(ctx context.Context, dr Draft) (*DryRunResult, error) {
	d, cand, err := m.parseDraft(ctx, dr)
	if err != nil {
		return nil, err
	}
	active := m.Active()
	res := &DryRunResult{
		ActiveVersion: active.Version,
		Fixtures:      []FixtureResult{},
		NeedsSealer:   d.Words.any() && m.sealer == nil,
	}
	if m.fixtures == nil {
		return res, nil
	}
	names, err := fs.Glob(m.fixtures, "*.json") // sorted
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		res.Fixtures = append(res.Fixtures, runFixture(m.fixtures, name, active.Redactor, cand))
	}
	return res, nil
}

func runFixture(fsys fs.FS, name string, active, cand *redact.Redactor) FixtureResult {
	fr := FixtureResult{Name: name, Active: []Hit{}, Candidate: []Hit{}}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		fr.Error = "Fixture nicht lesbar"
		return fr
	}
	var f struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &f); err != nil || len(f.Body) == 0 {
		fr.Error = "Fixture hat kein Feld body"
		return fr
	}
	doc, err := jsonsplice.Scan(f.Body)
	if err != nil {
		fr.Error = "Fixture-Body ist keine gültige Anfrage"
		return fr
	}
	fr.Targets = len(doc.Targets)
	for _, t := range doc.Targets {
		fr.Active = append(fr.Active, hits(t.Path, active.Find(t.Value))...)
		fr.Candidate = append(fr.Candidate, hits(t.Path, cand.Find(t.Value))...)
	}
	return fr
}
