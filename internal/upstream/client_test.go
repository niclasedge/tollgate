package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsNonLoopback(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"ipv4 loopback", "http://127.0.0.1:8317", false},
		{"localhost", "http://localhost:8317", false},
		{"ipv6 loopback", "http://[::1]:8317", false},
		{"trailing slash", "http://127.0.0.1:8317/", false},
		{"lan address", "http://10.0.0.5:8317", true},
		{"public host", "http://example.com:8317", true},
		{"tailnet-like address", "http://100.64.0.1:8317", true},
		{"https scheme", "https://127.0.0.1:8317", true},
		{"no scheme", "127.0.0.1:8317", true},
		{"empty", "", true},
		{"with path", "http://127.0.0.1:8317/v1", true},
		{"userinfo", "http://user:pw@127.0.0.1:8317", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "pw") {
				t.Fatalf("error leaks userinfo: %v", err)
			}
		})
	}
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{"ok", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}, false},
		{"unexpected status field", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status":"starting"}`))
		}, true},
		{"not json", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`<html>`))
		}, true},
		{"server error", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom secret-body", http.StatusInternalServerError)
		}, true},
		{"redirect is not followed", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://203.0.113.1/healthz", http.StatusFound)
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()
			c, err := New(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			err = c.Health(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Health() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "secret-body") {
				t.Fatalf("error leaks response body: %v", err)
			}
		})
	}
}

func TestHealthRequestShape(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL + "/")
	if err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotPath != "/healthz" {
		t.Fatalf("request = %s %s, want GET /healthz", gotMethod, gotPath)
	}
	if gotAuth != "" {
		t.Fatalf("health check must not send credentials, got %q", gotAuth)
	}
}

func TestHealthUnreachableAndCancel(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	c, _ := New(url)
	if err := c.Health(context.Background()); err == nil {
		t.Fatal("expected error for closed server")
	}

	block := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	defer slow.Close()
	defer close(block)
	c, _ = New(slow.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := c.Health(ctx); err == nil {
		t.Fatal("expected error on context deadline")
	}
}
