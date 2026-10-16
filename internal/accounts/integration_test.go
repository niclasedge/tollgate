package accounts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/niclasedge/tollgate/internal/upstream"
)

func TestHTTPManagementSwitchAndRollback(t *testing.T) {
	m, f, _ := managerEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "mgmt"
		if r.URL.Path == "/v1/messages" {
			key = "api"
		}
		if r.Header.Get("Authorization") != "Bearer "+key {
			w.WriteHeader(401)
			return
		}
		name := r.URL.Query().Get("name")
		switch r.URL.Path {
		case "/v8/management/credentials":
			switch r.Method {
			case "GET":
				c, _ := f.Credentials(r.Context(), "mgmt")
				json.NewEncoder(w).Encode(map[string]any{"files": c})
			case "DELETE":
				f.DeleteCredential(r.Context(), "mgmt", name)
				io.WriteString(w, `{"status":"ok"}`)
			case "POST":
				b, _ := io.ReadAll(r.Body)
				if f.UploadCredential(r.Context(), "mgmt", name, b) != nil {
					w.WriteHeader(500)
					io.WriteString(w, "private-refresh-token-error")
					return
				}
				io.WriteString(w, `{"status":"ok"}`)
			default:
				w.WriteHeader(405)
			}
		case "/v8/management/credentials/refresh-pause":
			io.WriteString(w, `{"status":"ok"}`)
		case "/v8/management/credentials/download":
			b, err := f.DownloadCredential(r.Context(), "mgmt", name)
			if err != nil {
				w.WriteHeader(404)
				return
			}
			w.Write(b)
		case "/v1/messages":
			f.tests++
			if len(f.files) != 1 {
				t.Error("test without exactly one active credential")
			}
			io.WriteString(w, `{"type":"message","model":"claude-sonnet-5-5","content":[{"type":"text","text":"OK"}]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	up, err := upstream.New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	m.up = up
	ctx := context.Background()
	list, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Import(ctx, "Second", tokenB)
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.Activate(ctx, b.ID)
	if err != nil || !result.Test.OK {
		t.Fatal(result, err)
	}
	f.failNext = true
	_, err = m.Activate(ctx, list[0].ID)
	if err == nil || strings.Contains(err.Error(), "private-refresh") {
		t.Fatal("error body escaped", err)
	}
	active, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range active {
		if a.ID == b.ID && !a.Active {
			t.Fatal("rollback did not restore B")
		}
	}
	if f.maxFiles > 1 {
		t.Fatal("two files existed")
	}
}
