// Day values above the hourly charts and both charts on the overview (#123).
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within, waitFor } from '@testing-library/svelte';
import App from './App.svelte';
import CostChart from './lib/components/CostChart.svelte';
import { auth, logout } from './lib/auth.svelte';
import { costRow, json, mockFetch, okStatus, type Handler } from './test-helpers';
import { costSeries, dayCosts } from './lib/costs';
import { dayBands, daySpans, hourAxis } from './lib/hours';
import { tpsByHour } from './lib/perf';
import type { CostHour, CostReport, ModelPerf, PerfReport } from './lib/types/dashboard';

function at(iso: string) {
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(Date.parse(iso));
}

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

const hour = (o: Partial<CostHour> = {}): CostHour => ({
  hour: '2026-10-04T10:00:00Z', account_id: 'acc-a', model: 'claude-opus-5-5', requests: 1,
  usage: { input: 1, output: 1, cache_read: 1, cache_creation: 1 }, cost_nano_usd: 1_000_000_000, ...o,
});

const costs = (): CostReport => ({
  from: '2026-09-29',
  to: '2026-10-05',
  rows: [
    costRow({ day: '2026-10-04', account_id: 'acc-a', cost_nano_usd: 1_500_000_000 }),
    costRow({ day: '2026-10-04', account_id: 'acc-a', client_id: 'c2', cost_nano_usd: 1_000_000_000 }),
    costRow({ day: '2026-10-05', account_id: 'acc-a', cost_nano_usd: 4_000_000_000 }),
    costRow({ day: '2026-10-05', account_id: 'acc-a', model: 'unknown', cost_nano_usd: null }),
  ],
  hours: [hour(), hour({ hour: '2026-10-05T09:00:00Z', cost_nano_usd: 4_000_000_000 })],
});

const model = (o: Partial<ModelPerf> = {}): ModelPerf => ({
  model: 'claude-opus-5-5', requests: 40, slow: 0, judged: 20, ttft: null,
  duration: { samples: 40, min_us: 1, median_us: 1, p90_us: 1, max_us: 1 },
  tollgate: null, output_tps: null,
  days: [
    { day: '2026-10-04', requests: 20, slow: 0, ttft_median_us: null, duration_median_us: 1, tps_median_milli: 61_500, cache_share_permille_median: null },
    { day: '2026-10-05', requests: 20, slow: 0, ttft_median_us: null, duration_median_us: 1, tps_median_milli: 48_000, cache_share_permille_median: null },
  ],
  hours: [
    { hour: '2026-10-04T10:00:00Z', samples: 3, tps_median_milli: 60_000 },
    { hour: '2026-10-05T09:00:00Z', samples: 3, tps_median_milli: 50_000 },
  ],
  influence: { cells: [], insights: [] },
  ...o,
});

const perf = (o: Partial<PerfReport> = {}): PerfReport => ({ from: '2026-09-29', to: '2026-10-05', models: [model()], redactor: null, speedtests: [], fastest_fill: null,
  capacity: { windows_per_week: null, fills_per_day: null, measured_hours: 0, sample_windows: 0, window_cost_nano_usd: null, sample_accounts: 0, accounts: 0, free_this_week: null }, ...o });

describe('day helpers', () => {
  it('splits the hour axis into UTC days, partial days included', () => {
    expect(daySpans(['2026-10-04T22:00:00Z', '2026-10-04T23:00:00Z', '2026-10-05T00:00:00Z', '2026-10-05T01:00:00Z', '2026-10-05T02:00:00Z'])).toEqual([
      { day: '2026-10-04', from: 0, to: 1 },
      { day: '2026-10-05', from: 2, to: 4 },
    ]);
    expect(daySpans([])).toEqual([]);
  });

  it('drops labels that would overlap, newest first, and separates the days', () => {
    // 2026-09-28T23 alone, then six full days and today until noon: 158 hours.
    const axis = hourAxis(['2026-09-28T23:00:00Z'], Date.parse('2026-10-05T12:00:00Z'));
    const x = (i: number) => 52 + (i / (axis.length - 1)) * 580;
    const bands = dayBands(axis, x, () => 40);
    expect(bands.map((b) => b.day)).toEqual(['2026-09-28', '2026-09-29', '2026-09-30', '2026-10-01', '2026-10-02', '2026-10-03', '2026-10-04', '2026-10-05']);
    expect(bands[0]).toMatchObject({ sep: null, at: null });
    expect(bands.slice(1).every((b) => b.sep !== null && b.at !== null)).toBe(true);
    // Today ends at the last hour; its label stays inside the plot.
    expect(bands[7].at! + 20).toBeLessThanOrEqual(x(axis.length - 1));
    expect(dayBands([], x, () => 40)).toEqual([]);
  });

  it('sums the cost per day, unpriced rows as lower bound', () => {
    const d = dayCosts(costs().rows);
    expect(d.get('2026-10-04')).toBe('$2.50');
    expect(d.get('2026-10-05')).toBe('≥ $4.00');
  });

  it('carries the day median of tokens/s per model', () => {
    at('2026-10-05T12:00:00Z');
    const { lines } = tpsByHour(perf({ models: [model(), model({ model: 'claude-haiku-4-5', days: [{ ...model().days[0], tps_median_milli: null }] })] }));
    expect(lines.map((l) => [l.model, [...l.days]])).toEqual([
      ['claude-opus-5-5', [['2026-10-04', 61.5], ['2026-10-05', 48]]],
      ['claude-haiku-4-5', []],
    ]);
  });
});

describe('CostChart day sums', () => {
  it('writes the sum of each day above its hours', () => {
    at('2026-10-05T12:00:00Z');
    const r = costs();
    render(CostChart, { data: costSeries(r, ['acc-a']), labels: ['Hauptabo'], label: 'Kosten', days: dayCosts(r.rows) });
    expect(screen.getAllByTestId('day-sum').map((t) => t.textContent)).toEqual(['$2.50', '≥ $4.00']);
  });

  it('leaves the day sums out without day values', () => {
    at('2026-10-05T12:00:00Z');
    render(CostChart, { data: costSeries(costs(), ['acc-a']), labels: ['Hauptabo'], label: 'Kosten' });
    expect(screen.queryAllByTestId('day-sum')).toEqual([]);
  });
});

describe('pages', () => {
  beforeEach(() => at('2026-10-05T12:00:00Z'));

  function routes(over: Record<string, Handler> = {}) {
    auth.token = 'tok';
    return mockFetch({
      'GET /api/costs': () => json(costs()),
      'GET /api/performance': () => json(perf()),
      'GET /api/accounts': () => json({ accounts: [] }),
      'GET /api/status': () => json(okStatus()),
      'GET /api/traces': () => json({ traces: [], next_before: '', next_before_id: '' }),
      'GET /api/redactions': () => json({ redactions: [] }),
      'GET /api/clients': () => json({ clients: [] }),
      ...over,
    });
  }

  it('shows day sums and day medians on Kosten & Performance', async () => {
    location.hash = '#/costs';
    routes();
    render(App);
    const cost = await screen.findByRole('figure', { name: 'Kosten pro Stunde (UTC)' });
    expect(within(cost).getAllByTestId('day-sum').map((t) => t.textContent)).toEqual(['$2.50', '≥ $4.00']);
    const tps = await screen.findByRole('figure', { name: /Tokens pro Sekunde/ });
    expect(within(tps).getAllByTestId('day-tps').map((t) => t.textContent)).toEqual(['61,5', '48,0']);
  });

  it('shows both charts of the last 7 days below the subscriptions on the overview', async () => {
    location.hash = '#/home';
    const f = routes();
    render(App);
    const trends = await screen.findByRole('region', { name: 'Verlauf' });
    const cost = await within(trends).findByRole('figure', { name: 'Kosten pro Stunde (UTC)' });
    expect(within(cost).getAllByTestId('day-sum').map((t) => t.textContent)).toEqual(['$2.50', '≥ $4.00']);
    expect(await within(trends).findByRole('figure', { name: /Tokens pro Sekunde/ })).toBeInTheDocument();
    const calls = f.mock.calls.map((c) => String(c[0]));
    expect(calls).toEqual(expect.arrayContaining(['/api/costs?days=7', '/api/performance?days=7']));
    // Below the subscriptions, above today.
    const limits = screen.getByRole('region', { name: 'Abo-Limits' });
    const today = screen.getByRole('region', { name: 'Heute' });
    expect(limits.compareDocumentPosition(trends) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(trends.compareDocumentPosition(today) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('keeps the overview when the performance report fails', async () => {
    location.hash = '#/home';
    routes({ 'GET /api/performance': () => json({ error: 'kaputt' }, 500) });
    render(App);
    const trends = await screen.findByRole('region', { name: 'Verlauf' });
    expect(await within(trends).findByRole('figure', { name: 'Kosten pro Stunde (UTC)' })).toBeInTheDocument();
    await waitFor(() => expect(within(trends).getByRole('alert')).toHaveTextContent('kaputt'));
    expect(screen.getByRole('region', { name: 'Heute' })).toBeInTheDocument();
  });
});
