// Minimal SSE client: JSON-decoded named events, status callbacks and
// reconnect with exponential backoff once the browser gave up on the source.

export type SseStatus = 'connecting' | 'open' | 'reconnecting' | 'closed';

export interface EventSourceLike {
  readyState: number;
  onopen: (() => void) | null;
  onerror: (() => void) | null;
  addEventListener(type: string, listener: (e: { data: string }) => void): void;
  close(): void;
}

export interface SseOptions {
  url: string;
  /** Event names to subscribe to (the default unnamed event is "message"). */
  events: string[];
  onEvent: (type: string, data: unknown) => void;
  onStatus?: (status: SseStatus) => void;
  /** Called when the connection is open again after a loss (events may have been missed). */
  onReconnect?: () => void;
  /** Injectable for tests; defaults to the global EventSource. */
  eventSource?: new (url: string) => EventSourceLike;
  retryMs?: number;
  maxRetryMs?: number;
}

export interface SseClient {
  close(): void;
}

const CLOSED = 2;

function decode(data: string): unknown {
  try {
    return JSON.parse(data);
  } catch {
    return data;
  }
}

export function createSseClient(opts: SseOptions): SseClient {
  const Source = opts.eventSource ?? (EventSource as unknown as new (url: string) => EventSourceLike);
  const base = opts.retryMs ?? 1000;
  const max = opts.maxRetryMs ?? 30000;
  let delay = base;
  let source: EventSourceLike | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let stopped = false;
  let everOpened = false;

  const status = (s: SseStatus) => opts.onStatus?.(s);

  function connect() {
    status('connecting');
    const es = new Source(opts.url);
    source = es;
    es.onopen = () => {
      delay = base;
      status('open');
      if (everOpened) opts.onReconnect?.();
      everOpened = true;
    };
    es.onerror = () => {
      if (stopped) return;
      status('reconnecting');
      // While readyState is CONNECTING the browser retries by itself.
      if (es.readyState !== CLOSED) return;
      es.close();
      timer = setTimeout(connect, delay);
      delay = Math.min(delay * 2, max);
    };
    for (const type of opts.events) {
      es.addEventListener(type, (e) => opts.onEvent(type, decode(e.data)));
    }
  }

  connect();

  return {
    close() {
      stopped = true;
      clearTimeout(timer);
      source?.close();
      status('closed');
    },
  };
}

// --- fetch-based source -----------------------------------------------------
// EventSource cannot send an Authorization header, and the admin token must
// never appear in a URL. This source speaks the same EventSourceLike interface
// on top of fetch + ReadableStream, so createSseClient (status, reconnect with
// backoff) works unchanged.

export interface SseMessage {
  event: string;
  data: string;
}

/** Incremental text/event-stream parser; chunks may split anywhere. */
const MAX_BUFFER = 1_000_000;

export function createSseParser(onMessage: (m: SseMessage) => void) {
  let buf = '';
  let event = '';
  let data: string[] = [];
  let dataSize = 0;

  function line(l: string) {
    if (l === '') {
      if (data.length) onMessage({ event: event || 'message', data: data.join('\n') });
      event = '';
      data = [];
      dataSize = 0;
      return;
    }
    if (l.startsWith(':')) return; // comment / heartbeat
    const i = l.indexOf(':');
    const field = i < 0 ? l : l.slice(0, i);
    let value = i < 0 ? '' : l.slice(i + 1);
    if (value.startsWith(' ')) value = value.slice(1);
    if (field === 'event') event = value;
    else if (field === 'data') {
      dataSize += value.length;
      if (dataSize > MAX_BUFFER) throw new Error('SSE event too large');
      data.push(value);
    }
  }

  return {
    push(chunk: string) {
      buf += chunk;
      if (buf.length > MAX_BUFFER) throw new Error('SSE line too long');
      for (;;) {
        const m = /\r\n|\n|\r/.exec(buf);
        if (!m) return;
        // A trailing CR may be the first half of CRLF: wait for the next chunk.
        if (m[0] === '\r' && m.index === buf.length - 1) return;
        line(buf.slice(0, m.index));
        buf = buf.slice(m.index + m[0].length);
      }
    },
  };
}

export interface FetchSourceOptions {
  /** Injectable for tests; defaults to the global fetch. */
  fetch?: typeof fetch;
  headers: () => Record<string, string>;
  onUnauthorized?: () => void;
}

export function createFetchEventSource(o: FetchSourceOptions): new (url: string) => EventSourceLike {
  return class FetchEventSource implements EventSourceLike {
    readyState = 0;
    onopen: (() => void) | null = null;
    onerror: (() => void) | null = null;
    private ctrl = new AbortController();
    private listeners = new Map<string, Array<(e: { data: string }) => void>>();
    private userClosed = false;

    constructor(url: string) {
      void this.run(url);
    }

    addEventListener(type: string, listener: (e: { data: string }) => void): void {
      this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
    }

    close(): void {
      this.userClosed = true;
      this.readyState = CLOSED;
      this.ctrl.abort();
    }

    private async run(url: string) {
      try {
        const f = o.fetch ?? ((...a: Parameters<typeof fetch>) => fetch(...a));
        const res = await f(url, { headers: { ...o.headers(), Accept: 'text/event-stream' }, signal: this.ctrl.signal });
        if (res.status === 401) o.onUnauthorized?.();
        if (!res.ok || !res.body) throw new Error(`SSE ${res.status}`);
        if (this.userClosed) return;
        this.readyState = 1;
        this.onopen?.();
        const parser = createSseParser((m) => this.listeners.get(m.event)?.forEach((l) => l({ data: m.data })));
        const dec = new TextDecoder();
        const reader = res.body.getReader();
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          parser.push(dec.decode(value, { stream: true }));
        }
      } catch {
        // fall through: reported as an error below
      }
      if (this.userClosed) return;
      this.readyState = CLOSED;
      this.onerror?.();
    }
  };
}
