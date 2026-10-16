<script lang="ts">
  // Round status indicator instead of loud badges.
  let { state, label }: { state: 'ok' | 'warn' | 'error' | 'idle' | 'busy' | 'slow'; label: string } = $props();
</script>

<span class="dot {state}" role="img" aria-label={label} title={label}></span>

<style>
  .dot { display: inline-block; flex: none; width: 0.6rem; height: 0.6rem; border-radius: 50%; background: var(--c-idle); transition: background var(--t); }
  .ok { background: var(--c-ok); }
  .warn { background: var(--c-warn); }
  .error { background: var(--c-error); }
  /* Slow (#87): red like an error, but hollow so it stays distinguishable. */
  .slow { background: transparent; box-shadow: inset 0 0 0 2px var(--c-error); }
  .busy { background: var(--c-accent); animation: pulse 1.2s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.35; } }
</style>
