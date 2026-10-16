package store

import (
	"context"
	"database/sql"
	"time"
)

// FillRow is one messages trace of an account for the 5-hour window
// evaluation (#134). Permille and Reset are nil when the response carried no
// 5-hour headers; such rows still count as requests.
type FillRow struct {
	AccountID string
	StartedAt time.Time
	Permille  *int64
	Reset     *int64 // unix seconds
	// SevenPermille and SevenReset are the 7-day window state (#136).
	SevenPermille *int64
	SevenReset    *int64
	// Model and Usage price the request for the capacity estimate (#136);
	// Usage is zero while the request has none.
	Model string
	Usage Usage
}

// FillRows returns the messages traces with an account started at or after
// since, ordered by start.
func (s *Store) FillRows(ctx context.Context, since time.Time) ([]FillRow, error) {
	rows, err := s.r.QueryContext(ctx,
		`SELECT account_id, started_at, five_hour_permille, five_hour_reset, seven_day_permille, seven_day_reset, model,
		        COALESCE(usage_input, 0), COALESCE(usage_output, 0), COALESCE(usage_cache_read, 0), COALESCE(usage_cache_creation, 0)
		   FROM traces
		  WHERE started_at >= ? AND endpoint = 'messages' AND account_id IS NOT NULL AND account_id != ''
		  ORDER BY started_at, id`, ms(since))
	if err != nil {
		return nil, mapErr("fill rows", err)
	}
	defer rows.Close()
	var out []FillRow
	for rows.Next() {
		var r FillRow
		var at int64
		var p, reset, sp, sr sql.NullInt64
		u := &r.Usage
		if err := rows.Scan(&r.AccountID, &at, &p, &reset, &sp, &sr, &r.Model, &u.Input, &u.Output, &u.CacheRead, &u.CacheCreation); err != nil {
			return nil, mapErr("fill rows", err)
		}
		r.StartedAt, r.Permille, r.Reset = time.UnixMilli(at).UTC(), ptrInt(p), ptrInt(reset)
		r.SevenPermille, r.SevenReset = ptrInt(sp), ptrInt(sr)
		out = append(out, r)
	}
	return out, mapErr("fill rows", rows.Err())
}
