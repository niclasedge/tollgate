package redactcfg

import (
	"net/http"

	"github.com/niclasedge/tollgate/internal/compress"
)

// handleCompressRules lists the compress rules of the active version (#21).
func (m *Manager) handleCompressRules(w http.ResponseWriter, _ *http.Request) {
	c := m.Active()
	writeJSON(w, http.StatusOK, c.doc.CompressConfig().RuleList(c.Version, m.compress))
}

// handlePlayground shows each step of the compress stage for one command and
// its output. The text is neither stored nor logged.
func (m *Manager) handlePlayground(w http.ResponseWriter, r *http.Request) {
	var req compress.PlaygroundRequest
	if !decode(w, r, &req) {
		return
	}
	c := m.Active()
	if req.Version != 0 {
		var err error
		if c, err = m.Get(r.Context(), req.Version); err != nil {
			m.fail(w, "compress playground", err)
			return
		}
	}
	e := c.doc.CompressConfig().Explain(req.Command, req.Output)
	e.Version = c.Version
	writeJSON(w, http.StatusOK, e)
}
