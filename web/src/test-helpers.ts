import { vi } from 'vitest';
import type { TraceSummary, CostRow, Status, Redaction, StageDetail } from './lib/types/dashboard';

export type Handler = (init: RequestInit | undefined, url: string) => Response | Promise<Response>;

export const json = (body: unknown, status = 200): Response =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

/** fetch mock routed by "METHOD /path" (query string ignored). */
export function mockFetch(routes: Record<string, Handler>) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const key = `${init?.method ?? 'GET'} ${url.split('?')[0]}`;
    const h = routes[key];
    if (!h) throw new Error(`unexpected request ${key}`);
    return h(init, url);
  });
  vi.stubGlobal('fetch', fn);
  return fn;
}

/** A controllable text/event-stream response. */
export function sseResponse() {
  const enc = new TextEncoder();
  let ctrl!: ReadableStreamDefaultController<Uint8Array>;
  const body = new ReadableStream<Uint8Array>({ start: (c) => (ctrl = c) });
  return {
    response: new Response(body, { status: 200, headers: { 'Content-Type': 'text/event-stream' } }),
    push: (s: string) => ctrl.enqueue(enc.encode(s)),
    end: () => ctrl.close(),
  };
}

export const trace = (o: Partial<TraceSummary> = {}): TraceSummary => ({
  id: 't1',
  client_id: 'c1',
  client_name: 'laptop',
  started_at: '2026-10-04T10:00:00Z',
  endpoint: 'messages',
  model: 'claude-opus-5-5',
  stream: true,
  status: 200,
  duration_ms: 1234,
  estimated_input_tokens: 900,
  usage: { input: 100, output: 50, cache_read: 800, cache_creation: 10 },
  saved_tokens: 0,
  cost_nano_usd: 12_300_000,
  error_kind: '',
  ttft_ms: null,
  output_tps: null,
  tollgate_ms: null,
  perf: null,
  tps: null,
  ...o,
});

export const costRow = (o: Partial<CostRow> = {}): CostRow => ({
  day: '2026-10-04',
  client_id: 'c1',
  client_name: 'laptop',
  model: 'claude-opus-5-5',
  requests: 2,
  usage: { input: 100, output: 50, cache_read: 800, cache_creation: 100 },
  estimated_input_tokens: 1000,
  saved_tokens: 40,
  cost_nano_usd: 1_500_000_000,
  cache_savings_nano_usd: 3_040_000,
  ...o,
});

export const okStatus = (o: Partial<Status> = {}): Status => ({
  tollgate: { ok: true, message: '', version: 'v0.1.0' },
  cliproxyapi: { ok: true, message: '', version: '6.1' },
  accounts: [{ name: 'claude-a.json', provider: 'claude', email: 'a@example.com', status: 'active', disabled: false, cooldown_until: '' }],
  limits: null,
  ...o,
});

export const redaction = (o: Partial<Redaction> = {}): Redaction => ({
  at: '2026-10-04T10:00:00.000Z',
  trace_id: 'abc',
  client_id: 'c1',
  client_name: 'laptop',
  stage: 'redact',
  type: 'email',
  rule: 'email',
  placeholder: '<redacted:email_1>',
  mask: 'n•••@e•••.org',
  ...o,
});

export const stage = (o: Partial<StageDetail> = {}): StageDetail => ({
  name: 'redact',
  duration_us: 250,
  bytes_in: 0,
  bytes_out: 0,
  tokens_in: 120,
  tokens_out: 118,
  hash_in: 'a'.repeat(64),
  hash_out: 'b'.repeat(64),
  config_version: 3,
  diff: '',
  matches: [],
  ...o,
});
