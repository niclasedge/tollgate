<script lang="ts">
  // Kosten & Performance (#121): a dashboard with the key numbers and graphs
  // per account on top, the breakdown and the performance details below.
  import { api } from '../api';
  import { auth } from '../auth.svelte';
  import { accountLabel, breakdown, costLabel, costSeries, dayCosts, metrics, seriesColor, total, type BreakdownBy } from '../costs';
  import { cacheHitRate, formatCost, formatDateTime, formatPercent, formatTokens, totalTokens } from '../format';
  import { formatUs, tpsByHour } from '../perf';
  import { fillLine, formatSpan } from '../fill';
  import CostChart from '../components/CostChart.svelte';
  import TpsChart from '../components/TpsChart.svelte';
  import Performance from './Performance.svelte';
  import CapacityCard from '../components/CapacityCard.svelte';
  import type { CostReport, PerfReport } from '../types/dashboard';

  const periods = [
    { days: 1, label: 'Heute' },
    { days: 7, label: '7 Tage' },
    { days: 30, label: '30 Tage' },
  ];
  let days = $state(7);
  let report = $state<CostReport | undefined>();
  let perf = $state<PerfReport | undefined>();
  let error = $state('');
  let perfError = $state('');
  let perfReload = $state(0);
  let names = $state<ReadonlyMap<string, string>>(new Map());

  $effect(() => {
    if (!auth.token) return;
    const d = days;
    let stale = false;
    error = '';
    api
      .costs(d)
      .then((r) => !stale && (report = r))
      .catch((e: Error) => !stale && (error = e.message));
    return () => (stale = true);
  });

  $effect(() => {
    if (!auth.token) return;
    const d = days;
    void perfReload;
    let stale = false;
    perfError = '';
    api
      .performance(d)
      .then((r) => !stale && (perf = r))
      .catch((e: Error) => !stale && (perfError = e.message));
    return () => (stale = true);
  });

  // Names as in the Accounts area; without account management the fallback label stays.
  $effect(() => {
    if (!auth.token) return;
    let stale = false;
    api.listAccounts().then(
      (list) => !stale && (names = new Map(list.map((a) => [a.id, a.name]))),
      () => {},
    );
    return () => (stale = true);
  });

  const label = (id: string) => accountLabel(id, names);
  const rows = $derived(report?.rows ?? []);
  const sum = $derived(total(rows));
  const perAccount = $derived(breakdown(rows, 'account', label));
  const figures = $derived(metrics(rows));
  const curve = $derived(report ? costSeries(report, perAccount.map((a) => a.key)) : undefined);
  const curveLabel = $derived(days > 7 ? 'Kosten pro Stunde (UTC) · letzte 7 Tage' : 'Kosten pro Stunde (UTC)');

  const views: { by: BreakdownBy; label: string }[] = [
    { by: 'account', label: 'Account' },
    { by: 'model', label: 'Modell' },
    { by: 'client', label: 'Client' },
    { by: 'day', label: 'Tag' },
  ];
  let by = $state<BreakdownBy>('account');
  const table = $derived(breakdown(rows, by, label));

  const tpsTrend = $derived(perf ? tpsByHour(perf) : undefined);
  const savings = $derived(
    figures.cache_savings_partial && figures.cache_savings_nano_usd !== null
      ? `≥ ${formatCost(figures.cache_savings_nano_usd)}`
      : formatCost(figures.cache_savings_nano_usd),
  );
  const share = (s: number | null) => formatPercent(s);
  const fill = $derived(perf?.fastest_fill ?? null);
</script>

<div class="page-head">
  <h1>Kosten &amp; Performance</h1>
  <div class="seg" role="group" aria-label="Zeitraum">
    {#each periods as p}
      <button type="button" aria-pressed={days === p.days} onclick={() => (days = p.days)}>{p.label}</button>
    {/each}
  </div>
</div>

{#if error}<p class="err" role="alert">{error}</p>{/if}
{#if report}
  <div class="dash">
    <section class="card total" aria-label="Gesamtkosten">
      <p class="muted cap">Gesamtkosten (API-Schätzung)</p>
      <p class="big">{costLabel(sum)}</p>
      <p class="muted">{formatTokens(sum.requests)} Anfragen</p>
      <ul class="accounts" aria-label="Kosten je Account">
        {#each perAccount as a, i (a.key)}
          <li>
            <span class="swatch" style="color: {seriesColor(i)}" aria-hidden="true"></span>
            <span class="name">{a.label}</span>
            <span class="num">{costLabel(a)}</span>
            <span class="num muted">{share(a.share)}</span>
            <span class="num muted tok">{formatTokens(totalTokens(a.usage))} Tokens</span>
          </li>
        {:else}
          <li class="muted">Keine Anfragen im Zeitraum.</li>
        {/each}
      </ul>
    </section>
    <section class="card curve" aria-label="Kostenkurve">
      {#if curve && curve.buckets.length > 0}
        <CostChart data={curve} labels={perAccount.map((a) => a.label)} label={curveLabel} days={dayCosts(rows)} />
      {:else if curve}
        <p class="muted">Keine Kosten im Zeitraum.</p>
      {/if}
    </section>
  </div>

  <!-- Tiles: one big number, details in a small line below (#134). -->
  <dl class="figures" aria-label="Kennzahlen">
    <div>
      <dt>Tokens verarbeitet</dt><dd>{formatTokens(figures.processed)}</dd>
      <dd class="small" title="Eingabe ungecacht: ohne Cache-Treffer, einschließlich Cache-Schreiben">
        Ausgabe {formatTokens(figures.output)} · ungecacht {formatTokens(figures.uncached_input)}
      </dd>
    </div>
    <div title="Preisdifferenz Eingabe zu Cache-Lesen">
      <dt>Cache-Ersparnis</dt><dd>{savings}</dd>
      <dd class="small">{formatTokens(figures.cache_read)} gelesen · Treffer {formatPercent(cacheHitRate(sum.usage))}</dd>
    </div>
    <div>
      <dt>Von tollgate gespart</dt><dd>{formatTokens(figures.saved_tokens)}</dd>
      <dd class="small">Tokens vor dem Senden (Schätzung)</dd>
    </div>
    <div title={fill ? `Fenster bis ${formatDateTime(fill.reset)}` : undefined}>
      <dt>Schnellstes 5-h-Fenster</dt>
      {#if fill}
        <dd>{formatSpan(fill.full_us)} <span class="unit">bis 100 %</span></dd>
        <dd class="small">{fillLine(fill)} · {label(fill.account_id)}</dd>
      {:else}
        <dd>–</dd>
        <dd class="small">Noch kein Fenster mit 30 Punkten Anstieg gemessen</dd>
      {/if}
    </div>
  </dl>
{:else if !error}
  <p class="muted">Lade …</p>
{/if}

{#if perfError}<p class="err" role="alert">{perfError}</p>{/if}
{#if perf}<CapacityCard capacity={perf.capacity} />{/if}
{#if perf && perf.models.length > 0}
  <div class="dash perf">
    <section class="card" aria-label="Tokens pro Sekunde">
      {#if tpsTrend && tpsTrend.lines.length > 0}
        <TpsChart label="Tokens pro Sekunde · Median je Stunde (UTC)" hours={tpsTrend.hours} lines={tpsTrend.lines} />
      {:else}
        <p class="muted">Noch keine Tokens/s im Zeitraum.</p>
      {/if}
    </section>
    <ul class="card models" aria-label="Performance je Modell">
      {#each perf.models as m (m.model)}
        <li>
          <span class="mono">{m.model}</span>
          <span class="muted num">TTFT {formatUs(m.ttft?.median_us)}</span>
          <span class="num" class:err={m.slow > 0}>langsam {m.judged > 0 ? formatPercent(m.slow / m.judged) : '–'}</span>
        </li>
      {/each}
    </ul>
  </div>
{/if}

{#if report}
  <div class="break-head">
    <h2 class="section">Aufschlüsselung</h2>
    <div class="seg" role="group" aria-label="Aufschlüsselung nach">
      {#each views as v}
        <button type="button" aria-pressed={by === v.by} onclick={() => (by = v.by)}>{v.label}</button>
      {/each}
    </div>
  </div>
  <div class="table-wrap">
    <table class="tbl" aria-label="Aufschlüsselung">
      <thead>
        <tr>
          <th>{views.find((v) => v.by === by)?.label}</th><th class="num">Anfragen</th><th class="num">Tokens</th><th class="num">Cache</th>
          <th class="num">Cache-Ersparnis</th><th class="num">Kosten</th><th class="num">Anteil</th>
        </tr>
      </thead>
      <tbody>
        {#each table as a (a.key)}
          <tr>
            <td class:mono={by === 'model' || by === 'day'}>{a.label}</td>
            <td class="num">{formatTokens(a.requests)}</td>
            <td class="num">{formatTokens(totalTokens(a.usage))}</td>
            <td class="num">{formatPercent(cacheHitRate(a.usage))}</td>
            <td class="num">{formatCost(a.cache_savings_nano_usd)}</td>
            <td class="num cost">{costLabel(a)}</td>
            <td class="num muted">{share(a.share)}</td>
          </tr>
        {:else}
          <tr><td colspan="7" class="muted">Keine Daten im Zeitraum.</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if perf}<Performance report={perf} onreload={() => perfReload++} />{/if}

<style>
  .dash { display: grid; grid-template-columns: minmax(16rem, 1fr) 2fr; gap: 1rem; margin: 0 0 1rem; }
  .dash.perf { grid-template-columns: 2fr minmax(16rem, 1fr); }
  .card { margin: 0; padding: 1rem 1.25rem; background: var(--c-sidebar); border-radius: var(--radius); min-width: 0; }
  .cap { margin: 0; font-size: 0.8rem; }
  .big { margin: 0.1rem 0 0; font-size: clamp(2rem, 5vw, 2.75rem); font-weight: 700; letter-spacing: -0.02em; font-variant-numeric: tabular-nums; }
  .total > .muted { margin: 0 0 1rem; }
  .accounts, .models { list-style: none; margin: 0; padding: 0; display: grid; gap: 0.45rem; font-size: 0.9rem; }
  .accounts li { display: grid; grid-template-columns: 0.75rem 1fr auto 3.75rem; gap: 0.25rem 0.6rem; align-items: center; }
  .accounts .tok { grid-column: 2 / -1; text-align: left; font-size: 0.8rem; margin-top: -0.3rem; }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  /* Same color as the account's lane in the cost curve (#128). */
  .swatch { width: 0.7rem; height: 0.7rem; border-radius: 50%; background: currentColor; flex: none; }
  .models { align-content: start; }
  .models li { display: flex; flex-wrap: wrap; gap: 0.15rem 0.75rem; align-items: baseline; padding-bottom: 0.45rem; border-bottom: 1px solid var(--c-line); }
  .models li:last-child { border-bottom: 0; padding-bottom: 0; }
  .models .mono { flex: 1 0 100%; }
  .figures { display: grid; grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr)); gap: 1rem; margin: 0 0 1.5rem; }
  .figures div { padding: 0.75rem 1rem; background: var(--c-sidebar); border-radius: var(--radius); }
  dt { font-size: 0.8rem; color: var(--c-muted); }
  dd { margin: 0.1rem 0 0; font-size: 1.75rem; font-weight: 700; letter-spacing: -0.01em; font-variant-numeric: tabular-nums; }
  dd.small { margin-top: 0.2rem; font-size: 0.8rem; font-weight: 400; letter-spacing: 0; color: var(--c-muted); }
  .unit { font-size: 0.85rem; font-weight: 400; color: var(--c-muted); }
  .break-head { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 0.5rem 1rem; margin: 2rem 0 0.5rem; }
  .break-head .section { margin: 0; }
  @media (max-width: 720px) {
    .dash, .dash.perf { grid-template-columns: 1fr; }
  }
</style>
