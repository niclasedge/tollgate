import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import { json, mockFetch, sseResponse, stage, trace } from './test-helpers';
import type { TraceDetail } from './lib/types/dashboard';

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
});

const detail = (): TraceDetail => ({
  trace: trace({ id: 'abc', client_name: 'laptop', duration_ms: 1234 }),
  stages: [
    stage({ name: 'intake', duration_us: 40, tokens_in: 120, tokens_out: 120, hash_out: 'a'.repeat(64), config_version: 0 }),
    stage({
      name: 'redact',
      diff: 'messages[0].content: mail <redacted:email_1> bitte\n',
      matches: [
        { offset: 30, length: 18, type: 'email', rule: 'email', placeholder: '<redacted:email_1>', mask: 'n•••@e•••.org' },
        { offset: 90, length: 18, type: 'email', rule: 'email', placeholder: '<redacted:email_1>', mask: 'n•••@e•••.org' },
        { offset: 140, length: 20, type: 'api_key', rule: 'anthropic_key', placeholder: '<redacted:api_key_1>', mask: 'sk-ant-api03-a••• (108)' },
      ],
    }),
  ],
});

function live(d: TraceDetail | (() => Response)) {
  auth.token = 'tok';
  location.hash = '#/live';
  const sse = sseResponse();
  return mockFetch({
    'GET /api/traces': () => json({ traces: [trace({ id: 'abc', client_name: 'laptop' })], next_before: '', next_before_id: '' }),
    'GET /api/traces/events': () => sse.response,
    'GET /api/traces/abc': typeof d === 'function' ? d : () => json(d),
  });
}

describe('Stufen-Timeline', () => {
  it('shows every stage with duration, tokens, hashes, config version, matches and diff, plus the return path', async () => {
    const f = live(detail());
    render(App);
    await fireEvent.click((await screen.findByText('laptop')).closest('button')!);
    const tl = await screen.findByRole('list', { name: 'Stufen-Timeline' });
    const items = within(tl).getAllByRole('listitem');
    expect(items).toHaveLength(3);
    const [intake, red, back] = items;

    expect(within(intake).getByText('Eingang')).toBeInTheDocument();
    expect(within(intake).getByText('0,04 ms')).toBeInTheDocument();
    expect(within(intake).getByText('120 → 120')).toBeInTheDocument();

    expect(within(red).getByText('Redact')).toBeInTheDocument();
    expect(within(red).getByText('0,25 ms')).toBeInTheDocument();
    expect(within(red).getByText('120 → 118')).toBeInTheDocument();
    expect(within(red).getByText('Konfiguration v3')).toBeInTheDocument();
    expect(within(red).getByText('2 × email')).toBeInTheDocument();
    expect(within(red).getByText('1 × api_key')).toBeInTheDocument();
    const hin = within(red).getByText('aaaaaaaa…');
    expect(hin).toHaveAttribute('title', 'a'.repeat(64));
    expect(within(red).getByText('bbbbbbbb…')).toHaveAttribute('title', 'b'.repeat(64));
    await fireEvent.click(within(red).getByRole('button', { name: /Diff/ }));
    expect(within(red).getByText(/mail <redacted:email_1> bitte/)).toBeInTheDocument();

    expect(within(back).getByText('Rückweg')).toBeInTheDocument();
    expect(within(back).getByText(/2 Platzhalter rückübersetzbar/)).toBeInTheDocument();
    expect(within(back).getByText('1.234 ms')).toBeInTheDocument();
    expect(f.mock.calls.map((c) => String(c[0]))).toContain('/api/traces/abc');
  });

  it('says when a stage changed nothing and reports a failed detail load', async () => {
    const d = detail();
    d.stages = [stage({ name: 'redact', hash_in: 'c'.repeat(64), hash_out: 'c'.repeat(64) })];
    live(d);
    const { unmount } = render(App);
    await fireEvent.click((await screen.findByText('laptop')).closest('button')!);
    const tl = await screen.findByRole('list', { name: 'Stufen-Timeline' });
    expect(within(tl).getByText('keine Änderung')).toBeInTheDocument();
    unmount();

    live(() => json({ error: 'kaputt' }, 500));
    render(App);
    await fireEvent.click((await screen.findByText('laptop')).closest('button')!);
    expect(await screen.findByRole('alert')).toHaveTextContent('kaputt');
  });
});

describe('Masken in der Redact-Stufe (#112)', () => {
  it('lists type, placeholder, mask and count per placeholder', async () => {
    live(detail());
    render(App);
    await fireEvent.click((await screen.findByText('laptop')).closest('button')!);
    const tl = await screen.findByRole('list', { name: 'Stufen-Timeline' });
    const hits = within(tl).getByRole('table', { name: 'Treffer der Stufe Redact' });
    const rows = within(hits).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(2);
    const cells = (r: HTMLElement) => within(r).getAllByRole('cell').map((c) => c.textContent?.trim());
    expect(cells(rows[0])).toEqual(['email', '<redacted:email_1>', 'n•••@e•••.org', '2 ×']);
    expect(cells(rows[1])).toEqual(['api_key', '<redacted:api_key_1>', 'sk-ant-api03-a••• (108)', '1 ×']);
  });

  it('shows a dash for matches recorded before masks existed', async () => {
    const d = detail();
    d.stages[1].matches = [{ offset: 1, length: 18, type: 'email', rule: 'email', placeholder: '<redacted:email_1>', mask: '' }];
    live(d);
    render(App);
    await fireEvent.click((await screen.findByText('laptop')).closest('button')!);
    const hits = await screen.findByRole('table', { name: 'Treffer der Stufe Redact' });
    const [row] = within(hits).getAllByRole('row').slice(1);
    expect(within(row).getAllByRole('cell')[2]).toHaveTextContent('–');
  });
});
