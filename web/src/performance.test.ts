import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/svelte';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import { costRow, json, mockFetch, okStatus, sseResponse, trace, type Handler } from './test-helpers';
import { explainMetric, formatDeviation, insightSentence, tpsByHour, tpsCell, traceDot } from './lib/perf';
import type { InfluenceCell, ModelPerf, PerfReport, TracePerf, TPSJudgement } from './lib/types/dashboard';

function signIn() {
  auth.token = 'tok';
  localStorage.setItem('tollgate.admin_token', 'tok');
}

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

const slowPerf: TracePerf = {
  slow: true,
  ttft: { verdict: 'normal', class: 'i0', samples: 37, median_ms: 800, p90_ms: 1200, value_ms: 900 },
  duration: { verdict: 'langsam', class: 'i0o1', samples: 37, median_ms: 4200, p90_ms: 6100, value_ms: 9800 },
};
const tps = (o: Partial<TPSJudgement> = {}): TPSJudgement => ({
  verdict: 'langsamer', class: 'i2', samples: 87, tps: 48.2, p10: 51.3, median: 62, p90: 70.1, deviation_permille: -223, ...o,
});

describe('perf helpers', () => {
  it('explains verdicts', () => {
    expect(explainMetric('duration', slowPerf.duration!)).toBe('Gesamtdauer: erwartet ~4,2 s, gebraucht 9,8 s (p90 6,1 s, 37 Vergleichswerte)');
    expect(explainMetric('ttft', slowPerf.ttft!)).toBe('TTFT: im üblichen Rahmen (Median 800 ms)');
    expect(explainMetric('ttft', { verdict: 'unbekannt', class: 'i0', samples: 7, median_ms: null, p90_ms: null, value_ms: 1 })).toBe(
      'TTFT: noch zu wenig Vergleichswerte (7 von 20)',
    );
  });

  it('ranks the dot running > error > slow > ok', () => {
    expect(traceDot(trace({ status: 0, perf: slowPerf })).state).toBe('busy');
    expect(traceDot(trace({ status: 500, perf: slowPerf })).state).toBe('error');
    expect(traceDot(trace({ perf: slowPerf })).state).toBe('slow');
    expect(traceDot(trace()).state).toBe('ok');
  });

  it('forms the tokens/s cell per verdict', () => {
    const t = (o: Partial<TPSJudgement>) => trace({ output_tps: 48.2, tps: tps(o) });
    expect(tpsCell(t({}))).toMatchObject({ text: '48', sub: '−22 %', tone: 'slower' });
    expect(tpsCell(t({}))).toHaveProperty('title', '48,2 t/s · üblich 62,0 (p10 51,3 – p90 70,1), 87 Vergleichswerte, Klasse 32k–128k');
    expect(tpsCell(t({ verdict: 'schneller', deviation_permille: 250 })).tone).toBe('faster');
    expect(tpsCell(t({ verdict: 'üblich', deviation_permille: 0 }))).toMatchObject({ tone: 'usual', sub: '±0 %' });
    expect(tpsCell(t({ verdict: 'unbekannt' }))).toMatchObject({ text: '48', sub: '', tone: 'plain' });
    expect(tpsCell(trace({ status: 0, output_tps: 48.2 })).text).toBe('–');
    expect(tpsCell(trace({ stream: false, output_tps: 48.2 })).text).toBe('–');
    expect(formatDeviation(null)).toBe('');
  });

  it('forms insight sentences', () => {
    expect(insightSentence({ kind: 'cache_ttft', input: 'i2', from: 'c0', to: 'c2', delta_permille: -410, samples_from: 5, samples_to: 9 })).toBe(
      'Warmer Cache (≥ 90 %) senkt TTFT bei 32k–128k Eingabe im Median um 41 %.',
    );
    expect(insightSentence({ kind: 'input_tps', input: '', from: 'i0', to: 'i3', delta_permille: -180, samples_from: 5, samples_to: 5 })).toBe(
      'Bei ≥ 128k Eingabe generiert das Modell 18 % langsamer als bei < 8k.',
    );
  });
});

describe('Live performance', () => {
  it('shows the slow dot, the t/s column and the detail rows', async () => {
    location.hash = '#/live';
    signIn();
    mockFetch({
      'GET /api/traces': () =>
        json({ traces: [trace({ ttft_ms: 900, output_tps: 48.2, tollgate_ms: 1.5, perf: slowPerf, tps: tps() })], next_before: '', next_before_id: '' }),
      'GET /api/traces/events': () => sseResponse().response,
      'GET /api/traces/t1': () => json({ trace: trace(), stages: [] }),
    });
    render(App);
    const dot = await screen.findByRole('img', { name: /^langsam: / });
    expect(dot).toHaveClass('slow');
    const cell = screen.getByTestId('tps');
    expect(cell).toHaveTextContent('48−22 %');
    expect(cell).toHaveClass('slower');
    await fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText(/Gesamtdauer: erwartet ~4,2 s, gebraucht 9,8 s/)).toBeInTheDocument();
    expect(screen.getByText('900 ms')).toBeInTheDocument();
  });

  it('says there are too few samples without a verdict', async () => {
    location.hash = '#/live';
    signIn();
    mockFetch({
      'GET /api/traces': () => json({ traces: [trace()], next_before: '', next_before_id: '' }),
      'GET /api/traces/events': () => sseResponse().response,
      'GET /api/traces/t1': () => json({ trace: trace(), stages: [] }),
    });
    render(App);
    await fireEvent.click(await screen.findByRole('button', { expanded: false }));
    expect(screen.getByText('noch zu wenig Vergleichswerte')).toBeInTheDocument();
    expect(screen.getByTestId('tps')).toHaveTextContent('–');
  });
});

const cells = (): InfluenceCell[] =>
  ['i0', 'i1', 'i2', 'i3'].flatMap((input) =>
    ['c0', 'c1', 'c2'].map((cache) => ({ input, cache, samples: 0, ttft_median_us: null, duration_median_us: null, tps_median: null })),
  );

const model = (o: Partial<ModelPerf> = {}): ModelPerf => ({
  model: 'claude-opus-5-5', requests: 40, slow: 2, judged: 20,
  ttft: { samples: 40, min_us: 300_000, median_us: 800_000, p90_us: 1_500_000, max_us: 3_000_000 },
  duration: { samples: 40, min_us: 1_000_000, median_us: 4_200_000, p90_us: 6_100_000, max_us: 9_800_000 },
  tollgate: { samples: 40, min_us: 100, median_us: 1_500, p90_us: 3_000, max_us: 9_000 },
  output_tps: { samples: 30, min: 20, median: 62, p90: 70.1, max: 80 },
  days: [
    { day: '2026-10-04', requests: 20, slow: 0, ttft_median_us: 700_000, duration_median_us: 4e6, tps_median_milli: 60_000, cache_share_permille_median: 950 },
    { day: '2026-10-05', requests: 20, slow: 2, ttft_median_us: 900_000, duration_median_us: 5e6, tps_median_milli: 50_000, cache_share_permille_median: 400 },
  ],
  hours: [
    { hour: '2026-10-05T08:00:00Z', samples: 12, tps_median_milli: 60_000 },
    { hour: '2026-10-05T10:00:00Z', samples: 8, tps_median_milli: 50_000 },
  ],
  influence: {
    cells: cells().map((c) => (c.input === 'i0' && c.cache === 'c2' ? { ...c, samples: 9, ttft_median_us: 600_000, duration_median_us: 3e6, tps_median: 61.5 } : c.input === 'i1' && c.cache === 'c0' ? { ...c, samples: 3 } : c)),
    insights: [{ kind: 'cache_ttft', input: 'i0', from: 'c0', to: 'c2', delta_permille: -400, samples_from: 6, samples_to: 9 }],
  },
  ...o,
});

const perfReport = (o: Partial<PerfReport> = {}): PerfReport => ({
  from: '2026-09-29', to: '2026-10-05', models: [model()], redactor: null, speedtests: [], fastest_fill: null,
  capacity: { windows_per_week: null, fills_per_day: null, measured_hours: 0, sample_windows: 0, window_cost_nano_usd: null, sample_accounts: 0, accounts: 0, free_this_week: null }, ...o,
});

// Since #121 performance lives on the page Kosten & Performance, which also
// loads the cost report and the account names.
function perfPage(report: () => PerfReport, extra: Record<string, Handler> = {}) {
  return mockFetch({
    'GET /api/performance': () => json(report()),
    'GET /api/costs': () => json({ from: '2026-09-29', to: '2026-10-05', rows: [], hours: [] }),
    'GET /api/accounts': () => json({ accounts: [] }),
    ...extra,
  });
}

describe('Performance', () => {
  it('is part of Kosten & Performance; the old address redirects', async () => {
    location.hash = '#/performance';
    signIn();
    perfPage(() => perfReport());
    render(App);
    expect(await screen.findByRole('heading', { level: 1, name: 'Kosten & Performance' })).toBeInTheDocument();
    expect(location.hash).toBe('#/costs');
    expect(await screen.findByRole('table', { name: 'Modelle' })).toBeInTheDocument();
    const links = screen.getAllByRole('link').map((a) => a.textContent?.trim());
    expect(links).not.toContain('Performance');
  });

  it('shows TTFT median and the slow share per model in the dashboard', async () => {
    location.hash = '#/costs';
    signIn();
    perfPage(() => perfReport());
    render(App);
    const list = await screen.findByRole('list', { name: 'Performance je Modell' });
    const row = within(list).getByText('claude-opus-5-5').closest('li')!;
    expect(row).toHaveTextContent('TTFT 800 ms');
    expect(row).toHaveTextContent('langsam 10,0 %');
  });

  it('shows min/median/p90/max per model and unfolds the matrix', async () => {
    location.hash = '#/performance';
    signIn();
    perfPage(() => perfReport());
    render(App);
    const table = await screen.findByRole('table', { name: 'Modelle' });
    const row = within(table).getByText('claude-opus-5-5').closest('tr')!;
    expect(row).toHaveTextContent('300 ms / 800 ms / 1,5 s / 3,0 s');
    expect(row).toHaveTextContent('1,0 s / 4,2 s / 6,1 s / 9,8 s');
    expect(row).toHaveTextContent('62,0 / 70,1');
    expect(row).toHaveTextContent('10,0 %');
    await fireEvent.click(within(row).getByRole('button'));
    expect(screen.getByText('Warmer Cache (≥ 90 %) senkt TTFT bei < 8k Eingabe im Median um 40 %.')).toBeInTheDocument();
    expect(screen.getByTestId('cell-i0-c2')).toHaveTextContent(/600 ms.*61,5 t\/s.*9×/);
    expect(screen.getByTestId('cell-i1-c0')).toHaveTextContent('3×');
    expect(screen.getByTestId('cell-i3-c2')).toHaveTextContent('–');
    expect(screen.getByRole('img', { name: 'Tokens/s je Tag' })).toBeInTheDocument();
  });

  it('runs the speedtest only after confirmation', async () => {
    location.hash = '#/performance';
    signIn();
    const f = perfPage(() => perfReport(), {
      'POST /api/speedtest': () =>
        json({
          results: [
            { model: 'claude-opus-5-5', redact: false, trace_id: 'a', started_at: '', status: 200, ttft_us: 700_000, duration_us: 3_000_000, output_tps: 80.5, error: '', usual_ttft_median_us: 800_000, usual_duration_median_us: 4_200_000 },
            { model: 'claude-opus-5-5', redact: true, trace_id: '', started_at: '', status: 0, ttft_us: null, duration_us: null, output_tps: null, error: 'Redact-Stufe aus', usual_ttft_median_us: null, usual_duration_median_us: null },
          ],
        }),
    });
    render(App);
    await fireEvent.click(await screen.findByRole('checkbox'));
    await fireEvent.click(screen.getByRole('button', { name: 'Speedtest starten' }));
    expect(f.mock.calls.some((c) => (c[1] as RequestInit | undefined)?.method === 'POST')).toBe(false);
    expect(screen.getByRole('alertdialog')).toHaveTextContent('verbraucht Abo-Kontingent (2 Anfragen)');
    await fireEvent.click(screen.getByRole('button', { name: 'Ja, testen' }));
    const results = await screen.findByRole('table', { name: 'Speedtest-Ergebnisse' });
    expect(results).toHaveTextContent('80,5');
    expect(results).toHaveTextContent('Redact-Stufe aus');
    const post = f.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === 'POST')!;
    expect(JSON.parse(String((post[1] as RequestInit).body))).toEqual({ models: ['claude-opus-5-5'], confirm: true });
  });
});

describe('Performance details', () => {
  it('reloads with the chosen period', async () => {
    location.hash = '#/performance';
    signIn();
    const f = perfPage(() => perfReport());
    render(App);
    await screen.findByRole('table', { name: 'Modelle' });
    await fireEvent.click(screen.getByRole('button', { name: '30 Tage' }));
    await vi.waitFor(() => expect(f.mock.calls.some((c) => String(c[0]).includes('days=30'))).toBe(true));
    expect(screen.getByRole('button', { name: '30 Tage' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('shows the redactor card or its empty state', async () => {
    location.hash = '#/performance';
    signIn();
    perfPage(() => perfReport());
    const { unmount } = render(App);
    expect(await screen.findByText('Keine Anfragen mit Redact-Stufe im Zeitraum.')).toBeInTheDocument();
    unmount();
    const redactor = {
      requests: 12, stage: { samples: 12, min_us: 100, median_us: 400, p90_us: 900, max_us: 2_000 }, return: null,
      share_median_permille: 3, bytes_in_median: 20_000, matches_median: 2, matches_total: 31,
      return_replacements_total: 7, tokens_added_total: 40, tokens_added_median: 3,
    };
    perfPage(() => perfReport({ redactor }));
    render(App);
    expect(await screen.findByText(/Der Redactor ändert den Prompt-Cache nicht/)).toBeInTheDocument();
    expect(screen.getByText('31 (Median 2)')).toBeInTheDocument();
  });

  it('sends nothing when the speedtest is cancelled', async () => {
    location.hash = '#/performance';
    signIn();
    const f = perfPage(() => perfReport());
    render(App);
    await fireEvent.click(await screen.findByRole('checkbox'));
    await fireEvent.click(screen.getByRole('button', { name: 'Speedtest starten' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    expect(f.mock.calls.some((c) => (c[1] as RequestInit | undefined)?.method === 'POST')).toBe(false);
  });
});

describe('Home performance', () => {
  it('shows the slow dot in the latest requests', async () => {
    location.hash = '#/home';
    signIn();
    mockFetch({
      'GET /api/status': () => json(okStatus()),
      'GET /api/costs': () => json({ from: '2026-10-05', to: '2026-10-05', rows: [costRow()] }),
      'GET /api/traces': () => json({ traces: [trace({ perf: slowPerf })], next_before: '', next_before_id: '' }),
      'GET /api/redactions': () => json({ redactions: [] }),
      'GET /api/clients': () => json({ clients: [] }),
    });
    render(App);
    expect(await screen.findByRole('img', { name: /^langsam: / })).toHaveClass('slow');
  });
});

describe('Tokens/s per hour', () => {
  it('spans the hours from the first value up to now and leaves gaps', () => {
    const series = tpsByHour(perfReport(), Date.parse('2026-10-05T11:40:00Z'));
    expect(series.hours).toEqual(['2026-10-05T08:00:00Z', '2026-10-05T09:00:00Z', '2026-10-05T10:00:00Z', '2026-10-05T11:00:00Z']);
    expect(series.lines).toEqual([{ model: 'claude-opus-5-5', values: [60, null, 50, null], last: 50, days: new Map([['2026-10-04', 60], ['2026-10-05', 50]]) }]);
    expect(tpsByHour(perfReport({ models: [model({ hours: [] })] })).lines).toEqual([]);
  });

  it('draws one line per model with a legend', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(Date.parse('2026-10-05T12:10:00Z'));
    location.hash = '#/performance';
    signIn();
    const other = model({ model: 'claude-sonnet-5', requests: 10, hours: [{ hour: '2026-10-05T09:00:00Z', samples: 10, tps_median_milli: 90_500 }] });
    perfPage(() => perfReport({ from: '2026-09-29', models: [model(), other] }));
    render(App);
    const chart = await screen.findByRole('figure', { name: 'Tokens pro Sekunde · Median je Stunde (UTC)' });
    expect(within(chart).getAllByTestId('tps-line')).toHaveLength(2);
    const legend = within(chart).getByRole('list');
    expect(legend).toHaveTextContent('claude-opus-5-5');
    expect(legend).toHaveTextContent('50,0 t/s');
    expect(legend).toHaveTextContent('claude-sonnet-5');
    expect(legend).toHaveTextContent('90,5 t/s');
    expect(chart).toHaveTextContent('08:00');
    expect(chart).toHaveTextContent('12:00');
  });
});
