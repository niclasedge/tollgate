package cryptobox

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestAuthenticatedRoundTrip(t *testing.T) {
	b, err := Open(filepath.Join(t.TempDir(), "crypto.key"), true)
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("test-private-refresh-token")
	a, err := b.Seal("account-a", secret)
	if err != nil {
		t.Fatal(err)
	}
	c, err := b.Seal("account-a", secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, c) || bytes.Contains(a, secret) {
		t.Fatal("nonce reuse or plaintext")
	}
	got, err := b.Unseal("account-a", a)
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatal("round trip", err)
	}
	if _, err = b.Unseal("account-b", a); err == nil {
		t.Fatal("AAD swap accepted")
	}
	a[len(a)-1] ^= 1
	if _, err = b.Unseal("account-a", a); err == nil {
		t.Fatal("tampering accepted")
	}
}
func TestMissingKeyWithVaultFails(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing.key"), false); err == nil {
		t.Fatal("replaced missing key")
	}
}
