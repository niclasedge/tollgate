<script lang="ts">
  // Median tokens/s per hour, one line per model (#116). Models differ by gray
  // tone and dash so the accent blue stays the only color.
  import { formatTps, type TpsLine } from '../perf';
  import { dayBands, hourTicks, labelWidth } from '../hours';

  let { hours, lines, label }: { hours: string[]; lines: TpsLine[]; label: string } = $props();

  const W = 640;
  const H = 180;
  const PAD = { l: 36, r: 8, t: 20, b: 22 };
  const styles = [
    { stroke: 'var(--c-accent)', dash: '' },
    { stroke: 'var(--c-text)', dash: '' },
    { stroke: 'var(--c-muted)', dash: '6 4' },
    { stroke: 'var(--c-muted)', dash: '2 3' },
    { stroke: 'var(--c-idle)', dash: '' },
  ];
  const style = (i: number) => styles[i % styles.length];

  const hi = $derived(Math.max(1, ...lines.flatMap((l) => l.values.filter((v): v is number => v !== null))));
  // A round upper bound, so the grid labels read well.
  const top = $derived.by(() => {
    const step = 10 ** Math.floor(Math.log10(hi));
    return Math.ceil(hi / step) * step;
  });
  const x = (i: number) => (hours.length > 1 ? PAD.l + (i / (hours.length - 1)) * (W - PAD.l - PAD.r) : (PAD.l + W - PAD.r) / 2);
  const y = (v: number) => H - PAD.b - (v / top) * (H - PAD.t - PAD.b);

  // Hours without requests are skipped: the line joins the known medians.
  const points = (values: (number | null)[]) =>
    values.flatMap((v, i) => (v === null ? [] : [`${x(i).toFixed(1)},${y(v).toFixed(1)}`])).join(' ');
  const ticks = $derived(hourTicks(hours));
  // Above the plot one row per model with its median per UTC day (#123).
  const ROW = 14;
  const band = $derived(lines.some((l) => l.days.size > 0) ? lines.length * ROW + 4 : 0);
  const dayText = (l: TpsLine, day: string) => (l.days.has(day) ? formatTps(l.days.get(day)) : '');
  const bands = $derived(band ? dayBands(hours, x, (d) => Math.max(...lines.map((l) => labelWidth(dayText(l, d))))) : []);
</script>

<figure class="chart" aria-label={label}>
  <figcaption class="muted">{label}</figcaption>
  <svg viewBox="0 0 {W} {H + band}" role="img" aria-label="{label}: {lines.map((l) => `${l.model} zuletzt ${formatTps(l.last)} t/s`).join(', ')}">
    {#each bands as b (b.day)}
      {#if b.sep !== null}<line class="day" x1={b.sep} x2={b.sep} y1={2} y2={band + H - PAD.b} />{/if}
      {#if b.at !== null}
        {#each lines as l, li (l.model)}
          {#if l.days.has(b.day)}
            <g>
              <title>{l.model} {b.day}: Median {dayText(l, b.day)} t/s</title>
              <text class="median" data-testid="day-tps" x={b.at} y={(li + 1) * ROW - 2} text-anchor="middle" fill={style(li).stroke}>{dayText(l, b.day)}</text>
            </g>
          {/if}
        {/each}
      {/if}
    {/each}
    <g transform="translate(0 {band})">
      {#each [0, top / 2, top] as g}
        <line class="grid" x1={PAD.l} x2={W - PAD.r} y1={y(g)} y2={y(g)} />
        <text class="axis" x={PAD.l - 6} y={y(g) + 4} text-anchor="end">{formatTps(g).replace(/,0$/, '')}</text>
      {/each}
      <text class="axis" x={PAD.l - 6} y={PAD.t - 12} text-anchor="end">t/s</text>
      {#each ticks as t (t.i)}
        <text class="axis" x={x(t.i)} y={H - 6} text-anchor="middle">{t.text}</text>
      {/each}
      {#each lines as l, li (l.model)}
        <g data-testid="tps-line" stroke={style(li).stroke} fill={style(li).stroke}>
          <title>{l.model}</title>
          <polyline points={points(l.values)} stroke-dasharray={style(li).dash} />
          {#each l.values as v, i}{#if v !== null}<circle cx={x(i)} cy={y(v)} r="2.5" />{/if}{/each}
        </g>
      {/each}
    </g>
  </svg>
  <ul class="legend">
    {#each lines as l, li (l.model)}
      <li>
        <svg width="22" height="8" aria-hidden="true"><line x1="1" x2="21" y1="4" y2="4" stroke={style(li).stroke} stroke-dasharray={style(li).dash} /></svg>
        <span class="mono">{l.model}</span>
        <span class="muted num">{formatTps(l.last)} t/s</span>
      </li>
    {/each}
  </ul>
</figure>

<style>
  .chart { margin: 0 0 1.5rem; display: grid; gap: 0.5rem; }
  figcaption { font-size: 0.85rem; }
  svg { display: block; width: 100%; height: auto; overflow: visible; }
  polyline { fill: none; stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
  circle { stroke: none; }
  .grid { stroke: var(--c-line); stroke-width: 1; }
  .axis { fill: var(--c-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
  .median { font-size: 11px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .day { stroke: var(--c-line); stroke-width: 1; stroke-dasharray: 2 3; }
  .legend { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: 0.35rem 1.25rem; font-size: 0.85rem; }
  .legend li { display: flex; align-items: center; gap: 0.4rem; }
  .legend svg { width: 22px; flex: none; }
  .legend line { stroke-width: 2; }
</style>
