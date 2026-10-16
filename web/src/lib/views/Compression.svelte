<script lang="ts">
  // Compress rules per command and a playground that shows every step (#21).
  import Disclosure from '../components/Disclosure.svelte';
  import StatusDot from '../components/StatusDot.svelte';
  import { api } from '../api';
  import type { Explanation, RuleInfo, RuleList } from '../types/compress';

  let list = $state<RuleList | undefined>();
  let loadError = $state('');

  async function load() {
    try {
      list = await api.compressRules();
      loadError = '';
    } catch (e) {
      loadError = (e as Error).message;
    }
  }
  $effect(() => void load());

  const limits = (r: RuleInfo) =>
    [
      [r.max_errors, 'Fehler'],
      [r.context_lines, 'Kontextzeilen'],
      [r.max_files, 'Dateien'],
      [r.max_hunks, 'Hunks'],
    ]
      .filter(([n]) => (n as number) > 0)
      .map(([n, what]) => `max. ${n} ${what}`)
      .join(' · ');

  let command = $state('');
  let output = $state('');
  let result = $state<Explanation | undefined>();
  let runError = $state('');
  let busy = $state(false);

  async function run(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    try {
      result = await api.compressPlayground(command, output);
      runError = '';
    } catch (err) {
      result = undefined;
      runError = (err as Error).message;
    } finally {
      busy = false;
    }
  }

  // Strategy steps keep their technical names; only the frame is translated.
  const stepTitle = (name: string) => (name === 'input' ? 'Eingabe' : name === 'result' ? 'Ergebnis mit Markerzeile' : name);
  const bytes = (n: number) => `${n.toLocaleString('de-DE')} Bytes`;
  const saved = (e: Explanation) => (e.bytes_before > 0 ? Math.round((1 - e.bytes_after / e.bytes_before) * 100) : 0);
</script>

<div class="page-head">
  <h1>Kompression</h1>
  {#if list}
    <span class="state muted">
      <StatusDot state={list.enabled ? 'ok' : 'idle'} label={list.enabled ? 'Kompression läuft' : 'Kompression aus'} />
      {list.enabled ? 'Läuft' : 'Aus'} · Version {list.version}
    </span>
  {/if}
</div>

{#if loadError}<p class="err" role="alert">{loadError}</p>{/if}

<section aria-labelledby="h-rules">
  <h2 id="h-rules" class="section first">Regeln pro Befehl</h2>
  {#if !list}
    {#if !loadError}<p class="muted">Lade …</p>{/if}
  {:else}
    <p class="muted small">
      Nur Bash-Ausgaben ab {bytes(list.min_bytes)}. Die erste passende Regel gewinnt; eigene Regeln stehen vor den eingebauten.
      {#if !list.enabled}Die Stufe ist aus (<code>TOLLGATE_COMPRESS_ENABLED</code>), der Playground zeigt trotzdem, was sie täte.{/if}
    </p>
    <ul class="rules">
      {#each list.rules as r (r.name)}
        <li data-testid="compress-rule">
          <span class="name">{r.name}</span>
          <span class="muted small">{r.strategy}{r.builtin ? '' : ' · eigene'}</span>
          {#if limits(r)}<span class="muted small">{limits(r)}</span>{/if}
          <ul class="chips">
            {#each r.match as m (m)}<li><code>{m}</code></li>{/each}
          </ul>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<section aria-labelledby="h-play">
  <h2 id="h-play" class="section">Playground</h2>
  <form class="stack" onsubmit={run}>
    <label>Befehl<input bind:value={command} placeholder="go test ./..." spellcheck="false" autocomplete="off" /></label>
    <label>Ausgabe<textarea bind:value={output} rows="8" spellcheck="false"></textarea></label>
    <div><button type="submit" class="btn" disabled={busy}>Ausführen</button> <span class="muted small">mit den Regeln der aktiven Version, nichts wird gespeichert</span></div>
  </form>
  {#if runError}<p class="err" role="alert">{runError}</p>{/if}
  {#if result}
    <div class="summary" data-testid="compress-summary">
      {#if result.applied}
        <p>
          <StatusDot state="ok" label="komprimiert" />
          Regel <strong>{result.rule}</strong> ({result.strategy}) · {bytes(result.bytes_before)} → {bytes(result.bytes_after)}
          (−{saved(result)} %) · {result.omitted} Zeilen ausgelassen
        </p>
      {:else}
        <p>
          <StatusDot state="idle" label="unverändert" />
          {#if result.rule}Regel <strong>{result.rule}</strong> ({result.strategy}) ·{' '}{/if}{result.reason}
        </p>
      {/if}
      <p class="muted small">
        {#if result.normalized}Geprüfter Befehl: <code>{result.normalized}</code>{:else}Kein einzelner Befehl erkannt.{/if}
      </p>
    </div>
    <ol class="steps">
      {#each result.steps as s, i (i)}
        <li data-testid="compress-step">
          <Disclosure title="{i + 1}. {stepTitle(s.name)} · {s.lines} Zeilen · {bytes(s.bytes)}" open={s.name === 'result'}>
            <pre class="out">{s.text}</pre>
          </Disclosure>
        </li>
      {/each}
    </ol>
  {/if}
</section>

<style>
  .first { margin-top: 0; }
  .small { font-size: 0.8rem; }
  .state { display: inline-flex; align-items: center; gap: 0.45rem; }
  ul, ol { list-style: none; margin: 0; padding: 0; }
  .rules > li { display: flex; align-items: baseline; gap: 0.6rem; padding: 0.45rem 0; border-bottom: 1px solid var(--c-line); flex-wrap: wrap; }
  .rules .name { font-weight: 500; min-width: 7rem; }
  .chips { display: flex; flex-wrap: wrap; gap: 0.35rem; flex-basis: 100%; }
  .chips li { font-size: 0.82rem; padding: 0.1rem 0.6rem; border-radius: 999px; background: var(--c-hover); }
  code { font-family: var(--mono); font-size: 0.82rem; }
  .stack { display: grid; gap: 0.5rem; margin-top: 0.5rem; }
  label { display: flex; flex-direction: column; gap: 0.2rem; font-size: 0.8rem; color: var(--c-muted); }
  input, textarea { font: inherit; font-size: 0.9rem; color: var(--c-text); background: var(--c-surface); border: 1px solid var(--c-line); border-radius: var(--radius); padding: 0.4rem 0.6rem; }
  textarea { font-family: var(--mono); font-size: 0.82rem; }
  input:focus-visible, textarea:focus-visible { outline: 2px solid var(--c-accent); outline-offset: -1px; }
  .summary { margin-top: 1rem; }
  .summary p { display: flex; align-items: center; gap: 0.45rem; flex-wrap: wrap; margin: 0.3rem 0; }
  .steps { margin-top: 0.5rem; }
  .steps > li { border-bottom: 1px solid var(--c-line); }
  .out { margin: 0; padding: 0.6rem 0.75rem; background: var(--c-sidebar); border-radius: var(--radius); font: 0.82rem/1.45 var(--mono); white-space: pre-wrap; overflow-wrap: anywhere; max-height: 28rem; overflow: auto; }
</style>
