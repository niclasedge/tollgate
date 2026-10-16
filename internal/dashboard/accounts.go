package dashboard

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/niclasedge/tollgate/internal/accounts"
)

func (h *Handler) accountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/accounts", h.listAccounts)
	mux.HandleFunc("GET /api/accounts/switch-status", h.accountSwitchStatus)
	mux.HandleFunc("POST /api/accounts/switch/force", h.forceAccountSwitch)
	mux.HandleFunc("GET /api/accounts/{id}/usage", h.accountUsage)
	mux.HandleFunc("POST /api/accounts/login", h.startAccountLogin)
	mux.HandleFunc("POST /api/accounts/login/complete", h.completeAccountLogin)
	mux.HandleFunc("POST /api/accounts/import", h.importAccount)
	mux.HandleFunc("POST /api/accounts/{id}/activate", h.activateAccount)
	mux.HandleFunc("POST /api/accounts/{id}/test", h.testAccount)
	mux.HandleFunc("DELETE /api/accounts/{id}", h.removeAccount)
}
func (h *Handler) accountsAvailable(w http.ResponseWriter) bool {
	if h.accounts == nil {
		writeJSON(w, 503, errorBody{Error: "Account-Verwaltung nicht verfügbar: Management-Key fehlt"})
		return false
	}
	return true
}
func accountBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, (1<<20)+8192)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		badRequest(w, "Ungültige oder zu große Account-Daten")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		badRequest(w, "Ungültige Account-Daten")
		return false
	}
	return true
}
func accountError(w http.ResponseWriter, err error) { writeJSON(w, 409, errorBody{Error: err.Error()}) }
func (h *Handler) listAccounts(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	list, err := h.accounts.List(r.Context())
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 200, accounts.ListResponse{Accounts: list})
}
func (h *Handler) startAccountLogin(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	l, err := h.oauth.Start()
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 200, l)
}
func (h *Handler) completeAccountLogin(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	var b struct {
		ID          string `json:"id"`
		CallbackURL string `json:"callback_url"`
		Name        string `json:"name"`
	}
	if !accountBody(w, r, &b) {
		return
	}
	token, err := h.oauth.Complete(r.Context(), b.ID, b.CallbackURL)
	if err != nil {
		accountError(w, err)
		return
	}
	a, err := h.accounts.Import(r.Context(), b.Name, token)
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 201, a)
}
func (h *Handler) importAccount(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	var b struct {
		Name  string          `json:"name"`
		Token json.RawMessage `json:"token"`
	}
	if !accountBody(w, r, &b) {
		return
	}
	a, err := h.accounts.Import(r.Context(), b.Name, b.Token)
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 201, a)
}
func (h *Handler) activateAccount(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	a, err := h.accounts.Activate(r.Context(), r.PathValue("id"))
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 200, a)
}
func (h *Handler) testAccount(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	result, err := h.accounts.Test(r.Context(), r.PathValue("id"))
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (h *Handler) removeAccount(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	if err := h.accounts.Remove(r.Context(), r.PathValue("id")); err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 200, struct {
		OK bool `json:"ok"`
	}{true})
}

func (h *Handler) accountUsage(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	usage, err := h.accounts.Usage(r.Context(), r.PathValue("id"))
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 200, usage)
}

func (h *Handler) accountSwitchStatus(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	writeJSON(w, 200, h.accounts.SwitchStatus())
}

// forceAccountSwitch aborts the running requests of a reserved switch (#142).
func (h *Handler) forceAccountSwitch(w http.ResponseWriter, r *http.Request) {
	if !h.accountsAvailable(w) {
		return
	}
	status, err := h.accounts.ForceSwitch()
	if err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 200, status)
}
