package pipeline

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/niclasedge/tollgate/internal/store"
)

// SessionStore is the part of the store the SessionPinner needs.
type SessionStore interface {
	ActiveConfigVersionID(ctx context.Context) (int64, error)
	PinSession(ctx context.Context, in store.Session) (store.Session, error)
}

// SessionPinner implements ConfigPinner over the store (ADR-0006): the first
// request of a session pins the active configuration version, later requests
// of the session keep it. Without an active version it returns 0, the
// built-in configuration, and writes nothing.
type SessionPinner struct {
	Store SessionStore
}

// Pin implements ConfigPinner.
func (p *SessionPinner) Pin(ctx context.Context, clientID, sessionKey string) (int64, error) {
	active, err := p.Store.ActiveConfigVersionID(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if sessionKey == "" {
		return active, nil // client without session: always the active version
	}
	key, err := hex.DecodeString(sessionKey)
	if err != nil {
		return 0, errors.New("pipeline: session key is not hex")
	}
	s, err := p.Store.PinSession(ctx, store.Session{KeyHash: key, ClientID: clientID, ConfigVersionID: active})
	if err != nil {
		return 0, err
	}
	return s.ConfigVersionID, nil
}
