import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within, waitFor } from '@testing-library/svelte';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import { costRow, json, mockFetch, okStatus, sseResponse, trace } from './test-helpers';

function signIn(token = 'tok') {
  auth.token = token;
  localStorage.setItem('tollgate.admin_token', token);
}

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
});

describe('login', () => {
  it('asks for the admin token, stores it and sends it as Bearer', async () => {
    location.hash = '#/accounts';
    const f = mockFetch({ 'GET /api/status': () => json(okStatus()) });
    render(App);
    expect(screen.getByRole('heading', { level: 1, name: 'Accounts' })).toBeInTheDocument();
    expect(f).not.toHaveBeenCalled();
    await fireEvent.input(screen.getByLabelText('Admin-Token'), { target: { value: ' secret ' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Anmelden' }));
    expect(await screen.findByText('a@example.com')).toBeInTheDocument();
    expect(localStorage.getItem('tollgate.admin_token')).toBe('secret');
    const init = f.mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer secret');
  });

  it('discards the token on 401 and prompts again', async () => {
    location.hash = '#/accounts';
    signIn('old');
    mockFetch({ 'GET /api/status': () => json({ error: 'unauthorized' }, 401) });
    render(App);
    expect(await screen.findByRole('alert')).toHaveTextContent('Token wurde abgelehnt');
    expect(screen.getByLabelText('Admin-Token')).toBeInTheDocument();
    expect(localStorage.getItem('tollgate.admin_token')).toBeNull();
  });

  it('logs out on request', async () => {
    location.hash = '#/accounts';
    signIn();
    mockFetch({ 'GET /api/status': () => json(okStatus()) });
    render(App);
    await screen.findByText('a@example.com');
    await fireEvent.click(screen.getByRole('button', { name: 'Abmelden' }));
    expect(screen.getByLabelText('Admin-Token')).toBeInTheDocument();
    expect(localStorage.getItem('tollgate.admin_token')).toBeNull();
  });
});

describe('Live', () => {
  it('lists traces, upserts running to finished from events and expands details', async () => {
    location.hash = '#/live';
    signIn();
    const sse = sseResponse();
    const f = mockFetch({
      'GET /api/traces': () => json({ traces: [trace({ id: 'old', started_at: '2026-10-04T09:00:00Z', client_name: 'alt', cost_nano_usd: null, saved_tokens: 30 })], next_before: '', next_before_id: '' }),
      'GET /api/traces/events': () => sse.response,
    });
    render(App);
    expect(await screen.findByText('alt')).toBeInTheDocument();
    // null cost shows a dash, saved tokens are shown
    const oldRow = screen.getByText('alt').closest('li')!;
    expect(within(oldRow).getByText('−30')).toBeInTheDocument();
    expect(within(oldRow).getAllByText('–').length).toBeGreaterThan(0);
    await screen.findByText('Live', { selector: '.conn' });

    const running = trace({ id: 'new', status: 0, usage: null, cost_nano_usd: null, client_name: 'laptop', started_at: '2026-10-04T10:00:00Z' });
    sse.push(`event: trace\ndata: ${JSON.stringify({ trace: running }).slice(0, 40)}`);
    sse.push(`${JSON.stringify({ trace: running }).slice(40)}\n\n`);
    const row = (await screen.findByText('laptop')).closest('li')!;
    expect(within(row).getByRole('img', { name: 'läuft' })).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')[0]).toBe(row); // newest on top

    sse.push(`event: trace\ndata: ${JSON.stringify({ trace: { ...running, status: 200, usage: trace().usage, cost_nano_usd: 12_300_000 } })}\n\n`);
    await waitFor(() => expect(within(row).getByRole('img', { name: 'OK (200)' })).toBeInTheDocument());
    expect(screen.getAllByText('laptop')).toHaveLength(1);
    expect(within(row).getByText('$0.0123')).toBeInTheDocument();

    const btn = within(row).getByRole('button');
    expect(btn).toHaveAttribute('aria-expanded', 'false');
    await fireEvent.click(btn);
    expect(btn).toHaveAttribute('aria-expanded', 'true');
    expect(within(row).getByText('messages (Stream)')).toBeInTheDocument();
    expect(within(row).getByText('new')).toBeInTheDocument();

    // token only in the header, never in a URL
    const evCall = f.mock.calls.find((c) => String(c[0]).includes('/events'))!;
    expect(String(evCall[0])).not.toContain('tok');
    expect(((evCall[1] as RequestInit).headers as Record<string, string>).Authorization).toBe('Bearer tok');
  });

  it('refetches the snapshot after a reconnect so stale running rows finish', async () => {
    location.hash = '#/live';
    signIn();
    const first = sseResponse();
    const second = sseResponse();
    let events = 0;
    let lists = 0;
    mockFetch({
      'GET /api/traces': () => {
        lists++;
        const t = lists === 1 ? trace({ id: 'r1', status: 0, usage: null, cost_nano_usd: null }) : trace({ id: 'r1', status: 200 });
        return json({ traces: [t], next_before: '', next_before_id: '' });
      },
      'GET /api/traces/events': () => (events++ === 0 ? first.response : second.response),
    });
    render(App);
    expect(await screen.findByRole('img', { name: 'läuft' })).toBeInTheDocument();
    await screen.findByText('Live', { selector: '.conn' });
    first.end(); // connection lost, the gap swallows the finish event
    expect(await screen.findByRole('img', { name: 'OK (200)' }, { timeout: 4000 })).toBeInTheDocument();
    expect(lists).toBe(2);
  }, 8000);

  it('shows an error trace and the empty state', async () => {
    location.hash = '#/live';
    signIn();
    mockFetch({
      'GET /api/traces': () => json({ traces: [trace({ status: 502, error_kind: 'upstream_error', usage: null })], next_before: '', next_before_id: '' }),
      'GET /api/traces/events': () => sseResponse().response,
    });
    render(App);
    expect(await screen.findByRole('img', { name: 'Fehler (502)' })).toBeInTheDocument();
  });

  it('shows a hint when there are no requests yet', async () => {
    location.hash = '#/live';
    signIn();
    mockFetch({
      'GET /api/traces': () => json({ traces: [], next_before: '', next_before_id: '' }),
      'GET /api/traces/events': () => sseResponse().response,
    });
    render(App);
    expect(await screen.findByText(/Noch keine Anfragen/)).toBeInTheDocument();
  });
});

describe('Kosten', () => {
  it('shows totals, cache hit rate, the breakdown by model and a dash for null cost', async () => {
    location.hash = '#/costs';
    signIn();
    const f = mockFetch({
      'GET /api/costs': () =>
        json({
          from: '2026-10-03',
          to: '2026-10-04',
          rows: [costRow(), costRow({ client_id: 'c2', client_name: 'server', model: 'mystery-1', cost_nano_usd: null, cache_savings_nano_usd: null, day: '2026-10-03' })],
          hours: [],
        }),
      'GET /api/performance': () => json({ from: '2026-10-03', to: '2026-10-04', models: [], redactor: null, speedtests: [], fastest_fill: null,
        capacity: { windows_per_week: null, fills_per_day: null, measured_hours: 0, sample_windows: 0, window_cost_nano_usd: null, sample_accounts: 0, accounts: 0, free_this_week: null } }),
      'GET /api/accounts': () => json({ accounts: [] }),
    });
    render(App);
    // totals: 4 requests, lower bound because one row has no price
    const total = await screen.findByRole('region', { name: 'Gesamtkosten' });
    expect(within(total).getByText('≥ $1.50', { selector: '.big' })).toBeInTheDocument();
    expect(total).toHaveTextContent('4 Anfragen');
    const table = screen.getByRole('table', { name: 'Aufschlüsselung' });
    await fireEvent.click(screen.getByRole('button', { name: 'Modell' }));
    const mystery = within(table).getByText('mystery-1').closest('tr')!;
    expect(within(mystery).getByText('–', { selector: 'td.cost' })).toBeInTheDocument();
    // hit rate per row: 800 / (100 + 800 + 100) = 80 %
    expect(within(mystery).getByText('80,0 %')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Tag' }));
    expect(within(table).getAllByRole('row')).toHaveLength(3);
    const costCalls = () => f.mock.calls.map((c) => String(c[0])).filter((u) => u.startsWith('/api/costs'));
    expect(costCalls()).toEqual(['/api/costs?days=7']);

    await fireEvent.click(screen.getByRole('button', { name: '30 Tage' }));
    await waitFor(() => expect(costCalls().at(-1)).toBe('/api/costs?days=30'));
    expect(screen.getByRole('button', { name: '30 Tage' })).toHaveAttribute('aria-pressed', 'true');
  });
});

describe('Clients', () => {
  const info = (o = {}) => ({ id: 'c1', name: 'laptop', key_hint: 'tg_ab…yz', status: 'aktiv', created_at: '2026-10-01T08:00:00Z', rotated_at: null, ...o });

  it('lists clients, creates one and shows the key once with a copy-ready snippet', async () => {
    location.hash = '#/clients';
    signIn();
    let list = [info()];
    mockFetch({
      'GET /api/clients': () => json({ clients: list }),
      'POST /api/clients': (init) => {
        const name = JSON.parse(init!.body as string).name;
        list = [...list, info({ id: 'c2', name })];
        return json({ client: info({ id: 'c2', name }), key: 'tg_SECRETKEY' }, 201);
      },
    });
    const clip = vi.fn(async () => {});
    Object.defineProperty(navigator, 'clipboard', { value: { writeText: clip }, configurable: true });
    render(App);
    expect(await screen.findByText('tg_ab…yz')).toBeInTheDocument();
    await fireEvent.input(screen.getByLabelText('Name des neuen Clients'), { target: { value: 'ci' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Anlegen' }));
    const snip = await screen.findByTestId('snippet');
    expect(snip.textContent).toBe(`export ANTHROPIC_BASE_URL=${window.location.origin}\nexport ANTHROPIC_AUTH_TOKEN=tg_SECRETKEY`);
    expect(await screen.findByText('ci')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Kopieren' }));
    expect(clip).toHaveBeenCalledWith(snip.textContent);
    expect(await screen.findByRole('button', { name: 'Kopiert' })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Schließen' }));
    expect(screen.queryByTestId('snippet')).toBeNull();
    expect(document.body.textContent).not.toContain('tg_SECRETKEY');
  });

  it('blocks only after confirmation and can cancel', async () => {
    location.hash = '#/clients';
    signIn();
    const block = vi.fn(() => json({ client: info({ status: 'gesperrt' }) }));
    let status = 'aktiv';
    mockFetch({
      'GET /api/clients': () => json({ clients: [info({ status })] }),
      'POST /api/clients/c1/block': () => {
        status = 'gesperrt';
        return block();
      },
    });
    render(App);
    await fireEvent.click(await screen.findByRole('button', { name: 'Sperren: laptop' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));
    expect(block).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole('button', { name: 'Sperren: laptop' }));
    expect(screen.getByText('Wirklich sperren?')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Bestätigen' }));
    expect(await screen.findByText('gesperrt')).toBeInTheDocument();
    expect(block).toHaveBeenCalledOnce();
    expect(screen.queryByRole('button', { name: 'Sperren: laptop' })).toBeNull();
  });

  it('rotates after confirmation and shows the new key once', async () => {
    location.hash = '#/clients';
    signIn();
    mockFetch({
      'GET /api/clients': () => json({ clients: [info()] }),
      'POST /api/clients/c1/rotate': () => json({ client: info({ rotated_at: '2026-10-04T08:00:00Z' }), key: 'tg_NEW' }),
    });
    render(App);
    await fireEvent.click(await screen.findByRole('button', { name: 'Rotieren: laptop' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Bestätigen' }));
    expect((await screen.findByTestId('snippet')).textContent).toContain('ANTHROPIC_AUTH_TOKEN=tg_NEW');
  });

  it('shows API errors', async () => {
    location.hash = '#/clients';
    signIn();
    mockFetch({
      'GET /api/clients': () => json({ clients: [] }),
      'POST /api/clients': () => json({ error: 'name must be 1 to 128 characters' }, 400),
    });
    render(App);
    await screen.findByText('Noch keine Clients.');
    await fireEvent.input(screen.getByLabelText('Name des neuen Clients'), { target: { value: 'x' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Anlegen' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('name must be 1 to 128 characters');
  });
});

describe('Accounts', () => {
  it('shows health, the account and no warning for a single account', async () => {
    location.hash = '#/accounts';
    signIn();
    mockFetch({ 'GET /api/status': () => json(okStatus()) });
    render(App);
    expect(await screen.findByText('a@example.com')).toBeInTheDocument();
    expect(screen.getByText('v0.1.0')).toBeInTheDocument();
    expect(screen.getAllByRole('img', { name: 'in Ordnung' })).toHaveLength(2);
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('warns about more than one account and shows a failing service and cooldown', async () => {
    location.hash = '#/accounts';
    signIn();
    const a = okStatus().accounts[0];
    mockFetch({
      'GET /api/status': () =>
        json(okStatus({ cliproxyapi: { ok: false, message: 'nicht erreichbar', version: '' }, accounts: [a, { ...a, name: 'b.json', email: 'b@example.com', cooldown_until: '2026-10-04T12:00:00Z' }] })),
    });
    render(App);
    expect(await screen.findByRole('status')).toHaveTextContent('2 Accounts');
    expect(screen.getByRole('img', { name: 'gestört' })).toBeInTheDocument();
    expect(screen.getByText('nicht erreichbar')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Cooldown' })).toBeInTheDocument();
  });

  it('shows the subscription limits with percent, status and reset', async () => {
    location.hash = '#/accounts';
    signIn();
    const inMs = (ms: number) => new Date(Date.now() + ms).toISOString();
    mockFetch({
      'GET /api/status': () =>
        json(okStatus({
          limits: {
            status: 'allowed', reset: '', representative_claim: 'five_hour', observed_at: new Date().toISOString(),
            five_hour: { utilization: 0.42, reset: inMs((2 * 60 + 13) * 60000 - 1000), status: 'allowed' },
            seven_day: { utilization: null, reset: '', status: '' },
          },
        })),
    });
    render(App);
    expect(await screen.findByText('42 %')).toBeInTheDocument();
    expect(screen.getByText(/zurückgesetzt in 2 h 13 min/)).toBeInTheDocument();
    expect(screen.getByRole('meter', { name: 'Auslastung 5 Stunden' })).toHaveAttribute('aria-valuenow', '42');
    expect(screen.getByRole('img', { name: 'frei' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'unbekannt' })).toBeInTheDocument();
    expect(screen.getByText('Reset unbekannt')).toBeInTheDocument();
  });

  it('says there is no limit data yet instead of guessing', async () => {
    location.hash = '#/accounts';
    signIn();
    mockFetch({ 'GET /api/status': () => json(okStatus()) });
    render(App);
    expect(await screen.findByText(/Noch keine Daten/)).toBeInTheDocument();
    expect(screen.queryByRole('meter')).toBeNull();
  });

  it('explains missing accounts using the CLIProxyAPI message', async () => {
    location.hash = '#/accounts';
    signIn();
    mockFetch({ 'GET /api/status': () => json(okStatus({ accounts: [], cliproxyapi: { ok: true, message: 'Accounts nicht verfügbar', version: '' } })) });
    render(App);
    expect(await screen.findByText(/Keine Accounts verfügbar/)).toHaveTextContent('Accounts nicht verfügbar');
  });
});
