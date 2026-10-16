<script lang="ts">
  // Subscription limits of exactly one subscription (5 hours, week), so a list
  // of subscriptions can render one card each.
  import StatusDot from './StatusDot.svelte';
  import { formatTime, formatUntil } from '../format';
  import { elapsedPercent, FIVE_HOURS_MS, WEEK_MS } from '../limits';
  import type { LimitWindow, RateLimits } from '../types/dashboard';

  let {
    title,
    subtitle = '',
    limits,
    active = false,
  }: { title: string; subtitle?: string; limits: RateLimits | null; active?: boolean } = $props();

  type Dot = 'ok' | 'warn' | 'error' | 'idle';
  function windowDot(w: LimitWindow): [Dot, string] {
    switch (w.status) {
      case 'allowed': return ['ok', 'frei'];
      case 'allowed_warning': return ['warn', 'fast erreicht'];
      case 'rejected': return ['error', 'erreicht'];
      default: return ['idle', 'unbekannt'];
    }
  }

  function shortDate(iso: string): string {
    return new Date(iso).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  }

  function resetText(w: LimitWindow): string {
    if (!w.reset || Number.isNaN(new Date(w.reset).getTime())) return 'Reset unbekannt';
    const left = formatUntil(w.reset);
    return left ? `zurückgesetzt in ${left} · ${shortDate(w.reset)}` : 'bereits zurückgesetzt';
  }

  let now = $state(Date.now());
  $effect(() => { const timer = setInterval(() => { now = Date.now(); }, 30_000); return () => clearInterval(timer); });

  // Label, window, window length, and how the elapsed share is named.
  type Win = { label: string; w: LimitWindow; ms: number; aria: string };
  const id = $props.id();
  const windows = $derived<Win[]>(limits ? [
    { label: '5 Stunden', w: limits.five_hour, ms: FIVE_HOURS_MS, aria: 'des 5-Stunden-Fensters' },
    { label: 'Woche', w: limits.seven_day, ms: WEEK_MS, aria: 'der Woche' },
  ] : []);
</script>

<section class="card" class:active aria-labelledby="{id}-t">
  <header>
    {#if active}<StatusDot state="ok" label="aktiv" />{/if}
    <h3 id="{id}-t">{title}</h3>
    {#if subtitle}<span class="muted sub">{subtitle}</span>{/if}
  </header>
  {#if limits}
    <div class="windows">
      {#each windows as { label, w, ms, aria } (label)}
        {@const [dot, dotLabel] = windowDot(w)}
        {@const u = w.utilization}
        {@const elapsed = u === null ? null : elapsedPercent(w.reset, ms, now)}
        <div class="win">
          <div class="win-head">
            <span class="muted">{label}</span>
            <StatusDot state={dot} label={dotLabel} />
          </div>
          <div class="pct-row">
            <p class="pct">{u === null ? '–' : `${Math.round(u * 100)} %`}</p>
            <span class="muted small">{resetText(w)}</span>
          </div>
          <div class="comparison">
            <div class="bar" role="meter" aria-label="Auslastung {label}" aria-valuemin="0" aria-valuemax="100" aria-valuenow={u === null ? undefined : Math.round(u * 100)}>
              <span class={dot} style:width="{u === null ? 0 : Math.min(100, u * 100)}%"></span>
            </div>
            {#if elapsed !== null}<span class="now" style:left="{elapsed}%" aria-label="{Math.round(elapsed)} % {aria} vergangen" title="{Math.round(elapsed)} % {aria} vergangen"></span>{/if}
          </div>
        </div>
      {/each}
    </div>
    <p class="muted small foot">Gemessen um {formatTime(limits.observed_at)}</p>
  {:else}
    <p class="muted">Noch keine Daten, erscheint nach der nächsten Anfrage über tollgate.</p>
  {/if}
</section>

<style>
  .card { background: var(--c-sidebar); border-radius: var(--radius); padding: 1rem 1.15rem; display: grid; gap: 0.75rem; }
  .card.active { box-shadow: inset 0 0 0 1.5px var(--c-ok); }
  header { display: flex; align-items: center; gap: 0.5rem; min-width: 0; }
  h3 { margin: 0; font-size: 0.95rem; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub { margin-left: auto; font-size: 0.8rem; }
  .windows { display: grid; gap: 1.25rem; grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr)); }
  .win { display: grid; gap: 0.35rem; }
  .win-head { display: flex; align-items: center; justify-content: space-between; font-size: 0.85rem; }
  .pct-row { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 0 0.6rem; }
  .pct-row .small { text-align: right; }
  .pct { margin: 0; font-size: 2rem; font-weight: 700; letter-spacing: -0.02em; line-height: 1.1; font-variant-numeric: tabular-nums; }
  .comparison { position: relative; }
  .bar { height: 0.4rem; border-radius: 999px; background: var(--c-line); overflow: hidden; }
  .bar span { display: block; height: 100%; border-radius: inherit; background: var(--c-accent); transition: width var(--t); }
  .bar span.warn { background: var(--c-warn); }
  .bar span.error { background: var(--c-error); }
  .now { position: absolute; top: -4px; height: calc(0.4rem + 8px); width: 2px; border-radius: 1px; background: var(--c-text); transform: translateX(-1px); }
  .small { font-size: 0.8rem; }
  .foot { margin: 0; }
</style>
