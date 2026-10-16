package dashboard

// Types of the dashboard API (/api/traces, /api/costs, /api/status). They are
// the contract with the frontend; web/src/lib/types/dashboard.ts is generated
// from this file (make generate). Timestamps are RFC 3339 in UTC with
// millisecond precision. No field ever carries request content or secrets.

// Usage is the token usage reported by the upstream.
type Usage struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheRead     int64 `json:"cache_read"`
	CacheCreation int64 `json:"cache_creation"`
}

// TraceSummary is one request in the live list.
type TraceSummary struct {
	ID         string `json:"id"`
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name"`
	AccountID  string `json:"account_id,omitempty"`
	StartedAt  string `json:"started_at"`
	Endpoint   string `json:"endpoint"` // messages, count_tokens, passthrough
	Model      string `json:"model"`
	Stream     bool   `json:"stream"`
	// Status is the HTTP status sent to the client; 0 while still running.
	Status               int    `json:"status"`
	DurationMs           int64  `json:"duration_ms"`
	EstimatedInputTokens int64  `json:"estimated_input_tokens"`
	Usage                *Usage `json:"usage" tstype:"Usage | null,required"`
	// SavedTokens is the estimate before the first stage minus after the last
	// (0 until redaction/compression change bodies).
	SavedTokens int64 `json:"saved_tokens"`
	// CostNanoUSD is the list-price cost; null for unknown models or no usage.
	CostNanoUSD *int64 `json:"cost_nano_usd" tstype:"number | null,required"`
	ErrorKind   string `json:"error_kind"`
	// TTFTMs is the time to the first token (#87); null when unknown.
	TTFTMs *float64 `json:"ttft_ms" tstype:"number | null,required"`
	// OutputTPS is output tokens per second after the generation start
	// (message_start), one decimal; null for non-streams, no output or an
	// unknown generation start.
	OutputTPS *float64 `json:"output_tps" tstype:"number | null,required"`
	// TollgateMs is tollgate's own time (stages plus return path); null while
	// running.
	TollgateMs *float64 `json:"tollgate_ms" tstype:"number | null,required"`
	// Perf is the stored verdict; null without one.
	Perf *TracePerf `json:"perf" tstype:"TracePerf | null,required"`
	// TPS is the stored tokens/s verdict (#116); null without one.
	TPS *TPSJudgement `json:"tps" tstype:"TPSJudgement | null,required"`
}

// PerfMetric is the verdict of one metric of a request (#87). Median and P90
// are null when the verdict is "unbekannt".
type PerfMetric struct {
	Verdict  string   `json:"verdict"` // normal, langsam, unbekannt
	Class    string   `json:"class"`   // e.g. i2 or i2o1
	Samples  int64    `json:"samples"`
	MedianMs *float64 `json:"median_ms" tstype:"number | null,required"`
	P90Ms    *float64 `json:"p90_ms" tstype:"number | null,required"`
	ValueMs  float64  `json:"value_ms"`
}

// TracePerf is the "slow" verdict of a request.
type TracePerf struct {
	Slow     bool        `json:"slow"`
	TTFT     *PerfMetric `json:"ttft" tstype:"PerfMetric | null,required"`
	Duration *PerfMetric `json:"duration" tstype:"PerfMetric | null,required"`
}

// TPSJudgement is the tokens/s verdict of a request (#116). Values in
// tokens/s with one decimal; the statistics are null when "unbekannt".
type TPSJudgement struct {
	Verdict           string   `json:"verdict"` // schneller, üblich, langsamer, unbekannt
	Class             string   `json:"class"`   // i0..i3 or *
	Samples           int64    `json:"samples"`
	TPS               float64  `json:"tps"`
	P10               *float64 `json:"p10" tstype:"number | null,required"`
	Median            *float64 `json:"median" tstype:"number | null,required"`
	P90               *float64 `json:"p90" tstype:"number | null,required"`
	DeviationPermille *int64   `json:"deviation_permille" tstype:"number | null,required"`
}

// Dist is a nearest-rank distribution in microseconds.
type Dist struct {
	Samples  int64 `json:"samples"`
	MinUs    int64 `json:"min_us"`
	MedianUs int64 `json:"median_us"`
	P90Us    int64 `json:"p90_us"`
	MaxUs    int64 `json:"max_us"`
}

// TPSDist is a nearest-rank distribution of tokens/s, one decimal.
type TPSDist struct {
	Samples int64   `json:"samples"`
	Min     float64 `json:"min"`
	Median  float64 `json:"median"`
	P90     float64 `json:"p90"`
	Max     float64 `json:"max"`
}

// DayPerf is one UTC day of a model.
type DayPerf struct {
	Day              string `json:"day"` // YYYY-MM-DD
	Requests         int64  `json:"requests"`
	Slow             int64  `json:"slow"`
	TTFTMedianUs     *int64 `json:"ttft_median_us" tstype:"number | null,required"`
	DurationMedianUs *int64 `json:"duration_median_us" tstype:"number | null,required"`
	// TPSMedianMilli and CacheSharePermilleMedian are the trend of #116.
	TPSMedianMilli           *int64 `json:"tps_median_milli" tstype:"number | null,required"`
	CacheSharePermilleMedian *int64 `json:"cache_share_permille_median" tstype:"number | null,required"`
}

// HourPerf is one UTC hour of a model with judgeable tokens/s values; hours
// without values are absent.
type HourPerf struct {
	Hour           string `json:"hour"` // YYYY-MM-DDTHH:00:00Z
	Samples        int64  `json:"samples"`
	TPSMedianMilli int64  `json:"tps_median_milli"`
}

// InfluenceCell is one input × cache class of a model (#116). Medians are
// null below 5 values.
type InfluenceCell struct {
	Input            string   `json:"input"` // i0..i3
	Cache            string   `json:"cache"` // c0..c2
	Samples          int64    `json:"samples"`
	TTFTMedianUs     *int64   `json:"ttft_median_us" tstype:"number | null,required"`
	DurationMedianUs *int64   `json:"duration_median_us" tstype:"number | null,required"`
	TPSMedian        *float64 `json:"tps_median" tstype:"number | null,required"`
}

// Insight is one statement from the matrix; the frontend forms the sentence.
type Insight struct {
	Kind          string `json:"kind"`  // cache_ttft, input_tps
	Input         string `json:"input"` // cache_ttft: the input class
	From          string `json:"from"`
	To            string `json:"to"`
	DeltaPermille int64  `json:"delta_permille"`
	SamplesFrom   int64  `json:"samples_from"`
	SamplesTo     int64  `json:"samples_to"`
}

// Influence is the matrix of how input size and cache share go with speed.
type Influence struct {
	Cells    []InfluenceCell `json:"cells"`
	Insights []Insight       `json:"insights"`
}

// ModelPerf is the statistics of one model.
type ModelPerf struct {
	Model     string    `json:"model"`
	Requests  int64     `json:"requests"`
	Slow      int64     `json:"slow"`
	Judged    int64     `json:"judged"` // verdict normal or langsam
	TTFT      *Dist     `json:"ttft" tstype:"Dist | null,required"`
	Duration  Dist      `json:"duration"`
	Tollgate  *Dist     `json:"tollgate" tstype:"Dist | null,required"`
	OutputTPS *TPSDist  `json:"output_tps" tstype:"TPSDist | null,required"`
	Days      []DayPerf `json:"days"`
	// Hours is the tokens/s curve of the last 7 days of the period at most.
	Hours     []HourPerf `json:"hours"`
	Influence Influence  `json:"influence"`
}

// RedactorCost is what the redact stage and the return path cost.
type RedactorCost struct {
	Requests                int64 `json:"requests"`
	Stage                   *Dist `json:"stage" tstype:"Dist | null,required"`
	Return                  *Dist `json:"return" tstype:"Dist | null,required"`
	ShareMedianPermille     int64 `json:"share_median_permille"`
	BytesInMedian           int64 `json:"bytes_in_median"`
	MatchesMedian           int64 `json:"matches_median"`
	MatchesTotal            int64 `json:"matches_total"`
	ReturnReplacementsTotal int64 `json:"return_replacements_total"`
	TokensAddedTotal        int64 `json:"tokens_added_total"`
	TokensAddedMedian       int64 `json:"tokens_added_median"`
}

// SpeedtestResult is one speedtest run of a model and variant.
type SpeedtestResult struct {
	Model               string   `json:"model"`
	Redact              bool     `json:"redact"`
	TraceID             string   `json:"trace_id"`
	StartedAt           string   `json:"started_at"`
	Status              int      `json:"status"`
	TTFTUs              *int64   `json:"ttft_us" tstype:"number | null,required"`
	DurationUs          *int64   `json:"duration_us" tstype:"number | null,required"`
	OutputTPS           *float64 `json:"output_tps" tstype:"number | null,required"`
	Error               string   `json:"error"`
	UsualTTFTMedianUs   *int64   `json:"usual_ttft_median_us" tstype:"number | null,required"`
	UsualDurationMedian *int64   `json:"usual_duration_median_us" tstype:"number | null,required"`
}

// PerfReport is the body of GET /api/performance?days=N (1..90, default 7).
type PerfReport struct {
	From       string            `json:"from"`
	To         string            `json:"to"`
	Models     []ModelPerf       `json:"models"`
	Redactor   *RedactorCost     `json:"redactor" tstype:"RedactorCost | null,required"`
	Speedtests []SpeedtestResult `json:"speedtests"`
	// FastestFill is the fastest-filling 5-hour window (#134); null without
	// a window that rose by at least 30 points.
	FastestFill *WindowFill `json:"fastest_fill" tstype:"WindowFill | null,required"`
	// Capacity estimates how far the subscriptions reach (#136).
	Capacity Capacity `json:"capacity"`
}

// Capacity is measured from the 5-hour marks and the cost of the traces and
// the weekly use of the accounts.
type Capacity struct {
	// WindowsPerWeek is how many full 5-hour windows one weekly limit holds:
	// the cost since the start of each sample account's weekly window in
	// full-window prices, over its weekly use; null without a sample account.
	WindowsPerWeek *float64 `json:"windows_per_week" tstype:"number | null,required"`
	// FillsPerDay is the 5-hour rise of all windows in full windows per
	// measured day; null under one measured hour.
	FillsPerDay   *float64 `json:"fills_per_day" tstype:"number | null,required"`
	MeasuredHours float64  `json:"measured_hours"`
	// SampleWindows are the 5-hour windows behind WindowCostNanoUSD.
	SampleWindows int64 `json:"sample_windows"`
	// WindowCostNanoUSD is the list price of one full 5-hour window; null
	// without a window that rose by at least 30 points.
	WindowCostNanoUSD *int64 `json:"window_cost_nano_usd" tstype:"number | null,required"`
	// SampleAccounts are the accounts behind WindowsPerWeek: at least 25 %
	// weekly use and traces covering their whole weekly window.
	SampleAccounts int64 `json:"sample_accounts"`
	// Accounts is the number of subscriptions.
	Accounts int64 `json:"accounts"`
	// FreeThisWeek is the room left in the running weekly windows of the
	// accounts with a known weekly use, in full 5-hour windows; null without.
	FreeThisWeek *float64 `json:"free_this_week" tstype:"number | null,required"`
}

// WindowFill is one 5-hour window of an account: from the first observed
// utilization to the peak. FullUs extrapolates the observed rise to 100 %.
type WindowFill struct {
	AccountID    string  `json:"account_id"`
	Reset        string  `json:"reset"` // RFC 3339, end of the window
	FromPermille int64   `json:"from_permille"`
	ToPermille   int64   `json:"to_permille"`
	ObservedUs   int64   `json:"observed_us"`
	FullUs       int64   `json:"full_us"`
	Requests     int64   `json:"requests"`   // messages requests between first and peak
	PerMinute    float64 `json:"per_minute"` // one decimal
}

// SpeedtestRequest is the body of POST /api/speedtest.
type SpeedtestRequest struct {
	Models  []string `json:"models"`
	Confirm bool     `json:"confirm"`
}

// SpeedtestResponse is the answer of POST /api/speedtest.
type SpeedtestResponse struct {
	Results []SpeedtestResult `json:"results"`
}

// TraceList is the body of GET /api/traces?limit=&before=&before_id=.
// NextBefore/NextBeforeID are the cursor for the next (older) page, both
// empty on the last page.
type TraceList struct {
	Traces       []TraceSummary `json:"traces"`
	NextBefore   string         `json:"next_before"`
	NextBeforeID string         `json:"next_before_id"`
}

// TraceEvent is the data of the SSE event "trace" on GET /api/traces/events,
// sent when a trace starts (status 0) and again when it is finished.
type TraceEvent struct {
	Trace TraceSummary `json:"trace"`
}

// CallRate is the body of GET /api/calls (#132): the requests (traces) of
// the current window [now-5 min, now) against the Windows windows before it.
type CallRate struct {
	WindowSeconds int64 `json:"window_seconds"` // 300
	Windows       int64 `json:"windows"`        // 10
	Current       int64 `json:"current"`
	PreviousTotal int64 `json:"previous_total"`
	// Average is PreviousTotal / Windows with one decimal.
	Average float64 `json:"average"`
	Trend   string  `json:"trend"` // above, below, equal
}

// CostRow aggregates finished traces of one UTC day, client and model.
type CostRow struct {
	Day                  string `json:"day"` // YYYY-MM-DD (UTC)
	ClientID             string `json:"client_id"`
	ClientName           string `json:"client_name"`
	AccountID            string `json:"account_id,omitempty"`
	Model                string `json:"model"`
	Requests             int64  `json:"requests"`
	Usage                Usage  `json:"usage"`
	EstimatedInputTokens int64  `json:"estimated_input_tokens"`
	SavedTokens          int64  `json:"saved_tokens"`
	// CostNanoUSD is null when the model has no known price.
	CostNanoUSD *int64 `json:"cost_nano_usd" tstype:"number | null,required"`
	// CacheSavingsNanoUSD is what cache reads saved against the input price;
	// null when the model has no known price.
	CacheSavingsNanoUSD *int64 `json:"cache_savings_nano_usd" tstype:"number | null,required"`
}

// CostHour aggregates finished traces of one UTC hour, account and model.
type CostHour struct {
	Hour      string `json:"hour"`       // YYYY-MM-DDTHH:00:00Z (UTC)
	AccountID string `json:"account_id"` // empty for traces without account
	Model     string `json:"model"`
	Requests  int64  `json:"requests"`
	Usage     Usage  `json:"usage"`
	// CostNanoUSD is null when the model has no known price.
	CostNanoUSD *int64 `json:"cost_nano_usd" tstype:"number | null,required"`
}

// CostReport is the body of GET /api/costs?days=N (1..366, default 30).
type CostReport struct {
	From string    `json:"from"` // first day included, YYYY-MM-DD
	To   string    `json:"to"`   // last day included, YYYY-MM-DD
	Rows []CostRow `json:"rows"`
	// Hours covers the last min(N, 8) days; hours without traces are absent.
	Hours []CostHour `json:"hours"`
}

// Account is one credential file in the CLIProxyAPI auth dir.
type Account struct {
	Name     string `json:"name"`     // file name
	Provider string `json:"provider"` // e.g. "claude"
	Email    string `json:"email"`
	Status   string `json:"status"`
	Disabled bool   `json:"disabled"`
	// CooldownUntil is set while the account is rate limited.
	CooldownUntil string `json:"cooldown_until"`
}

// ServiceHealth is the state of one service.
type ServiceHealth struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Version string `json:"version"`
}

// Status is the body of GET /api/status.
type Status struct {
	AccountManagement bool          `json:"account_management,omitempty"`
	Tollgate          ServiceHealth `json:"tollgate"`
	CLIProxyAPI       ServiceHealth `json:"cliproxyapi"`
	// Accounts lists the auth dir; by the AGENTS.md rule it holds exactly one
	// active account. More than one is reported as a warning in Message.
	Accounts []Account `json:"accounts"`
	// Limits is the latest subscription rate-limit state read from upstream
	// response headers; null before the first response that carried them.
	Limits *RateLimits `json:"limits" tstype:"RateLimits | null,required"`
}

// LimitWindow is one subscription rate-limit window.
type LimitWindow struct {
	// Utilization is the used share, 0..1 (above 1: over the limit); null if
	// unknown.
	Utilization *float64 `json:"utilization" tstype:"number | null,required"`
	Reset       string   `json:"reset"`  // RFC 3339, "" if unknown
	Status      string   `json:"status"` // allowed, allowed_warning, rejected or ""
}

// RateLimits is the unified subscription rate-limit state (5 hours, 7 days).
type RateLimits struct {
	Status              string      `json:"status"`
	Reset               string      `json:"reset"` // RFC 3339, "" if unknown
	RepresentativeClaim string      `json:"representative_claim"`
	FiveHour            LimitWindow `json:"five_hour"`
	SevenDay            LimitWindow `json:"seven_day"`
	ObservedAt          string      `json:"observed_at"` // RFC 3339
}

// Match is one redaction hit of a stage: position in the redacted body,
// type, rule and placeholder. It never carries the replaced value.
type Match struct {
	Offset      int    `json:"offset"`
	Length      int    `json:"length"`
	Type        string `json:"type"`
	Rule        string `json:"rule"`
	Placeholder string `json:"placeholder"` // "" for traces recorded before #16
	Mask        string `json:"mask"`        // display mask, "" for traces recorded before #112
}

// StageDetail is the stored record of one pipeline stage. Hashes are hex
// SHA-256 over the redacted bytes; Diff is built from the redacted version.
type StageDetail struct {
	Name          string  `json:"name"`
	DurationUs    int64   `json:"duration_us"`
	BytesIn       int64   `json:"bytes_in"`
	BytesOut      int64   `json:"bytes_out"`
	TokensIn      int64   `json:"tokens_in"`
	TokensOut     int64   `json:"tokens_out"`
	HashIn        string  `json:"hash_in"`
	HashOut       string  `json:"hash_out"`
	ConfigVersion int64   `json:"config_version"` // 0 = none
	Diff          string  `json:"diff"`
	Matches       []Match `json:"matches"`
}

// TraceDetail is the body of GET /api/traces/{id}: the summary plus all
// stages in pipeline order.
type TraceDetail struct {
	Trace  TraceSummary  `json:"trace"`
	Stages []StageDetail `json:"stages"`
}

// Redaction is one recent hit for the overview. No field carries the value.
type Redaction struct {
	At          string `json:"at"` // start of the request, RFC 3339
	TraceID     string `json:"trace_id"`
	ClientID    string `json:"client_id"`
	ClientName  string `json:"client_name"`
	Stage       string `json:"stage"`
	Type        string `json:"type"`
	Rule        string `json:"rule"`
	Placeholder string `json:"placeholder"`
	Mask        string `json:"mask"` // display mask, "" before #112
}

// RedactionList is the body of GET /api/redactions?limit=N (1..50, default
// 10), newest request first.
type RedactionList struct {
	Redactions []Redaction `json:"redactions"`
}
