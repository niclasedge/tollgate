<script lang="ts">
  // Cost per hour, one colored line per account on a shared scale (#128).
  // Lines are drawn over each other, not stacked, so each shows its own cost.
  import { formatCost } from '../format';
  import { seriesColor, type CostSeries } from '../costs';
  import { dayBands, hourTicks, labelWidth } from '../hours';

  // days maps a UTC day to its cost sum, written above the day's hours (#123).
  let { data, labels, label, days }: { data: CostSeries; labels: string[]; label: string; days?: ReadonlyMap<string, string> } = $props();

  const W = 640;
  const H = 200;
  const PAD = { l: 52, r: 8, t: 12, b: 22 };
  const n = $derived(data.buckets.length);

  const hi = $derived(Math.max(0, ...data.series.flatMap((s) => s.values)));
  // A round upper bound (1, 2 or 5 times a power of ten), so the grid reads well.
  const top = $derived.by(() => {
    if (hi <= 0) return 1;
    const step = 10 ** Math.floor(Math.log10(hi));
    return ([1, 2, 5, 10].find((f) => f * step >= hi) ?? 10) * step;
  });
  const x = (i: number) => (n > 1 ? PAD.l + (i / (n - 1)) * (W - PAD.l - PAD.r) : (PAD.l + W - PAD.r) / 2);
  const y = (v: number) => H - PAD.b - (v / top) * (H - PAD.t - PAD.b);
  const line = (values: number[]) => (n === 0 ? '' : `M${values.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join('L')}`);

  const ticks = $derived(hourTicks(data.buckets));
  // The day row sits above the plot; the plot keeps its height.
  const band = $derived(days ? 18 : 0);
  const bands = $derived(days ? dayBands(data.buckets, x, (d) => labelWidth(days.get(d) ?? '')) : []);

  const summary = $derived(data.series.map((s, i) => `${labels[i]} ${formatCost(s.values.reduce((a, v) => a + v, 0))}`).join(', '));
  // Axis labels without trailing zeros: $0.5, $2.
  const usd = (v: number) => formatCost(v).replace(/(\.\d*?)0+$/, '$1').replace(/\.$/, '');
</script>

<figure class="chart" aria-label={label}>
  <figcaption class="muted">{label}</figcaption>
  <svg viewBox="0 0 {W} {H + band}" role="img" aria-label="{label}: {summary || 'keine Kosten'}">
    {#each bands as b (b.day)}
      {#if b.sep !== null}<line class="day" x1={b.sep} x2={b.sep} y1={2} y2={band + H - PAD.b} />{/if}
      {#if b.at !== null}<text class="sum" data-testid="day-sum" x={b.at} y={band - 5} text-anchor="middle">{days?.get(b.day)}</text>{/if}
    {/each}
    <g transform="translate(0 {band})">
      {#each [0, top / 2, top] as g}
        <line class="grid" x1={PAD.l} x2={W - PAD.r} y1={y(g)} y2={y(g)} />
        <text class="axis" x={PAD.l - 6} y={y(g) + 4} text-anchor="end">{usd(g)}</text>
      {/each}
      {#each ticks as t (t.i)}
        <text class="axis" x={x(t.i)} y={H - 6} text-anchor="middle">{t.text}</text>
      {/each}
      {#each data.series as s, i (s.key)}
        <g data-testid="cost-line" style="color: {seriesColor(i)}">
          <title>{labels[i]}</title>
          <path class="edge" d={line(s.values)} />
        </g>
      {/each}
    </g>
  </svg>
</figure>

<style>
  .chart { margin: 0; display: grid; gap: 0.5rem; min-width: 0; }
  figcaption { font-size: 0.85rem; }
  svg { display: block; width: 100%; height: auto; overflow: visible; }
  .edge { fill: none; stroke: currentColor; stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
  .grid { stroke: var(--c-line); stroke-width: 1; }
  .axis { fill: var(--c-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
  .sum { fill: var(--c-text); font-size: 11px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .day { stroke: var(--c-line); stroke-width: 1; stroke-dasharray: 2 3; }
</style>
