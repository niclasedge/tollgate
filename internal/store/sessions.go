package store

import (
	"context"
	"database/sql"
	"time"
)

// Session pins a configuration version to a session key (ADR-0006). KeyHash is
// SHA-256(client_id || session_id); the store never sees the session ID.
type Session struct {
	KeyHash         []byte
	ClientID        string
	ConfigVersionID int64
	CreatedAt       time.Time
	LastSeenAt      time.Time
}

// PinSession records the first request of a session with the given version
// and returns the stored row. For an existing session the originally pinned
// version wins; only last_seen_at is refreshed.
func (s *Store) PinSession(ctx context.Context, in Session) (Session, error) {
	if err := checkHash("session key hash", in.KeyHash); err != nil {
		return Session{}, err
	}
	if err := checkIdent("client id", in.ClientID); err != nil {
		return Session{}, err
	}
	if in.ConfigVersionID <= 0 {
		return Session{}, invalid("config version id")
	}
	now := ms(s.now())
	var out Session
	var created, seen int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sessions (key_hash, client_id, config_version_id, created_at, last_seen_at)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT (key_hash) DO UPDATE SET last_seen_at = excluded.last_seen_at`,
			in.KeyHash, in.ClientID, in.ConfigVersionID, now, now); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx,
			`SELECT key_hash, client_id, config_version_id, created_at, last_seen_at FROM sessions WHERE key_hash = ?`,
			in.KeyHash).Scan(&out.KeyHash, &out.ClientID, &out.ConfigVersionID, &created, &seen)
	})
	if err != nil {
		return Session{}, mapErr("pin session", err)
	}
	out.CreatedAt, out.LastSeenAt = fromMS(created), fromMS(seen)
	return out, nil
}

// PurgeSessions deletes sessions whose last activity is before idleBefore and
// returns how many were removed (ADR-0006: 7 days of inactivity).
func (s *Store) PurgeSessions(ctx context.Context, idleBefore time.Time) (int64, error) {
	res, err := s.w.ExecContext(ctx, `DELETE FROM sessions WHERE last_seen_at < ?`, ms(idleBefore))
	if err != nil {
		return 0, mapErr("purge sessions", err)
	}
	return res.RowsAffected()
}
