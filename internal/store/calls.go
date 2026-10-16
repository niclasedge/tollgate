package store

import (
	"context"
	"time"
)

// CountTraces returns how many traces started in [from, to), finished or
// not. The range scan uses the started_at index.
func (s *Store) CountTraces(ctx context.Context, from, to time.Time) (int64, error) {
	var n int64
	err := s.r.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM traces WHERE started_at >= ? AND started_at < ?`, ms(from), ms(to)).Scan(&n)
	return n, err
}
