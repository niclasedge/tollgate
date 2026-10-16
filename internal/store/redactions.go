package store

import (
	"context"
	"time"
)

// Redaction is one stored match with the request it belongs to. It never
// carries the replaced value.
type Redaction struct {
	TraceID     string
	StartedAt   time.Time
	ClientID    string
	ClientName  string
	Stage       string
	Type        string
	Rule        string
	Placeholder string
	Mask        string // "" for rows written before #112
}

// RecentRedactions returns the latest limit matches over all traces: newest
// request first, within a request in stage and document order. Every stage
// row with matches has at least one, so the newest limit such rows suffice.
func (s *Store) RecentRedactions(ctx context.Context, limit int) ([]Redaction, error) {
	if limit <= 0 {
		return nil, nil
	}
	if limit > maxList {
		limit = maxList
	}
	rows, err := s.r.QueryContext(ctx,
		`WITH st AS (
		   SELECT ts.trace_id, ts.seq, ts.name, ts.matches, t.started_at, t.client_id, c.name AS client_name
		   FROM traces t
		   JOIN trace_stages ts ON ts.trace_id = t.id
		   JOIN clients c ON c.id = t.client_id
		   WHERE ts.matches <> '[]'
		   ORDER BY t.started_at DESC, t.id DESC, ts.seq
		   LIMIT ?)
		 SELECT st.trace_id, st.started_at, st.client_id, st.client_name, st.name,
		   json_extract(j.value, '$.type'), COALESCE(json_extract(j.value, '$.rule'), ''),
		   COALESCE(json_extract(j.value, '$.placeholder'), ''),
		   COALESCE(json_extract(j.value, '$.mask'), '')
		 FROM st, json_each(st.matches) j
		 ORDER BY st.started_at DESC, st.trace_id DESC, st.seq, j.key
		 LIMIT ?`, limit, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Redaction
	for rows.Next() {
		var r Redaction
		var started int64
		if err := rows.Scan(&r.TraceID, &started, &r.ClientID, &r.ClientName, &r.Stage, &r.Type, &r.Rule, &r.Placeholder, &r.Mask); err != nil {
			return nil, err
		}
		r.StartedAt = fromMS(started)
		out = append(out, r)
	}
	return out, rows.Err()
}
