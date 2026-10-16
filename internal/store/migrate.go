package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var migrationNameRe = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

type migration struct {
	version  int
	name     string
	sql      string
	checksum string
}

// parseMigrations validates file names (NNNN_name.sql), requires versions
// 1..n without gaps or duplicates, and returns them in ascending order.
func parseMigrations(files map[string]string) ([]migration, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("store: no migrations found")
	}
	var out []migration
	for file, body := range files {
		m := migrationNameRe.FindStringSubmatch(file)
		if m == nil {
			return nil, fmt.Errorf("store: migration file name %q does not match NNNN_name.sql", file)
		}
		v, _ := strconv.Atoi(m[1])
		sum := sha256.Sum256([]byte(body))
		out = append(out, migration{version: v, name: m[2], sql: body, checksum: hex.EncodeToString(sum[:])})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].version != out[j].version {
			return out[i].version < out[j].version
		}
		return out[i].name < out[j].name
	})
	for i, m := range out {
		if i > 0 && m.version == out[i-1].version {
			return nil, fmt.Errorf("store: duplicate migration version %d", m.version)
		}
		if m.version != i+1 {
			return nil, fmt.Errorf("store: migration version gap before %d", m.version)
		}
	}
	return out, nil
}

func loadEmbeddedMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}
	files := make(map[string]string, len(entries))
	for _, e := range entries {
		b, err := fs.ReadFile(migrationFS, "migrations/"+e.Name())
		if err != nil {
			return nil, err
		}
		files[e.Name()] = string(b)
	}
	return parseMigrations(files)
}

// migrate applies all pending migrations in ONE transaction. The writer DSN
// uses BEGIN IMMEDIATE, so concurrent openers (processes or Stores) serialize:
// the second one waits, then finds everything applied. Already applied
// versions are skipped but their checksum must still match, so the result is
// idempotent and a modified migration is detected.
func migrate(ctx context.Context, db *sql.DB, migs []migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version  INTEGER PRIMARY KEY,
		name     TEXT NOT NULL,
		checksum TEXT NOT NULL
	) STRICT`); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}
	applied := map[int]string{}
	rows, err := tx.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("store: read schema_migrations: %w", err)
	}
	for rows.Next() {
		var v int
		var sum string
		if err := rows.Scan(&v, &sum); err != nil {
			rows.Close()
			return err
		}
		applied[v] = sum
	}
	if err := rows.Close(); err != nil {
		return err
	}

	known := map[int]bool{}
	for _, m := range migs {
		known[m.version] = true
	}
	for v := range applied {
		if !known[v] {
			return fmt.Errorf("store: database has migration %d which this binary does not know", v)
		}
	}

	for _, m := range migs {
		if sum, ok := applied[m.version]; ok {
			if sum != m.checksum {
				return fmt.Errorf("store: migration %04d_%s: checksum mismatch (file was modified after it was applied)", m.version, m.name)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("store: apply migration %04d_%s: %w", m.version, m.name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, checksum) VALUES (?, ?, ?)`,
			m.version, m.name, m.checksum); err != nil {
			return fmt.Errorf("store: apply migration %04d_%s: %w", m.version, m.name, err)
		}
	}
	return tx.Commit()
}
