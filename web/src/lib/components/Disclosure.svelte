<script lang="ts">
  // Inline detail: unfolds in place instead of opening a new page.
  import type { Snippet } from 'svelte';
  let { title, children, open = $bindable(false) }: { title: string; children: Snippet; open?: boolean } = $props();
</script>

<div class="disclosure" class:open>
  <button type="button" aria-expanded={open} onclick={() => (open = !open)}>
    <span class="chev" aria-hidden="true">›</span>{title}
  </button>
  {#if open}<div class="body">{@render children()}</div>{/if}
</div>

<style>
  button { all: unset; box-sizing: border-box; display: flex; gap: 0.5rem; width: 100%; padding: 0.75rem 0.25rem; cursor: pointer; border-radius: var(--radius); }
  button:hover { background: var(--c-hover); }
  button:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 2px; }
  .chev { display: inline-block; color: var(--c-muted); transition: transform var(--t); }
  .open .chev { transform: rotate(90deg); }
  .body { padding: 0 0.25rem 1rem 1.25rem; animation: fade var(--t) ease-out; }
  @keyframes fade { from { opacity: 0; transform: translateY(-2px); } }
</style>
