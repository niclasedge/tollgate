<script lang="ts">
  // Own simple stroke icons (24x24 grid), no third-party assets.
  type Name = 'home' | 'collapse' | 'expand' | 'live' | 'requests' | 'costs' | 'performance' | 'filters' | 'compression' | 'clients' | 'accounts' | 'sun' | 'moon';
  let { name, size = 22 }: { name: Name; size?: number } = $props();

  const paths: Record<Name, string> = {
    home: 'M4 4h7v7H4zM13 4h7v4h-7zM13 10h7v10h-7zM4 13h7v7H4z',
    collapse: 'M4 4h16v16H4zM9 4v16M16 9l-3 3 3 3',
    expand: 'M4 4h16v16H4zM9 4v16M13 9l3 3-3 3',
    live: 'M3 12h4l3-8 4 16 3-8h4',
    requests: 'M4 6h16M4 12h16M4 18h10',
    costs: 'M5 20V10M12 20V4M19 20v-7',
    performance: 'M4 18a8 8 0 1 1 16 0M12 18l4-6',
    filters: 'M4 5h16l-6 7v6l-4 2v-8z',
    compression: 'M9 4v5H4M15 4v5h5M9 20v-5H4M15 20v-5h5',
    clients: 'M15 8a3 3 0 1 1-6 0 3 3 0 0 1 6 0zM5 20c0-3.5 3-6 7-6s7 2.5 7 6',
    accounts: 'M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z',
    sun: 'M12 8a4 4 0 1 1 0 8 4 4 0 0 1 0-8zM12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5L19 19M5 19l1.5-1.5M17.5 6.5L19 5',
    moon: 'M20 14.5A8 8 0 0 1 9.5 4 8 8 0 1 0 20 14.5z',
  };
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">
  <path d={paths[name]} />
</svg>
