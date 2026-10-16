<script lang="ts">
  import { api } from '../api';
  import { auth, logout } from '../auth.svelte';
  import { createFetchEventSource, createSseClient, type SseStatus } from '../sse';
  import { formatCost, formatSavedDelta, formatTime, formatTokens, totalTokens } from '../format';
  import StatusDot from '../components/StatusDot.svelte';
  import Timeline from '../components/Timeline.svelte';
  import { router } from '../router.svelte';
  import { explainPerf, formatMs, tpsCell, tpsSentence, traceDot } from '../perf';
  import type { TraceEvent, TraceSummary } from '../types/dashboard';

  const CAP = 200;

  let traces = $state<TraceSummary[]>([]);
  let conn = $state<SseStatus>('connecting');
  let error = $state('');
  let loaded = $state(false);
  // #/live/<id> (e.g. from the overview) opens that request.
  let openId = $state<string | undefined>(router.current.detail);

  function newestFirst(a: TraceSummary, b: TraceSummary): number {
    return a.started_at < b.started_at ? 1 : a.started_at > b.started_at ? -1 : a.id < b.id ? 1 : -1;
  }

  function upsert(list: TraceSummary[], t: TraceSummary, replace = true): TraceSummary[] {
    const i = list.findIndex((x) => x.id === t.id);
    if (i >= 0 && !replace) return list;
    const next = i >= 0 ? list.map((x, j) => (j === i ? t : x)) : [...list, t];
    return next.sort(newestFirst).slice(0, CAP);
  }

  // Events win over the snapshot on first load (they may be newer); after a
  // reconnect the snapshot wins, because it holds what the gap swallowed.
  async function snapshot(replace: boolean) {
    try {
      const l = await api.traces({ limit: 50 });
      for (const t of l.traces) traces = upsert(traces, t, replace);
      loaded = true;
      error = '';
    } catch (e) {
      error = (e as Error).message;
    }
  }

  $effect(() => {
    if (!auth.token) return;
    const sse = createSseClient({
      url: '/api/traces/events',
      events: ['trace'],
      eventSource: createFetchEventSource({ headers: () => api.headers(), onUnauthorized: () => logout(true) }),
      onEvent: (_, data) => {
        const t = (data as TraceEvent | undefined)?.trace;
        if (t) traces = upsert(traces, t);
      },
      onStatus: (s) => (conn = s),
      // Events from the gap are lost: refetch so stale "läuft" rows get their result.
      onReconnect: () => void snapshot(true),
    });
    void snapshot(false);
    return () => sse.close();
  });

  const dot = traceDot;

  function arrows(e: KeyboardEvent) {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    const btns = [...(e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('.row-btn')];
    const i = btns.indexOf(document.activeElement as HTMLButtonElement);
    if (i < 0) return;
    e.preventDefault();
    btns[Math.max(0, Math.min(btns.length - 1, i + (e.key === 'ArrowDown' ? 1 : -1)))]?.focus();
  }

  const connLabel: Record<SseStatus, string> = { connecting: 'Verbinde …', open: 'Live', reconnecting: 'Verbindung unterbrochen', closed: 'Getrennt' };
  const connState = (s: SseStatus) => (s === 'open' ? 'ok' : s === 'reconnecting' ? 'error' : 'idle');
</script>

<div class="page-head">
  <h1>Live</h1>
  <span class="conn"><StatusDot state={connState(conn)} label="Verbindung" />{connLabel[conn]}</span>
</div>

{#if error}<p class="err" role="alert">{error}</p>{/if}
{#if loaded && traces.length === 0}
  <p class="muted">Noch keine Anfragen. Sie erscheinen hier, sobald ein Client tollgate nutzt.</p>
{/if}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<ul class="list" onkeydown={arrows}>
  {#each traces as t (t.id)}
    {@const d = dot(t)}
    {@const c = tpsCell(t)}
    <li>
      <button type="button" class="row-btn" aria-expanded={openId === t.id} onclick={() => (openId = openId === t.id ? undefined : t.id)}>
        <StatusDot state={d.state} label={d.label} />
        <span class="time num">{formatTime(t.started_at)}</span>
        <span class="client">{t.client_name || '–'}<small class="account" title={t.account_id}>{t.account_id ? `Account ${t.account_id.slice(0,12)}` : 'Account unbekannt'}</small></span>
        <span class="model mono">{t.model || '–'}</span>
        <span class="tok num" title="Tokens gesamt (Ein / Aus / Cache)">
          {#if t.usage}{formatTokens(totalTokens(t.usage))}{:else}–{/if}
        </span>
        <span class="tps num {c.tone}" title={c.title} data-testid="tps">{c.text}{#if c.sub}<small>{c.sub}</small>{/if}</span>
        <span class="saved num" title="Gesparte Tokens">{formatSavedDelta(t.saved_tokens)}</span>
        <span class="cost num">{formatCost(t.cost_nano_usd)}</span>
      </button>
      {#if openId === t.id}
        <dl class="detail">
          <dt>Endpunkt</dt><dd>{t.endpoint}{t.stream ? ' (Stream)' : ''}</dd>
          <dt>Status</dt><dd>{t.status === 0 ? 'läuft' : t.status}</dd>
          <dt>Dauer</dt><dd>{t.status === 0 ? '–' : `${formatTokens(t.duration_ms)} ms`}</dd>
          {#if t.status !== 0}
            <dt>TTFT</dt><dd>{formatMs(t.ttft_ms)}</dd>
            <dt>Tokens/s</dt><dd>{t.output_tps === null ? '–' : tpsSentence(t.output_tps, t.tps)}</dd>
            <dt>tollgate-Anteil</dt><dd>{formatMs(t.tollgate_ms)}</dd>
            <dt>Einordnung</dt>
            <dd class:slow={t.perf?.slow}>
              {#if t.perf}{#each explainPerf(t) as line}<span class="line">{line}</span>{/each}{:else}noch zu wenig Vergleichswerte{/if}
            </dd>
          {/if}
          <dt>Schätzung</dt><dd>{formatTokens(t.estimated_input_tokens)} Tokens Eingabe</dd>
          {#if t.usage}
            <dt>Usage</dt>
            <dd>Eingabe {formatTokens(t.usage.input)} · Ausgabe {formatTokens(t.usage.output)} · Cache gelesen {formatTokens(t.usage.cache_read)} · Cache angelegt {formatTokens(t.usage.cache_creation)}</dd>
          {/if}
          {#if t.error_kind}<dt>Fehlerart</dt><dd class="err">{t.error_kind}</dd>{/if}
          <dt>Account-ID</dt><dd class="mono">{t.account_id || 'Nicht zugeordnet'}</dd>
          <dt>Trace-ID</dt><dd class="mono">{t.id}</dd>
        </dl>
        <div class="stages"><Timeline id={t.id} status={t.status} /></div>
      {/if}
    </li>
  {/each}
</ul>

<style>
  .list { list-style: none; margin: 0; padding: 0; }
  li { border-bottom: 1px solid var(--c-line); }
  .row-btn { all: unset; box-sizing: border-box; width: 100%; display: grid; grid-template-columns: 1rem 5rem 1fr 1.4fr 5rem 3.5rem 4.5rem 4.5rem; align-items: center; gap: 0.75rem; padding: 0.7rem 0.4rem; cursor: pointer; border-radius: var(--radius); font-size: 0.92rem; }
  .row-btn:focus-visible { outline: 2px solid var(--c-accent); outline-offset: -2px; }
  .row-btn:hover { background: var(--c-hover); }
  .row-btn > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .account { display:block; color:var(--c-muted); font-size:0.75rem; overflow:hidden; text-overflow:ellipsis; }
  .time, .saved { color: var(--c-muted); }
  .tps { line-height: 1.1; }
  .tps small { display: block; font-size: 0.72rem; }
  .tps.usual small { color: var(--c-muted); }
  .tps.slower { color: var(--c-error); }
  .tps.faster { color: var(--c-accent); }
  .line { display: block; }
  dd.slow { color: var(--c-error); }
  .detail { display: grid; grid-template-columns: max-content 1fr; gap: 0.25rem 1rem; margin: 0; padding: 0.25rem 0.4rem 1rem 2.15rem; font-size: 0.9rem; animation: fade var(--t) ease-out; }
  .stages { padding: 0 0.4rem 1rem 2.15rem; }
  dt { color: var(--c-muted); }
  dd { margin: 0; overflow-wrap: anywhere; }
  @keyframes fade { from { opacity: 0; transform: translateY(-2px); } }
  @media (max-width: 720px) {
    .row-btn { grid-template-columns: 1rem 1fr 4.5rem 4rem; }
    .time, .model, .saved, .tps { display: none; }
  }
</style>
