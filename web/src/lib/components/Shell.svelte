<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  import { groups, router, hrefFor, type RouteId } from '../router.svelte';
  import { theme, toggleTheme } from '../theme.svelte';
  import { auth } from '../auth.svelte';
  import { api } from '../api';

  let { children }: { children: Snippet } = $props();

  const KEY = 'tollgate.sidebar';
  function storedCollapsed(): boolean {
    try {
      return localStorage.getItem(KEY) === 'collapsed';
    } catch {
      return false;
    }
  }
  let collapsed = $state(storedCollapsed());
  function toggleCollapsed() {
    collapsed = !collapsed;
    try {
      localStorage.setItem(KEY, collapsed ? 'collapsed' : 'expanded');
    } catch {
      // storage unavailable: the choice then only lasts for this page view
    }
  }

  // Quiet counters; a failing request just leaves the counter out.
  let counts = $state<Partial<Record<RouteId, number>>>({});
  $effect(() => {
    if (!auth.token) {
      counts = {};
      return;
    }
    void router.current.id; // refresh when switching areas (e.g. after creating a client)
    let stale = false;
    api.listClients().then((c) => !stale && (counts = { ...counts, clients: c.length }), () => {});
    api.status().then((s) => !stale && (counts = { ...counts, accounts: s.accounts.length }), () => {});
    return () => (stale = true);
  });

  // The hash is the router state, so the skip link must not change it.
  function skipToMain(e: MouseEvent) {
    e.preventDefault();
    document.getElementById('main')?.focus();
  }
</script>

<a class="skip" href="#main" onclick={skipToMain}>Zum Inhalt springen</a>
<div class="shell" class:collapsed>
  <aside>
    <div class="brand"><span class="mark" aria-hidden="true"></span><span class="name">tollgate</span></div>
    <nav aria-label="Bereiche">
      {#each groups as g (g.label)}
        <div class="group" role="group" aria-label={g.label}>
          <p class="group-h" aria-hidden="true">{g.label}</p>
          {#each g.routes as r (r.id)}
            <a
              href={hrefFor(r.id)}
              title="{r.label} ({r.key})"
              aria-label={r.label}
              aria-current={router.current.id === r.id ? 'page' : undefined}
            >
              <Icon name={r.id} />
              <span class="label">{r.label}</span>
              {#if counts[r.id] !== undefined}<span class="count" aria-hidden="true">{counts[r.id]}</span>{/if}
            </a>
          {/each}
        </div>
      {/each}
    </nav>
    <div class="tools">
      <button type="button" class="tool" onclick={toggleTheme} title="Darstellung wechseln (d)" aria-label="Darstellung wechseln, aktuell {theme.value === 'dark' ? 'dunkel' : 'hell'}">
        <Icon name={theme.value === 'dark' ? 'sun' : 'moon'} />
      </button>
      <button type="button" class="tool collapse" onclick={toggleCollapsed} aria-expanded={!collapsed} aria-label={collapsed ? 'Seitenleiste ausklappen' : 'Seitenleiste einklappen'} title={collapsed ? 'Seitenleiste ausklappen' : 'Seitenleiste einklappen'}>
        <Icon name={collapsed ? 'expand' : 'collapse'} />
      </button>
    </div>
  </aside>
  <main id="main" tabindex="-1">{@render children()}</main>
</div>

<style>
  .skip { position: absolute; left: -999px; top: 0; background: var(--c-surface); padding: 0.5rem 1rem; z-index: 10; }
  .skip:focus { left: 0.5rem; top: 0.5rem; }
  .shell { display: grid; grid-template-columns: var(--sidebar-wide) 1fr; min-height: 100dvh; }
  .shell.collapsed { grid-template-columns: var(--sidebar-w) 1fr; }
  aside { position: sticky; top: 0; height: 100dvh; display: flex; flex-direction: column; gap: 1rem; padding: 1.25rem 0.75rem; background: var(--c-sidebar); border-right: 1px solid var(--c-line); overflow-y: auto; }
  .brand { display: flex; align-items: center; gap: 0.6rem; padding: 0 0.6rem; height: 1.5rem; }
  .mark { width: 0.75rem; height: 0.75rem; flex: none; border-radius: 50%; background: var(--c-accent); }
  .name { font-weight: 600; letter-spacing: -0.01em; }
  nav { display: flex; flex-direction: column; gap: 0.9rem; flex: 1; }
  .group { display: flex; flex-direction: column; gap: 0.1rem; }
  .group-h { margin: 0 0 0.2rem; padding: 0 0.6rem; font-size: 0.68rem; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; color: var(--c-muted); }
  nav a { display: flex; align-items: center; gap: 0.7rem; height: 2.25rem; padding: 0 0.6rem; border-radius: var(--radius); color: var(--c-muted); text-decoration: none; font-size: 0.92rem; transition: background var(--t), color var(--t); }
  nav a .label { color: var(--c-text); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .count { font-size: 0.78rem; font-variant-numeric: tabular-nums; color: var(--c-muted); }
  nav a:hover, .tool:hover { background: var(--c-hover); color: var(--c-text); }
  nav a[aria-current='page'] { color: var(--c-accent); background: var(--c-accent-soft); }
  .tools { display: flex; gap: 0.25rem; }
  .tool { all: unset; box-sizing: border-box; display: grid; place-items: center; width: 2.5rem; height: 2.5rem; border-radius: var(--radius); color: var(--c-muted); cursor: pointer; transition: background var(--t), color var(--t); }
  .collapse { margin-left: auto; }
  a:focus-visible, .tool:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 2px; }
  main { padding: 2.5rem clamp(1.25rem, 5vw, 4rem); width: 100%; min-width: 0; outline: none; }
  /* Full width (issue #115); prose paragraphs keep a readable line length. */
  main :global(p) { max-width: 75ch; }

  /* Collapsed: icons only, as before. */
  .collapsed aside { align-items: center; padding: 1.25rem 0; }
  .collapsed .brand { padding: 0; }
  .collapsed .name, .collapsed .count, .collapsed .group-h { display: none; }
  .collapsed .group + .group { border-top: 1px solid var(--c-line); padding-top: 0.9rem; }
  .collapsed nav a { justify-content: center; width: 2.75rem; height: 2.75rem; padding: 0; }
  .collapsed .label { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .collapsed .tools { flex-direction: column; }
  .collapsed .collapse { margin-left: 0; }

  @media (max-width: 720px) {
    .shell, .shell.collapsed { grid-template-columns: 1fr; padding-bottom: calc(4rem + env(safe-area-inset-bottom)); }
    aside, .collapsed aside { position: fixed; inset: auto 0 0 0; height: auto; flex-direction: row; align-items: center; justify-content: space-around; padding: 0.4rem 0.25rem calc(0.4rem + env(safe-area-inset-bottom)); border-right: 0; border-top: 1px solid var(--c-line); z-index: 5; overflow: visible; }
    .brand, .group-h, .count, .collapse { display: none; }
    nav { flex-direction: row; flex: 1; justify-content: space-around; gap: 0; }
    .group { display: contents; }
    .collapsed .group + .group { border-top: 0; padding-top: 0; }
    nav a, .collapsed nav a { justify-content: center; width: 2.4rem; height: 2.4rem; padding: 0; }
    .label { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
    .tool { width: 2.4rem; height: 2.4rem; }
    main { padding-top: 1.5rem; }
  }
</style>
