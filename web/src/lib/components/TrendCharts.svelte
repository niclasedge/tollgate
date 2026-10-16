<script lang="ts">
  // Cost and tokens/s curves of the last 7 days for the overview (#123),
  // the same charts as on Kosten & Performance. refresh reloads them.
  import { api } from '../api';
  import { auth } from '../auth.svelte';
  import { accountLabel, breakdown, costSeries, dayCosts, seriesColor } from '../costs';
  import { tpsByHour } from '../perf';
  import CostChart from './CostChart.svelte';
  import TpsChart from './TpsChart.svelte';
  import type { CostReport, PerfReport } from '../types/dashboard';

  let { refresh = 0, names = new Map() }: { refresh?: number; names?: ReadonlyMap<string, string> } = $props();

  let report = $state<CostReport | undefined>();
  let perf = $state<PerfReport | undefined>();
  let errors = $state<{ costs?: string; perf?: string }>({});

  $effect(() => {
    if (!auth.token) return;
    void refresh;
    let stale = false;
    void Promise.allSettled([api.costs(7), api.performance(7)]).then(([c, p]) => {
      if (stale) return;
      const msg = (r: PromiseSettledResult<unknown>) => (r.status === 'rejected' ? (r.reason as Error).message : undefined);
      errors = { costs: msg(c), perf: msg(p) };
      if (c.status === 'fulfilled') report = c.value;
      if (p.status === 'fulfilled') perf = p.value;
    });
    return () => (stale = true);
  });

  const perAccount = $derived(report ? breakdown(report.rows, 'account', (id) => accountLabel(id, names)) : []);
  const curve = $derived(report ? costSeries(report, perAccount.map((a) => a.key)) : undefined);
  const tps = $derived(perf ? tpsByHour(perf) : undefined);
</script>

<div class="trends">
  <div class="card">
    {#if errors.costs}<p class="err" role="alert">{errors.costs}</p>{/if}
    {#if curve && curve.buckets.length > 0}
      <CostChart data={curve} labels={perAccount.map((a) => a.label)} label="Kosten pro Stunde (UTC)" days={dayCosts(report?.rows ?? [])} />
      <ul class="legend" aria-label="Accounts">
        {#each perAccount as a, i (a.key)}
          <li><span class="swatch" style="color: {seriesColor(i)}" aria-hidden="true"></span>{a.label}</li>
        {/each}
      </ul>
    {:else if curve}
      <p class="muted">Keine Kosten in den letzten 7 Tagen.</p>
    {:else if !errors.costs}
      <p class="muted">Lade …</p>
    {/if}
  </div>
  <div class="card">
    {#if errors.perf}<p class="err" role="alert">{errors.perf}</p>{/if}
    {#if tps && tps.lines.length > 0}
      <TpsChart label="Tokens pro Sekunde · Median je Stunde (UTC)" hours={tps.hours} lines={tps.lines} />
    {:else if tps}
      <p class="muted">Noch keine Tokens/s in den letzten 7 Tagen.</p>
    {:else if !errors.perf}
      <p class="muted">Lade …</p>
    {/if}
  </div>
</div>

<style>
  .trends { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
  .card { margin: 0; padding: 1rem 1.25rem; background: var(--c-sidebar); border-radius: var(--radius); min-width: 0; }
  .card :global(.chart) { margin: 0; }
  .legend { list-style: none; margin: 0.6rem 0 0; padding: 0; display: flex; flex-wrap: wrap; gap: 0.35rem 1.25rem; font-size: 0.85rem; }
  .legend li { display: flex; align-items: center; gap: 0.4rem; }
  .swatch { width: 0.7rem; height: 0.7rem; border-radius: 50%; background: currentColor; flex: none; }
  @media (max-width: 900px) { .trends { grid-template-columns: 1fr; } }
</style>
