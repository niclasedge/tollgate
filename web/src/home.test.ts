import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import App from './App.svelte';
import LimitCard from './lib/components/LimitCard.svelte';
import { auth, logout } from './lib/auth.svelte';
import { costRow, json, mockFetch, okStatus, redaction, sseResponse, trace, type Handler } from './test-helpers';
import type { RateLimits } from './lib/types/dashboard';

const inMs = (ms: number) => new Date(Date.now() + ms).toISOString();

const limits = (o: Partial<RateLimits> = {}): RateLimits => ({
  status: 'allowed',
  reset: '',
  representative_claim: 'five_hour',
  observed_at: new Date().toISOString(),
  five_hour: { utilization: 0.42, reset: inMs((2 * 60 + 13) * 60000 - 1000), status: 'allowed' },
  seven_day: { utilization: 0.81, reset: inMs(3 * 86400000), status: 'allowed_warning' },
  ...o,
});

function home(over: Record<string, Handler> = {}) {
  auth.token = 'tok';
  location.hash = '#/home';
  return mockFetch({
    'GET /api/status': () => json(okStatus({ limits: limits() })),
    'GET /api/costs': () => json({ from: '2026-10-04', to: '2026-10-04', rows: [costRow(), costRow({ client_id: 'c2', saved_tokens: 60 })] }),
    'GET /api/traces': () => json({ traces: [trace({ id: 'abc', client_name: 'laptop' }), trace({ id: 'def', client_name: 'server', status: 500 })], next_before: '', next_before_id: '' }),
    'GET /api/redactions': () => json({ redactions: [] }),
    'GET /api/clients': () => json({ clients: [] }),
    ...over,
  });
}

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
});

describe('Übersicht', () => {
  it('shows both limit windows with big percent, bar and reset with date', async () => {
    home();
    render(App);
    const card = await screen.findByRole('region', { name: 'Abo-Limits' });
    expect(await within(card).findByText('42 %')).toBeInTheDocument();
    expect(within(card).getByText('81 %')).toBeInTheDocument();
    expect(within(card).getByRole('meter', { name: 'Auslastung 5 Stunden' })).toHaveAttribute('aria-valuenow', '42');
    expect(within(card).getByText(/^zurückgesetzt in 2 h 13 min · \d{2}\.\d{2}\., \d{2}:\d{2}$/)).toBeInTheDocument();
    expect(within(card).getByText(/^zurückgesetzt in 3 T/)).toBeInTheDocument();
    expect(within(card).getByRole('img', { name: 'fast erreicht' })).toBeInTheDocument();
  });

  it('says there is no limit data instead of guessing', async () => {
    home({ 'GET /api/status': () => json(okStatus()) });
    render(App);
    const card = await screen.findByRole('region', { name: 'Abo-Limits' });
    expect(await within(card).findByText(/Noch keine Daten/)).toBeInTheDocument();
    expect(within(card).queryByRole('meter')).toBeNull();
  });

  it('shows today from the cost report of two weeks', async () => {
    const f = home();
    render(App);
    const today = await screen.findByRole('region', { name: 'Heute' });
    // two rows: 4 requests; in = 2 * (100 + 800 + 100), out = 2 * 50; hit rate 1600 / 2000
    const value = (label: string) => within(today).getByText(label).nextElementSibling;
    expect(await within(today).findByText('Anfragen')).toBeInTheDocument();
    expect(value('Anfragen')).toHaveTextContent('4');
    expect(value('Tokens ein')).toHaveTextContent('2.000');
    expect(value('Tokens aus')).toHaveTextContent('100');
    expect(value('Cache-Trefferquote')).toHaveTextContent('80,0 %');
    expect(value('Gespart')).toHaveTextContent('100');
    expect(f.mock.calls.map((c) => String(c[0]))).toContain('/api/costs?days=14');
  });

  it('shows health of both services and the active account', async () => {
    home({
      'GET /api/status': () => json(okStatus({ cliproxyapi: { ok: false, message: 'nicht erreichbar', version: '' } })),
    });
    render(App);
    const health = await screen.findByRole('region', { name: 'Health' });
    expect(await within(health).findByText('a@example.com')).toBeInTheDocument();
    expect(within(health).getByText('nicht erreichbar')).toBeInTheDocument();
    expect(within(health).getAllByRole('img', { name: 'in Ordnung' })).toHaveLength(1);
    expect(within(health).getByRole('img', { name: 'aktiv' })).toBeInTheDocument();
    expect(within(health).getByRole('img', { name: 'gestört' })).toBeInTheDocument();
  });

  it('marks the active account as unclear with several or unknown without accounts', async () => {
    const two = okStatus().accounts.concat({ ...okStatus().accounts[0], name: 'b.json', email: 'b@example.com' });
    home({ 'GET /api/status': () => json(okStatus({ accounts: two })) });
    const { unmount } = render(App);
    const health = await screen.findByRole('region', { name: 'Health' });
    expect(await within(health).findByText('unklar: 2 Accounts im auth-dir')).toBeInTheDocument();
    unmount();
    home({ 'GET /api/status': () => json(okStatus({ accounts: [] })) });
    render(App);
    expect(await within(await screen.findByRole('region', { name: 'Health' })).findByText('unbekannt')).toBeInTheDocument();
  });

  it('lists the last 10 requests linking into the open Live detail', async () => {
    const sse = sseResponse();
    const f = home({ 'GET /api/traces/events': () => sse.response });
    render(App);
    const recent = await screen.findByRole('region', { name: 'Letzte Anfragen' });
    const link = await within(recent).findByRole('link', { name: /laptop/ });
    expect(link).toHaveAttribute('href', '#/live/abc');
    expect(within(recent).getByRole('img', { name: 'Fehler (500)' })).toBeInTheDocument();
    expect(f.mock.calls.map((c) => String(c[0]))).toContain('/api/traces?limit=10');

    location.hash = '#/live/abc';
    window.dispatchEvent(new HashChangeEvent('hashchange'));
    expect(await screen.findByRole('heading', { level: 1, name: 'Live' })).toBeInTheDocument();
    const row = (await screen.findByText('laptop')).closest('button')!;
    expect(row).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('abc', { selector: 'dd' })).toBeInTheDocument();
  });

  it('lists the last redactions with type, placeholder and rule, linking into Live', async () => {
    const f = home({
      'GET /api/redactions': () =>
        json({
          redactions: [
            redaction({ trace_id: 'abc', client_name: 'laptop' }),
            redaction({ trace_id: 'def', client_name: 'server', type: 'customer_id', rule: 'kundennummer', placeholder: '<redacted:customer_id_2>' }),
          ],
        }),
    });
    render(App);
    const panel = await screen.findByRole('region', { name: 'Letzte Redactions' });
    const items = await within(panel).findAllByRole('listitem');
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText('<redacted:email_1>')).toBeInTheDocument();
    expect(within(items[0]).getByText('email')).toBeInTheDocument();
    expect(within(items[0]).getByRole('link')).toHaveAttribute('href', '#/live/abc');
    expect(within(items[1]).getByText('<redacted:customer_id_2>')).toBeInTheDocument();
    expect(within(items[1]).getByText('kundennummer')).toBeInTheDocument();
    expect(within(items[1]).getByText('server')).toBeInTheDocument();
    expect(f.mock.calls.map((c) => String(c[0]))).toContain('/api/redactions?limit=10');
  });

  it('shows the mask of every redaction and a dash for old ones (#112)', async () => {
    home({
      'GET /api/redactions': () =>
        json({
          redactions: [
            redaction({ trace_id: 'abc', mask: 'n•••@e•••.org' }),
            redaction({ trace_id: 'def', type: 'word', rule: 'words', placeholder: '<redacted:word_1>', mask: '' }),
          ],
        }),
    });
    render(App);
    const panel = await screen.findByRole('region', { name: 'Letzte Redactions' });
    const items = await within(panel).findAllByRole('listitem');
    expect(within(items[0]).getByTitle('Maske')).toHaveTextContent('n•••@e•••.org');
    expect(within(items[1]).getByTitle('Maske')).toHaveTextContent('–');
  });

  it('says when there are no redactions yet', async () => {
    home();
    render(App);
    const panel = await screen.findByRole('region', { name: 'Letzte Redactions' });
    expect(await within(panel).findByText('Noch keine Redactions.')).toBeInTheDocument();
  });

  it('keeps other sections when one request fails', async () => {
    home({ 'GET /api/costs': () => json({ error: 'kaputt' }, 500) });
    render(App);
    expect(await within(await screen.findByRole('region', { name: 'Heute' })).findByRole('alert')).toHaveTextContent('kaputt');
    expect(await screen.findByText('42 %')).toBeInTheDocument();
  });
});

describe('Übersicht mit Account-Verwaltung', () => {
  it('shows the same card per subscription, active first, without actions', async () => {
    const usage = (token: string) => ({ five_hour: { utilization: 20, reset: inMs(3600000) }, seven_day: { utilization: 50, reset: inMs(3 * 86400000) }, observed_at: new Date().toISOString(), error: '', license: { token, checked_at: new Date().toISOString(), organization_type: 'claude_max', rate_limit_tier: 'default_claude_max_5x', subscription_status: '', profile_observed_at: new Date().toISOString() } });
    home({
      'GET /api/status': () => json(okStatus({ account_management: true })),
      'GET /api/accounts': () => json({ accounts: [
        { id: 'b', name: 'Zweites Abo', email: 'b@example.com', active: false, status: 'gespeichert', expires_at: '', last_refresh: '', updated_at: '' },
        { id: 'a', name: 'Erstes Abo', email: 'a@example.com', active: true, status: 'active', expires_at: '', last_refresh: '', updated_at: '' },
      ] }),
      'GET /api/accounts/a/usage': () => json(usage('valid')),
      'GET /api/accounts/b/usage': () => json(usage('expired')),
    });
    render(App);
    const region = await screen.findByRole('region', { name: 'Abo-Limits' });
    const cards = await within(region).findAllByRole('article');
    expect(cards.map((c) => within(c).getByRole('heading').textContent)).toEqual(['a@example.com', 'b@example.com']);
    expect(await within(cards[0]).findByText('Token gültig')).toBeInTheDocument();
    expect(await within(cards[1]).findByText(/^Zugang abgelaufen/)).toBeInTheDocument();
    expect(within(region).getAllByRole('meter', { name: '5-Stunden-Verbrauch' })).toHaveLength(2);
    expect(within(region).queryByRole('button', { name: 'Aktivieren' })).toBeNull();
    expect(within(region).queryByRole('region', { name: 'Aktives Abo' })).toBeNull();
  });
});

describe('LimitCard', () => {
  it('renders one subscription with title and an active marker for reuse per account', () => {
    render(LimitCard, { title: 'b@example.com', subtitle: 'Max', limits: limits(), active: true });
    const card = screen.getByRole('region', { name: 'b@example.com' });
    expect(within(card).getByText('Max')).toBeInTheDocument();
    expect(within(card).getByRole('img', { name: 'aktiv' })).toBeInTheDocument();
    expect(within(card).getAllByRole('meter')).toHaveLength(2);
  });

  it('caps the bar at 100 % but shows the real share', () => {
    render(LimitCard, {
      title: 'x',
      limits: limits({ five_hour: { utilization: 1.3, reset: '', status: 'rejected' } }),
    });
    expect(screen.getByText('130 %')).toBeInTheDocument();
    const bar = screen.getByRole('meter', { name: 'Auslastung 5 Stunden' }).firstElementChild as HTMLElement;
    expect(bar.style.width).toBe('100%');
    expect(screen.getByText('Reset unbekannt')).toBeInTheDocument();
  });
});
