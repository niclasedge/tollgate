package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/niclasedge/tollgate/internal/pipeline"
	"github.com/niclasedge/tollgate/internal/store"
)

// ClientStore is the part of *store.Store the auth package needs.
type ClientStore interface {
	CreateClient(ctx context.Context, c store.Client) (store.Client, error)
	ClientByKeyHash(ctx context.Context, keyHash []byte) (store.Client, error)
	ListClients(ctx context.Context) ([]store.Client, error)
	SetClientStatus(ctx context.Context, id string, status store.ClientStatus) error
	RotateClientKey(ctx context.Context, id string, newHash []byte, newHint string) error
}

// Verifier implements pipeline.KeyVerifier on top of the client table.
// It deliberately has no cache: every request reads the current row, so
// blocking and rotating take effect for the very next request (#8). A
// SHA-256 index lookup is cheap for the expected client count.
type Verifier struct{ store ClientStore }

var _ pipeline.KeyVerifier = (*Verifier)(nil)

// NewVerifier returns a KeyVerifier backed by st.
func NewVerifier(st ClientStore) *Verifier { return &Verifier{store: st} }

// Verify returns the client ID for an active client's key. Unknown,
// malformed, blocked and rotated-away keys yield pipeline.ErrInvalidKey;
// store failures are returned as other errors (internal failure).
func (v *Verifier) Verify(ctx context.Context, key string) (string, error) {
	if !wellFormed(key) {
		return "", pipeline.ErrInvalidKey
	}
	hash := HashKey(key)
	c, err := v.store.ClientByKeyHash(ctx, hash)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return "", pipeline.ErrInvalidKey
	case err != nil:
		return "", fmt.Errorf("auth: look up client: %w", err)
	}
	// Defense in depth: the lookup already matched the hash; compare in
	// constant time so the decision never depends on a byte-wise early exit.
	if subtle.ConstantTimeCompare(c.KeyHash, hash) != 1 || c.Status != store.ClientActive {
		return "", pipeline.ErrInvalidKey
	}
	return c.ID, nil
}
