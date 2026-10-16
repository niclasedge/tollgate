package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(t *testing.T, h http.Handler, path string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Result()
}

func body(t *testing.T, r *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHandlerServesShellWithoutFrontendBuild(t *testing.T) {
	// Holds in a fresh checkout (placeholder) and after `make web` (real shell).
	res := get(t, Handler(), "/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type = %q", ct)
	}
	if !strings.Contains(body(t, res), "tollgate") {
		t.Fatal("index does not mention tollgate")
	}
}

func TestNewHandlerPrefersBuiltDist(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":     {Data: []byte("<title>built</title>")},
		"assets/a.js":    {Data: []byte("console.log(1)")},
		"assets/a.css":   {Data: []byte("body{}")},
		".gitkeep":       {},
		"assets/.hidden": {Data: []byte("x")},
	}
	h := newHandler(dist, []byte("<title>placeholder</title>"))

	if b := body(t, get(t, h, "/")); !strings.Contains(b, "built") {
		t.Fatalf("index = %q", b)
	}
	js := get(t, h, "/assets/a.js")
	if js.StatusCode != http.StatusOK {
		t.Fatalf("asset status = %d", js.StatusCode)
	}
	if cc := js.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("asset Cache-Control = %q", cc)
	}
	if cc := get(t, h, "/").Header.Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("index Cache-Control = %q", cc)
	}
}

func TestNewHandlerFallsBackToPlaceholder(t *testing.T) {
	h := newHandler(fstest.MapFS{".gitkeep": {}}, []byte("<title>placeholder</title>"))
	res := get(t, h, "/")
	if res.StatusCode != http.StatusOK || !strings.Contains(body(t, res), "placeholder") {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestUnknownPathsAre404AndOnlyGETIsAllowed(t *testing.T) {
	h := newHandler(fstest.MapFS{"index.html": {Data: []byte("x")}}, nil)
	if s := get(t, h, "/nope.js").StatusCode; s != http.StatusNotFound {
		t.Fatalf("unknown path status = %d", s)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", rec.Code)
	}
}

func TestDirectoriesAndDotfilesAre404(t *testing.T) {
	h := newHandler(fstest.MapFS{
		"index.html":      {Data: []byte("x")},
		".gitkeep":        {},
		"assets/app-1.js": {Data: []byte("js")},
		"assets/.hidden":  {Data: []byte("h")},
	}, nil)
	for _, p := range []string{"/assets/", "/assets", "/.gitkeep", "/assets/.hidden", "/assets/../.gitkeep"} {
		if s := get(t, h, p).StatusCode; s != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", p, s)
		}
	}
	if s := get(t, h, "/assets/app-1.js").StatusCode; s != http.StatusOK {
		t.Fatalf("asset status = %d", s)
	}
}
