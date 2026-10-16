package store

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/niclasedge/tollgate/internal/perf"
)

// SpeedtestClientID is the reserved internal client of the speedtest (#87).
// Migration 0006 creates it blocked with a key hash that has no known
// preimage; it cannot be created, rotated or re-activated.
const SpeedtestClientID = "speedtest"

// reservedNamePrefix marks client names reserved for tollgate itself.
const reservedNamePrefix = "tollgate:"

// Perf is the stored verdict of one metric of a trace (#87). Median and P90
// are nil when the verdict is "unbekannt".
type Perf struct {
	Metric       string
	Class        string
	Samples      int64
	MedianMicros *int64
	P90Micros    *int64
	ValueMicros  int64
	Verdict      string
}

var perfClassRe = regexp.MustCompile(`^(i[0-3]o[0-3]|i[0-3]|o[0-3])$`)

func (p Perf) validate() error {
	if p.Metric != perf.MetricTTFT && p.Metric != perf.MetricDuration {
		return invalid("perf metric")
	}
	if !perfClassRe.MatchString(p.Class) {
		return invalid("perf class")
	}
	switch perf.Verdict(p.Verdict) {
	case perf.Normal, perf.Slow, perf.Unknown:
	default:
		return invalid("perf verdict")
	}
	if p.Samples < 0 || p.ValueMicros < 0 || (p.MedianMicros != nil && *p.MedianMicros < 0) || (p.P90Micros != nil && *p.P90Micros < 0) {
		return invalid("perf measurement")
	}
	return nil
}

// PerfFromJudgement converts a perf.Judgement into its stored form.
func PerfFromJudgement(j perf.Judgement) Perf {
	p := Perf{Metric: j.Metric, Class: j.Class, Samples: int64(j.Samples), ValueMicros: j.Value, Verdict: string(j.Verdict)}
	if j.Verdict != perf.Unknown {
		med, p90 := j.Median, j.P90
		p.MedianMicros, p.P90Micros = &med, &p90
	}
	return p
}

// PutPerf stores the verdicts of a trace in one transaction.
func (s *Store) PutPerf(ctx context.Context, traceID string, ps []Perf) error {
	if err := checkIdent("trace id", traceID); err != nil {
		return err
	}
	for _, p := range ps {
		if err := p.validate(); err != nil {
			return err
		}
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, p := range ps {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO trace_perf (trace_id, metric, class, samples, median_us, p90_us, value_us, verdict)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				traceID, p.Metric, p.Class, p.Samples, optInt(p.MedianMicros), optInt(p.P90Micros), p.ValueMicros, p.Verdict); err != nil {
				return err
			}
		}
		return nil
	})
	return mapErr("put perf", err)
}

// perfFor loads the verdicts of the given traces, ordered by metric.
func (s *Store) perfFor(ctx context.Context, ids []string) (map[string][]Perf, error) {
	out := map[string][]Perf{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.r.QueryContext(ctx,
		`SELECT trace_id, metric, class, samples, median_us, p90_us, value_us, verdict FROM trace_perf
		 WHERE trace_id IN (?`+strings.Repeat(", ?", len(ids)-1)+`) ORDER BY trace_id, metric`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var p Perf
		var med, p90 sql.NullInt64
		if err := rows.Scan(&id, &p.Metric, &p.Class, &p.Samples, &med, &p90, &p.ValueMicros, &p.Verdict); err != nil {
			return nil, err
		}
		p.MedianMicros, p.P90Micros = ptrInt(med), ptrInt(p90)
		out[id] = append(out[id], p)
	}
	return out, rows.Err()
}

// PerfSamples returns the comparison values for a request of model started
// at before (#87): successful messages traces of the same model from
// [before - perf.Window, before) with usage and the metric, without the
// speedtest client, the newest perf.MaxSamples first.
func (s *Store) PerfSamples(ctx context.Context, model string, before time.Time, metric string) ([]perf.Sample, error) {
	cond := `t.duration_us IS NOT NULL`
	if metric == perf.MetricTTFT {
		cond = `t.ttft_us IS NOT NULL`
	} else if metric != perf.MetricDuration {
		return nil, invalid("perf metric")
	}
	rows, err := s.r.QueryContext(ctx,
		`SELECT t.usage_input + t.usage_cache_read + t.usage_cache_creation, t.usage_output, t.ttft_us, t.duration_us
		 FROM traces t
		 WHERE t.model = ? AND t.started_at >= ? AND t.started_at < ? AND t.endpoint = 'messages'
		   AND t.status BETWEEN 200 AND 299 AND t.error_kind = '' AND t.usage_input IS NOT NULL
		   AND t.client_id != ? AND `+cond+`
		 ORDER BY t.started_at DESC, t.id DESC LIMIT ?`,
		model, ms(before.Add(-perf.Window)), ms(before), SpeedtestClientID, perf.MaxSamples)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []perf.Sample
	for rows.Next() {
		var sm perf.Sample
		var ttft, dur sql.NullInt64
		if err := rows.Scan(&sm.InputTotal, &sm.Output, &ttft, &dur); err != nil {
			return nil, err
		}
		sm.TTFT, sm.HasTTFT, sm.Duration = ttft.Int64, ttft.Valid, dur.Int64
		out = append(out, sm)
	}
	return out, rows.Err()
}

// RedactStageStats are the numbers of a trace's redact stage.
type RedactStageStats struct {
	DurationMicros int64
	BytesIn        int64
	TokensIn       int64
	TokensOut      int64
	Matches        int64
}

// PerfRow is one finished messages trace for the performance report.
type PerfRow struct {
	ID                 string
	ClientID           string
	Model              string
	StartedAt          time.Time
	Stream             bool
	Status             int
	ErrorKind          string
	Usage              *Usage
	DurationMicros     int64
	TTFTMicros         *int64
	GenStartMicros     *int64
	ReturnMicros       *int64
	ReturnReplacements *int64
	StageMicros        int64
	Redact             *RedactStageStats // nil without a redact stage
	Slow               bool              // a metric was judged "langsam"
	Judged             bool              // a metric was judged normal or slow
}

const perfRowCols = `t.id, t.client_id, t.model, t.started_at, t.stream, t.status, t.error_kind,
	t.usage_input, t.usage_output, t.usage_cache_read, t.usage_cache_creation,
	t.duration_us, t.ttft_us, t.gen_start_us, t.return_us, t.return_replacements,
	COALESCE((SELECT SUM(duration_us) FROM trace_stages WHERE trace_id = t.id), 0),
	rs.duration_us, rs.bytes_in, rs.tokens_in, rs.tokens_out, json_array_length(rs.matches),
	EXISTS (SELECT 1 FROM trace_perf p WHERE p.trace_id = t.id AND p.verdict = 'langsam'),
	EXISTS (SELECT 1 FROM trace_perf p WHERE p.trace_id = t.id AND p.verdict != 'unbekannt')`

const perfRowFrom = ` FROM traces t LEFT JOIN trace_stages rs ON rs.trace_id = t.id AND rs.name = 'redact'`

// PerfRows returns the finished messages traces with status 2xx started at or
// after since, oldest first, without the speedtest client.
func (s *Store) PerfRows(ctx context.Context, since time.Time) ([]PerfRow, error) {
	return s.perfRows(ctx, `SELECT `+perfRowCols+perfRowFrom+`
		WHERE t.started_at >= ? AND t.endpoint = 'messages' AND t.status BETWEEN 200 AND 299 AND t.client_id != ?
		ORDER BY t.started_at, t.id`, ms(since), SpeedtestClientID)
}

// SpeedtestRows returns the newest finished traces of the speedtest client,
// newest first.
func (s *Store) SpeedtestRows(ctx context.Context, limit int) ([]PerfRow, error) {
	return s.perfRows(ctx, `SELECT `+perfRowCols+perfRowFrom+`
		WHERE t.client_id = ? AND t.status IS NOT NULL
		ORDER BY t.started_at DESC, t.id DESC LIMIT ?`, SpeedtestClientID, limit)
}

func (s *Store) perfRows(ctx context.Context, q string, args ...any) ([]PerfRow, error) {
	rows, err := s.r.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PerfRow
	for rows.Next() {
		var r PerfRow
		var started int64
		var status, ui, uo, ucr, ucc, dur, ttft, gen, ret, repl sql.NullInt64
		var rd, rb, rti, rto, rm sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ClientID, &r.Model, &started, &r.Stream, &status, &r.ErrorKind,
			&ui, &uo, &ucr, &ucc, &dur, &ttft, &gen, &ret, &repl, &r.StageMicros,
			&rd, &rb, &rti, &rto, &rm, &r.Slow, &r.Judged); err != nil {
			return nil, err
		}
		r.StartedAt, r.Status, r.DurationMicros = fromMS(started), int(status.Int64), dur.Int64
		if ui.Valid {
			r.Usage = &Usage{Input: ui.Int64, Output: uo.Int64, CacheRead: ucr.Int64, CacheCreation: ucc.Int64}
		}
		r.TTFTMicros, r.GenStartMicros = ptrInt(ttft), ptrInt(gen)
		r.ReturnMicros, r.ReturnReplacements = ptrInt(ret), ptrInt(repl)
		if rd.Valid {
			r.Redact = &RedactStageStats{DurationMicros: rd.Int64, BytesIn: rb.Int64, TokensIn: rti.Int64, TokensOut: rto.Int64, Matches: rm.Int64}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func optInt(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func ptrInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}
