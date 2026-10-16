package store

import "context"

// AccountRecord contains ciphertext only; it is never an API response.
type AccountRecord struct {
	ID, Filename, Name, Email string
	Cipher                    []byte
	UpdatedAt                 int64
}

func (s *Store) ListAccounts(ctx context.Context) ([]AccountRecord, error) {
	rows, err := s.r.QueryContext(ctx, `SELECT id,filename,name,email,token_cipher,updated_at FROM accounts ORDER BY updated_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccountRecord{}
	for rows.Next() {
		var a AccountRecord
		if err := rows.Scan(&a.ID, &a.Filename, &a.Name, &a.Email, &a.Cipher, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) SaveAccount(ctx context.Context, a AccountRecord) error {
	_, err := s.w.ExecContext(ctx, `INSERT INTO accounts(id,filename,name,email,token_cipher,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET filename=excluded.filename,name=excluded.name,email=excluded.email,token_cipher=excluded.token_cipher,updated_at=excluded.updated_at`, a.ID, a.Filename, a.Name, a.Email, a.Cipher, s.now().UnixMilli())
	return err
}
func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	_, err := s.w.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id)
	return err
}
func (s *Store) AuditAccount(ctx context.Context, id, result string) error {
	_, err := s.w.ExecContext(ctx, `INSERT INTO account_audit(account_id,at,result) VALUES(?,?,?)`, id, s.now().UnixMilli(), result)
	return err
}

// Account usage stores only whitelisted numeric limits and timestamps.
func (s *Store) SaveAccountUsage(ctx context.Context, id string, snapshot []byte) error {
	_, err := s.w.ExecContext(ctx, `INSERT INTO account_usage(account_id,snapshot) VALUES(?,?) ON CONFLICT(account_id) DO UPDATE SET snapshot=excluded.snapshot`, id, string(snapshot))
	return err
}
func (s *Store) AccountUsage(ctx context.Context, id string) ([]byte, error) {
	var data string
	err := s.r.QueryRowContext(ctx, `SELECT snapshot FROM account_usage WHERE account_id=?`, id).Scan(&data)
	return []byte(data), err
}
