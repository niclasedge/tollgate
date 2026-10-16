package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const (
	keyPrefix  = "tg_"
	keyEntropy = 32
	// keyLen is len("tg_") plus 32 bytes as unpadded base64url (43 chars).
	keyLen = len(keyPrefix) + 43
)

// GenerateKey returns a new client key: "tg_" plus 32 bytes from crypto/rand,
// base64url without padding (46 characters, ADR-0009).
func GenerateKey() (string, error) {
	b := make([]byte, keyEntropy)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate key: %w", err)
	}
	return keyPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// HashKey returns SHA-256(key), the only form in which a key is stored.
func HashKey(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}

// KeyHint is the display hint stored next to the hash: prefix and the last
// four characters, e.g. "tg_…wxyz".
func KeyHint(key string) string {
	if len(key) < keyLen {
		return keyPrefix + "…"
	}
	return keyPrefix + "…" + key[len(key)-4:]
}

// wellFormed checks the key shape cheaply before any lookup.
func wellFormed(key string) bool {
	if len(key) != keyLen || key[:len(keyPrefix)] != keyPrefix {
		return false
	}
	for i := len(keyPrefix); i < len(key); i++ {
		c := key[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate id: %w", err)
	}
	return "c_" + hex.EncodeToString(b), nil
}
