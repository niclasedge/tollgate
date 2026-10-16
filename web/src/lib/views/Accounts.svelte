<script lang="ts">
  import { api } from '../api';
  import AccountManager from './AccountManager.svelte';
  import { auth } from '../auth.svelte';
  import { formatDateTime, formatTime, formatUntil } from '../format';
  import StatusDot from '../components/StatusDot.svelte';
  import type { LimitWindow, ServiceHealth, Status } from '../types/dashboard';

  let status = $state<Status | undefined>();
  let error = $state('');

  async function load() {
    error = '';
    try {
      status = await api.status();
    } catch (e) {
      error = (e as Error).message;
    }
  }

  $effect(() => {
    if (!auth.token) return;
    void load();
    // The limits change with every request; refresh while the view is open.
    const id = setInterval(() => void load(), 30_000);
    return () => clearInterval(id);
  });

  type Dot = 'ok' | 'warn' | 'error' | 'idle';
  function windowDot(w: LimitWindow): [Dot, string] {
    switch (w.status) {
      case 'allowed': return ['ok', 'frei'];
      case 'allowed_warning': return ['warn', 'fast erreicht'];
      case 'rejected': return ['error', 'erreicht'];
      default: return ['idle', 'unbekannt'];
    }
  }
  function percent(u: number | null): string {
    return u === null ? '–' : `${Math.round(u * 100)} %`;
  }
  function resetText(w: LimitWindow): string {
    if (!w.reset) return 'Reset unbekannt';
    const left = formatUntil(w.reset);
    return left ? `zurückgesetzt in ${left} (${formatDateTime(w.reset)})` : 'bereits zurückgesetzt';
  }

  const accounts = $derived(status?.accounts ?? []);
  const services = $derived<Array<[string, ServiceHealth]>>(status ? [['tollgate', status.tollgate], ['CLIProxyAPI', status.cliproxyapi]] : []);
</script>

<div class="page-head">
  <h1>Accounts</h1>
  <button type="button" class="btn" onclick={load}>Aktualisieren</button>
</div>

{#if error}<p class="err" role="alert">{error}</p>{/if}
{#if status}
  <h2 class="section first">Dienste</h2>
  <ul class="svc">
    {#each services as [label, h] (label)}
      <li>
        <StatusDot state={h.ok ? 'ok' : 'error'} label={h.ok ? 'in Ordnung' : 'gestört'} />
        <strong>{label}</strong>
        <span class="muted">{[h.version, h.message].filter(Boolean).join(' · ') || (h.ok ? 'in Ordnung' : 'gestört')}</span>
      </li>
    {/each}
  </ul>

  {#if !status.account_management}
  <h2 class="section">Abo-Limits</h2>
  {#if status.limits}
    {@const windows: Array<[string, LimitWindow]> = [['5 Stunden', status.limits.five_hour], ['Woche', status.limits.seven_day]]}
    <ul class="limits">
      {#each windows as [label, w] (label)}
        {@const [dot, dotLabel] = windowDot(w)}
        {@const u = w.utilization}
        <li>
          <div class="limit-head">
            <StatusDot state={dot} label={dotLabel} />
            <strong>{label}</strong>
            <span class="pct">{percent(u)}</span>
          </div>
          <div class="bar" role="meter" aria-label="Auslastung {label}" aria-valuemin="0" aria-valuemax="100" aria-valuenow={u === null ? undefined : Math.round(u * 100)}>
            <span class={dot} style:width="{u === null ? 0 : Math.min(100, u * 100)}%"></span>
          </div>
          <span class="muted small">{resetText(w)}</span>
        </li>
      {/each}
    </ul>
    <p class="muted small">Gemessen um {formatTime(status.limits.observed_at)} aus der letzten Antwort von Anthropic.</p>
  {:else}
    <p class="muted">Noch keine Daten, erscheint nach der nächsten Anfrage über tollgate.</p>
  {/if}

  {/if}
  <h2 class="section">Accounts</h2>
  {#if status.account_management}
    <AccountManager onchange={() => void load()} />
  {:else}
  {#if accounts.length > 1}
    <p class="warn" role="status">
      Es liegen {accounts.length} Accounts im auth-dir. Vorgesehen ist genau einer, sonst ist unklar, welcher Account aktiv ist.
    </p>
  {/if}
  {#if accounts.length === 0}
    <p class="muted">Keine Accounts verfügbar.{status.cliproxyapi.message ? ` ${status.cliproxyapi.message}` : ''}</p>
  {:else}
    <div class="table-wrap">
      <table class="tbl">
        <thead><tr><th></th><th>E-Mail</th><th>Anbieter</th><th>Status</th><th>Deaktiviert</th><th>Cooldown bis</th></tr></thead>
        <tbody>
          {#each accounts as a (a.name)}
            <tr>
              <td><StatusDot state={a.disabled ? 'idle' : a.cooldown_until ? 'warn' : 'ok'} label={a.disabled ? 'deaktiviert' : a.cooldown_until ? 'Cooldown' : 'aktiv'} /></td>
              <td>{a.email || a.name}</td>
              <td>{a.provider}</td>
              <td>{a.status || '–'}</td>
              <td>{a.disabled ? 'ja' : 'nein'}</td>
              <td class="muted">{a.cooldown_until ? formatDateTime(a.cooldown_until) : '–'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
  {/if}
{:else if !error}
  <p class="muted">Lade …</p>
{/if}

<style>
  .first { margin-top: 0; }
  .svc { list-style: none; margin: 0; padding: 0; display: grid; gap: 0.6rem; }
  .svc li { display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap; }
  .limits { list-style: none; margin: 0 0 0.5rem; padding: 0; display: grid; gap: 1rem; grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr)); }
  .limits li { display: grid; gap: 0.4rem; }
  .limit-head { display: flex; align-items: center; gap: 0.6rem; }
  .pct { margin-left: auto; font-variant-numeric: tabular-nums; font-size: 1.25rem; font-weight: 600; }
  .bar { height: 0.5rem; border-radius: 999px; background: var(--c-line); overflow: hidden; }
  .bar span { display: block; height: 100%; border-radius: inherit; background: var(--c-accent); transition: width var(--t); }
  .bar span.warn { background: var(--c-warn); }
  .bar span.error { background: var(--c-error); }
  .small { font-size: 0.85rem; }
  .warn { background: color-mix(in srgb, var(--c-warn) 18%, transparent); border-radius: var(--radius); padding: 0.75rem 1rem; margin: 0 0 1rem; }
</style>
