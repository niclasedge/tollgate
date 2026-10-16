<script lang="ts">
  // Performance details (#87, #116): speed per model, the slow verdict,
  // tokens/s, the influence of input and cache, redactor costs and the
  // speedtest. Since #121 a section of Kosten & Performance, which loads the
  // report for the shared period; onreload asks it to fetch again.
  import { api } from '../api';
  import { formatPercent, formatTokens } from '../format';
  import { formatTps, formatUs } from '../perf';
  import Sparkline from '../components/Sparkline.svelte';
  import InfluenceMatrix from '../components/InfluenceMatrix.svelte';
  import type { Dist, ModelPerf, PerfReport, SpeedtestResult } from '../types/dashboard';

  let { report, onreload }: { report: PerfReport; onreload: () => void } = $props();
  let open = $state<string | undefined>();

  const dist = (d: Dist | null) => (d ? `${formatUs(d.min_us)} / ${formatUs(d.median_us)} / ${formatUs(d.p90_us)} / ${formatUs(d.max_us)}` : '–');
  const slowShare = (m: ModelPerf) => (m.judged > 0 ? formatPercent(m.slow / m.judged) : '–');

  // Speedtest: models to test (up to 4) and an explicit confirmation.
  let selected = $state<string[]>([]);
  let extra = $state('');
  let confirming = $state(false);
  let running = $state(false);
  let results = $state<SpeedtestResult[] | undefined>();
  let testError = $state('');
  const models = $derived(report.models.map((m) => m.model));
  const chosen = $derived([...new Set([...selected, ...extra.split(',').map((s) => s.trim()).filter(Boolean)])]);

  function toggle(m: string) {
    selected = selected.includes(m) ? selected.filter((x) => x !== m) : [...selected, m];
  }

  async function runTest() {
    confirming = false;
    running = true;
    testError = '';
    try {
      results = (await api.speedtest(chosen)).results;
      onreload();
    } catch (e) {
      testError = (e as Error).message;
    } finally {
      running = false;
    }
  }

  const shown = $derived(results ?? report.speedtests);
</script>

<h2 class="section">Performance je Modell</h2>
{#if report.models.length === 0}
  <p class="muted">Noch keine abgeschlossenen Anfragen im Zeitraum.</p>
{:else}
  <div class="table-wrap">
    <table class="tbl" aria-label="Modelle">
      <thead>
        <tr>
          <th>Modell</th><th class="num">Anfragen</th><th class="num">langsam</th>
          <th class="num">TTFT min / median / p90 / max</th><th class="num">Dauer min / median / p90 / max</th>
          <th class="num">t/s median / p90</th>
        </tr>
      </thead>
      <tbody>
        {#each report.models as m (m.model)}
          <tr class="row" aria-expanded={open === m.model} onclick={() => (open = open === m.model ? undefined : m.model)}>
            <td><button type="button" class="link mono" aria-expanded={open === m.model}>{m.model}</button></td>
            <td class="num">{formatTokens(m.requests)}</td>
            <td class="num" class:err={m.slow > 0}>{slowShare(m)}</td>
            <td class="num">{dist(m.ttft)}</td>
            <td class="num">{dist(m.duration)}</td>
            <td class="num">{m.output_tps ? `${formatTps(m.output_tps.median)} / ${formatTps(m.output_tps.p90)}` : '–'}</td>
          </tr>
          {#if open === m.model}
            <tr class="detail"><td colspan="6">
              <div class="trend">
                <div><span class="muted">TTFT (Median)</span><Sparkline label="TTFT je Tag" values={m.days.map((d) => d.ttft_median_us)} marks={m.days.map((d) => d.slow > 0)} /></div>
                <div><span class="muted">Dauer (Median)</span><Sparkline label="Dauer je Tag" values={m.days.map((d) => d.duration_median_us)} marks={m.days.map((d) => d.slow > 0)} /></div>
                <div><span class="muted">Tokens/s (Median)</span><Sparkline label="Tokens/s je Tag" values={m.days.map((d) => d.tps_median_milli)} /></div>
                <div><span class="muted">Cache-Anteil (Median)</span><Sparkline label="Cache-Anteil je Tag" values={m.days.map((d) => d.cache_share_permille_median)} /></div>
              </div>
              <p class="muted">tollgate-Eigenanteil (min / median / p90 / max): {dist(m.tollgate)} · Rest ist Upstream und Netz.</p>
              <h3 class="sub">Einfluss von Eingabe und Cache</h3>
              <InfluenceMatrix influence={m.influence} />
            </td></tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<h2 class="section">Redactor-Kosten</h2>
{#if report.redactor}
  {@const r = report.redactor}
  <dl class="totals">
    <div><dt>Stufe Redact</dt><dd>{dist(r.stage)}</dd></div>
    <div><dt>Rückweg</dt><dd>{dist(r.return)}</dd></div>
    <div><dt>Anteil an der Dauer</dt><dd>{formatPercent(r.share_median_permille / 1000)}</dd></div>
    <div><dt>Bytes (Median)</dt><dd>{formatTokens(r.bytes_in_median)}</dd></div>
    <div><dt>Treffer</dt><dd>{formatTokens(r.matches_total)} (Median {formatTokens(r.matches_median)})</dd></div>
    <div><dt>Rückübersetzt</dt><dd>{formatTokens(r.return_replacements_total)}</dd></div>
    <div><dt>Zusätzliche Tokens</dt><dd>{formatTokens(r.tokens_added_total)} (Median {formatTokens(r.tokens_added_median)})</dd></div>
  </dl>
  <p class="muted hint">Die Platzhalter sind deterministisch. Der Redactor ändert den Prompt-Cache nicht; zusätzliche Tokens sind eine Schätzung (Bytes / 4).</p>
{:else}
  <p class="muted">Keine Anfragen mit Redact-Stufe im Zeitraum.</p>
{/if}

<h2 class="section">Speedtest</h2>
<p class="muted hint">Schickt je Modell eine feste Anfrage ohne und mit Redact-Stufe. Der Test verbraucht Abo-Kontingent.</p>
<div class="pick">
  {#each models as m}
    <label><input type="checkbox" checked={selected.includes(m)} onchange={() => toggle(m)} /> <span class="mono">{m}</span></label>
  {/each}
  <label class="extra">Weitere Modelle <input type="text" bind:value={extra} placeholder="kommagetrennt" /></label>
</div>
{#if confirming}
  <div class="confirm" role="alertdialog" aria-label="Speedtest bestätigen">
    <span>Der Test verbraucht Abo-Kontingent ({chosen.length * 2} Anfragen). Fortfahren?</span>
    <button type="button" class="btn primary" onclick={runTest}>Ja, testen</button>
    <button type="button" class="btn" onclick={() => (confirming = false)}>Abbrechen</button>
  </div>
{:else}
  <button type="button" class="btn" disabled={running || chosen.length === 0 || chosen.length > 4} onclick={() => (confirming = true)}>
    {running ? 'Test läuft …' : 'Speedtest starten'}
  </button>
  {#if chosen.length > 4}<span class="err"> höchstens 4 Modelle</span>{/if}
{/if}
{#if testError}<p class="err" role="alert">{testError}</p>{/if}
{#if shown.length > 0}
  <div class="table-wrap">
    <table class="tbl" aria-label="Speedtest-Ergebnisse">
      <thead><tr><th>Modell</th><th>Variante</th><th class="num">TTFT</th><th class="num">Dauer</th><th class="num">t/s</th><th class="num">üblich (Dauer)</th></tr></thead>
      <tbody>
        {#each shown as s (s.model + s.redact)}
          <tr>
            <td class="mono">{s.model}</td>
            <td>{s.redact ? 'mit Redact' : 'ohne Redact'}</td>
            {#if s.error}
              <td colspan="4" class="err">{s.error}</td>
            {:else}
              <td class="num">{formatUs(s.ttft_us)}</td>
              <td class="num">{formatUs(s.duration_us)}</td>
              <td class="num">{formatTps(s.output_tps)}</td>
              <td class="num">{formatUs(s.usual_duration_median_us)}</td>
            {/if}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .row { cursor: pointer; }
  .row:hover { background: var(--c-hover); }
  .link { all: unset; cursor: pointer; }
  .link:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 2px; }
  .detail td { background: var(--c-surface); animation: fade var(--t) ease-out; }
  .trend { display: flex; flex-wrap: wrap; gap: 1rem 2rem; margin: 0.25rem 0 0.75rem; }
  .trend > div { display: grid; gap: 0.25rem; font-size: 0.8rem; }
  .sub { font-size: 0.95rem; font-weight: 600; margin: 1rem 0 0.4rem; }
  .totals { display: flex; flex-wrap: wrap; gap: 1rem 2rem; margin: 0; }
  .totals dt { color: var(--c-muted); font-size: 0.8rem; }
  .totals dd { margin: 0; font-variant-numeric: tabular-nums; }
  .hint { font-size: 0.85rem; }
  .pick { display: flex; flex-wrap: wrap; gap: 0.5rem 1.25rem; margin: 0.5rem 0 0.75rem; font-size: 0.9rem; }
  .extra input { font: inherit; font-size: 0.85rem; padding: 0.2rem 0.4rem; border: 1px solid var(--c-line); border-radius: var(--radius); background: var(--c-surface); color: var(--c-text); }
  .confirm { display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem; }
  @keyframes fade { from { opacity: 0; } }
</style>
