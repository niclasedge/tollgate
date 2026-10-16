import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import App from './App.svelte';
import AccountCard from './lib/components/AccountCard.svelte';
import { auth, logout } from './lib/auth.svelte';
import { FIVE_HOURS_MS, WEEK_MS } from './lib/limits';
import { deltaLabel, pointsLabel, weekComparison, windowCost } from './lib/windows';
import type { Account } from './lib/types/accounts';
import type { CostHour } from './lib/types/dashboard';
import { costRow, json, mockFetch, okStatus } from './test-helpers';

const NOW = Date.parse('2026-10-07T12:30:00Z'); // Wednesday
const hour = (iso: string, o: Partial<CostHour> = {}): CostHour => ({
  hour: iso,
  account_id: 'a',
  model: 'm',
  requests: 1,
  usage: { input: 0, output: 0, cache_read: 0, cache_creation: 0 },
  cost_nano_usd: 1_000_000_000,
  ...o,
});

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
});

describe('windowCost', () => {
  const hours = [
    hour('2026-10-07T08:00:00Z'), // before the 5 h window start hour
    hour('2026-10-07T09:00:00Z'), // hour the window starts in (09:45)
    hour('2026-10-07T12:00:00Z', { cost_nano_usd: 500_000_000, requests: 2 }),
    hour('2026-10-07T12:00:00Z', { account_id: 'b' }),
  ];

  it('sums the account hours from the hour the window starts in', () => {
    expect(windowCost(hours, 'a', '2026-10-07T14:45:00Z', FIVE_HOURS_MS, NOW)).toEqual({ requests: 3, cost_nano_usd: 1_500_000_000, cost_partial: false });
  });

  it('marks unpriced hours as a lower bound', () => {
    const r = windowCost([...hours, hour('2026-10-07T11:00:00Z', { cost_nano_usd: null })], 'a', '2026-10-07T14:45:00Z', FIVE_HOURS_MS, NOW);
    expect(r).toEqual({ requests: 4, cost_nano_usd: 1_500_000_000, cost_partial: true });
  });

  it('is null without a running window', () => {
    expect(windowCost(hours, 'a', undefined, FIVE_HOURS_MS, NOW)).toBeNull();
    expect(windowCost(hours, 'a', '2026-10-07T12:00:00Z', FIVE_HOURS_MS, NOW)).toBeNull();
    expect(windowCost(hours, 'a', '2026-10-20T00:00:00Z', WEEK_MS, NOW)).toBeNull();
  });

  it('covers the weekly window', () => {
    const week = [hour('2026-10-01T10:00:00Z'), hour('2026-10-01T09:00:00Z')];
    expect(windowCost(week, 'a', '2026-10-08T10:15:00Z', WEEK_MS, NOW)?.requests).toBe(1);
  });
});

describe('weekComparison', () => {
  const rows = [
    costRow({ day: '2026-09-28', cost_nano_usd: 1_000_000_000 }), // last Monday
    costRow({ day: '2026-09-30', cost_nano_usd: 2_000_000_000 }), // last Wednesday
    costRow({ day: '2026-10-02', cost_nano_usd: 4_000_000_000 }), // last Friday, after today's weekday
    costRow({ day: '2026-10-05', cost_nano_usd: 3_000_000_000 }), // Monday
    costRow({ day: '2026-10-07', cost_nano_usd: null }), // today, unpriced
  ];

  it('compares this week with last week up to the same weekday', () => {
    const w = weekComparison(rows, '2026-10-07');
    expect(w.current.cost_nano_usd).toBe(3_000_000_000);
    expect(w.current.cost_partial).toBe(true);
    expect(w.current.requests).toBe(4);
    expect(w.previous.cost_nano_usd).toBe(3_000_000_000);
    expect(w.previous_full.cost_nano_usd).toBe(7_000_000_000);
  });

  it('lists all seven weekdays, future days empty', () => {
    const w = weekComparison(rows, '2026-10-07');
    expect(w.days.map((d) => d.weekday)).toEqual(['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So']);
    expect(w.days[0]).toMatchObject({ day: '2026-10-05', prev_day: '2026-09-28' });
    expect(w.days[1].current?.requests).toBe(0);
    expect(w.days[3].current).toBeNull();
    expect(w.days[4].previous.cost_nano_usd).toBe(4_000_000_000);
  });

  it('starts the week on Monday also on Sunday', () => {
    const w = weekComparison([], '2026-10-11');
    expect(w.days[0].day).toBe('2026-10-05');
    expect(w.days.every((d) => d.current !== null)).toBe(true);
  });
});

describe('delta labels', () => {
  it('formats relative change and percentage points', () => {
    expect(deltaLabel(150, 100)).toBe('+50 %');
    expect(deltaLabel(50, 100)).toBe('−50 %');
    expect(deltaLabel(100, 100)).toBe('±0 %');
    expect(deltaLabel(5, 0)).toBeNull();
    expect(deltaLabel(null, 5)).toBeNull();
    expect(pointsLabel(0.8, 0.75)).toBe('+5,0 Pp.');
    expect(pointsLabel(0.7, 0.75)).toBe('−5,0 Pp.');
    expect(pointsLabel(null, 0.5)).toBeNull();
  });
});

describe('Übersicht: Kosten im Fenster', () => {
  const account = (o: Partial<Account> = {}): Account => ({
    id: 'a',
    name: 'a.json',
    email: 'a@example.com',
    status: 'active',
    active: true,
    expires_at: '',
    last_refresh: '',
    updated_at: '',
    ...o,
    usage: {
      five_hour: { utilization: 40, reset: new Date(Date.now() + 2 * 3_600_000).toISOString() },
      seven_day: { utilization: 20, reset: new Date(Date.now() + 2 * 86_400_000).toISOString() },
      observed_at: new Date().toISOString(),
      error: '',
    },
  });

  it('shows the cost of both windows when hours are given', () => {
    const at = new Date(Math.floor(Date.now() / 3_600_000) * 3_600_000).toISOString().replace('.000Z', 'Z');
    const old = new Date(Math.floor((Date.now() - 4 * 86_400_000) / 3_600_000) * 3_600_000).toISOString().replace('.000Z', 'Z');
    render(AccountCard, { account: account(), hours: [hour(at), hour(old, { cost_nano_usd: 2_000_000_000 })] });
    const costs = screen.getAllByTestId('window-cost');
    expect(costs[0]).toHaveTextContent('Kosten im Fenster $1.00 · 1 Anfr.');
    expect(costs[1]).toHaveTextContent('Kosten im Fenster $3.00 · 2 Anfr.');
  });

  it('shows no window cost without hours (Accounts page)', () => {
    render(AccountCard, { account: account() });
    expect(screen.queryByTestId('window-cost')).toBeNull();
  });

  it('shows this week against last week with a row per day', async () => {
    auth.token = 'tok';
    location.hash = '#/home';
    const f = mockFetch({
      'GET /api/status': () => json(okStatus()),
      'GET /api/costs': () =>
        json({
          from: '2026-09-24',
          to: '2026-10-07',
          rows: [costRow({ day: '2026-09-28', cost_nano_usd: 1_000_000_000 }), costRow({ day: '2026-10-06', cost_nano_usd: 1_500_000_000 })],
          hours: [],
        }),
      'GET /api/traces': () => json({ traces: [], next_before: '', next_before_id: '' }),
      'GET /api/redactions': () => json({ redactions: [] }),
    });
    render(App);
    const week = await screen.findByRole('region', { name: 'Woche' });
    const tile = (label: string) => within(week).getByText(label, { selector: 'dt' }).parentElement!;
    expect(await within(week).findByText('Kosten', { selector: 'dt' })).toBeInTheDocument();
    expect(tile('Kosten')).toHaveTextContent('$1.50');
    expect(tile('Kosten')).toHaveTextContent('Vorwoche $1.00 · +50 %');
    const rows = within(week).getAllByRole('row');
    expect(rows).toHaveLength(8);
    expect(rows[2]).toHaveTextContent(/^Di 06\.10\.\s*\$1\.50/);
    expect(rows[4]).toHaveTextContent(/^Do 08\.10\./);
    expect(f.mock.calls.map((c) => String(c[0]))).toContain('/api/costs?days=14');
  });
});
