<script lang="ts">
  import Shell from './lib/components/Shell.svelte';
  import EmptyView from './lib/components/EmptyView.svelte';
  import Login from './lib/components/Login.svelte';
  import Home from './lib/views/Home.svelte';
  import Live from './lib/views/Live.svelte';
  import Costs from './lib/views/Costs.svelte';
  import Clients from './lib/views/Clients.svelte';
  import Accounts from './lib/views/Accounts.svelte';
  import Filters from './lib/views/Filters.svelte';
  import Compression from './lib/views/Compression.svelte';
  import { router, routes, startRouter } from './lib/router.svelte';
  import { handleKey } from './lib/keys';
  import { auth, logout } from './lib/auth.svelte';

  $effect(() => startRouter());

  const hints: Record<string, string> = {
    requests: 'Hier erscheint die Stufen-Timeline jeder Anfrage.',
  };
  const views = { home: Home, live: Live, costs: Costs, clients: Clients, accounts: Accounts, filters: Filters, compression: Compression } as const;
  const current = $derived(routes.find((r) => r.id === router.current.id)!);
  const View = $derived(views[current.id as keyof typeof views]);
</script>

<svelte:window onkeydown={handleKey} />

<Shell>
  {#if !View}
    <EmptyView title={current.label} hint={hints[current.id]} />
  {:else if !auth.token}
    <Login title={current.label} />
  {:else}
    <!-- Keyed by area so each view starts fresh (and its SSE stream closes). -->
    {#key current.id}<View />{/key}
  {/if}
  {#if auth.token}
    <p class="logout"><button type="button" class="btn" onclick={() => logout()}>Abmelden</button></p>
  {/if}
</Shell>

<style>
  .logout { margin: 3rem 0 0; }
</style>
