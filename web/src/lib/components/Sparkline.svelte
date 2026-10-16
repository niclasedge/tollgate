<script lang="ts">
  // A narrow trend line over the days; gaps (null) break the line.
  let { values, label, marks = [] }: { values: (number | null)[]; label: string; marks?: boolean[] } = $props();

  const W = 160;
  const H = 28;
  const pts = $derived.by(() => {
    const known = values.filter((v): v is number => v !== null);
    const lo = Math.min(...known);
    const hi = Math.max(...known);
    const x = (i: number) => (values.length > 1 ? (i / (values.length - 1)) * (W - 4) + 2 : W / 2);
    const y = (v: number) => (hi === lo ? H / 2 : H - 3 - ((v - lo) / (hi - lo)) * (H - 6));
    const segs: string[][] = [];
    let cur: string[] = [];
    values.forEach((v, i) => {
      if (v === null) {
        if (cur.length) segs.push(cur);
        cur = [];
      } else cur.push(`${x(i).toFixed(1)},${y(v).toFixed(1)}`);
    });
    if (cur.length) segs.push(cur);
    return { segs, dots: marks.map((m, i) => (m ? x(i) : null)) };
  });
</script>

<svg class="spark" width={W} height={H} viewBox="0 0 {W} {H}" role="img" aria-label={label}>
  {#each pts.segs as s}
    {#if s.length === 1}<circle cx={s[0].split(',')[0]} cy={s[0].split(',')[1]} r="1.5" class="pt" />{:else}<polyline points={s.join(' ')} />{/if}
  {/each}
  {#each pts.dots as d}{#if d !== null}<circle cx={d} cy={H - 2} r="2" class="slow" />{/if}{/each}
</svg>

<style>
  .spark { display: block; overflow: visible; }
  polyline { fill: none; stroke: var(--c-accent); stroke-width: 1.5; stroke-linejoin: round; }
  .pt { fill: var(--c-accent); }
  .slow { fill: var(--c-error); }
</style>
