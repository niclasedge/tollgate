<script lang="ts">
  // Requests of the last five minutes against the average of the ten
  // five-minute windows before (#132). refresh reloads it with the overview.
  import { api } from '../api';
  import { auth } from '../auth.svelte';
  import { formatTokens } from '../format';
  import StatusDot from './StatusDot.svelte';
  import type { CallRate } from '../types/dashboard';

  let { refresh = 0 }: { refresh?: number } = $props();

  let rate = $state<CallRate | undefined>();
  let error = $state<string | undefined>();

  $effect(() => {
    if (!auth.token) return;
    void refresh;
    let stale = false;
    api.calls().then(
      (r) => { if (!stale) { rate = r; error = undefined; } },
      (e: Error) => { if (!stale) error = e.message; },
    );
    return () => (stale = true);
  });

  const verdicts: Record<string, { state: 'warn' | 'idle'; label: string }> = {
    above: { state: 'warn', label: 'über Ø' },
    below: { state: 'idle', label: 'unter Ø' },
    equal: { state: 'idle', label: 'wie Ø' },
  };
  const verdict = $derived(rate ? (verdicts[rate.trend] ?? { state: 'idle', label: 'unbekannt' }) : undefined);
  const minutes = (s: number) => Math.round(s / 60);
  const average = (n: number) => n.toLocaleString('de-DE', { maximumFractionDigits: 1 });
</script>

{#if error}<p class="err" role="alert">{error}</p>{/if}
{#if rate && verdict}
  <dl class="tiles">
    <div>
      <dt>Anfragen</dt>
      <dd>{formatTokens(rate.current)}</dd>
      <p class="vs">
        Ø {average(rate.average)} je {minutes(rate.window_seconds)} min in den {minutes(rate.window_seconds * rate.windows)} min davor
      </p>
      <p class="verdict"><StatusDot state={verdict.state} label={verdict.label} /><span>{verdict.label}</span></p>
    </div>
  </dl>
{:else if !error}
  <p class="muted">Lade …</p>
{/if}

<style>
  .tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 17rem), 22rem)); gap: 0.75rem; margin: 0; }
  .tiles div { padding: 0.8rem 1rem; background: var(--c-sidebar); border-radius: var(--radius); }
  dt { font-size: 0.8rem; color: var(--c-muted); }
  dd { margin: 0; font-size: 1.35rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  .vs { margin: 0.2rem 0 0; font-size: 0.75rem; color: var(--c-muted); font-variant-numeric: tabular-nums; }
  .verdict { display: flex; align-items: center; gap: 0.4rem; margin: 0.35rem 0 0; font-size: 0.8rem; color: var(--c-text); }
</style>
