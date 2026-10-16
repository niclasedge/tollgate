package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Match is one redaction hit: where, how long, which type and rule. It never
// carries the matched value (ADR-0009).
type Match struct {
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	Type   string `json:"type"`
	Rule   string `json:"rule"`
	// Placeholder is the numbered placeholder that replaced the value, e.g.
	// "<redacted:email_1>"; empty for rows written before #16.
	Placeholder string `json:"placeholder,omitempty"`
	// Mask is the display mask of the value (issue #112); empty for rows
	// written before #112.
	Mask string `json:"mask,omitempty"`
}

// Stage holds the measurements of one pipeline stage. Hashes are hex SHA-256
// over the redacted bytes; Diff is built from the redacted version only.
type Stage struct {
	Name            string
	DurationMicros  int64
	BytesIn         int64
	BytesOut        int64
	TokensIn        int64
	TokensOut       int64
	HashIn          string
	HashOut         string
	ConfigVersionID int64 // 0 = none
	Diff            string
	Matches         []Match
}

// Usage is the token usage reported by the upstream.
type Usage struct {
	Input         int64
	Output        int64
	CacheRead     int64
	CacheCreation int64
}

// Trace is one gateway request with its stage measurements.
type Trace struct {
	ID                   string
	ClientID             string
	AccountID            string // immutable request snapshot; empty for unknown/legacy
	SessionKeyHash       []byte // nil = client without session
	ConfigVersionID      int64  // 0 = none
	StartedAt            time.Time
	Endpoint             string
	Model                string
	Stream               bool
	EstimatedInputTokens int64
	Status               int    // 0 until FinishTrace
	DurationMicros       int64  // 0 until FinishTrace
	Usage                *Usage // nil until FinishTrace
	ErrorKind            string
	Stages               []Stage
	// Performance measurements (#87); nil = unknown or not measured.
	TTFTMicros         *int64
	GenStartMicros     *int64 // flush of message_start, base of tokens/s (#116)
	ReturnMicros       *int64
	ReturnReplacements *int64
	// Perf is the stored verdict per metric (empty without a verdict).
	Perf []Perf
	// TPS is the stored tokens/s verdict (#116); nil without one.
	TPS *TPS
}

// TraceResult is written after the response.
type TraceResult struct {
	Status         int
	DurationMicros int64
	Usage          *Usage
	ErrorKind      string
	// TTFTMicros, GenStartMicros, ReturnMicros and ReturnReplacements are
	// nil when unknown.
	TTFTMicros         *int64
	GenStartMicros     *int64
	ReturnMicros       *int64
	ReturnReplacements *int64
	// FiveHourPermille and FiveHourReset (unix seconds) are the 5-hour window
	// state of the response (#134); nil when the headers were absent.
	FiveHourPermille *int64
	FiveHourReset    *int64
	// SevenDayPermille and SevenDayReset are the 7-day window state (#136).
	SevenDayPermille *int64
	SevenDayReset    *int64
}

func (t Trace) validate() error {
	if err := checkIdent("trace id", t.ID); err != nil {
		return err
	}
	if err := checkIdent("client id", t.ClientID); err != nil {
		return err
	}
	if err := checkOptIdent("account id", t.AccountID); err != nil {
		return err
	}
	if t.SessionKeyHash != nil {
		if err := checkHash("session key hash", t.SessionKeyHash); err != nil {
			return err
		}
	}
	if t.StartedAt.IsZero() {
		return invalid("started at")
	}
	if t.EstimatedInputTokens < 0 {
		return invalid("estimated input tokens")
	}
	if err := checkOptIdent("error kind", t.ErrorKind); err != nil {
		return err
	}
	for _, st := range t.Stages {
		if err := st.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (st Stage) validate() error {
	if err := checkIdent("stage name", st.Name); err != nil {
		return err
	}
	if err := checkHex("stage hash in", st.HashIn); err != nil {
		return err
	}
	if err := checkHex("stage hash out", st.HashOut); err != nil {
		return err
	}
	if len(st.Diff) > MaxDiffBytes {
		return invalid("stage diff too large")
	}
	for _, v := range []int64{st.DurationMicros, st.BytesIn, st.BytesOut, st.TokensIn, st.TokensOut} {
		if v < 0 {
			return invalid("stage measurement")
		}
	}
	for _, m := range st.Matches {
		if m.Offset < 0 || m.Length < 0 {
			return invalid("match range")
		}
		if err := checkIdent("match type", m.Type); err != nil {
			return err
		}
		if err := checkOptIdent("match rule", m.Rule); err != nil {
			return err
		}
		if m.Placeholder != "" && !placeholderRe.MatchString(m.Placeholder) {
			return invalid("match placeholder")
		}
		if !validMask(m.Mask) {
			return invalid("match mask")
		}
	}
	return nil
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// Normalized returns t with the untrusted client input (model, endpoint)
// replaced by "unknown" if it is not a plausible identifier, exactly as
// RecordTrace stores it. A bad value must neither reach the database nor drop
// the trace.
func (t Trace) Normalized() Trace {
	t.Model = normIdent(modelRe, t.Model)
	t.Endpoint = normIdent(identRe, t.Endpoint)
	return t
}

// RecordTrace writes a trace and all its stages in one transaction.
func (s *Store) RecordTrace(ctx context.Context, t Trace) error {
	if err := t.validate(); err != nil {
		return err
	}
	t = t.Normalized()
	var session any
	if t.SessionKeyHash != nil {
		session = t.SessionKeyHash
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO traces (id, client_id, session_key_hash, config_version_id, started_at, endpoint, model, stream, estimated_input_tokens, account_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.ClientID, session, nullInt(t.ConfigVersionID), ms(t.StartedAt), t.Endpoint, t.Model, t.Stream, t.EstimatedInputTokens, nullString(t.AccountID)); err != nil {
			return err
		}
		for i, st := range t.Stages {
			matches := st.Matches
			if matches == nil {
				matches = []Match{}
			}
			mj, err := json.Marshal(matches)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO trace_stages (trace_id, seq, name, duration_us, bytes_in, bytes_out, tokens_in, tokens_out,
				   hash_in, hash_out, config_version_id, diff, matches)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				t.ID, i, st.Name, st.DurationMicros, st.BytesIn, st.BytesOut, st.TokensIn, st.TokensOut,
				st.HashIn, st.HashOut, nullInt(st.ConfigVersionID), st.Diff, string(mj)); err != nil {
				return err
			}
		}
		return nil
	})
	return mapErr("record trace", err)
}

// FinishTrace stores status, duration and usage after the response.
func (s *Store) FinishTrace(ctx context.Context, id string, r TraceResult) error {
	if err := checkOptIdent("error kind", r.ErrorKind); err != nil {
		return err
	}
	if r.Status < 100 || r.Status > 599 {
		return invalid("status")
	}
	if r.DurationMicros < 0 {
		return invalid("duration")
	}
	if u := r.Usage; u != nil && (u.Input < 0 || u.Output < 0 || u.CacheRead < 0 || u.CacheCreation < 0) {
		return invalid("usage")
	}
	for _, v := range []*int64{r.TTFTMicros, r.GenStartMicros, r.ReturnMicros, r.ReturnReplacements, r.FiveHourPermille, r.SevenDayPermille} {
		if v != nil && *v < 0 {
			return invalid("performance measurement")
		}
	}
	var in, out, cr, cc any
	if r.Usage != nil {
		in, out, cr, cc = r.Usage.Input, r.Usage.Output, r.Usage.CacheRead, r.Usage.CacheCreation
	}
	res, err := s.w.ExecContext(ctx,
		`UPDATE traces SET status = ?, duration_us = ?, usage_input = ?, usage_output = ?, usage_cache_read = ?,
		   usage_cache_creation = ?, error_kind = ?, ttft_us = ?, gen_start_us = ?, return_us = ?, return_replacements = ?,
		   five_hour_permille = ?, five_hour_reset = ?, seven_day_permille = ?, seven_day_reset = ? WHERE id = ?`,
		r.Status, r.DurationMicros, in, out, cr, cc, r.ErrorKind, optInt(r.TTFTMicros), optInt(r.GenStartMicros), optInt(r.ReturnMicros), optInt(r.ReturnReplacements),
		optInt(r.FiveHourPermille), optInt(r.FiveHourReset), optInt(r.SevenDayPermille), optInt(r.SevenDayReset), id)
	return affected(res, err, "finish trace")
}

const traceCols = `id, client_id, session_key_hash, config_version_id, started_at, endpoint, model, stream,
	estimated_input_tokens, status, duration_us, usage_input, usage_output, usage_cache_read, usage_cache_creation, error_kind, account_id,
	ttft_us, gen_start_us, return_us, return_replacements`

func scanTrace(sc interface{ Scan(...any) error }) (Trace, error) {
	return scanTraceExtra(sc)
}

// scanTraceExtra scans the traceCols columns followed by extra destinations.
func scanTraceExtra(sc interface{ Scan(...any) error }, extra ...any) (Trace, error) {
	var t Trace
	var cfg, status, dur, ui, uo, ucr, ucc sql.NullInt64
	var started int64
	var account sql.NullString
	var ttft, gen, ret, repl sql.NullInt64
	dest := append([]any{&t.ID, &t.ClientID, &t.SessionKeyHash, &cfg, &started, &t.Endpoint, &t.Model, &t.Stream,
		&t.EstimatedInputTokens, &status, &dur, &ui, &uo, &ucr, &ucc, &t.ErrorKind, &account, &ttft, &gen, &ret, &repl}, extra...)
	if err := sc.Scan(dest...); err != nil {
		return Trace{}, err
	}
	t.AccountID = account.String
	t.ConfigVersionID = cfg.Int64
	t.StartedAt = fromMS(started)
	t.Status = int(status.Int64)
	t.DurationMicros = dur.Int64
	if ui.Valid {
		t.Usage = &Usage{Input: ui.Int64, Output: uo.Int64, CacheRead: ucr.Int64, CacheCreation: ucc.Int64}
	}
	t.TTFTMicros, t.GenStartMicros = ptrInt(ttft), ptrInt(gen)
	t.ReturnMicros, t.ReturnReplacements = ptrInt(ret), ptrInt(repl)
	return t, nil
}

// GetTrace returns a trace with its stages in pipeline order.
func (s *Store) GetTrace(ctx context.Context, id string) (Trace, error) {
	t, err := scanTrace(s.r.QueryRowContext(ctx, `SELECT `+traceCols+` FROM traces WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Trace{}, ErrNotFound
	}
	if err != nil {
		return Trace{}, err
	}
	rows, err := s.r.QueryContext(ctx,
		`SELECT name, duration_us, bytes_in, bytes_out, tokens_in, tokens_out, hash_in, hash_out, config_version_id, diff, matches
		 FROM trace_stages WHERE trace_id = ? ORDER BY seq`, id)
	if err != nil {
		return Trace{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var st Stage
		var cfg sql.NullInt64
		var mj string
		if err := rows.Scan(&st.Name, &st.DurationMicros, &st.BytesIn, &st.BytesOut, &st.TokensIn, &st.TokensOut,
			&st.HashIn, &st.HashOut, &cfg, &st.Diff, &mj); err != nil {
			return Trace{}, err
		}
		st.ConfigVersionID = cfg.Int64
		if err := json.Unmarshal([]byte(mj), &st.Matches); err != nil {
			return Trace{}, err
		}
		if len(st.Matches) == 0 {
			st.Matches = nil
		}
		t.Stages = append(t.Stages, st)
	}
	if err := rows.Err(); err != nil {
		return Trace{}, err
	}
	perf, err := s.perfFor(ctx, []string{id})
	if err != nil {
		return Trace{}, err
	}
	t.Perf = perf[id]
	tps, err := s.tpsFor(ctx, []string{id})
	if err != nil {
		return Trace{}, err
	}
	t.TPS = tps[id]
	return t, nil
}

// ListTracesFilter selects traces for ListTraces. Page through results with
// the keyset cursor (Before, BeforeID) = (StartedAt, ID) of the last row of
// the previous page; rows sharing a millisecond are neither lost nor repeated.
type ListTracesFilter struct {
	ClientID string    // empty = all clients
	Before   time.Time // zero = no upper bound (exclusive)
	BeforeID string    // tie-break for Before: rows with started_at == Before and id < BeforeID follow
	Limit    int       // <= 0 means 100, capped at maxList
}

// ListTraces returns traces newest first (started_at DESC, id DESC), without
// stages.
func (s *Store) ListTraces(ctx context.Context, f ListTracesFilter) ([]Trace, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > maxList {
		limit = maxList
	}
	q, args := listTracesQuery(f, limit, qualifiedTraceCols, tracesFrom)
	rows, err := s.r.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Trace
	for rows.Next() {
		t, err := scanTrace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// tracesFrom is the FROM clause of ListTraces; the table alias is "t".
const tracesFrom = `traces t`

// qualifiedTraceCols is traceCols with the alias "t" in front of every column.
var qualifiedTraceCols = func() string {
	cols := strings.Split(traceCols, ",")
	for i, c := range cols {
		cols[i] = "t." + strings.TrimSpace(c)
	}
	return strings.Join(cols, ", ")
}()

// savedTokensSQL is the estimate before the first stage minus after the last
// (0 without stages), for the trace row aliased "t".
const savedTokensSQL = `COALESCE((SELECT tokens_in FROM trace_stages WHERE trace_id = t.id ORDER BY seq ASC LIMIT 1), 0)
	 - COALESCE((SELECT tokens_out FROM trace_stages WHERE trace_id = t.id ORDER BY seq DESC LIMIT 1), 0)`

// stageMicrosSQL is the sum of the stage durations of the trace row "t".
const stageMicrosSQL = `COALESCE((SELECT SUM(duration_us) FROM trace_stages WHERE trace_id = t.id), 0)`

// TraceListItem is a trace without stages plus the data the dashboard shows
// next to it.
type TraceListItem struct {
	Trace
	ClientName  string
	SavedTokens int64
	// StageMicros is the sum of all stage durations (tollgate's own share
	// before the forward).
	StageMicros int64
}

const (
	tracesWithClientFrom = `traces t JOIN clients c ON c.id = t.client_id`
)

// ListTracesWithClient is ListTraces with the client name (join) and the
// tokens saved by the pipeline per trace. Same order and keyset cursor.
func (s *Store) ListTracesWithClient(ctx context.Context, f ListTracesFilter) ([]TraceListItem, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > maxList {
		limit = maxList
	}
	q, args := listTracesQuery(f, limit, qualifiedTraceCols+`, c.name, `+savedTokensSQL+`, `+stageMicrosSQL, tracesWithClientFrom)
	rows, err := s.r.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TraceListItem
	for rows.Next() {
		var it TraceListItem
		t, err := scanTraceExtra(rows, &it.ClientName, &it.SavedTokens, &it.StageMicros)
		if err != nil {
			return nil, err
		}
		it.Trace = t
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]string, len(out))
	for i, it := range out {
		ids[i] = it.ID
	}
	perf, err := s.perfFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	tps, err := s.tpsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Perf, out[i].TPS = perf[out[i].ID], tps[out[i].ID]
	}
	return out, nil
}

// CostAggregate sums finished traces of one UTC day, client and model.
type CostAggregate struct {
	Day         string // YYYY-MM-DD (UTC)
	ClientID    string
	ClientName  string
	AccountID   string
	Model       string
	Requests    int64
	Usage       Usage
	Estimated   int64
	SavedTokens int64
}

// CostByDay aggregates finished traces (status set) that started at or after
// since by UTC day, client and model, ordered by day, client name, client ID,
// model. Traces without usage count as requests and add no tokens.
func (s *Store) CostByDay(ctx context.Context, since time.Time) ([]CostAggregate, error) {
	rows, err := s.r.QueryContext(ctx,
		`SELECT strftime('%Y-%m-%d', t.started_at / 1000, 'unixepoch') AS day, t.client_id, c.name, COALESCE(t.account_id,''), t.model,
		   COUNT(*), SUM(COALESCE(t.usage_input, 0)), SUM(COALESCE(t.usage_output, 0)),
		   SUM(COALESCE(t.usage_cache_read, 0)), SUM(COALESCE(t.usage_cache_creation, 0)),
		   SUM(t.estimated_input_tokens), SUM(`+savedTokensSQL+`)
		 FROM traces t JOIN clients c ON c.id = t.client_id
		 WHERE t.status IS NOT NULL AND t.started_at >= ?
		 GROUP BY day, t.client_id, t.account_id, t.model
		 ORDER BY day, c.name, t.client_id, t.account_id, t.model`, ms(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CostAggregate
	for rows.Next() {
		var a CostAggregate
		if err := rows.Scan(&a.Day, &a.ClientID, &a.ClientName, &a.AccountID, &a.Model, &a.Requests,
			&a.Usage.Input, &a.Usage.Output, &a.Usage.CacheRead, &a.Usage.CacheCreation,
			&a.Estimated, &a.SavedTokens); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// HourAggregate sums finished traces of one UTC hour, account and model.
type HourAggregate struct {
	Hour      string // YYYY-MM-DDTHH:00:00Z (UTC)
	AccountID string // empty for traces without account
	Model     string
	Requests  int64
	Usage     Usage
}

// CostByHour aggregates finished traces (status set) that started at or after
// since by UTC hour, account and model, ordered by hour, account, model.
// Hours without traces are absent. Traces without usage add no tokens.
func (s *Store) CostByHour(ctx context.Context, since time.Time) ([]HourAggregate, error) {
	rows, err := s.r.QueryContext(ctx,
		`SELECT strftime('%Y-%m-%dT%H:00:00Z', t.started_at / 1000, 'unixepoch') AS hour, COALESCE(t.account_id,'') AS account, t.model,
		   COUNT(*), SUM(COALESCE(t.usage_input, 0)), SUM(COALESCE(t.usage_output, 0)),
		   SUM(COALESCE(t.usage_cache_read, 0)), SUM(COALESCE(t.usage_cache_creation, 0))
		 FROM traces t
		 WHERE t.status IS NOT NULL AND t.started_at >= ?
		 GROUP BY hour, account, t.model
		 ORDER BY hour, account, t.model`, ms(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HourAggregate
	for rows.Next() {
		var a HourAggregate
		if err := rows.Scan(&a.Hour, &a.AccountID, &a.Model, &a.Requests,
			&a.Usage.Input, &a.Usage.Output, &a.Usage.CacheRead, &a.Usage.CacheCreation); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// listTracesQuery builds the keyset query for ListTraces.
// cols and from use the alias "t" for the traces table.
func listTracesQuery(f ListTracesFilter, limit int, cols, from string) (string, []any) {
	q := `SELECT ` + cols + ` FROM ` + from
	var where []string
	var args []any
	if f.ClientID != "" {
		where = append(where, `t.client_id = ?`)
		args = append(args, f.ClientID)
	}
	if !f.Before.IsZero() {
		if f.BeforeID != "" {
			where = append(where, `(t.started_at < ? OR (t.started_at = ? AND t.id < ?))`)
			args = append(args, ms(f.Before), ms(f.Before), f.BeforeID)
		} else {
			where = append(where, `t.started_at < ?`)
			args = append(args, ms(f.Before))
		}
	}
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, ` AND `)
	}
	q += ` ORDER BY t.started_at DESC, t.id DESC LIMIT ?`
	args = append(args, limit)
	return q, args
}

// PurgeTraces deletes traces (and, by cascade, their stages) started before
// the given time.
func (s *Store) PurgeTraces(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.w.ExecContext(ctx, `DELETE FROM traces WHERE started_at < ?`, ms(before))
	if err != nil {
		return 0, mapErr("purge traces", err)
	}
	return res.RowsAffected()
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
