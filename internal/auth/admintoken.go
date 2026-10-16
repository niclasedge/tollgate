package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const minAdminTokenLen = 32

// LoadOrCreateAdminToken reads the admin token from path. A missing file is
// created with a fresh random token (mode 0600). An existing file must be a
// regular file that only its owner can access. The token is never logged.
func LoadOrCreateAdminToken(path string) (string, error) {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return createAdminToken(path)
	}
	if err != nil {
		return "", fmt.Errorf("auth: stat admin token file: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("auth: admin token file %s is not a regular file", filepath.Base(path))
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("auth: admin token file %s is accessible by group or others (chmod 600)", filepath.Base(path))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("auth: read admin token file: %w", err)
	}
	tok := strings.TrimSpace(string(b))
	if len(tok) < minAdminTokenLen {
		return "", fmt.Errorf("auth: admin token in %s is shorter than %d characters", filepath.Base(path), minAdminTokenLen)
	}
	return tok, nil
}

func createAdminToken(path string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("auth: create admin token dir: %w", err)
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate admin token: %w", err)
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("auth: create admin token file: %w", err)
	}
	if _, err := f.WriteString(tok + "\n"); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("auth: write admin token file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("auth: write admin token file: %w", err)
	}
	return tok, nil
}

// RequireAdmin wraps next so that only requests with
// "Authorization: Bearer <token>" pass. The comparison runs in constant time
// over SHA-256 digests, so neither content nor length leaks. Client keys are
// not accepted here (ADR-0009: strictly separate paths).
func RequireAdmin(token string, next http.Handler) http.Handler {
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, rest, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		got := sha256.Sum256([]byte(strings.TrimSpace(rest)))
		match := subtle.ConstantTimeCompare(got[:], want[:]) == 1
		if !ok || !strings.EqualFold(scheme, "Bearer") || !match || token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tollgate-admin"`)
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "invalid or missing admin token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
