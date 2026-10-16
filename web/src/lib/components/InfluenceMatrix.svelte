<script lang="ts">
  // Input × cache classes of one model (#116); background by median TTFT in
  // the single accent tone (lighter = faster).
  import type { Influence } from '../types/dashboard';
  import { cacheClassLabel, formatTps, formatUs, inputClassLabel, insightSentence } from '../perf';

  let { influence }: { influence: Influence } = $props();

  const inputs = ['i0', 'i1', 'i2', 'i3'];
  const caches = ['c0', 'c1', 'c2'];
  const cell = (i: string, c: string) => influence.cells.find((x) => x.input === i && x.cache === c);
  const ttfts = $derived(influence.cells.map((c) => c.ttft_median_us).filter((v): v is number => v !== null));
  function shade(v: number | null): string {
    if (v === null || ttfts.length === 0) return 'transparent';
    const lo = Math.min(...ttfts);
    const hi = Math.max(...ttfts);
    const share = hi === lo ? 0.3 : 0.1 + ((v - lo) / (hi - lo)) * 0.5;
    return `color-mix(in srgb, var(--c-accent) ${Math.round(share * 100)}%, transparent)`;
  }
</script>

{#each influence.insights as i}<p class="insight">{insightSentence(i)}</p>{/each}
<div class="table-wrap">
  <table class="tbl matrix" aria-label="Einfluss von Eingabe und Cache">
    <thead>
      <tr><th>Eingabe \ Cache-Anteil</th>{#each caches as c}<th class="num">{cacheClassLabel(c)}</th>{/each}</tr>
    </thead>
    <tbody>
      {#each inputs as i}
        <tr>
          <th scope="row">{inputClassLabel(i)}</th>
          {#each caches as c}
            {@const x = cell(i, c)}
            <td class="num" style="background: {shade(x?.ttft_median_us ?? null)}" data-testid="cell-{i}-{c}">
              {#if x && x.samples > 0}
                {#if x.ttft_median_us !== null}<span>{formatUs(x.ttft_median_us)}</span><span class="muted"> · {formatTps(x.tps_median)} t/s</span>{/if}
                <small class="muted">{x.samples}×</small>
              {:else}<span class="muted">–</span>{/if}
            </td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>
<p class="muted hint">Zusammenhang aus den eigenen Daten, keine Ursache. Zellen mit weniger als 5 Anfragen zeigen nur die Anzahl.</p>

<style>
  .insight { margin: 0 0 0.4rem; }
  .matrix td small { display: block; font-size: 0.72rem; }
  .matrix th[scope='row'] { font-weight: 500; color: var(--c-muted); font-size: 0.85rem; }
  .hint { font-size: 0.8rem; margin: 0.4rem 0 0; }
</style>
