<script lang="ts">
  // Einschätzung (#136): do the accounts' weekly limits hold the 5-hour
  // windows we fill? A bar for need over capacity, the figures below.
  import { estimate, formatCount, missing } from '../capacity';
  import { formatCost } from '../format';
  import StatusDot from './StatusDot.svelte';
  import type { Capacity } from '../types/dashboard';

  let { capacity }: { capacity: Capacity } = $props();

  const est = $derived(estimate(capacity));
  const dot = $derived(est ? ({ reicht: 'ok', knapp: 'warn', 'reicht nicht': 'error' } as const)[est.verdict] : 'idle');
  // The verdict as a sentence about the accounts.
  const verdictText = $derived(est ? `${capacity.accounts} ${capacity.accounts === 1 ? `Account ${est.verdict}` : `Accounts ${est.verdict.replace('reicht', 'reichen')}`}` : '');
  const pct = $derived(est ? Math.round(est.share * 100) : 0);
  const wpw = $derived(capacity.windows_per_week);
  const fills = $derived(capacity.fills_per_day);
</script>

<section class="card" aria-label="Einschätzung">
  <div class="head">
    <h2>Einschätzung</h2>
    {#if est}
      <span class="verdict"><StatusDot state={dot} label={verdictText} />{verdictText}</span>
    {/if}
  </div>
  {#if est}
    <div class="bar" role="meter" aria-label="Bedarf an der Kapazität" aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.min(100, pct)}>
      <span class={dot} style:width="{Math.min(100, pct)}%"></span>
    </div>
    <p class="muted line">
      {formatCount(est.needPerWeek)} von {formatCount(est.capacityPerWeek)} 5-h-Fenstern pro Woche · {pct} %
      {#if capacity.free_this_week !== null}{' · '}diese Woche noch {formatCount(capacity.free_this_week)} frei{/if}
    </p>
  {:else}
    <p class="muted line">{missing(capacity)}.</p>
  {/if}
  <dl aria-label="Kennzahlen Einschätzung">
    <div>
      <dt>Ø 5-h-Fenster je Wochenlimit</dt>
      <dd>{wpw === null ? '–' : formatCount(wpw)}</dd>
      <dd class="small">
        aus {capacity.sample_accounts} {capacity.sample_accounts === 1 ? 'Woche' : 'Wochen'}{#if capacity.window_cost_nano_usd !== null}{' · '}1 Fenster ≈ {formatCost(capacity.window_cost_nano_usd)}{/if}
      </dd>
    </div>
    <div>
      <dt>Gefüllte 5-h-Fenster pro Tag</dt>
      <dd>{fills === null ? '–' : formatCount(fills)}</dd>
      <dd class="small">über {formatCount(capacity.measured_hours)} h gemessen</dd>
    </div>
    <div>
      <dt>Bedarf pro Woche</dt>
      <dd>{est ? formatCount(est.needPerWeek) : '–'}</dd>
      <dd class="small">5-h-Fenster (Prognose)</dd>
    </div>
    <div>
      <dt>Kapazität pro Woche</dt>
      <dd>{est ? formatCount(est.capacityPerWeek) : '–'}</dd>
      <dd class="small">{capacity.accounts} Accounts × Wochenlimit</dd>
    </div>
  </dl>
</section>

<style>
  .card { margin: 0 0 1.5rem; padding: 1rem 1.25rem; background: var(--c-sidebar); border-radius: var(--radius); }
  .head { display: flex; align-items: baseline; justify-content: space-between; flex-wrap: wrap; gap: 0.25rem 1rem; margin: 0 0 0.6rem; }
  h2 { margin: 0; font-size: 1rem; }
  .verdict { display: inline-flex; align-items: center; gap: 0.4rem; font-size: 0.9rem; }
  .bar { background: var(--c-line); height: 0.5rem; border-radius: 999px; overflow: hidden; }
  .bar > span { display: block; height: 100%; background: var(--c-accent); border-radius: inherit; transition: width var(--t); }
  .bar > .warn { background: var(--c-warn); }
  .bar > .error { background: var(--c-error); }
  .line { margin: 0.4rem 0 0.9rem; font-size: 0.85rem; }
  dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr)); gap: 0.75rem 1rem; margin: 0; }
  dt { font-size: 0.8rem; color: var(--c-muted); }
  dd { margin: 0.1rem 0 0; font-size: 1.5rem; font-weight: 700; letter-spacing: -0.01em; font-variant-numeric: tabular-nums; }
  dd.small { margin-top: 0.2rem; font-size: 0.8rem; font-weight: 400; letter-spacing: 0; color: var(--c-muted); }
</style>
