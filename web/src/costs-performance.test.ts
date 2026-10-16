// Kosten & Performance on one page (#121).
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within, waitFor } from '@testing-library/svelte';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import { costRow, json, mockFetch } from './test-helpers';
import { breakdown, costSeries, metrics, withShares, aggregate } from './lib/costs';
import { hourAxis, hourTicks } from './lib/hours';
import type { CostHour, CostReport, PerfReport } from './lib/types/dashboard';
import type { Account } from './lib/types/accounts';

function signIn() {
  auth.token = 'tok';
  localStorage.setItem('tollgate.admin_token', 'tok');
}

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

// The charts end at the current hour; pin it to the fixture data.
function at(iso: string) {
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(Date.parse(iso));
}

const hour = (o: Partial<CostHour> = {}): CostHour => ({
  hour: '2026-10-01T01:00:00Z', account_id: 'acc-a', model: 'claude-opus-5-5', requests: 1,
  usage: { input: 1, output: 1, cache_read: 1, cache_creation: 1 }, cost_nano_usd: 1_000_000_000, ...o,
});

const account = (o: Partial<Account> = {}): Account => ({
  id: 'acc-a', name: 'Hauptabo', email: '', active: true, status: 'active', expires_at: '', last_refresh: '', updated_at: '', ...o,
});

const emptyPerf = (o: Partial<PerfReport> = {}): PerfReport => ({ from: '2026-09-25', to: '2026-10-01', models: [], redactor: null, speedtests: [], fastest_fill: null,
  capacity: { windows_per_week: null, fills_per_day: null, measured_hours: 0, sample_windows: 0, window_cost_nano_usd: null, sample_accounts: 0, accounts: 0, free_this_week: null }, ...o });

function page(cost: CostReport, accounts: Account[] = []) {
  return mockFetch({
    'GET /api/costs': () => json(cost),
    'GET /api/performance': () => json(emptyPerf()),
    'GET /api/accounts': () => json({ accounts }),
  });
}

describe('navigation', () => {
  it('redirects #/performance to #/costs and shows the merged page', async () => {
    location.hash = '#/performance';
    signIn();
    page({ from: '2026-09-25', to: '2026-10-01', rows: [], hours: [] });
    render(App);
    expect(await screen.findByRole('heading', { level: 1, name: 'Kosten & Performance' })).toBeInTheDocument();
    expect(location.hash).toBe('#/costs');
    expect(screen.getByRole('link', { name: 'Kosten & Performance' })).toHaveAttribute('aria-current', 'page');
    expect(screen.queryByRole('link', { name: 'Performance' })).not.toBeInTheDocument();
  });

  it('opens with key 3 and defaults to 7 days', async () => {
    location.hash = '#/home';
    signIn();
    const f = mockFetch({
      'GET /api/costs': () => json({ from: '2026-09-25', to: '2026-10-01', rows: [], hours: [] }),
      'GET /api/performance': () => json(emptyPerf()),
      'GET /api/accounts': () => json({ accounts: [] }),
      'GET /api/status': () => json({ error: 'x' }, 500),
      'GET /api/traces': () => json({ error: 'x' }, 500),
      'GET /api/redactions': () => json({ error: 'x' }, 500),
      'GET /api/clients': () => json({ clients: [] }),
    });
    render(App);
    await fireEvent.keyDown(window, { key: '3' });
    expect(location.hash).toBe('#/costs');
    expect(await screen.findByRole('heading', { level: 1, name: 'Kosten & Performance' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '7 Tage' })).toHaveAttribute('aria-pressed', 'true');
    await waitFor(() => expect(f.mock.calls.map((c) => String(c[0]))).toEqual(expect.arrayContaining(['/api/costs?days=7', '/api/performance?days=7'])));
    await fireEvent.click(screen.getByRole('button', { name: 'Heute' }));
    await waitFor(() => expect(f.mock.calls.map((c) => String(c[0]))).toEqual(expect.arrayContaining(['/api/costs?days=1', '/api/performance?days=1'])));
  });
});

describe('cost helpers', () => {
  it('computes shares of the known cost', () => {
    const rows = [
      costRow({ account_id: 'acc-a', cost_nano_usd: 3_000_000_000 }),
      costRow({ account_id: 'acc-b', cost_nano_usd: 1_000_000_000 }),
      costRow({ account_id: 'acc-b', model: 'mystery', cost_nano_usd: null }),
    ];
    const list = withShares(aggregate(rows, (r) => r.account_id ?? ''));
    expect(list.map((a) => [a.key, a.share])).toEqual([['acc-a', 0.75], ['acc-b', 0.25]]);
    expect(withShares(aggregate([costRow({ cost_nano_usd: null })], () => 'x'))[0].share).toBeNull();
  });

  it('spans the hours from the first cost up to now, one series per account', () => {
    const report: CostReport = {
      from: '2026-10-01', to: '2026-10-01', rows: [],
      hours: [
        hour(),
        hour({ hour: '2026-10-01T03:00:00Z', account_id: 'acc-b', cost_nano_usd: 500_000_000 }),
        hour({ hour: '2026-10-01T03:00:00Z', model: 'mystery', cost_nano_usd: null }),
      ],
    };
    const s = costSeries(report, ['acc-a', 'acc-b'], Date.parse('2026-10-01T04:30:00Z'));
    expect(s.buckets).toEqual(['2026-10-01T01:00:00Z', '2026-10-01T02:00:00Z', '2026-10-01T03:00:00Z', '2026-10-01T04:00:00Z']);
    expect(s.series).toEqual([
      { key: 'acc-a', values: [1_000_000_000, 0, 0, 0] },
      { key: 'acc-b', values: [0, 0, 500_000_000, 0] },
    ]);
    expect(costSeries({ ...report, hours: [] }, ['acc-a']).buckets).toEqual([]);
  });

  it('keeps at most the last 7 days', () => {
    const report: CostReport = { from: '2026-09-02', to: '2026-10-01', rows: [], hours: [hour({ hour: '2026-09-20T05:00:00Z' }), hour()] };
    const s = costSeries(report, ['acc-a'], Date.parse('2026-10-01T23:10:00Z'));
    expect(s.buckets).toHaveLength(168);
    expect(s.buckets[0]).toBe('2026-09-25T00:00:00Z');
    expect(s.series[0].values.reduce((a, v) => a + v, 0)).toBe(1_000_000_000);
  });

  it('labels short spans by hour and long spans by date', () => {
    expect(hourTicks(hourAxis(['2026-10-01T09:00:00Z'], Date.parse('2026-10-01T13:20:00Z'))).map((t) => t.text))
      .toEqual(['09:00', '10:00', '11:00', '12:00', '13:00']);
    expect(hourTicks(hourAxis(['2026-10-01T20:00:00Z'], Date.parse('2026-10-02T05:00:00Z'))).map((t) => t.text))
      .toEqual(['20:00', '22:00', '02.10.', '02:00', '04:00']);
    expect(hourTicks(hourAxis(['2026-09-28T07:00:00Z'], Date.parse('2026-10-01T05:00:00Z'))).map((t) => t.text))
      .toEqual(['29.09.', '30.09.', '01.10.']);
  });

  it('forms the key figures', () => {
    const m = metrics([
      costRow({ usage: { input: 100, output: 50, cache_read: 800, cache_creation: 10 }, cache_savings_nano_usd: 3_040_000, saved_tokens: 40 }),
      costRow({ usage: { input: 0, output: 0, cache_read: 0, cache_creation: 0 }, cache_savings_nano_usd: null, saved_tokens: 0 }),
    ]);
    expect(m).toEqual({ processed: 960, cache_read: 800, uncached_input: 110, output: 50, cache_savings_nano_usd: 3_040_000, cache_savings_partial: true, saved_tokens: 40 });
  });

  it('breaks down by day, newest first, only days with requests', () => {
    const rows = [costRow({ day: '2026-09-27' }), costRow({ day: '2026-10-01' }), costRow({ day: '2026-10-01', model: 'x' })];
    expect(breakdown(rows, 'day', () => '').map((a) => a.key)).toEqual(['2026-10-01', '2026-09-27']);
    expect(breakdown(rows, 'model', () => '').map((a) => a.key)).toEqual(['claude-opus-5-5', 'x']);
  });
});

describe('dashboard', () => {
  beforeEach(() => at('2026-10-01T12:00:00Z'));
  const report = (): CostReport => ({
    from: '2026-09-25',
    to: '2026-10-01',
    rows: [
      costRow({ day: '2026-09-27', account_id: 'acc-a', client_id: 'c1', client_name: 'laptop', cost_nano_usd: 2_000_000_000, usage: { input: 100, output: 50, cache_read: 800, cache_creation: 10 }, cache_savings_nano_usd: 3_040_000, saved_tokens: 40 }),
      costRow({ day: '2026-10-01', account_id: 'acc-a', client_id: 'c1', client_name: 'laptop', cost_nano_usd: 1_000_000_000, usage: { input: 0, output: 0, cache_read: 0, cache_creation: 0 }, cache_savings_nano_usd: 0, saved_tokens: 0 }),
      costRow({ day: '2026-10-01', account_id: 'acc-b', client_id: 'c2', client_name: 'server', model: 'claude-sonnet-5', cost_nano_usd: 1_000_000_000, usage: { input: 0, output: 0, cache_read: 0, cache_creation: 0 }, cache_savings_nano_usd: 0, saved_tokens: 0 }),
    ],
    hours: [
      hour({ hour: '2026-09-27T01:00:00Z', cost_nano_usd: 2_000_000_000 }),
      hour({ hour: '2026-10-01T03:00:00Z', cost_nano_usd: 1_000_000_000 }),
      hour({ hour: '2026-10-01T03:00:00Z', account_id: 'acc-b', model: 'claude-sonnet-5', cost_nano_usd: 1_000_000_000 }),
    ],
  });

  it('shows the total, shares per account with account names and the cost curve', async () => {
    location.hash = '#/costs';
    signIn();
    page(report(), [account()]);
    render(App);
    const list = await screen.findByRole('list', { name: 'Kosten je Account' });
    const total = screen.getByRole('region', { name: 'Gesamtkosten' });
    expect(within(total).getByText('$4.00')).toHaveClass('big');
    expect(total).toHaveTextContent('6 Anfragen');
    const a = await within(list).findByText('Hauptabo');
    expect(a.closest('li')).toHaveTextContent(/\$3\.00.*75,0 %.*960 Tokens/);
    expect(within(list).getByText('Account acc-b').closest('li')).toHaveTextContent('25,0 %');
    const chart = screen.getByRole('figure', { name: 'Kosten pro Stunde (UTC)' });
    expect(within(chart).getAllByTestId('cost-line')).toHaveLength(2);
    expect(within(chart).getByRole('img')).toHaveAccessibleName(/Hauptabo \$3\.00.*Account acc-b \$1\.00/);
  });

  it('draws the last 7 days by hour for 30 days', async () => {
    location.hash = '#/costs';
    signIn();
    page({ ...report(), from: '2026-09-02' });
    render(App);
    await fireEvent.click(await screen.findByRole('button', { name: '30 Tage' }));
    expect(await screen.findByRole('figure', { name: 'Kosten pro Stunde (UTC) · letzte 7 Tage' })).toBeInTheDocument();
  });

  it('says so when there are no costs', async () => {
    location.hash = '#/costs';
    signIn();
    page({ ...report(), rows: [], hours: [] });
    render(App);
    expect(await screen.findByText('Keine Kosten im Zeitraum.')).toBeInTheDocument();
    expect(screen.queryByRole('figure', { name: /Kosten pro Stunde/ })).toBeNull();
  });

  it('shows the key figures', async () => {
    location.hash = '#/costs';
    signIn();
    page(report());
    render(App);
    const dl = await screen.findByLabelText('Kennzahlen');
    const value = (label: string) => within(dl).getByText(label).nextElementSibling?.textContent;
    expect(value('Tokens verarbeitet')).toBe('960');
    expect(within(dl).getByText('Ausgabe 50 · ungecacht 110')).toHaveClass('small');
    expect(within(dl).getByText(/^800 gelesen · Treffer /)).toHaveClass('small');
    expect(value('Cache-Ersparnis')).toBe('$0.0030');
    expect(value('Von tollgate gespart')).toBe('40');
  });

  it('switches the breakdown and hides days without value', async () => {
    location.hash = '#/costs';
    signIn();
    page(report(), [account()]);
    render(App);
    const table = await screen.findByRole('table', { name: 'Aufschlüsselung' });
    const group = screen.getByRole('group', { name: 'Aufschlüsselung nach' });
    expect(within(group).getByRole('button', { name: 'Account' })).toHaveAttribute('aria-pressed', 'true');
    await waitFor(() => expect(within(table).getByText('Hauptabo')).toBeInTheDocument());
    const bodyRows = () => within(table).getAllByRole('row').slice(1);
    expect(bodyRows()).toHaveLength(2);

    await fireEvent.click(within(group).getByRole('button', { name: 'Modell' }));
    expect(within(group).getByRole('button', { name: 'Modell' })).toHaveAttribute('aria-pressed', 'true');
    expect(bodyRows().map((r) => (r as HTMLTableRowElement).cells[0].textContent)).toEqual(['claude-opus-5-5', 'claude-sonnet-5']);

    await fireEvent.click(within(group).getByRole('button', { name: 'Client' }));
    expect(bodyRows().map((r) => (r as HTMLTableRowElement).cells[0].textContent)).toEqual(['laptop', 'server']);

    await fireEvent.click(within(group).getByRole('button', { name: 'Tag' }));
    // 7 days, but only two with requests; newest first.
    expect(bodyRows().map((r) => (r as HTMLTableRowElement).cells[0].textContent)).toEqual(['2026-10-01', '2026-09-27']);
  });
});

describe('key figure tiles (#134)', () => {
  const report: CostReport = { from: '2026-09-25', to: '2026-10-01', rows: [costRow({ usage: { input: 100, output: 50, cache_read: 300, cache_creation: 50 } })], hours: [] };
  function show(perf: PerfReport) {
    location.hash = '#/costs';
    signIn();
    mockFetch({
      'GET /api/costs': () => json(report),
      'GET /api/performance': () => json(perf),
      'GET /api/accounts': () => json({ accounts: [account()] }),
    });
    render(App);
  }

  it('shows the fastest 5-hour window with requests per minute', async () => {
    show(emptyPerf({ fastest_fill: { account_id: 'acc-a', reset: '2026-10-01T17:00:00Z', from_permille: 0, to_permille: 1000,
      observed_us: 100 * 60e6, full_us: 100 * 60e6, requests: 250, per_minute: 2.5 } }));
    const tiles = await screen.findByLabelText('Kennzahlen');
    await waitFor(() => expect(within(tiles).getByText(/1 h 40 min/, { selector: 'dd:not(.small)' })).toBeInTheDocument());
    expect(within(tiles).getByText('0 → 100 % in 1 h 40 min · Ø 2,5 Anfragen/min · Hauptabo')).toBeInTheDocument();
    expect(within(tiles).getByText('Ausgabe 50 · ungecacht 150')).toBeInTheDocument();
  });

  it('shows a dash before the first measured window', async () => {
    show(emptyPerf());
    const tiles = await screen.findByLabelText('Kennzahlen');
    expect(await within(tiles).findByText('Noch kein Fenster mit 30 Punkten Anstieg gemessen')).toBeInTheDocument();
  });
});

describe('Einschätzung (#136)', () => {
  const report: CostReport = { from: '2026-09-25', to: '2026-10-01', rows: [], hours: [] };
  function show(perf: PerfReport) {
    location.hash = '#/costs';
    signIn();
    mockFetch({
      'GET /api/costs': () => json(report),
      'GET /api/performance': () => json(perf),
      'GET /api/accounts': () => json({ accounts: [] }),
    });
    render(App);
  }

  it('shows need against capacity and that four accounts are not enough', async () => {
    show(emptyPerf({ capacity: { windows_per_week: 8, fills_per_day: 5, measured_hours: 48, sample_windows: 3,
      window_cost_nano_usd: 100e9, sample_accounts: 2, accounts: 4, free_this_week: 6.5 } }));
    const card = await screen.findByRole('region', { name: 'Einschätzung' });
    await waitFor(() => expect(within(card).getByText('4 Accounts reichen nicht')).toBeInTheDocument());
    expect(within(card).getByRole('meter', { name: 'Bedarf an der Kapazität' })).toHaveAttribute('aria-valuenow', '100');
    expect(within(card).getByText(/35 von 32 5-h-Fenstern pro Woche/)).toBeInTheDocument();
    expect(within(card).getByText(/diese Woche noch 6,5 frei/)).toBeInTheDocument();
    expect(within(card).getByText(/aus 2 Wochen · 1 Fenster ≈ \$100\.00/)).toBeInTheDocument();
    expect(within(card).getByLabelText('4 Accounts reichen nicht')).toBeInTheDocument();
  });

  it('shows a hint instead of the bar while measurements are missing', async () => {
    show(emptyPerf());
    const card = await screen.findByRole('region', { name: 'Einschätzung' });
    expect(within(card).queryByRole('meter')).not.toBeInTheDocument();
    expect(within(card).getByText(/Noch kein 5-h-Fenster mit 30 Punkten Anstieg gemessen/)).toBeInTheDocument();
  });
});
