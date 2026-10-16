package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/niclasedge/tollgate/internal/store"
)

const maxBody = 4 << 10

// NewAdminAPI returns the client-key admin API (/api/clients...), protected
// by the admin token. logger may be nil. Keys and tokens are never logged.
func NewAdminAPI(st ClientStore, adminToken string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	RegisterClientRoutes(mux, st, logger)
	return RequireAdmin(adminToken, mux)
}

// RegisterClientRoutes adds the /api/clients routes to mux. The caller must
// put RequireAdmin in front of mux; the routes check no token themselves.
func RegisterClientRoutes(mux *http.ServeMux, st ClientStore, logger *slog.Logger) {
	a := &adminAPI{st: st, log: logger}
	mux.HandleFunc("POST /api/clients", a.create)
	mux.HandleFunc("GET /api/clients", a.list)
	mux.HandleFunc("POST /api/clients/{id}/block", a.block)
	mux.HandleFunc("POST /api/clients/{id}/rotate", a.rotate)
}

type adminAPI struct {
	st  ClientStore
	log *slog.Logger
}

func (a *adminAPI) create(w http.ResponseWriter, r *http.Request) {
	var req CreateClientRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	err := dec.Decode(&req)
	if err == nil && dec.Decode(&struct{}{}) != io.EOF {
		err = errors.New("trailing data after JSON value")
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 128 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "name must be 1 to 128 characters"})
		return
	}
	id, err := randomID()
	if err != nil {
		a.fail(w, r, "create client", err)
		return
	}
	key, err := GenerateKey()
	if err != nil {
		a.fail(w, r, "create client", err)
		return
	}
	c, err := a.st.CreateClient(r.Context(), store.Client{
		ID: id, Name: name, KeyHash: HashKey(key), KeyHint: KeyHint(key), Status: store.ClientActive,
	})
	if err != nil {
		a.fail(w, r, "create client", err)
		return
	}
	a.info("client created", c.ID)
	writeJSON(w, http.StatusCreated, CreateClientResponse{Client: toInfo(c), Key: key})
}

func (a *adminAPI) list(w http.ResponseWriter, r *http.Request) {
	cs, err := a.st.ListClients(r.Context())
	if err != nil {
		a.fail(w, r, "list clients", err)
		return
	}
	out := ListClientsResponse{Clients: make([]ClientInfo, 0, len(cs))}
	for _, c := range cs {
		if c.ID == store.SpeedtestClientID {
			continue // reserved internal client (#87), not managed here
		}
		out.Clients = append(out.Clients, toInfo(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *adminAPI) block(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.st.SetClientStatus(r.Context(), id, store.ClientBlocked); err != nil {
		a.fail(w, r, "block client", err)
		return
	}
	c, err := a.byID(r.Context(), id)
	if err != nil {
		a.fail(w, r, "block client", err)
		return
	}
	a.info("client blocked", id)
	writeJSON(w, http.StatusOK, ClientResponse{Client: toInfo(c)})
}

func (a *adminAPI) rotate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	key, err := GenerateKey()
	if err != nil {
		a.fail(w, r, "rotate client key", err)
		return
	}
	if err := a.st.RotateClientKey(r.Context(), id, HashKey(key), KeyHint(key)); err != nil {
		a.fail(w, r, "rotate client key", err)
		return
	}
	// Read back by ID: a concurrent rotation may already have replaced hash.
	c, err := a.byID(r.Context(), id)
	if err != nil {
		a.fail(w, r, "rotate client key", err)
		return
	}
	a.info("client key rotated", id)
	writeJSON(w, http.StatusOK, CreateClientResponse{Client: toInfo(c), Key: key})
}

func (a *adminAPI) byID(ctx context.Context, id string) (store.Client, error) {
	cs, err := a.st.ListClients(ctx)
	if err != nil {
		return store.Client{}, err
	}
	for _, c := range cs {
		if c.ID == id {
			return c, nil
		}
	}
	return store.Client{}, store.ErrNotFound
}

func pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if id == "" || len(id) > 64 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid client id"})
		return "", false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.' || c == ':') {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid client id"})
			return "", false
		}
	}
	return id, true
}

func (a *adminAPI) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "client not found"})
	case errors.Is(err, store.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	default:
		if a.log != nil {
			a.log.ErrorContext(r.Context(), "admin api failure", "op", op, "err", err)
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
	}
}

func (a *adminAPI) info(msg, clientID string) {
	if a.log != nil {
		a.log.Info(msg, "client_id", clientID)
	}
}

func toInfo(c store.Client) ClientInfo {
	info := ClientInfo{
		ID: c.ID, Name: c.Name, KeyHint: c.KeyHint, Status: string(c.Status),
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
	}
	if c.RotatedAt != nil {
		s := c.RotatedAt.UTC().Format(time.RFC3339)
		info.RotatedAt = &s
	}
	return info
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
