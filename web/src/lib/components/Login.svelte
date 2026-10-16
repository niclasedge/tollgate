<script lang="ts">
  import { auth, login } from '../auth.svelte';
  let { title }: { title: string } = $props();
  let value = $state('');

  function submit(e: SubmitEvent) {
    e.preventDefault();
    login(value);
    value = '';
  }
</script>

<header><h1>{title}</h1></header>
<form onsubmit={submit}>
  <p class="lead">Melde dich mit dem Admin-Token an. Es bleibt nur in diesem Browser gespeichert.</p>
  {#if auth.rejected}<p class="err" role="alert">Das Token wurde abgelehnt. Bitte erneut eingeben.</p>{/if}
  <label for="admin-token">Admin-Token</label>
  <div class="row">
    <!-- svelte-ignore a11y_autofocus -->
    <input id="admin-token" type="password" autocomplete="off" autofocus bind:value />
    <button type="submit" class="primary" disabled={!value.trim()}>Anmelden</button>
  </div>
</form>

<style>
  h1 { font-size: clamp(1.75rem, 4vw, 2.25rem); font-weight: 700; letter-spacing: -0.02em; margin: 0 0 1.5rem; }
  form { max-width: 26rem; }
  .lead { color: var(--c-muted); margin: 0 0 1rem; }
  .err { color: var(--c-error); margin: 0 0 1rem; }
  label { display: block; font-size: 0.85rem; color: var(--c-muted); margin-bottom: 0.25rem; }
  .row { display: flex; gap: 0.5rem; }
  input { flex: 1; min-width: 0; font: inherit; padding: 0.55rem 0.75rem; border: 1px solid var(--c-line); border-radius: var(--radius); background: var(--c-surface); color: var(--c-text); }
  input:focus-visible, button:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 2px; }
  .primary { font: inherit; padding: 0.55rem 1rem; border: 0; border-radius: var(--radius); background: var(--c-accent); color: var(--c-on-accent); cursor: pointer; }
  .primary:disabled { opacity: 0.4; cursor: default; }
</style>
