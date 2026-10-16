package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ClientStatus is the state of a client key (ADR-0009).
type ClientStatus string

const (
	ClientActive  ClientStatus = "aktiv"
	ClientBlocked ClientStatus = "gesperrt"
)

// Client is a gateway client. The key itself is never stored, only its
// SHA-256 hash and a short display hint (prefix and last four characters).
type Client struct {
	ID        string
	Name      string
	KeyHash   []byte
	KeyHint   string
	Status    ClientStatus
	CreatedAt time.Time
	RotatedAt *time.Time
}

func validateKeyFields(hash []byte, hint string) error {
	if err := checkHash("key hash", hash); err != nil {
		return err
	}
	if len(hint) == 0 || len([]rune(hint)) > maxKeyHint {
		return invalid("key hint")
	}
	return nil
}

// CreateClient stores a new client; CreatedAt is set from the store clock.
func (s *Store) CreateClient(ctx context.Context, c Client) (Client, error) {
	if err := checkIdent("client id", c.ID); err != nil {
		return Client{}, err
	}
	if c.Name == "" || len(c.Name) > 128 || strings.HasPrefix(c.Name, reservedNamePrefix) {
		return Client{}, invalid("client name")
	}
	if c.ID == SpeedtestClientID {
		return Client{}, invalid("client id")
	}
	if err := validateKeyFields(c.KeyHash, c.KeyHint); err != nil {
		return Client{}, err
	}
	if c.Status != ClientActive && c.Status != ClientBlocked {
		return Client{}, invalid("client status")
	}
	c.CreatedAt = s.nowMS()
	c.RotatedAt = nil
	_, err := s.w.ExecContext(ctx,
		`INSERT INTO clients (id, name, key_hash, key_hint, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.KeyHash, c.KeyHint, string(c.Status), ms(c.CreatedAt))
	if err != nil {
		return Client{}, mapErr("create client", err)
	}
	return c, nil
}

const clientCols = `id, name, key_hash, key_hint, status, created_at, rotated_at`

func scanClient(sc interface{ Scan(...any) error }) (Client, error) {
	var c Client
	var status string
	var created int64
	var rotated sql.NullInt64
	if err := sc.Scan(&c.ID, &c.Name, &c.KeyHash, &c.KeyHint, &status, &created, &rotated); err != nil {
		return Client{}, err
	}
	c.Status = ClientStatus(status)
	c.CreatedAt = fromMS(created)
	if rotated.Valid {
		t := fromMS(rotated.Int64)
		c.RotatedAt = &t
	}
	return c, nil
}

// ClientByKeyHash looks a client up by the SHA-256 of its key.
func (s *Store) ClientByKeyHash(ctx context.Context, keyHash []byte) (Client, error) {
	if err := checkHash("key hash", keyHash); err != nil {
		return Client{}, err
	}
	c, err := scanClient(s.r.QueryRowContext(ctx, `SELECT `+clientCols+` FROM clients WHERE key_hash = ?`, keyHash))
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	return c, err
}

// ListClients returns all clients ordered by creation time, then ID.
func (s *Store) ListClients(ctx context.Context) ([]Client, error) {
	rows, err := s.r.QueryContext(ctx, `SELECT `+clientCols+` FROM clients ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetClientStatus blocks or re-activates a client.
func (s *Store) SetClientStatus(ctx context.Context, id string, status ClientStatus) error {
	if status != ClientActive && status != ClientBlocked {
		return invalid("client status")
	}
	if id == SpeedtestClientID {
		return ErrNotFound // reserved, not managed through the client API
	}
	res, err := s.w.ExecContext(ctx, `UPDATE clients SET status = ? WHERE id = ?`, string(status), id)
	return affected(res, err, "set client status")
}

// RotateClientKey replaces the key hash and hint and records the rotation
// time. The old key is invalid immediately (ADR-0009, default 0 grace).
func (s *Store) RotateClientKey(ctx context.Context, id string, newHash []byte, newHint string) error {
	if err := validateKeyFields(newHash, newHint); err != nil {
		return err
	}
	if id == SpeedtestClientID {
		return ErrNotFound // reserved, has no key
	}
	res, err := s.w.ExecContext(ctx, `UPDATE clients SET key_hash = ?, key_hint = ?, rotated_at = ? WHERE id = ?`,
		newHash, newHint, ms(s.now()), id)
	return affected(res, err, "rotate client key")
}

func affected(res sql.Result, err error, op string) error {
	if err != nil {
		return mapErr(op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
