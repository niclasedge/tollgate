// Manual switch on the overview once the active subscription is full (#126).
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import { isFull, switchCandidate } from './lib/subscriptions';
import type { Account, UsageSnapshot } from './lib/types/accounts';
import { json, mockFetch, okStatus } from './test-helpers';

const NOW = Date.parse('2026-10-06T12:00:00Z');
const at = (h: number, now = NOW) => new Date(now + h * 3_600_000).toISOString();
const usage = (five: number | null, week: number | null, now = NOW): UsageSnapshot => ({
  five_hour: { utilization: five, reset: at(2, now) },
  seven_day: { utilization: week, reset: at(72, now) },
  observed_at: at(0, now),
  error: '',
});
const account = (id: string, o: Partial<Account> = {}): Account => ({
  id, name: `Abo ${id}`, email: `${id}@example.com`, active: false, status: 'gespeichert', expires_at: '', last_refresh: '', updated_at: '', ...o,
});

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
});

describe('switchCandidate', () => {
  it('is the first free account once the active one is full', () => {
    const list = [
      account('a', { active: true, usage: usage(100, 50) }),
      account('weekdone', { usage: usage(0, 100) }),
      account('fivefull', { usage: usage(100, 10) }),
      account('unknown'),
      account('late', { usage: { ...usage(40, 10), five_hour: { utilization: 40, reset: at(4) } } }),
      account('soon', { usage: usage(60, 30) }),
    ];
    expect(isFull(list[0], NOW)).toBe(true);
    // lowest weekly use first (#127)
    expect(switchCandidate(list, NOW)?.id).toBe('late');
  });

  it('counts a full week as full and ignores passed windows', () => {
    expect(isFull(account('a', { usage: usage(10, 100) }), NOW)).toBe(true);
    expect(isFull(account('a', { usage: usage(100, 100) }), NOW + 80 * 3_600_000)).toBe(false);
    expect(isFull(account('a'), NOW)).toBe(false);
  });

  it('is undefined while the active account has room or nobody is free', () => {
    expect(switchCandidate([account('a', { active: true, usage: usage(99, 50) }), account('b', { usage: usage(0, 0) })], NOW)).toBeUndefined();
    expect(switchCandidate([account('a', { active: true, usage: usage(100, 50) }), account('b', { usage: usage(100, 0) })], NOW)).toBeUndefined();
  });
});

describe('Übersicht: manueller Wechsel', () => {
  function home(activeFive: number, extra: Parameters<typeof mockFetch>[0] = {}) {
    const now = Date.now();
    let list = [account('a', { active: true }), account('b'), account('c')];
    const use: Record<string, UsageSnapshot> = { a: usage(activeFive, 50, now), b: usage(100, 20, now), c: usage(30, 20, now) };
    auth.token = 'tok';
    location.hash = '#/home';
    const f = mockFetch({
      'GET /api/status': () => json(okStatus({ account_management: true })),
      'GET /api/accounts': () => json({ accounts: list }),
      'GET /api/accounts/a/usage': () => json(use.a),
      'GET /api/accounts/b/usage': () => json(use.b),
      'GET /api/accounts/c/usage': () => json(use.c),
      'POST /api/accounts/c/activate': () => {
        list = [account('a'), account('b'), account('c', { active: true })];
        return json({ account: list[2], cache_notice: 'Prompt-Cache startet neu', test: { ok: true, status: 200, model: 'm', message: 'ok' } });
      },
      'GET /api/costs': () => json({ from: '2026-10-06', to: '2026-10-06', rows: [], hours: [] }),
      'GET /api/traces': () => json({ traces: [], next_before: '', next_before_id: '' }),
      'GET /api/redactions': () => json({ redactions: [] }),
      ...extra,
    });
    render(App);
    return f;
  }
  const card = (email: string) => screen.getByRole('article', { name: email });

  it('marks the full active account red and offers the next one, without switching', async () => {
    const f = home(100);
    const button = await screen.findByRole('button', { name: 'Account wechseln' });
    expect(card('a@example.com')).toHaveClass('full');
    expect(within(card('a@example.com')).getByText('Aktiv · Limit erreicht')).toBeInTheDocument();
    expect(card('c@example.com')).toHaveClass('next');
    expect(within(card('c@example.com')).getByRole('button', { name: 'Account wechseln' })).toBe(button);
    expect(card('b@example.com')).not.toHaveClass('next');
    expect(f.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(false);
  });

  it('switches only when the button is pressed', async () => {
    const f = home(100);
    await fireEvent.click(await screen.findByRole('button', { name: 'Account wechseln' }));
    expect(f.mock.calls.filter(([, init]) => init?.method === 'POST').map(([u]) => String(u))).toEqual(['/api/accounts/c/activate']);
    expect(await screen.findByText('c@example.com ist jetzt aktiv.')).toBeInTheDocument();
    await waitFor(() => expect(card('c@example.com')).toHaveClass('active'));
    expect(card('c@example.com')).not.toHaveClass('next');
    expect(screen.queryByRole('button', { name: 'Account wechseln' })).toBeNull();
  });

  it('forces the switch while requests are still running (#142)', async () => {
    let finish!: (r: Response) => void;
    const f = home(100, {
      'POST /api/accounts/c/activate': () => new Promise<Response>((resolve) => { finish = resolve; }),
      'GET /api/accounts/switch-status': () => json({ waiting: true, switching: true, running_requests: 2 }),
      'POST /api/accounts/switch/force': () => json({ waiting: true, switching: true, running_requests: 2 }),
    });
    await fireEvent.click(await screen.findByRole('button', { name: 'Account wechseln' }));
    await fireEvent.click(await screen.findByRole('button', { name: 'Sofort wechseln' }));
    expect(f.mock.calls.some(([u, init]) => u === '/api/accounts/switch/force' && init?.method === 'POST')).toBe(true);
    finish(json({ account: account('c', { active: true }), cache_notice: '', test: { ok: true, status: 200, model: 'm', message: 'ok' } }));
    expect(await screen.findByText('c@example.com ist jetzt aktiv.')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Sofort wechseln' })).toBeNull());
  });

  it('shows no signal while the active account has room', async () => {
    home(80);
    await screen.findByRole('article', { name: 'c@example.com' });
    expect(card('a@example.com')).not.toHaveClass('full');
    expect(screen.queryByRole('button', { name: 'Account wechseln' })).toBeNull();
  });
});
