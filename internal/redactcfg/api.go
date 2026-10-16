package redactcfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/niclasedge/tollgate/config/schema"
	"github.com/niclasedge/tollgate/internal/store"
)

// maxRequestBody bounds YAML and test texts.
const maxRequestBody = 1 << 20

// ExportWarningHeader is set on exports that contain the word list.
const ExportWarningHeader = "X-Tollgate-Warning"

// Register adds the /api/config routes to mux. The caller must put
// auth.RequireAdmin in front of mux; the routes check no token themselves.
func (m *Manager) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/config/schema", m.handleSchema)
	mux.HandleFunc("POST /api/config/dry-run", m.handleDryRun)
	mux.HandleFunc("POST /api/config/import", m.handleImport)
	mux.HandleFunc("GET /api/config/versions", m.handleVersions)
	mux.HandleFunc("GET /api/config/versions/{id}", m.handleVersion)
	mux.HandleFunc("POST /api/config/versions/{id}/activate", m.handleActivate)
	mux.HandleFunc("POST /api/config/versions/{id}/rollback", m.handleRollback)
	mux.HandleFunc("GET /api/config/export", m.handleExport)
	mux.HandleFunc("GET /api/config/lists", m.handleLists)
	mux.HandleFunc("POST /api/config/test", m.handleTest)
	mux.HandleFunc("GET /api/compress/rules", m.handleCompressRules)
	mux.HandleFunc("POST /api/compress/playground", m.handlePlayground)
}

// ErrorResponse is the body of every error. Issues is set for validation
// errors (status 422).
type ErrorResponse struct {
	Error  string  `json:"error"`
	Issues []Issue `json:"issues,omitempty"`
}

// WordOps edits the word lists of a draft without sending the stored ones.
// WordsRemove and WordsAdd edit the general list (the one of WordsFrom, else
// the draft list): the base without the entries at WordsRemove, then WordsAdd
// in input order, empty entries and duplicates (per case_sensitive) dropped.
// ListsSet replaces single lists of customers, first names and last names
// completely (#143).
type WordOps struct {
	WordsRemove []int     `json:"words_remove,omitempty"`
	WordsAdd    []string  `json:"words_add,omitempty"`
	ListsSet    *ListsSet `json:"lists_set,omitempty"`
}

// ListsSet is the operation lists_set: every list that is present replaces
// the list of the draft (or of words_from) completely, an empty array clears
// it; a missing list stays as it is.
type ListsSet struct {
	Customers  *[]string `json:"customers,omitempty"`
	FirstNames *[]string `json:"first_names,omitempty"`
	LastNames  *[]string `json:"last_names,omitempty"`
}

// NameLists are the lists of customers (placeholder kunde), first names
// (vorname) and last names (nachname) in plain text: the body of
// GET /api/config/lists.
type NameLists struct {
	Customers  []string `json:"customers"`
	FirstNames []string `json:"first_names"`
	LastNames  []string `json:"last_names"`
}

// DryRunRequest is the body of POST /api/config/dry-run.
type DryRunRequest struct {
	YAML string `json:"yaml"`
	// WordsFrom takes the word list of this stored version (0 = off); the
	// draft list must then be empty.
	WordsFrom int64 `json:"words_from"`
	WordOps   `tstype:",extends"`
}

// ImportBody is the body of POST /api/config/import.
type ImportBody struct {
	YAML      string `json:"yaml"`
	Comment   string `json:"comment"`
	Activate  bool   `json:"activate"`
	WordsFrom int64  `json:"words_from"`
	WordOps   `tstype:",extends"`
}

// TestBody is the body of POST /api/config/test. Version 0 is the active
// version; a non-empty YAML tests a draft instead.
type TestBody struct {
	Text      string `json:"text"`
	Version   int64  `json:"version"`
	YAML      string `json:"yaml"`
	WordsFrom int64  `json:"words_from"`
	WordOps   `tstype:",extends"`
}

// VersionList is the body of GET /api/config/versions.
type VersionList struct {
	Active   int64         `json:"active"`
	Versions []VersionInfo `json:"versions"`
}

func (m *Manager) handleSchema(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/schema+json")
	_, _ = w.Write(schema.ConfigV1)
}

func (m *Manager) handleDryRun(w http.ResponseWriter, r *http.Request) {
	var req DryRunRequest
	if !decode(w, r, &req) {
		return
	}
	res, err := m.DryRunDraft(r.Context(), Draft{YAML: []byte(req.YAML), WordsFrom: req.WordsFrom, WordOps: req.WordOps})
	if err != nil {
		m.fail(w, "dry run", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (m *Manager) handleImport(w http.ResponseWriter, r *http.Request) {
	var req ImportBody
	if !decode(w, r, &req) {
		return
	}
	v, err := m.Import(r.Context(), ImportRequest{YAML: []byte(req.YAML), Author: AuthorImport, Comment: req.Comment, Activate: req.Activate, WordsFrom: req.WordsFrom, WordOps: req.WordOps})
	if err != nil {
		m.fail(w, "import", err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (m *Manager) handleVersions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	vs, err := m.Versions(r.Context(), limit)
	if err != nil {
		m.fail(w, "list versions", err)
		return
	}
	writeJSON(w, http.StatusOK, VersionList{Active: m.Active().Version, Versions: vs})
}

func (m *Manager) handleVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, err := m.Version(r.Context(), id)
	if err != nil {
		m.fail(w, "read version", err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (m *Manager) handleActivate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := m.Activate(r.Context(), id); err != nil {
		m.fail(w, "activate", err)
		return
	}
	v, err := m.Version(r.Context(), id)
	if err != nil {
		m.fail(w, "read version", err)
		return
	}
	writeJSON(w, http.StatusOK, v.VersionInfo)
}

func (m *Manager) handleRollback(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, err := m.Rollback(r.Context(), id, AuthorAdmin)
	if err != nil {
		m.fail(w, "rollback", err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (m *Manager) handleExport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var id int64
	if s := q.Get("version"); s != "" {
		var err error
		if id, err = strconv.ParseInt(s, 10, 64); err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "version muss eine positive Zahl sein"})
			return
		}
	}
	mode := ExportOmitWords
	switch q.Get("words") {
	case "", "omit":
	case "plain":
		mode = ExportPlainWords
	default:
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "words muss omit oder plain sein"})
		return
	}
	if id == 0 {
		id = m.Active().Version
	}
	out, err := m.Export(r.Context(), id, mode)
	if err != nil {
		m.fail(w, "export", err)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="tollgate-config-v%d.yaml"`, id))
	if mode == ExportPlainWords {
		w.Header().Set(ExportWarningHeader, "contains the confidential word list - do not commit")
	}
	_, _ = w.Write(out)
}

// handleLists returns the plain text of the three name lists of one version
// (default: the active one). It is the one read path of the lists besides
// the export with words=plain (ADR-0021); the filter view calls it only on
// the click "Bearbeiten".
func (m *Manager) handleLists(w http.ResponseWriter, r *http.Request) {
	var id int64
	if s := r.URL.Query().Get("version"); s != "" {
		var err error
		if id, err = strconv.ParseInt(s, 10, 64); err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "version muss eine positive Zahl sein"})
			return
		}
	}
	ls, err := m.Lists(r.Context(), id)
	if err != nil {
		m.fail(w, "read name lists", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, ls)
}

func (m *Manager) handleTest(w http.ResponseWriter, r *http.Request) {
	var req TestBody
	if !decode(w, r, &req) {
		return
	}
	res, err := m.Test(r.Context(), TestRequest{Text: req.Text, Version: req.Version, YAML: []byte(req.YAML), WordsFrom: req.WordsFrom, WordOps: req.WordOps})
	if err != nil {
		m.fail(w, "test", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// fail maps errors to responses. Logs carry the operation and the error,
// never YAML, words or test texts (none of the mapped errors contain them).
func (m *Manager) fail(w http.ResponseWriter, op string, err error) {
	var ve *ValidationError
	var oe *WordOpError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: "Konfiguration ungültig", Issues: ve.Issues})
	case errors.As(err, &oe):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: oe.Error()})
	case errors.Is(err, ErrNoSealer):
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: ErrNoSealer.Error()})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Version nicht gefunden"})
	case errors.Is(err, store.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ungültige Angabe (z. B. Kommentar zu lang)"})
	default:
		m.log.Error("redactcfg: "+op+" failed", "error", err.Error())
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "interner Fehler"})
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil && dec.Decode(&struct{}{}) != io.EOF {
		err = errors.New("trailing data")
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Anfrage zu groß"})
			return false
		}
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ungültiger Anfrage-Body"})
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ungültige Versions-ID"})
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
