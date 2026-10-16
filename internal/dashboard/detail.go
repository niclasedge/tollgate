package dashboard

import (
	"context"
	"errors"
	"net/http"

	"github.com/niclasedge/tollgate/internal/store"
)

const (
	defaultRedactions = 10
	maxRedactions     = 50
)

// DetailStore is the part of the store behind the trace detail and the
// recent redactions.
type DetailStore interface {
	GetTrace(ctx context.Context, id string) (store.Trace, error)
	RecentRedactions(ctx context.Context, limit int) ([]store.Redaction, error)
}

// traceDetail serves GET /api/traces/{id}.
func (h *Handler) traceDetail(w http.ResponseWriter, r *http.Request) {
	t, err := h.st.GetTrace(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "trace not found"})
		return
	}
	if err != nil {
		h.fail(w, r, "get trace", err)
		return
	}
	names, err := h.clientNames(r.Context())
	if err != nil {
		h.fail(w, r, "list clients", err)
		return
	}
	out := TraceDetail{Trace: summary(t, names[t.ClientID], savedTokens(t.Stages)), Stages: make([]StageDetail, 0, len(t.Stages))}
	for _, s := range t.Stages {
		sd := StageDetail{
			Name: s.Name, DurationUs: s.DurationMicros, BytesIn: s.BytesIn, BytesOut: s.BytesOut,
			TokensIn: s.TokensIn, TokensOut: s.TokensOut, HashIn: s.HashIn, HashOut: s.HashOut,
			ConfigVersion: s.ConfigVersionID, Diff: s.Diff, Matches: make([]Match, 0, len(s.Matches)),
		}
		for _, m := range s.Matches {
			sd.Matches = append(sd.Matches, Match{Offset: m.Offset, Length: m.Length, Type: m.Type, Rule: m.Rule, Placeholder: m.Placeholder, Mask: m.Mask})
		}
		out.Stages = append(out.Stages, sd)
	}
	writeJSON(w, http.StatusOK, out)
}

// redactions serves GET /api/redactions?limit=N.
func (h *Handler) redactions(w http.ResponseWriter, r *http.Request) {
	limit, err := intParam(r, "limit", defaultRedactions, 1, maxRedactions)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	rs, err := h.st.RecentRedactions(r.Context(), limit)
	if err != nil {
		h.fail(w, r, "recent redactions", err)
		return
	}
	out := RedactionList{Redactions: make([]Redaction, 0, len(rs))}
	for _, x := range rs {
		out.Redactions = append(out.Redactions, Redaction{
			At: formatTime(x.StartedAt), TraceID: x.TraceID, ClientID: x.ClientID, ClientName: x.ClientName,
			Stage: x.Stage, Type: x.Type, Rule: x.Rule, Placeholder: x.Placeholder, Mask: x.Mask,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) clientNames(ctx context.Context) (map[string]string, error) {
	cs, err := h.st.ListClients(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(cs))
	for _, c := range cs {
		m[c.ID] = c.Name
	}
	return m, nil
}
