<script lang="ts">
  // The lists of customers, first names and last names (#143, ADR-0021).
  // Entries are shown masked; "Bearbeiten" opens a textarea with one entry
  // per line. The plain text lives only in the parent's edit state while the
  // textarea is open (or as a pending replacement), never in the YAML draft
  // of a stored version.
  import { nameListLabels, type NameListKey } from '../filterdraft';

  type Props = {
    /** Masked entries per list. */
    entries: Record<NameListKey, string[]>;
    /** Lists replaced in the draft that are not saved yet. */
    pending: Record<NameListKey, boolean>;
    /** The list whose textarea is open. */
    editing: NameListKey | undefined;
    text: string;
    loading: boolean;
    /** Error of the plain-text request and the list it belongs to. */
    error: string;
    errorKey: NameListKey | undefined;
    onedit: (key: NameListKey) => void;
    onapply: () => void;
    oncancel: () => void;
  };
  let { entries, pending, editing, text = $bindable(), loading, error, errorKey, onedit, onapply, oncancel }: Props = $props();

  let box = $state<HTMLTextAreaElement>();
  $effect(() => {
    if (editing) box?.focus();
  });

  function key(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      oncancel();
    }
  }

  const count = (n: number) => (n === 0 ? 'Keine Einträge' : n === 1 ? '1 Eintrag' : `${n} Einträge`);
</script>

{#each nameListLabels as { key: k, title, type } (k)}
  <div class="names" role="group" aria-labelledby={`nl-${k}`}>
    <div class="head">
      <h3 id={`nl-${k}`}>{title}</h3>
      <span class="muted small">{count(entries[k].length)} · <code>{type}</code></span>
      {#if pending[k]}<span class="muted small">· geändert, nach „Prüfen“ speicherbar</span>{/if}
      {#if editing !== k}
        <button type="button" class="btn" aria-label={`${title} bearbeiten`} disabled={loading || editing !== undefined} onclick={() => onedit(k)}>Bearbeiten</button>
      {/if}
    </div>
    {#if editing === k}
      <form
        class="edit"
        onsubmit={(e) => {
          e.preventDefault();
          onapply();
        }}
      >
        <label>
          {title} (eine Zeile je Eintrag)
          <textarea bind:this={box} bind:value={text} rows="8" autocomplete="off" spellcheck="false" onkeydown={key}></textarea>
        </label>
        <div class="row">
          <button type="submit" class="btn">Übernehmen</button>
          <button type="button" class="btn" onclick={oncancel}>Abbrechen</button>
        </div>
      </form>
    {:else if entries[k].length > 0}
      <ul class="chips">
        {#each entries[k] as m, i (i)}<li class="mono" class:new={pending[k]}><span>{m}</span></li>{/each}
      </ul>
    {/if}
    {#if error && errorKey === k}<p class="err small" role="alert">{error}</p>{/if}
  </div>
{/each}

<style>
  .names { margin-top: 0.9rem; }
  .head { display: flex; align-items: baseline; gap: 0.5rem; flex-wrap: wrap; }
  h3 { font-size: 0.9rem; font-weight: 600; margin: 0; }
  .small { font-size: 0.8rem; }
  .head .btn { margin-left: auto; padding: 0.15rem 0.6rem; font-size: 0.8rem; }
  code { font-family: var(--mono); font-size: 0.78rem; }
  ul { list-style: none; margin: 0; padding: 0; }
  .chips { display: flex; flex-wrap: wrap; gap: 0.35rem; margin: 0.35rem 0; }
  .chips li { display: inline-flex; align-items: center; font-size: 0.82rem; padding: 0.1rem 0.6rem; border-radius: 999px; background: var(--c-hover); }
  .chips li.new { background: var(--c-accent-soft); color: var(--c-accent); }
  .edit { display: grid; gap: 0.5rem; margin-top: 0.4rem; }
  label { display: flex; flex-direction: column; gap: 0.2rem; font-size: 0.8rem; color: var(--c-muted); }
  textarea { font: inherit; font-family: var(--mono); font-size: 0.85rem; color: var(--c-text); background: var(--c-surface); border: 1px solid var(--c-line); border-radius: var(--radius); padding: 0.4rem 0.6rem; resize: vertical; min-height: 6rem; }
  textarea:focus-visible { outline: 2px solid var(--c-accent); outline-offset: -1px; }
  .row { display: flex; gap: 0.5rem; flex-wrap: wrap; }
</style>
