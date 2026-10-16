// Package cryptobox encrypts confidential records with AES-256-GCM.
package cryptobox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"os"
)

type Box struct{ aead cipher.AEAD }

// Open creates a key only for an empty vault. Existing vaults fail closed if
// their key is missing. The caller must back up this file with the database.
func Open(path string, allowCreate bool) (*Box, error) {
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && allowCreate {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, errors.New("crypto: random key failed")
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(e, os.ErrExist) {
			key, err = os.ReadFile(path)
		} else if e != nil {
			return nil, errors.New("crypto: cannot create key file")
		} else {
			_, err = f.Write(key)
			if err == nil {
				err = f.Sync()
			}
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
	}
	if err != nil {
		return nil, errors.New("crypto: key file unavailable; restore it from backup")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || len(key) != 32 {
		return nil, errors.New("crypto: key must be a regular 0600 file containing 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("crypto: invalid key")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead}, nil
}
func (b *Box) Seal(id string, plain []byte) ([]byte, error) {
	n := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(n); err != nil {
		return nil, errors.New("crypto: random nonce failed")
	}
	return b.aead.Seal(n, n, plain, []byte(id)), nil
}
func (b *Box) Unseal(id string, data []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(data) < n {
		return nil, errors.New("crypto: invalid encrypted record")
	}
	out, err := b.aead.Open(nil, data[:n], data[n:], []byte(id))
	if err != nil {
		return nil, errors.New("crypto: cannot decrypt record")
	}
	return out, nil
}
