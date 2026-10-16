package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ConfigVersion is one immutable version of the runtime configuration
// (ADR-0008). Content has the word list masked; WordsEnc is the opaque
// AES-GCM ciphertext of the word list (ADR-0009). Diff is relative to the
// previous version, with the word list masked.
type ConfigVersion struct {
	ID        int64
	CreatedAt time.Time
	Author    string
	Comment   string
	Content   []byte
	WordsEnc  []byte
	Diff      string
}

// NewConfigVersion describes a version to create. If Seal is set it is called
// inside the write transaction with the ID the version will get (the AAD for
// the encryption) and its result becomes WordsEnc.
//
// Seal MUST NOT call back into the Store: the transaction holds the only
// writer connection, so any Store call from Seal deadlocks. Keep it a pure
// function (encrypt and return).
type NewConfigVersion struct {
	Author   string
	Comment  string
	Content  []byte
	WordsEnc []byte
	Diff     string
	Seal     func(id int64) ([]byte, error)
}

// CreateConfigVersion appends a new version. IDs are assigned as MAX(id)+1
// inside the single-writer transaction, so they are gap-free and known before
// the row is written.
func (s *Store) CreateConfigVersion(ctx context.Context, n NewConfigVersion) (ConfigVersion, error) {
	if err := checkIdent("author", n.Author); err != nil {
		return ConfigVersion{}, err
	}
	if len(n.Comment) > maxComment {
		return ConfigVersion{}, invalid("comment")
	}
	if len(n.Content) == 0 {
		return ConfigVersion{}, invalid("content")
	}
	if len(n.Diff) > MaxDiffBytes {
		return ConfigVersion{}, invalid("diff")
	}
	var v ConfigVersion
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var id int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) + 1 FROM config_versions`).Scan(&id); err != nil {
			return err
		}
		words := n.WordsEnc
		if n.Seal != nil {
			sealed, err := n.Seal(id)
			if err != nil {
				return err
			}
			words = sealed
		}
		if words == nil {
			words = []byte{}
		}
		v = ConfigVersion{ID: id, CreatedAt: s.nowMS(), Author: n.Author, Comment: n.Comment,
			Content: n.Content, WordsEnc: words, Diff: n.Diff}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO config_versions (id, created_at, author, comment, content, words_enc, diff) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			v.ID, ms(v.CreatedAt), v.Author, v.Comment, v.Content, v.WordsEnc, v.Diff)
		return err
	})
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			return ConfigVersion{}, err
		}
		return ConfigVersion{}, mapErr("create config version", err)
	}
	return v, nil
}

const configCols = `id, created_at, author, comment, content, words_enc, diff`

func scanConfig(sc interface{ Scan(...any) error }) (ConfigVersion, error) {
	var v ConfigVersion
	var created int64
	if err := sc.Scan(&v.ID, &created, &v.Author, &v.Comment, &v.Content, &v.WordsEnc, &v.Diff); err != nil {
		return ConfigVersion{}, err
	}
	v.CreatedAt = fromMS(created)
	return v, nil
}

// ConfigVersionByID returns one version.
func (s *Store) ConfigVersionByID(ctx context.Context, id int64) (ConfigVersion, error) {
	v, err := scanConfig(s.r.QueryRowContext(ctx, `SELECT `+configCols+` FROM config_versions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ConfigVersion{}, ErrNotFound
	}
	return v, err
}

// ListConfigVersions returns versions newest first. limit <= 0 means 100,
// capped at 1000.
func (s *Store) ListConfigVersions(ctx context.Context, limit int) ([]ConfigVersion, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > maxList {
		limit = maxList
	}
	rows, err := s.r.QueryContext(ctx, `SELECT `+configCols+` FROM config_versions ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConfigVersion
	for rows.Next() {
		v, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ActiveConfigVersionID returns the active version, or ErrNotFound if none
// was activated yet.
func (s *Store) ActiveConfigVersionID(ctx context.Context) (int64, error) {
	var id int64
	err := s.r.QueryRowContext(ctx, `SELECT version_id FROM config_active WHERE id = 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// SetActiveConfigVersion marks an existing version as active.
func (s *Store) SetActiveConfigVersion(ctx context.Context, id int64) error {
	_, err := s.w.ExecContext(ctx,
		`INSERT INTO config_active (id, version_id) VALUES (1, ?) ON CONFLICT (id) DO UPDATE SET version_id = excluded.version_id`, id)
	return mapErr("set active config version", err)
}
