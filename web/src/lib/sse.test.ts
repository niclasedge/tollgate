import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { createSseClient, createSseParser, createFetchEventSource, type EventSourceLike, type SseMessage, type SseStatus } from './sse';

class MockEventSource implements EventSourceLike {
  static instances: MockEventSource[] = [];
  static readonly CLOSED = 2;
  readyState = 0;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;
  listeners = new Map<string, (e: { data: string }) => void>();
  constructor(public url: string) {
    MockEventSource.instances.push(this);
  }
  addEventListener(type: string, fn: (e: { data: string }) => void) {
    this.listeners.set(type, fn);
  }
  close() {
    this.closed = true;
    this.readyState = 2;
  }
  emit(type: string, data: string) {
    this.listeners.get(type)?.({ data });
  }
}

const ctor = MockEventSource as unknown as new (url: string) => EventSourceLike;

beforeEach(() => {
  MockEventSource.instances = [];
  vi.useFakeTimers();
});
afterEach(() => vi.useRealTimers());

describe('createSseClient', () => {
  it('reports connecting, open and delivers parsed JSON events', () => {
    const statuses: SseStatus[] = [];
    const got: unknown[] = [];
    const c = createSseClient({
      url: '/api/live',
      eventSource: ctor,
      events: ['request'],
      onEvent: (type, data) => got.push([type, data]),
      onStatus: (s) => statuses.push(s),
    });
    const es = MockEventSource.instances[0];
    expect(es.url).toBe('/api/live');
    es.readyState = 1;
    es.onopen?.();
    es.emit('request', '{"id":"t1","tokens":42}');
    expect(statuses).toEqual(['connecting', 'open']);
    expect(got).toEqual([['request', { id: 't1', tokens: 42 }]]);
    c.close();
    expect(es.closed).toBe(true);
    expect(statuses.at(-1)).toBe('closed');
  });

  it('passes non-JSON payloads through as strings', () => {
    const got: unknown[] = [];
    createSseClient({
      url: '/x',
      eventSource: ctor,
      events: ['message'],
      onEvent: (_t, d) => got.push(d),
    });
    MockEventSource.instances[0].emit('message', 'plain text');
    expect(got).toEqual(['plain text']);
  });

  it('reconnects with exponential backoff after the source closed itself', () => {
    const statuses: SseStatus[] = [];
    createSseClient({
      url: '/x',
      eventSource: ctor,
      events: [],
      onEvent: () => {},
      onStatus: (s) => statuses.push(s),
      retryMs: 1000,
      maxRetryMs: 4000,
    });
    const fail = (i: number) => {
      const es = MockEventSource.instances[i];
      es.readyState = 2;
      es.onerror?.();
    };
    fail(0);
    expect(statuses.at(-1)).toBe('reconnecting');
    vi.advanceTimersByTime(999);
    expect(MockEventSource.instances).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(MockEventSource.instances).toHaveLength(2);
    fail(1);
    vi.advanceTimersByTime(1999);
    expect(MockEventSource.instances).toHaveLength(2);
    vi.advanceTimersByTime(1);
    expect(MockEventSource.instances).toHaveLength(3);
    fail(2);
    vi.advanceTimersByTime(4000);
    fail(3);
    vi.advanceTimersByTime(3999);
    expect(MockEventSource.instances).toHaveLength(4); // capped at 4000
    vi.advanceTimersByTime(1);
    expect(MockEventSource.instances).toHaveLength(5);
  });

  it('resets the backoff after a successful open', () => {
    createSseClient({ url: '/x', eventSource: ctor, events: [], onEvent: () => {}, retryMs: 1000 });
    let es = MockEventSource.instances[0];
    es.readyState = 2;
    es.onerror?.();
    vi.advanceTimersByTime(1000);
    es = MockEventSource.instances[1];
    es.readyState = 1;
    es.onopen?.();
    es.readyState = 2;
    es.onerror?.();
    vi.advanceTimersByTime(1000);
    expect(MockEventSource.instances).toHaveLength(3);
  });

  it('does not reconnect after close()', () => {
    const c = createSseClient({ url: '/x', eventSource: ctor, events: [], onEvent: () => {}, retryMs: 10 });
    const es = MockEventSource.instances[0];
    c.close();
    es.onerror?.();
    vi.advanceTimersByTime(100000);
    expect(MockEventSource.instances).toHaveLength(1);
  });

  it('lets the browser retry while the source is still connecting', () => {
    const statuses: SseStatus[] = [];
    createSseClient({
      url: '/x',
      eventSource: ctor,
      events: [],
      onEvent: () => {},
      onStatus: (s) => statuses.push(s),
    });
    const es = MockEventSource.instances[0];
    es.readyState = 0; // browser is retrying by itself
    es.onerror?.();
    vi.advanceTimersByTime(100000);
    expect(MockEventSource.instances).toHaveLength(1);
    expect(statuses.at(-1)).toBe('reconnecting');
  });
});

describe('onReconnect', () => {
  it('fires only when the connection opens again after a loss', () => {
    const onReconnect = vi.fn();
    createSseClient({ url: '/x', eventSource: ctor, events: [], onEvent: () => {}, onReconnect });
    const a = MockEventSource.instances[0];
    a.onopen?.();
    expect(onReconnect).not.toHaveBeenCalled();
    a.readyState = 2;
    a.onerror?.();
    vi.advanceTimersByTime(1000);
    MockEventSource.instances[1].onopen?.();
    expect(onReconnect).toHaveBeenCalledOnce();
  });
});

describe('createSseParser', () => {
  it('rejects unbounded lines and events', () => {
    expect(() => createSseParser(() => {}).push('x'.repeat(1_000_001))).toThrow();
    const p = createSseParser(() => {});
    expect(() => {
      for (let i = 0; i < 11; i++) p.push(`data: ${'y'.repeat(100_000)}\n`);
    }).toThrow();
  });

  const collect = (chunks: string[]) => {
    const out: SseMessage[] = [];
    const p = createSseParser((m) => out.push(m));
    chunks.forEach((c) => p.push(c));
    return out;
  };

  it('parses events split across chunks at arbitrary points', () => {
    const wire = 'event: trace\ndata: {"a":1}\n\n: heartbeat\n\nevent: trace\ndata: {"a":2}\n\n';
    for (let i = 1; i < wire.length; i++) {
      expect(collect([wire.slice(0, i), wire.slice(i)]).map((m) => m.data)).toEqual(['{"a":1}', '{"a":2}']);
    }
  });
  it('handles CRLF (also split between CR and LF), multi-line data and the default event', () => {
    expect(collect(['data: a\r', '\ndata: b\r\n\r\n'])).toEqual([{ event: 'message', data: 'a\nb' }]);
  });
  it('ignores comments and empty events', () => {
    expect(collect([': ping\n\n\n'])).toEqual([]);
  });
});

describe('createFetchEventSource', () => {
  beforeEach(() => vi.useRealTimers());
  const enc = new TextEncoder();
  const streamOf = (chunks: string[], status = 200) =>
    new Response(
      new ReadableStream<Uint8Array>({
        start(c) {
          chunks.forEach((s) => c.enqueue(enc.encode(s)));
          c.close();
        },
      }),
      { status },
    );
  const tick = () => new Promise((r) => setTimeout(r, 0));

  it('sends the Authorization header, keeps the token out of the URL and delivers split events', async () => {
    const f = vi.fn(async () => streamOf(['event: trace\nda', 'ta: {"x":1}\n', '\nevent: trace\ndata: {"x":2}\n\n']));
    const Source = createFetchEventSource({ fetch: f as unknown as typeof fetch, headers: () => ({ Authorization: 'Bearer tok' }) });
    const es = new Source('/api/traces/events');
    const got: string[] = [];
    let opened = false;
    es.onopen = () => (opened = true);
    es.addEventListener('trace', (e) => got.push(e.data));
    await tick();
    await tick();
    expect(opened).toBe(true);
    expect(got).toEqual(['{"x":1}', '{"x":2}']);
    const [url, init] = f.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('/api/traces/events');
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok');
  });

  it('signals an error when the stream ends so the client can reconnect', async () => {
    const f = vi.fn(async () => streamOf([]));
    const es = new (createFetchEventSource({ fetch: f as unknown as typeof fetch, headers: () => ({}) }))('/x');
    const onerror = vi.fn();
    es.onerror = onerror;
    await tick();
    await tick();
    expect(onerror).toHaveBeenCalledOnce();
    expect(es.readyState).toBe(2);
  });

  it('reports 401 and then errors', async () => {
    const f = vi.fn(async () => new Response('no', { status: 401 }));
    const onUnauthorized = vi.fn();
    const es = new (createFetchEventSource({ fetch: f as unknown as typeof fetch, headers: () => ({}), onUnauthorized }))('/x');
    const onerror = vi.fn();
    es.onerror = onerror;
    await tick();
    expect(onUnauthorized).toHaveBeenCalledOnce();
    expect(onerror).toHaveBeenCalledOnce();
  });

  it('does not report an error after close()', async () => {
    const f = vi.fn(async () => streamOf([]));
    const es = new (createFetchEventSource({ fetch: f as unknown as typeof fetch, headers: () => ({}) }))('/x');
    const onerror = vi.fn();
    es.onerror = onerror;
    es.close();
    await tick();
    await tick();
    expect(onerror).not.toHaveBeenCalled();
  });
});
