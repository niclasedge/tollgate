<script lang="ts">
  // Stage timeline of one request: what every pipeline stage did, plus the
  // return path. Everything shown is redacted data from the store.
  import { api } from '../api';
  import { formatTokens } from '../format';
  import type { StageDetail, TraceDetail } from '../types/dashboard';

  // status reloads the detail when a running request finishes.
  let { id, status }: { id: string; status: number } = $props();

  let detail = $state<TraceDetail | undefined>();
  let error = $state('');
  let openDiff = $state<Record<number, boolean>>({});

  $effect(() => {
    void status;
    let stale = false;
    api.traceDetail(id).then(
      (d) => {
        if (stale) return;
        detail = d;
        error = '';
      },
      (e: Error) => {
        if (!stale) error = e.message;
      },
    );
    return () => (stale = true);
  });

  const labels: Record<string, string> = {
    intake: 'Eingang',
    redact: 'Redact',
    compress: 'Kompression',
    meter: 'Messung',
    forward: 'Weiterleitung',
  };

  const ms = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 2 });
  const short = (h: string) => (h ? `${h.slice(0, 8)}…` : '–');

  function byType(s: StageDetail): Array<[string, number, string]> {
    const m = new Map<string, { n: number; ph: Set<string> }>();
    for (const x of s.matches) {
      const e = m.get(x.type) ?? { n: 0, ph: new Set<string>() };
      e.n++;
      if (x.placeholder) e.ph.add(x.placeholder);
      m.set(x.type, e);
    }
    return [...m].map(([t, e]) => [t, e.n, [...e.ph].join(', ')]);
  }

  // One row per placeholder: type, placeholder, mask (issue #112) and count.
  // The mask is computed by the redact stage; the value never gets here.
  function rows(s: StageDetail): Array<{ key: string; type: string; ph: string; mask: string; n: number }> {
    const m = new Map<string, { key: string; type: string; ph: string; mask: string; n: number }>();
    for (const x of s.matches) {
      const key = `${x.type}|${x.placeholder}|${x.mask}`;
      const e = m.get(key) ?? { key, type: x.type, ph: x.placeholder, mask: x.mask, n: 0 };
      e.n++;
      m.set(key, e);
    }
    return [...m.values()];
  }

  // Distinct placeholders the return path can translate back.
  const placeholders = $derived(new Set(detail?.stages.flatMap((s) => s.matches.map((m) => m.placeholder)).filter(Boolean)).size);
</script>

{#if error}
  <p class="err" role="alert">{error}</p>
{:else if detail}
  <ol class="tl" aria-label="Stufen-Timeline">
    {#each detail.stages as s, i (i)}
      <li>
        <span class="mark" aria-hidden="true"></span>
        <div class="head">
          <strong>{labels[s.name] ?? s.name}</strong>
          <span class="num muted">{ms.format(s.duration_us / 1000)} ms</span>
        </div>
        <div class="facts">
          <span title="Tokens vorher → nachher">{formatTokens(s.tokens_in)} → {formatTokens(s.tokens_out)}</span>
          {#if s.config_version > 0}<span>Konfiguration v{s.config_version}</span>{/if}
          {#if s.hash_in === s.hash_out}
            <span class="muted">keine Änderung</span>
            <span class="mono muted" title={s.hash_out}>{short(s.hash_out)}</span>
          {:else}
            <span class="mono muted"><span title={s.hash_in}>{short(s.hash_in)}</span> → <span title={s.hash_out}>{short(s.hash_out)}</span></span>
          {/if}
        </div>
        {#if s.matches.length > 0}
          <p class="hits">
            {#each byType(s) as [type, n, ph] (type)}
              <span title={ph}>{n} × {type}</span>
            {/each}
          </p>
          <table class="masks" aria-label={`Treffer der Stufe ${labels[s.name] ?? s.name}`}>
            <thead><tr><th scope="col">Typ</th><th scope="col">Platzhalter</th><th scope="col">Maske</th><th scope="col"><span class="sr">Anzahl</span></th></tr></thead>
            <tbody>
              {#each rows(s) as r (r.key)}
                <tr>
                  <td>{r.type}</td>
                  <td class="mono">{r.ph || '–'}</td>
                  <td class="mono" title="Maske">{r.mask || '–'}</td>
                  <td class="num muted">{r.n} ×</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
        {#if s.diff}
          <button type="button" class="link" aria-expanded={!!openDiff[i]} onclick={() => (openDiff[i] = !openDiff[i])}>
            {openDiff[i] ? 'Diff ausblenden' : 'Diff anzeigen'}
          </button>
          {#if openDiff[i]}<pre class="diff">{s.diff}</pre>{/if}
        {/if}
      </li>
    {/each}
    <li>
      <span class="mark back" aria-hidden="true"></span>
      <div class="head">
        <strong>Rückweg</strong>
        <span class="num muted">{status === 0 ? '–' : `${formatTokens(detail.trace.duration_ms)} ms`}</span>
      </div>
      <div class="facts">
        <span class="muted">
          {placeholders > 0 ? `${placeholders} Platzhalter rückübersetzbar` : 'nichts rückzuübersetzen'} · Antwort {status === 0 ? 'läuft' : status}
        </span>
      </div>
    </li>
  </ol>
{:else}
  <p class="muted">Lade Stufen …</p>
{/if}

<style>
  .tl { list-style: none; margin: 0.5rem 0 0; padding: 0; }
  .tl > li { position: relative; padding: 0 0 0.9rem 1.4rem; }
  .tl > li::before { content: ''; position: absolute; left: 0.27rem; top: 0.9rem; bottom: 0; width: 1px; background: var(--c-line); }
  .tl > li:last-child::before { display: none; }
  .mark { position: absolute; left: 0; top: 0.4rem; width: 0.6rem; height: 0.6rem; border-radius: 50%; background: var(--c-accent); }
  .mark.back { background: var(--c-idle); }
  .head { display: flex; gap: 0.75rem; align-items: baseline; }
  .facts { display: flex; flex-wrap: wrap; gap: 0.25rem 1rem; font-size: 0.85rem; }
  .hits { display: flex; flex-wrap: wrap; gap: 0.4rem; margin: 0.35rem 0 0; padding: 0; }
  .hits span { font-size: 0.8rem; padding: 0.1rem 0.55rem; border-radius: 999px; background: var(--c-accent-soft); color: var(--c-accent); }
  .masks { border-collapse: collapse; margin: 0.4rem 0 0; font-size: 0.8rem; max-width: 100%; }
  .masks th { text-align: left; font-weight: 500; color: var(--c-muted); padding: 0 0.9rem 0.15rem 0; }
  .masks td { padding: 0.12rem 0.9rem 0.12rem 0; vertical-align: baseline; overflow-wrap: anywhere; }
  .masks td:last-child, .masks th:last-child { padding-right: 0; text-align: right; white-space: nowrap; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
  @media (max-width: 520px) { .masks th:first-child, .masks td:first-child { display: none; } }
  .link { all: unset; cursor: pointer; font-size: 0.82rem; color: var(--c-accent); margin-top: 0.3rem; border-radius: 4px; }
  .link:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 2px; }
  .diff { margin: 0.4rem 0 0; padding: 0.6rem 0.75rem; background: var(--c-sidebar); border-radius: var(--radius); font: 0.8rem/1.45 var(--mono); white-space: pre-wrap; overflow-wrap: anywhere; max-height: 20rem; overflow: auto; }
</style>
