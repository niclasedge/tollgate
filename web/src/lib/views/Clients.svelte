<script lang="ts">
  import { api } from '../api';
  import { auth } from '../auth.svelte';
  import { formatDateTime } from '../format';
  import StatusDot from '../components/StatusDot.svelte';
  import type { ClientInfo } from '../types/auth';

  let clients = $state<ClientInfo[]>([]);
  let loaded = $state(false);
  let error = $state('');
  let name = $state('');
  let busy = $state(false);
  let pending = $state<{ id: string; action: 'block' | 'rotate' } | undefined>();
  /** The plaintext key is held here only until dismissed; it is never stored. */
  let fresh = $state<{ name: string; key: string } | undefined>();
  let copied = $state(false);

  const snippet = $derived(fresh ? `export ANTHROPIC_BASE_URL=${window.location.origin}\nexport ANTHROPIC_AUTH_TOKEN=${fresh.key}` : '');

  async function load() {
    try {
      clients = await api.listClients();
      loaded = true;
    } catch (e) {
      error = (e as Error).message;
    }
  }

  $effect(() => {
    if (auth.token) void load();
  });

  async function run(fn: () => Promise<void>) {
    busy = true;
    error = '';
    try {
      await fn();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }

  const create = (e: SubmitEvent) => {
    e.preventDefault();
    const n = name.trim();
    if (!n) return;
    return run(async () => {
      const r = await api.createClient(n);
      fresh = { name: r.client.name, key: r.key };
      copied = false;
      name = '';
      await load();
    });
  };

  const confirm = () =>
    run(async () => {
      if (!pending) return;
      const { id, action } = pending;
      if (action === 'block') await api.blockClient(id);
      else {
        const r = await api.rotateClient(id);
        fresh = { name: r.client.name, key: r.key };
        copied = false;
      }
      pending = undefined;
      await load();
    });

  /** Keeps keyboard focus after the row swaps to confirm/cancel. */
  function focusOnMount(node: HTMLElement) {
    node.focus();
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(snippet);
      copied = true;
    } catch {
      error = 'Kopieren nicht möglich. Bitte den Text markieren und manuell kopieren.';
    }
  }
</script>

<div class="page-head"><h1>Clients</h1></div>

{#if error}<p class="err" role="alert">{error}</p>{/if}

{#if fresh}
  <section class="fresh" aria-label="Neuer Key">
    <h2>Key für „{fresh.name}“</h2>
    <p class="muted">Dieser Key wird nur jetzt angezeigt. Kopiere das Setup und speichere es sicher.</p>
    <pre class="mono" data-testid="snippet">{snippet}</pre>
    <div class="actions">
      <button type="button" class="btn primary" onclick={copy}>{copied ? 'Kopiert' : 'Kopieren'}</button>
      <button type="button" class="btn" onclick={() => (fresh = undefined)}>Schließen</button>
    </div>
  </section>
{/if}

<form class="create" onsubmit={create}>
  <label class="sr" for="client-name">Name des neuen Clients</label>
  <input id="client-name" placeholder="Neuer Client, z. B. laptop" maxlength="128" bind:value={name} />
  <button type="submit" class="btn primary" disabled={busy || !name.trim()}>Anlegen</button>
</form>

{#if loaded && clients.length === 0}<p class="muted">Noch keine Clients.</p>{/if}
{#if clients.length}
  <div class="table-wrap">
    <table class="tbl">
      <thead><tr><th>Name</th><th>Key</th><th>Status</th><th>Angelegt</th><th>Rotiert</th><th></th></tr></thead>
      <tbody>
        {#each clients as c (c.id)}
          <tr>
            <td>{c.name}</td>
            <td class="mono">{c.key_hint}</td>
            <td><span class="st"><StatusDot state={c.status === 'aktiv' ? 'ok' : 'idle'} label={c.status} />{c.status}</span></td>
            <td class="muted">{formatDateTime(c.created_at)}</td>
            <td class="muted">{formatDateTime(c.rotated_at)}</td>
            <td class="act">
              {#if pending?.id === c.id}
                <span class="ask">{pending.action === 'block' ? 'Wirklich sperren?' : 'Key wirklich rotieren?'}</span>
                <button type="button" class="btn danger" disabled={busy} onclick={confirm} use:focusOnMount>Bestätigen</button>
                <button type="button" class="btn" onclick={() => (pending = undefined)}>Abbrechen</button>
              {:else}
                <button type="button" class="btn" aria-label="Rotieren: {c.name}" onclick={() => (pending = { id: c.id, action: 'rotate' })}>Rotieren</button>
                {#if c.status === 'aktiv'}
                  <button type="button" class="btn" aria-label="Sperren: {c.name}" onclick={() => (pending = { id: c.id, action: 'block' })}>Sperren</button>
                {/if}
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .fresh { background: var(--c-accent-soft); border-radius: var(--radius); padding: 1rem 1.25rem; margin-bottom: 1.5rem; }
  .fresh h2 { margin: 0 0 0.25rem; font-size: 1.1rem; }
  pre { margin: 0.75rem 0; padding: 0.75rem; background: var(--c-surface); border-radius: var(--radius); overflow-x: auto; user-select: all; }
  .actions, .act { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; }
  .create { display: flex; gap: 0.5rem; margin-bottom: 1.5rem; max-width: 30rem; }
  input { flex: 1; min-width: 0; font: inherit; padding: 0.5rem 0.75rem; border: 1px solid var(--c-line); border-radius: var(--radius); background: var(--c-surface); color: var(--c-text); }
  input:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 2px; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
  .st { display: inline-flex; align-items: center; gap: 0.4rem; }
  .ask { font-size: 0.85rem; color: var(--c-muted); }
</style>
