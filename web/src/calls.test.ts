import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import { json, mockFetch, okStatus, type Handler } from './test-helpers';
import type { CallRate } from './lib/types/dashboard';

const rate = (o: Partial<CallRate> = {}): CallRate => ({
  window_seconds: 300,
  windows: 10,
  current: 4,
  previous_total: 23,
  average: 2.3,
  trend: 'above',
  ...o,
});

function home(calls: Handler) {
  auth.token = 'tok';
  location.hash = '#/home';
  return mockFetch({
    'GET /api/status': () => json(okStatus()),
    'GET /api/costs': () => json({ from: '2026-10-07', to: '2026-10-07', rows: [], hours: [] }),
    'GET /api/performance': () => json({ from: '', to: '', models: [], redactor: null, speedtests: [] }),
    'GET /api/traces': () => json({ traces: [], next_before: '', next_before_id: '' }),
    'GET /api/redactions': () => json({ redactions: [] }),
    'GET /api/calls': calls,
  });
}

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
});

describe('Übersicht: Letzte 5 Minuten (#132)', () => {
  it.each([
    ['above', 'über Ø'],
    ['below', 'unter Ø'],
    ['equal', 'wie Ø'],
  ])('shows the count, the average and the %s verdict', async (trend, label) => {
    home(() => json(rate({ trend })));
    render(App);
    const sec = await screen.findByRole('region', { name: 'Letzte 5 Minuten' });
    expect(await within(sec).findByText('4')).toBeInTheDocument();
    expect(within(sec).getByText('Ø 2,3 je 5 min in den 50 min davor')).toBeInTheDocument();
    expect(within(sec).getByRole('img', { name: label })).toBeInTheDocument();
    expect(within(sec).getByText(label)).toBeInTheDocument();
  });

  it('shows the error inside the section and keeps the others', async () => {
    home(() => json({ error: 'internal error' }, 500));
    render(App);
    const sec = await screen.findByRole('region', { name: 'Letzte 5 Minuten' });
    expect(await within(sec).findByRole('alert')).toHaveTextContent('internal error');
    expect(screen.getByRole('region', { name: 'Health' })).toBeInTheDocument();
  });

  it('reloads with the overview', async () => {
    let n = 0;
    const f = home(() => json(rate({ current: ++n === 1 ? 4 : 9 })));
    render(App);
    const sec = await screen.findByRole('region', { name: 'Letzte 5 Minuten' });
    expect(await within(sec).findByText('4')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Aktualisieren' }));
    expect(await within(sec).findByText('9')).toBeInTheDocument();
    await waitFor(() => expect(f.mock.calls.filter((c) => String(c[0]) === '/api/calls').length).toBeGreaterThanOrEqual(2));
  });
});
