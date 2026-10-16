<script lang="ts">
  // One subscription: identity, license and limits. Accounts adds actions as
  // children; the home page renders the same card without them.
  import type { Snippet } from 'svelte';
  import type { Account } from '../types/accounts';
  import type { CostHour } from '../types/dashboard';
  import { formatDateTime } from '../format';
  import { planLabel, subscriptionLabel, tokenState } from '../license';
  import AccountUsage from './AccountUsage.svelte';
  import StatusDot from './StatusDot.svelte';

  // signal (overview only, #126): 'full' marks the active account at its limit, 'next' the one to switch to.
  let { account, loading = false, hours, signal, children }: { account: Account; loading?: boolean; hours?: readonly CostHour[]; signal?: 'full' | 'next'; children?: Snippet } = $props();
  const id = $props.id();
  const license = $derived(account.usage?.license);
  const token = $derived(tokenState(license, account.active));
  const plan = $derived([planLabel(license), subscriptionLabel(license)].filter(Boolean).join(' · '));
  const planOlder = $derived(!!license?.profile_observed_at && license.profile_observed_at !== license.checked_at);
</script>

<article class="account" class:active={account.active} class:full={signal === 'full'} class:next={signal === 'next'} aria-labelledby="{id}-name">
  <div class="account-head">
    <StatusDot state={account.active ? 'ok' : 'idle'} label={account.active ? 'Aktiv' : 'Inaktiv'} />
    <h3 id="{id}-name" title={account.email ? account.name : undefined}>{account.email || account.name}</h3>
    <span class="muted" class:alert={signal === 'full'}>{account.active ? 'Aktiv' : 'Inaktiv'}{signal === 'full' ? ' · Limit erreicht' : signal === 'next' ? ' · als Nächstes' : ''}</span>
  </div>
  <div class="license">
    <span class="token"><StatusDot state={token.state} label={token.label} /><span>{token.label}</span></span>
    <span class="plan">{plan}</span>
  </div>
  {#if !account.active && license?.checked_at}
    <p class="muted small">Lizenz: Stand vom {formatDateTime(license.checked_at)}{planOlder ? ` · Plan vom ${formatDateTime(license.profile_observed_at)}` : ''}</p>
  {/if}
  <p class="muted small">{account.status || 'Status unbekannt'}{account.last_refresh ? ` · Letzter Refresh ${formatDateTime(account.last_refresh)}` : ''}</p>
  <AccountUsage usage={account.usage} {loading} accountId={account.id} {hours} />
  {#if children}<div class="slot">{@render children()}</div>{/if}
</article>

<style>
  .account { padding: 1.1rem; border-radius: var(--radius); background: var(--c-surface, var(--c-bg)); box-shadow: 0 0 0 1px var(--c-line); min-width: 0; }
  .account.active { box-shadow: 0 0 0 1.5px var(--c-ok); }
  .account.full { box-shadow: 0 0 0 1.5px var(--c-error); }
  .account.next { box-shadow: 0 0 0 2px var(--c-accent); animation: next 1.4s ease-in-out infinite; }
  @keyframes next { 50% { box-shadow: 0 0 0 2px transparent, 0 0 0 1px var(--c-line); } }
  @media (prefers-reduced-motion: reduce) { .account.next { animation: none; } }
  .alert { color: var(--c-error); }
  .account-head { display: flex; align-items: center; flex-wrap: wrap; gap: 0.6rem; }
  h3 { font-size: 1rem; margin: 0; overflow-wrap: anywhere; }
  .license { margin-top: 0.5rem; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.3rem 1rem; font-size: 0.9rem; }
  .token { display: inline-flex; align-items: center; gap: 0.45rem; }
  .plan { font-weight: 600; }
  p { margin: 0.35rem 0 0; line-height: 1.45; }
  .small { font-size: 0.8rem; }
  .slot { margin-top: 1rem; }
</style>
