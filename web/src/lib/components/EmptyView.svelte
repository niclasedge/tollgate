<script lang="ts">
  let { title, hint }: { title: string; hint: string } = $props();
</script>

<header>
  <h1>{title}</h1>
</header>
<p class="empty">{hint}</p>

<style>
  h1 { font-size: clamp(1.75rem, 4vw, 2.25rem); font-weight: 700; letter-spacing: -0.02em; margin: 0 0 1.5rem; }
  .empty { color: var(--c-muted); margin: 0; }
</style>
