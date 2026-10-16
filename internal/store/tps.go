package store

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/niclasedge/tollgate/internal/perf"
)

// TPS is the stored tokens/s verdict of a trace (#116). P10, median, p90 and
// the deviation are nil when the verdict is "unbekannt".
type TPS struct {
	Class             string
	Samples           int64
	TPSMilli          int64
	P10Milli          *int64
	MedianMilli       *int64
	P90Milli          *int64
	DeviationPermille *int64
	Verdict           string
}

var tpsClassRe = regexp.MustCompile(`^(i[0-3]|\*)$`)

func (t TPS) validate() error {
	if !tpsClassRe.MatchString(t.Class) {
		return invalid("tps class")
	}
	switch perf.TPSVerdict(t.Verdict) {
	case perf.TPSFaster, perf.TPSUsual, perf.TPSSlower, perf.TPSUnknown:
	default:
		return invalid("tps verdict")
	}
	if t.Samples < 0 || t.TPSMilli < 0 {
		return invalid("tps measurement")
	}
	return nil
}

// TPSFromJudgement converts a perf.TPSJudgement into its stored form.
func TPSFromJudgement(j perf.TPSJudgement) TPS {
	t := TPS{Class: j.Class, Samples: int64(j.Samples), TPSMilli: j.TPSMilli, Verdict: string(j.Verdict)}
	if j.Verdict != perf.TPSUnknown {
		p10, med, p90, dev := j.P10, j.Median, j.P90, j.DeviationPermille
		t.P10Milli, t.MedianMilli, t.P90Milli, t.DeviationPermille = &p10, &med, &p90, &dev
	}
	return t
}

// PutTPS stores the tokens/s verdict of a trace.
func (s *Store) PutTPS(ctx context.Context, traceID string, t TPS) error {
	if err := checkIdent("trace id", traceID); err != nil {
		return err
	}
	if err := t.validate(); err != nil {
		return err
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO trace_tps (trace_id, class, samples, tps_milli, p10_milli, median_milli, p90_milli, deviation_permille, verdict)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			traceID, t.Class, t.Samples, t.TPSMilli, optInt(t.P10Milli), optInt(t.MedianMilli), optInt(t.P90Milli),
			optInt(t.DeviationPermille), t.Verdict)
		return err
	})
	return mapErr("put tps", err)
}

// tpsFor loads the tokens/s verdicts of the given traces.
func (s *Store) tpsFor(ctx context.Context, ids []string) (map[string]*TPS, error) {
	out := map[string]*TPS{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.r.QueryContext(ctx,
		`SELECT trace_id, class, samples, tps_milli, p10_milli, median_milli, p90_milli, deviation_permille, verdict FROM trace_tps
		 WHERE trace_id IN (?`+strings.Repeat(", ?", len(ids)-1)+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var t TPS
		var p10, med, p90, dev sql.NullInt64
		if err := rows.Scan(&id, &t.Class, &t.Samples, &t.TPSMilli, &p10, &med, &p90, &dev, &t.Verdict); err != nil {
			return nil, err
		}
		t.P10Milli, t.MedianMilli, t.P90Milli, t.DeviationPermille = ptrInt(p10), ptrInt(med), ptrInt(p90), ptrInt(dev)
		out[id] = &t
	}
	return out, rows.Err()
}

// TPSSamples returns the tokens/s comparison values for a request of model
// started at before (#116): the base of PerfSamples, limited to streams with
// a generation start, a duration beyond it and at least perf.MinTPSOutput output tokens,
// the newest perf.MaxSamples first.
func (s *Store) TPSSamples(ctx context.Context, model string, before time.Time) ([]perf.TPSSample, error) {
	rows, err := s.r.QueryContext(ctx,
		`SELECT t.usage_input + t.usage_cache_read + t.usage_cache_creation, t.usage_output, t.gen_start_us, t.duration_us
		 FROM traces t
		 WHERE t.model = ? AND t.started_at >= ? AND t.started_at < ? AND t.endpoint = 'messages'
		   AND t.status BETWEEN 200 AND 299 AND t.error_kind = '' AND t.usage_input IS NOT NULL
		   AND t.client_id != ? AND t.stream = 1 AND t.gen_start_us IS NOT NULL AND t.duration_us > t.gen_start_us
		   AND t.usage_output >= ?
		 ORDER BY t.started_at DESC, t.id DESC LIMIT ?`,
		model, ms(before.Add(-perf.Window)), ms(before), SpeedtestClientID, perf.MinTPSOutput, perf.MaxSamples)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []perf.TPSSample
	for rows.Next() {
		var in, output, gen, dur int64
		if err := rows.Scan(&in, &output, &gen, &dur); err != nil {
			return nil, err
		}
		v, _ := perf.TPSMilli(output, dur, gen, true, true)
		out = append(out, perf.TPSSample{InputTotal: in, TPSMilli: v})
	}
	return out, rows.Err()
}
