// Package web embeds the built frontend (web/dist) into the tollgate binary.
//
// dist/ only contains a .gitkeep in a fresh checkout, so `go build`, `go vet`
// and `go test` work without a frontend build. In that case the handler serves
// a small embedded placeholder page; `make web` fills dist/ with the real app.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

//go:embed placeholder.html
var placeholder []byte

// Handler serves the embedded frontend at "/". Only GET and HEAD are allowed;
// unknown paths are 404 (the app uses hash routing, so there is no SPA fallback).
func Handler() http.Handler {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // dist is a compile-time embedded directory
	}
	return newHandler(dist, placeholder)
}

func newHandler(dist fs.FS, fallbackIndex []byte) http.Handler {
	files := http.FileServerFS(dist)
	_, err := fs.Stat(dist, "index.html")
	built := err == nil

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/index.html" && !isServableFile(dist, r.URL.Path) {
			// No directory listings or redirects, no dotfiles (e.g. dist/.gitkeep).
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache")
			if !built {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(fallbackIndex)
				return
			}
			r.URL.Path = "/"
		} else if strings.HasPrefix(r.URL.Path, "/assets/") {
			// Vite names assets by content hash; do not cache 404s.
			if _, err := fs.Stat(dist, strings.TrimPrefix(path.Clean(r.URL.Path), "/")); err == nil {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}
		files.ServeHTTP(w, r)
	})
}

// isServableFile reports whether p names a regular file in dist whose path
// has no segment starting with a dot.
func isServableFile(dist fs.FS, p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ".") {
			return false
		}
	}
	fi, err := fs.Stat(dist, strings.TrimPrefix(path.Clean(p), "/"))
	return err == nil && fi.Mode().IsRegular()
}
