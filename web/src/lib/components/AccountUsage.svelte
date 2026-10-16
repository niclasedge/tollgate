<script lang="ts">
  import type { UsageSnapshot } from '../types/accounts';
  import { formatDateTime, formatUntil } from '../format';
  import { elapsedPercent, FIVE_HOURS_MS, WEEK_MS } from '../limits';
  import type { CostHour } from '../types/dashboard';
  import { costLabel } from '../costs';
  import { windowCost } from '../windows';
  import StatusDot from './StatusDot.svelte';
  // hours (overview only, #125) adds the account's costs inside each running window.
  let { usage, loading = false, accountId = '', hours }: { usage?: UsageSnapshot; loading?: boolean; accountId?: string; hours?: readonly CostHour[] } = $props();
  let now = $state(Date.now());
  $effect(() => { const timer = setInterval(() => { now = Date.now(); }, 30_000); return () => clearInterval(timer); });
  const week = $derived(usage?.seven_day);
  const reset = $derived(week?.reset ? Date.parse(week.reset) : NaN);
  const resetPassed = $derived(Number.isFinite(reset) && reset <= now);
  const weekObservedAt = $derived(week?.observed_at || usage?.observed_at || '');
  const stale = $derived(!!weekObservedAt && now - Date.parse(weekObservedAt) > 10 * 60_000);
  const used = $derived(week?.utilization ?? null);
  const observed = $derived(weekObservedAt ? Date.parse(weekObservedAt) : NaN);
  const knownWindow = $derived(Number.isFinite(reset) && reset > now && reset - now <= 7 * 86_400_000 && Number.isFinite(observed) && observed > 0 && observed <= now + 60_000);
  const progress = $derived(elapsedPercent(week?.reset, WEEK_MS, now));
  type Dot = 'ok' | 'warn' | 'error' | 'idle';
  const dotState = $derived<Dot>(!knownWindow || used === null || stale || !!usage?.error ? 'idle' : used >= 100 ? 'error' : used >= 80 ? 'warn' : 'ok');
  const label = $derived(resetPassed ? 'Reset erfolgt · neuer Messstand nötig' : !knownWindow || used === null ? 'Wochenstatus unbekannt' : used >= 100 ? 'Wochenlimit erreicht' : `${stale || usage?.error ? 'Zuletzt ' : ''}${Math.round(Math.max(0, 100 - used))} % übrig`);
  const five = $derived(usage?.five_hour);
  const fiveExpired = $derived(!!five?.reset && Date.parse(five.reset) <= now);
  const fiveUsed = $derived(five?.utilization ?? null);
  const fiveObserved = $derived(five?.observed_at || usage?.observed_at || '');
  const fiveStale = $derived(!fiveObserved || now - Date.parse(fiveObserved) > 10 * 60_000 || !!usage?.error);
  const fiveProgress = $derived(fiveUsed === null || fiveExpired ? null : elapsedPercent(five?.reset, FIVE_HOURS_MS, now));
  const fiveCost = $derived(hours ? windowCost(hours, accountId, five?.reset, FIVE_HOURS_MS, now) : null);
  const weekCost = $derived(hours ? windowCost(hours, accountId, week?.reset, WEEK_MS, now) : null);
  const fiveDot = $derived<[Dot, string]>(fiveUsed === null || fiveExpired ? ['idle', 'unbekannt'] : fiveStale ? ['idle', 'veraltet'] : fiveUsed >= 100 ? ['error', 'erreicht'] : fiveUsed >= 80 ? ['warn', 'fast erreicht'] : ['ok', 'frei']);
</script>

<div class="usage" aria-label="Abo-Nutzung">
  <div class="windows">
    <div class="win">
      <div class="head"><span class="muted">5 Stunden</span><StatusDot state={fiveDot[0]} label={fiveDot[1]} /></div>
      <div class="pct-row" data-testid="pct-row">
        <p class="pct">{fiveUsed === null || fiveExpired ? '–' : `${Math.round(fiveUsed)} %`}</p>
        {#if fiveExpired}<p class="muted small">5-Stunden-Reset erfolgt · neuer Messstand nötig</p>
        {:else if five?.reset}<p class="reset" class:exhausted={fiveUsed !== null && fiveUsed >= 100}>Reset in {formatUntil(five.reset, now)} · {formatDateTime(five.reset)}</p>
        {:else}<p class="muted small">Reset unbekannt</p>{/if}
      </div>
      <div class="comparison">
        <div class="bar" role="meter" aria-label="5-Stunden-Verbrauch" aria-valuemin="0" aria-valuemax="100" aria-valuenow={fiveUsed === null || fiveExpired ? undefined : Math.round(fiveUsed)}><span class={fiveDot[0]} style:width="{fiveUsed === null || fiveExpired ? 0 : Math.min(100, fiveUsed)}%"></span></div>
        {#if fiveProgress !== null}<span class="now" style:left="{fiveProgress}%" aria-label="{Math.round(fiveProgress)} % des 5-Stunden-Fensters vergangen" title="{Math.round(fiveProgress)} % des 5-Stunden-Fensters vergangen"></span>{/if}
      </div>
      {#if fiveCost}<p class="cost" data-testid="window-cost">Kosten im Fenster <strong>{costLabel(fiveCost)}</strong> · {fiveCost.requests} Anfr.</p>{/if}
    </div>
    <div class="win">
      <div class="head"><span class="muted">Woche</span><span class="availability"><StatusDot state={dotState} label={label}/>{label}</span></div>
      <div class="pct-row" data-testid="pct-row">
        <p class="pct">{used === null || resetPassed ? '–' : `${Math.round(used)} %`}</p>
        {#if Number.isFinite(reset) && !resetPassed}<p class="reset">Reset in {formatUntil(week?.reset ?? '', now)} · {formatDateTime(week?.reset)}</p>
        {:else if !resetPassed}<p class="muted small">Reset unbekannt</p>{/if}
      </div>
      <div class="comparison">
        <div class="bar" role="meter" aria-label="Wochenverbrauch" aria-valuemin="0" aria-valuemax="100" aria-valuenow={used === null || resetPassed ? undefined : Math.round(used)}><span class={dotState} style:width="{used === null || resetPassed ? 0 : Math.min(100, used)}%"></span></div>
        {#if progress !== null && used !== null && !resetPassed}<span class="now" style:left="{progress}%" aria-label="{Math.round(progress)} % der Woche vergangen" title="{Math.round(progress)} % der Woche vergangen"></span>{/if}
      </div>
      {#if weekCost}<p class="cost" data-testid="window-cost">Kosten im Fenster <strong>{costLabel(weekCost)}</strong> · {weekCost.requests} Anfr.</p>{/if}
    </div>
  </div>
  {#if weekObservedAt}<p class="muted small">{stale ? 'Veralteter Messstand vom' : 'Stand vom'} {formatDateTime(weekObservedAt)}</p>{/if}
  {#if usage?.error}<p class="err small" role="status">{usage.error}</p>{/if}
  {#if loading}<p class="muted small">Limits werden geladen …</p>{/if}
</div>

<style>
  .usage { margin: 0.9rem 0 0; }
  .windows { display: grid; gap: 1rem 1.5rem; grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr)); }
  .win { display: grid; gap: 0.35rem; align-content: start; min-width: 0; }
  .head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.3rem 0.8rem; font-size: 0.85rem; }
  .availability { display: flex; align-items: center; gap: 0.4rem; }
  .pct-row { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 0 0.6rem; }
  .pct-row .reset, .pct-row .small { text-align: right; }
  .pct { margin: 0; font-size: 1.6rem; font-weight: 700; letter-spacing: -0.02em; line-height: 1.1; font-variant-numeric: tabular-nums; }
  .comparison { position: relative; }
  .bar { background: var(--c-line); height: 0.4rem; border-radius: 999px; overflow: hidden; }
  .bar > span { display: block; height: 100%; background: var(--c-accent); border-radius: inherit; transition: width var(--t); }
  .bar > .idle { opacity: 0.4; }
  .bar > .warn { background: var(--c-warn); }
  .bar > .error { background: var(--c-error); }
  .now { position: absolute; top: -4px; height: calc(0.4rem + 8px); width: 2px; border-radius: 1px; background: var(--c-text); transform: translateX(-1px); }
  p { margin: 0; line-height: 1.45; }
  .usage > p { margin-top: 0.6rem; }
  .reset { font-size: 0.8rem; color: var(--c-muted); }
  .exhausted { color: var(--c-error); }
  .small { font-size: 0.8rem; }
  .cost { font-size: 0.8rem; color: var(--c-muted); font-variant-numeric: tabular-nums; }
  .cost strong { color: var(--c-text); font-weight: 600; }
</style>
