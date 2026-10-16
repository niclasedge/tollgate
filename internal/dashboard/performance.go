package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/niclasedge/tollgate/internal/perf"
	"github.com/niclasedge/tollgate/internal/store"
)

const (
	defaultPerfDays  = 7
	maxPerfDays      = 90
	maxSpeedModels   = 4
	speedtestTimeout = 60 * time.Second
	speedtestHistory = 50
)

// PerfStore is the part of the store behind the performance report (#87).
type PerfStore interface {
	PerfRows(ctx context.Context, since time.Time) ([]store.PerfRow, error)
	SpeedtestRows(ctx context.Context, limit int) ([]store.PerfRow, error)
}

// Prober runs one speedtest request through the proxy and returns its trace
// ID (server.Server.Probe with the fixed speedtest body).
type Prober func(ctx context.Context, model string, skipRedact bool) (string, error)

// speedtestLock allows one speedtest at a time.
type speedtestLock struct{ mu sync.Mutex }

func (h *Handler) perfRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/performance", h.performance)
	mux.HandleFunc("POST /api/speedtest", h.speedtest)
}

func (h *Handler) perfStore(w http.ResponseWriter) (PerfStore, bool) {
	ps, ok := h.st.(PerfStore)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, errorBody{Error: "performance not available"})
	}
	return ps, ok
}

func (h *Handler) performance(w http.ResponseWriter, r *http.Request) {
	days, err := intParam(r, "days", defaultPerfDays, 1, maxPerfDays)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	ps, ok := h.perfStore(w)
	if !ok {
		return
	}
	today := h.now().UTC().Truncate(24 * time.Hour)
	from := today.AddDate(0, 0, -(days - 1))
	rows, err := ps.PerfRows(r.Context(), from)
	if err != nil {
		h.fail(w, r, "perf rows", err)
		return
	}
	tests, err := ps.SpeedtestRows(r.Context(), speedtestHistory)
	if err != nil {
		h.fail(w, r, "speedtest rows", err)
		return
	}
	report := buildReport(from, today, rows, tests)
	if fs, ok := h.st.(FillStore); ok {
		// The capacity estimate needs a whole weekly window, whatever the period.
		since := h.now().Add(-capacityLookback)
		if from.Before(since) {
			since = from
		}
		fill, err := fs.FillRows(r.Context(), since)
		if err != nil {
			h.fail(w, r, "fill rows", err)
			return
		}
		i := slices.IndexFunc(fill, func(f store.FillRow) bool { return !f.StartedAt.Before(from) })
		if i < 0 {
			i = len(fill)
		}
		report.FastestFill = fastestFill(fill[i:])
		usage, n := h.cachedUsage(r)
		report.Capacity = capacity(fill, from, h.now(), weekMarks(usage), n)
	}
	writeJSON(w, http.StatusOK, report)
}

// buildReport is pure: equal rows give an equal report.
func buildReport(from, to time.Time, rows, tests []store.PerfRow) PerfReport {
	out := PerfReport{From: from.Format(time.DateOnly), To: to.Format(time.DateOnly),
		Models: []ModelPerf{}, Speedtests: []SpeedtestResult{}}
	byModel := map[string][]store.PerfRow{}
	var names []string
	for _, r := range rows {
		if _, ok := byModel[r.Model]; !ok {
			names = append(names, r.Model)
		}
		byModel[r.Model] = append(byModel[r.Model], r)
	}
	// The hourly tokens/s curve covers at most the last 7 days.
	hourSince := to.AddDate(0, 0, -(maxHourlyCostDays - 1))
	if from.After(hourSince) {
		hourSince = from
	}
	for _, m := range names {
		out.Models = append(out.Models, modelPerf(m, byModel[m], hourSince))
	}
	slices.SortFunc(out.Models, func(a, b ModelPerf) int {
		if a.Requests != b.Requests {
			return int(b.Requests - a.Requests)
		}
		return strings.Compare(a.Model, b.Model)
	})
	out.Redactor = redactorCost(rows)
	usual := map[string]ModelPerf{}
	for _, m := range out.Models {
		usual[m.Model] = m
	}
	seen := map[[2]any]bool{}
	for _, t := range tests { // newest first: the first per model and variant wins
		k := [2]any{t.Model, t.Redact != nil}
		if seen[k] {
			continue
		}
		seen[k] = true
		res := speedtestResult(t)
		if m, ok := usual[t.Model]; ok {
			if m.TTFT != nil {
				v := m.TTFT.MedianUs
				res.UsualTTFTMedianUs = &v
			}
			v := m.Duration.MedianUs
			res.UsualDurationMedian = &v
		}
		out.Speedtests = append(out.Speedtests, res)
	}
	slices.SortFunc(out.Speedtests, func(a, b SpeedtestResult) int {
		if c := strings.Compare(a.Model, b.Model); c != 0 {
			return c
		}
		switch {
		case a.Redact == b.Redact:
			return 0
		case !a.Redact:
			return -1
		}
		return 1
	})
	return out
}

func dist(vals []int64) *Dist {
	if len(vals) == 0 {
		return nil
	}
	s := slices.Clone(vals)
	slices.Sort(s)
	return &Dist{Samples: int64(len(s)), MinUs: s[0], MedianUs: perf.Percentile(s, 500), P90Us: perf.Percentile(s, 900), MaxUs: s[len(s)-1]}
}

func medianPtr(vals []int64) *int64 {
	if d := dist(vals); d != nil {
		return &d.MedianUs
	}
	return nil
}

func rowTTFT(r store.PerfRow) (int64, bool) {
	if r.TTFTMicros == nil {
		return 0, false
	}
	return *r.TTFTMicros, true
}

func rowTPSMilli(r store.PerfRow) (int64, bool) {
	if r.GenStartMicros == nil || r.Usage == nil {
		return 0, false
	}
	return perf.TPSMilli(r.Usage.Output, r.DurationMicros, *r.GenStartMicros, true, r.Stream)
}

func rowTPS(r store.PerfRow) (float64, bool) {
	if r.GenStartMicros == nil || r.Usage == nil {
		return 0, false
	}
	return perf.OutputTPS(r.Usage.Output, r.DurationMicros, *r.GenStartMicros, true, r.Stream)
}

func inputTotal(u *store.Usage) int64 {
	if u == nil {
		return 0
	}
	return u.Input + u.CacheRead + u.CacheCreation
}

func modelPerf(model string, rows []store.PerfRow, hourSince time.Time) ModelPerf {
	m := ModelPerf{Model: model, Requests: int64(len(rows)), Days: []DayPerf{}, Hours: []HourPerf{}}
	var ttft, dur, tg, tpsTenths []int64
	var infl []perf.InfluenceRow
	type day struct {
		n, slow                int64
		ttft, dur, tps, shares []int64
	}
	days := map[string]*day{}
	var order []string
	hours := map[string][]int64{}
	var hourOrder []string
	for _, r := range rows {
		if r.Slow {
			m.Slow++
		}
		if r.Judged {
			m.Judged++
		}
		dur = append(dur, r.DurationMicros)
		ret := int64(0)
		if r.ReturnMicros != nil {
			ret = *r.ReturnMicros
		}
		tg = append(tg, perf.TollgateMicros([]int64{r.StageMicros}, ret))
		v, hasTTFT := rowTTFT(r)
		if hasTTFT {
			ttft = append(ttft, v)
		}
		if tps, ok := rowTPS(r); ok {
			tpsTenths = append(tpsTenths, int64(tps*10+0.5))
		}
		milli, hasTPS := rowTPSMilli(r)
		hasTPS = hasTPS && r.Usage.Output >= perf.MinTPSOutput
		if r.Usage != nil {
			infl = append(infl, perf.InfluenceRow{InputTotal: inputTotal(r.Usage), CacheRead: r.Usage.CacheRead,
				TTFT: v, HasTTFT: hasTTFT, Duration: r.DurationMicros, TPSMilli: milli, HasTPS: hasTPS})
		}
		key := r.StartedAt.UTC().Format(time.DateOnly)
		d := days[key]
		if d == nil {
			d = &day{}
			days[key] = d
			order = append(order, key)
		}
		d.n++
		if r.Slow {
			d.slow++
		}
		d.dur = append(d.dur, r.DurationMicros)
		if hasTTFT {
			d.ttft = append(d.ttft, v)
		}
		if hasTPS {
			d.tps = append(d.tps, milli)
			if !r.StartedAt.Before(hourSince) {
				hk := r.StartedAt.UTC().Truncate(time.Hour).Format(time.RFC3339)
				if _, ok := hours[hk]; !ok {
					hourOrder = append(hourOrder, hk)
				}
				hours[hk] = append(hours[hk], milli)
			}
		}
		if in := inputTotal(r.Usage); in > 0 {
			d.shares = append(d.shares, r.Usage.CacheRead*1000/in)
		}
	}
	m.TTFT, m.Tollgate = dist(ttft), dist(tg)
	if d := dist(dur); d != nil {
		m.Duration = *d
	}
	if d := dist(tpsTenths); d != nil {
		m.OutputTPS = &TPSDist{Samples: d.Samples, Min: float64(d.MinUs) / 10, Median: float64(d.MedianUs) / 10,
			P90: float64(d.P90Us) / 10, Max: float64(d.MaxUs) / 10}
	}
	slices.Sort(order)
	for _, k := range order {
		d := days[k]
		m.Days = append(m.Days, DayPerf{Day: k, Requests: d.n, Slow: d.slow, TTFTMedianUs: medianPtr(d.ttft),
			DurationMedianUs: medianPtr(d.dur), TPSMedianMilli: medianPtr(d.tps), CacheSharePermilleMedian: medianPtr(d.shares)})
	}
	slices.Sort(hourOrder)
	for _, k := range hourOrder {
		m.Hours = append(m.Hours, HourPerf{Hour: k, Samples: int64(len(hours[k])), TPSMedianMilli: *medianPtr(hours[k])})
	}
	m.Influence = Influence{Cells: []InfluenceCell{}, Insights: []Insight{}}
	for _, c := range perf.Matrix(infl) {
		m.Influence.Cells = append(m.Influence.Cells, InfluenceCell{Input: c.Input, Cache: c.Cache, Samples: int64(c.Samples),
			TTFTMedianUs: c.TTFTMedian, DurationMedianUs: c.DurationMedian, TPSMedian: tpsPtr(c.TPS)})
	}
	for _, in := range perf.Insights(infl) {
		m.Influence.Insights = append(m.Influence.Insights, Insight{Kind: in.Kind, Input: in.Input, From: in.From, To: in.To,
			DeltaPermille: in.DeltaPermille, SamplesFrom: int64(in.SamplesFrom), SamplesTo: int64(in.SamplesTo)})
	}
	return m
}

// redactorCost sums the redact stage and the return path; nil without rows
// that ran the redact stage.
func redactorCost(rows []store.PerfRow) *RedactorCost {
	var stage, ret, shares, bytesIn, matches, added []int64
	c := &RedactorCost{}
	for _, r := range rows {
		if r.Redact == nil {
			continue
		}
		c.Requests++
		stage = append(stage, r.Redact.DurationMicros)
		bytesIn = append(bytesIn, r.Redact.BytesIn)
		matches = append(matches, r.Redact.Matches)
		c.MatchesTotal += r.Redact.Matches
		a := r.Redact.TokensOut - r.Redact.TokensIn
		added = append(added, a)
		c.TokensAddedTotal += a
		own := r.Redact.DurationMicros
		if r.ReturnMicros != nil {
			ret = append(ret, *r.ReturnMicros)
			own += *r.ReturnMicros
		}
		if r.ReturnReplacements != nil {
			c.ReturnReplacementsTotal += *r.ReturnReplacements
		}
		if r.DurationMicros > 0 {
			shares = append(shares, own*1000/r.DurationMicros)
		}
	}
	if c.Requests == 0 {
		return nil
	}
	c.Stage, c.Return = dist(stage), dist(ret)
	pick := func(v []int64) int64 {
		if m := medianPtr(v); m != nil {
			return *m
		}
		return 0
	}
	c.ShareMedianPermille, c.BytesInMedian, c.MatchesMedian, c.TokensAddedMedian = pick(shares), pick(bytesIn), pick(matches), pick(added)
	return c
}

func speedtestResult(r store.PerfRow) SpeedtestResult {
	res := SpeedtestResult{Model: r.Model, Redact: r.Redact != nil, TraceID: r.ID, StartedAt: formatTime(r.StartedAt),
		Status: r.Status, TTFTUs: r.TTFTMicros, Error: r.ErrorKind}
	if r.Status != 0 {
		d := r.DurationMicros
		res.DurationUs = &d
	}
	if tps, ok := rowTPS(r); ok {
		res.OutputTPS = &tps
	}
	if res.Error == "" && (r.Status < 200 || r.Status > 299) {
		res.Error = "status"
	}
	return res
}

// errNoConfirm is the answer without confirm: true.
const errNoConfirm = "Bestätigung fehlt: der Test verbraucht Abo-Kontingent"

func (h *Handler) speedtest(w http.ResponseWriter, r *http.Request) {
	var req SpeedtestRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF {
		badRequest(w, "invalid speedtest request")
		return
	}
	if !req.Confirm {
		badRequest(w, errNoConfirm)
		return
	}
	if len(req.Models) < 1 || len(req.Models) > maxSpeedModels {
		badRequest(w, "models must name 1 to 4 models")
		return
	}
	for _, m := range req.Models {
		if !store.ValidModel(m) {
			badRequest(w, "invalid model name")
			return
		}
	}
	if h.probe == nil {
		writeJSON(w, http.StatusNotImplemented, errorBody{Error: "speedtest not available"})
		return
	}
	ps, ok := h.perfStore(w)
	if !ok {
		return
	}
	if !h.speedLock.mu.TryLock() {
		writeJSON(w, http.StatusConflict, errorBody{Error: "Speedtest läuft bereits"})
		return
	}
	defer h.speedLock.mu.Unlock()

	today := h.now().UTC().Truncate(24 * time.Hour)
	rows, err := ps.PerfRows(r.Context(), today.AddDate(0, 0, -(defaultPerfDays-1)))
	if err != nil {
		h.fail(w, r, "perf rows", err)
		return
	}
	usual := buildReport(today, today, rows, nil).Models
	out := SpeedtestResponse{Results: []SpeedtestResult{}}
	for _, m := range req.Models {
		for _, withRedact := range []bool{false, true} {
			res := SpeedtestResult{Model: m, Redact: withRedact}
			if withRedact && !h.redactEnabled {
				res.Error = "Redact-Stufe aus"
			} else {
				res = h.runProbe(r.Context(), m, withRedact)
			}
			for _, u := range usual {
				if u.Model == m {
					if u.TTFT != nil {
						v := u.TTFT.MedianUs
						res.UsualTTFTMedianUs = &v
					}
					v := u.Duration.MedianUs
					res.UsualDurationMedian = &v
				}
			}
			out.Results = append(out.Results, res)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) runProbe(ctx context.Context, model string, withRedact bool) SpeedtestResult {
	res := SpeedtestResult{Model: model, Redact: withRedact}
	pctx, cancel := context.WithTimeout(ctx, speedtestTimeout)
	defer cancel()
	id, err := h.probe(pctx, model, !withRedact)
	if err != nil {
		res.Error = "kein Ergebnis"
		if errors.Is(pctx.Err(), context.DeadlineExceeded) {
			res.Error = "Zeitlimit"
		}
		return res
	}
	t, err := h.st.GetTrace(ctx, id)
	if err != nil {
		res.TraceID, res.Error = id, "Trace nicht lesbar"
		return res
	}
	row := store.PerfRow{ID: t.ID, Model: t.Model, StartedAt: t.StartedAt, Stream: t.Stream, Status: t.Status,
		ErrorKind: t.ErrorKind, Usage: t.Usage, DurationMicros: t.DurationMicros, TTFTMicros: t.TTFTMicros, GenStartMicros: t.GenStartMicros}
	for _, sg := range t.Stages {
		if sg.Name == "redact" {
			row.Redact = &store.RedactStageStats{}
		}
	}
	out := speedtestResult(row)
	out.Redact = withRedact
	if row.Redact == nil && withRedact {
		out.Error = "Redact-Stufe fehlt"
	}
	return out
}
