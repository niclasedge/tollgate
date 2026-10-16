<script lang="ts">
  // Redactor configuration: form and YAML editor edit one draft, a dry run
  // on the fixtures must pass before the draft is stored and activated.
  // The stored word lists are never sent back: drafts take them over via
  // words_from and edit the general list with words_remove/words_add (applied
  // by the server), the frontend only ever sees them masked. The one
  // exception (ADR-0021): "Bearbeiten" on Kunden, Vornamen or Nachnamen loads
  // that list in plain text once into a textarea; the result goes back as
  // lists_set. The plain text lives only in the component state while editing
  // or as a pending replacement, never in the YAML draft.
  import { api, ApiError } from '../api';
  import { formatDateTime } from '../format';
  import NameLists from '../components/NameLists.svelte';
  // CodeMirror and the schema linter are large: loaded only with this view.
  const editorModule = import('../components/YamlEditor.svelte');
  import {
    addAllow,
    addCustom,
    builtinLabels,
    draftFromVersion,
    mask,
    nameListKeys,
    parseLines,
    readDraft,
    removeAllow,
    removeCustom,
    removeWord,
    setBuiltin,
    setNameList,
    setWordOption,
    setWords,
    splitWords,
  } from '../filterdraft';
  import type { NameListKey } from '../filterdraft';
  import type { DryRunResult, Hit, Issue, ListsSet, NameLists as NameListsData, TestResult, VersionDetail, VersionInfo, WordOps } from '../types/redactcfg';

  let schema = $state<unknown>();
  let versions = $state<VersionInfo[]>([]);
  let active = $state(0);
  let loadError = $state('');
  // The form appears only with the loaded draft, never on the empty initial text.
  let ready = $state(false);

  let draft = $state('');
  let wordsFrom = $state(0);
  let storedMasked = $state<string[]>([]);
  // Masked name lists of the words_from version, and lists replaced since (plain text, pending lists_set).
  let storedNames = $state<NameListsData>({ customers: [], first_names: [], last_names: [] });
  let listsSet = $state<ListsSet>({});
  // The list whose textarea is open, its text, and the state of the plain-text request.
  let editingList = $state<NameListKey | undefined>();
  let editText = $state('');
  let listsLoading = $state(false);
  let listsError = $state('');
  let listsErrorKey = $state<NameListKey | undefined>();
  // Word-list operations on the words_from list: indices to drop, plain words to append.
  let wordsRemove = $state<number[]>([]);
  let wordsAdd = $state<string[]>([]);
  let formError = $state('');

  // The dry run belongs to exactly one draft; any edit invalidates it.
  let checked = $state<{ yaml: string; wordsFrom: number; ops: WordOps; key: string } | undefined>();
  let preview = $state<DryRunResult | undefined>();
  let issues = $state<Issue[]>([]);
  let checkError = $state('');
  let busy = $state(false);
  let comment = $state('');
  let saved = $state('');

  const doc = $derived(readDraft(draft));
  const baseWords = $derived(wordsFrom > 0 ? storedMasked : (doc.doc?.words.list ?? []).map(mask));
  // Name lists replace only while the draft takes the stored lists over; without words_from they live in the YAML.
  const setKeys = $derived(wordsFrom > 0 ? nameListKeys.filter((k) => listsSet[k] !== undefined) : []);
  const ops = $derived<WordOps>({
    ...(wordsRemove.length > 0 ? { words_remove: [...wordsRemove].sort((a, b) => a - b) } : {}),
    ...(wordsAdd.length > 0 ? { words_add: [...wordsAdd] } : {}),
    ...(setKeys.length > 0 ? { lists_set: Object.fromEntries(setKeys.map((k) => [k, [...(listsSet[k] ?? [])]])) as ListsSet } : {}),
  });
  const nameEntries = $derived.by(() => {
    const out = {} as Record<NameListKey, string[]>;
    for (const k of nameListKeys) {
      out[k] = wordsFrom > 0 ? (listsSet[k] ? listsSet[k].map(mask) : storedNames[k]) : (doc.doc?.words[k] ?? []).map(mask);
    }
    return out;
  });
  const namePending = $derived(Object.fromEntries(nameListKeys.map((k) => [k, setKeys.includes(k)])) as Record<NameListKey, boolean>);
  // A taken-over version with name lists: replacing or dropping the general list must keep words_from.
  const keepsNames = $derived(wordsFrom > 0 && nameListKeys.some((k) => nameEntries[k].length > 0));
  const opsKey = $derived(JSON.stringify(ops));
  const wordCount = $derived(baseWords.length - wordsRemove.length + wordsAdd.length);
  const canSave = $derived(!busy && !!checked && checked.yaml === draft && checked.wordsFrom === wordsFrom && checked.key === opsKey);

  function resetWordOps() {
    wordsRemove = [];
    wordsAdd = [];
    listsSet = {};
    cancelList();
  }

  async function loadVersions() {
    const l = await api.configVersions();
    versions = l.versions;
    active = l.active;
    return l.active;
  }

  async function loadDraft(id: number) {
    const v = await api.configVersion(id);
    const d = draftFromVersion(v.yaml, v.id, v.has_words);
    draft = d.yaml;
    wordsFrom = d.wordsFrom;
    storedMasked = d.masked;
    storedNames = d.names;
    resetWordOps();
    checked = undefined;
    preview = undefined;
    issues = [];
  }

  async function load() {
    try {
      const [s, id] = await Promise.all([api.configSchema(), loadVersions()]);
      schema = s;
      if (id > 0) await loadDraft(id);
      ready = true;
      loadError = '';
    } catch (e) {
      loadError = (e as Error).message;
    }
  }

  $effect(() => void load());

  function change(fn: (yaml: string) => string) {
    try {
      draft = fn(draft);
      formError = '';
    } catch (e) {
      formError = (e as Error).message;
    }
  }

  // --- form state ---
  let rule = $state({ name: '', pattern: '', placeholder: '', example: '' });
  let allow = $state('');
  let newWords = $state('');
  let addText = $state('');

  function addRule(e: SubmitEvent) {
    e.preventDefault();
    const r = rule;
    if (!r.name || !r.pattern || !r.placeholder) return;
    change((y) => addCustom(y, { name: r.name.trim(), pattern: r.pattern, placeholder: r.placeholder.trim(), examples: r.example ? [r.example] : [] }));
    rule = { name: '', pattern: '', placeholder: '', example: '' };
  }

  function addAllowed(e: SubmitEvent) {
    e.preventDefault();
    if (!allow.trim()) return;
    change((y) => addAllow(y, allow.trim()));
    allow = '';
  }

  function replaceWords(e: SubmitEvent) {
    e.preventDefault();
    const list = newWords.split('\n').map((w) => w.trim()).filter(Boolean);
    if (list.length === 0) return;
    if (keepsNames) {
      // Stay on words_from: replace only the general list, by operations.
      wordsRemove = storedMasked.map((_, i) => i);
      wordsAdd = list;
    } else {
      change((y) => setWords(y, list));
      wordsFrom = 0;
      resetWordOps();
    }
    newWords = '';
  }

  function dropWords() {
    if (keepsNames) {
      wordsRemove = storedMasked.map((_, i) => i);
      wordsAdd = [];
      return;
    }
    change((y) => setWords(y, []));
    wordsFrom = 0;
    resetWordOps();
  }

  // --- Kunden, Vornamen, Nachnamen ---
  async function editList(key: NameListKey) {
    listsError = '';
    let current: string[];
    if (wordsFrom > 0) {
      const pending = listsSet[key];
      if (pending) current = pending;
      else {
        listsLoading = true;
        try {
          current = (await api.nameLists(wordsFrom))[key];
        } catch (e) {
          listsError = (e as Error).message;
          listsErrorKey = key;
          return;
        } finally {
          listsLoading = false;
        }
      }
    } else {
      current = doc.doc?.words[key] ?? [];
    }
    editText = current.join('\n');
    editingList = key;
  }

  function applyList() {
    const key = editingList;
    if (!key) return;
    const list = parseLines(editText, doc.doc?.words.case_sensitive ?? false);
    if (wordsFrom > 0) listsSet = { ...listsSet, [key]: list };
    else change((y) => setNameList(y, key, list));
    cancelList();
  }

  // Drops the textarea and with it the plain text.
  function cancelList() {
    editingList = undefined;
    editText = '';
  }

  function addWords(e: SubmitEvent) {
    e.preventDefault();
    const list = splitWords(addText);
    if (list.length === 0) return;
    wordsAdd = [...wordsAdd, ...list];
    addText = '';
  }

  // A words_from entry is dropped by its index on the server; an entry of a
  // list in the draft (typed by the user or loaded from a file) directly in the YAML.
  function removeEntry(i: number) {
    if (wordsFrom > 0) wordsRemove = [...wordsRemove, i];
    else change((y) => removeWord(y, i));
  }

  // --- dry run and save ---
  function describe(i: Issue): string {
    const where = i.line > 0 ? ` (Zeile ${i.line}, Spalte ${i.column})` : '';
    return `${i.path || 'Dokument'}${where}: ${i.message}`;
  }

  function fail(e: unknown) {
    const err = e as Error;
    issues = err instanceof ApiError ? err.issues : [];
    checkError = issues.length > 0 ? '' : err.message;
  }

  async function check() {
    const snap = { yaml: draft, wordsFrom, ops, key: opsKey };
    busy = true;
    saved = '';
    try {
      preview = await api.dryRun(snap.yaml, snap.wordsFrom, snap.ops);
      issues = [];
      checkError = '';
      checked = snap;
    } catch (e) {
      preview = undefined;
      checked = undefined;
      fail(e);
    } finally {
      busy = false;
    }
  }

  async function save() {
    if (!checked) return;
    busy = true;
    try {
      const v = await api.importConfig(checked.yaml, checked.wordsFrom, comment.trim() || 'Filter-Bereich', true, checked.ops);
      saved = `Version ${v.id} gespeichert und aktiv.`;
      comment = '';
      await loadDraft(await loadVersions());
    } catch (e) {
      fail(e);
    } finally {
      busy = false;
    }
  }

  const isNew = (h: Hit, act: Hit[]) => !act.some((a) => a.path === h.path && a.start === h.start && a.end === h.end && a.type === h.type);

  // --- live test ---
  let testText = $state('');
  let testResult = $state<TestResult | undefined>();
  let testError = $state('');

  async function runTest(e: SubmitEvent) {
    e.preventDefault();
    try {
      testResult = await api.testFilter(testText, draft, wordsFrom, ops);
      testError = '';
    } catch (err) {
      testResult = undefined;
      const x = err as Error;
      testError = x instanceof ApiError && x.issues.length > 0 ? x.issues.map(describe).join(' · ') : x.message;
    }
  }

  // --- versions ---
  let openVersion = $state<number | undefined>();
  let details = $state<Record<number, VersionDetail>>({});
  let versionError = $state('');

  async function toggleVersion(id: number) {
    openVersion = openVersion === id ? undefined : id;
    if (openVersion !== undefined && !details[id]) {
      try {
        details[id] = await api.configVersion(id);
      } catch (e) {
        versionError = (e as Error).message;
      }
    }
  }

  async function rollback(id: number) {
    try {
      const v = await api.rollbackConfig(id);
      saved = `Version ${v.id} (Rollback auf Version ${id}) ist aktiv.`;
      openVersion = undefined;
      await loadDraft(await loadVersions());
      versionError = '';
    } catch (e) {
      versionError = (e as Error).message;
    }
  }

  function intoDraft(v: VersionDetail) {
    const d = draftFromVersion(v.yaml, v.id, v.has_words);
    draft = d.yaml;
    wordsFrom = d.wordsFrom;
    storedMasked = d.masked;
    storedNames = d.names;
    resetWordOps();
  }

  // --- import and export ---
  let confirmExport = $state(false);
  let ioError = $state('');
  let cancelBtn = $state<HTMLButtonElement>();

  $effect(() => {
    if (confirmExport) cancelBtn?.focus();
  });

  async function download(words: 'omit' | 'plain') {
    confirmExport = false;
    try {
      const r = await api.exportConfig(words);
      const url = URL.createObjectURL(new Blob([r.text], { type: 'application/yaml' }));
      const a = document.createElement('a');
      a.href = url;
      a.download = r.filename;
      a.click();
      URL.revokeObjectURL(url);
      ioError = '';
    } catch (e) {
      ioError = (e as Error).message;
    }
  }

  async function loadFile(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    // A file is a complete configuration: its own word list replaces the stored one.
    draft = await file.text();
    wordsFrom = 0;
    resetWordOps();
    input.value = '';
  }

  function dialogKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      confirmExport = false;
    }
  }
</script>

<div class="page-head">
  <h1>Filter</h1>
  <span class="muted">{active > 0 ? `Version ${active} aktiv` : ''}</span>
</div>

{#if loadError}<p class="err" role="alert">{loadError}</p>{/if}

<div class="cols">
  <section aria-labelledby="h-active" class="form">
    <h2 id="h-active" class="section first">Aktive Filter</h2>
    {#if !ready}
      <p class="muted">Lade …</p>
    {:else if doc.error}
      <p class="err">Der Entwurf ist kein gültiges YAML ({doc.error}). Bitte im Editor korrigieren.</p>
    {:else if doc.doc}
      {@const d = doc.doc}
      {#if formError}<p class="err" role="alert">{formError}</p>{/if}

      <h3>Eingebaute Filter</h3>
      <ul class="switches">
        {#each builtinLabels as [key, label] (key)}
          <li>
            <button type="button" role="switch" aria-checked={d.builtin[key]} class="sw" onclick={() => change((y) => setBuiltin(y, key, !d.builtin[key]))}>
              <span class="track" aria-hidden="true"></span>{label}
            </button>
          </li>
        {/each}
      </ul>

      <h3>Eigene Regeln</h3>
      {#if d.custom.length === 0}<p class="muted small">Keine eigenen Regeln.</p>{/if}
      <ul class="rules">
        {#each d.custom as r, i (i)}
          <li>
            <span class="name">{r.name}</span>
            <code>{r.pattern}</code>
            <span class="muted small">→ {r.placeholder}</span>
            <button type="button" class="x" aria-label={`Regel ${r.name} entfernen`} onclick={() => change((y) => removeCustom(y, i))}>×</button>
          </li>
        {/each}
      </ul>
      <form class="grid" onsubmit={addRule}>
        <label>Name<input bind:value={rule.name} autocomplete="off" /></label>
        <label>Muster (RE2)<input bind:value={rule.pattern} class="mono" autocomplete="off" spellcheck="false" /></label>
        <label>Platzhalter-Typ<input bind:value={rule.placeholder} autocomplete="off" /></label>
        <label>Beispiel<input bind:value={rule.example} autocomplete="off" /></label>
        <button type="submit" class="btn">Regel hinzufügen</button>
      </form>

      <h3>Wortliste <span class="muted small">(allgemein, maskiert)</span></h3>
      <ul class="switches">
        <li>
          <button type="button" role="switch" aria-checked={d.words.case_sensitive} class="sw" onclick={() => change((y) => setWordOption(y, 'case_sensitive', !d.words.case_sensitive))}>
            <span class="track" aria-hidden="true"></span>Groß-/Kleinschreibung beachten
          </button>
        </li>
        <li>
          <button type="button" role="switch" aria-checked={d.words.whole_word} class="sw" onclick={() => change((y) => setWordOption(y, 'whole_word', !d.words.whole_word))}>
            <span class="track" aria-hidden="true"></span>Nur ganze Wörter
          </button>
        </li>
      </ul>
      {#if wordCount === 0}
        <p class="muted small">Keine Wörter.</p>
      {:else}
        <ul class="chips">
          {#each baseWords as w, i (i)}
            {#if !wordsRemove.includes(i)}
              <li class="mono"><span>{w}</span><button type="button" class="x" aria-label={`Eintrag ${i + 1} entfernen`} onclick={() => removeEntry(i)}>×</button></li>
            {/if}
          {/each}
          {#each wordsAdd as w, j (j)}
            <li class="mono new"><span>{mask(w)}</span><button type="button" class="x" aria-label={`Neuen Eintrag ${j + 1} entfernen`} onclick={() => wordsAdd.splice(j, 1)}>×</button></li>
          {/each}
        </ul>
      {/if}
      {#if baseWords.length > 0 || wordsAdd.length > 0}
        <p class="muted small">
          {wordsFrom > 0 ? `Übernommen aus Version ${wordsFrom}.` : 'Neue Liste im Entwurf.'}
          {#if wordsAdd.length > 0 || wordsRemove.length > 0}{wordsAdd.length} neu, {wordsRemove.length} entfernt – nach „Prüfen“ speicherbar.{/if}
          <button type="button" class="link" onclick={dropWords}>Wortliste entfernen</button>
        </p>
      {/if}
      <form class="stack" onsubmit={addWords}>
        <label>Wörter hinzufügen (ein Eintrag je Zeile oder kommagetrennt)<textarea bind:value={addText} rows="2" autocomplete="off" spellcheck="false"></textarea></label>
        <button type="submit" class="btn">Hinzufügen</button>
      </form>
      <form class="stack" onsubmit={replaceWords}>
        <label>Neue Wortliste (ein Eintrag je Zeile)<textarea bind:value={newWords} rows="2" autocomplete="off" spellcheck="false"></textarea></label>
        <button type="submit" class="btn">Wortliste ersetzen</button>
      </form>

      <NameLists
        entries={nameEntries}
        pending={namePending}
        editing={editingList}
        bind:text={editText}
        loading={listsLoading}
        error={listsError}
        errorKey={listsErrorKey}
        onedit={editList}
        onapply={applyList}
        oncancel={cancelList}
      />

      <h3>Allowlist</h3>
      <ul class="chips">
        {#each d.allowlist as a, i (i)}
          <li><span>{a}</span><button type="button" class="x" aria-label={`${a} von der Allowlist entfernen`} onclick={() => change((y) => removeAllow(y, i))}>×</button></li>
        {/each}
      </ul>
      <form class="row" onsubmit={addAllowed}>
        <label class="grow">Neuer Allowlist-Eintrag<input bind:value={allow} autocomplete="off" /></label>
        <button type="submit" class="btn">Zur Allowlist</button>
      </form>
    {/if}
  </section>

  <section aria-labelledby="h-draft">
    <h2 id="h-draft" class="section first">Entwurf (YAML)</h2>
    {#await editorModule}
      <p class="muted">Editor wird geladen …</p>
    {:then { default: YamlEditor }}
      <YamlEditor value={draft} {schema} label="YAML-Entwurf" onchange={(v) => (draft = v)} oncheck={check} />
    {:catch e}
      <p class="err" role="alert">Editor nicht ladbar: {e.message}</p>
    {/await}
    <div class="actions">
      <label class="grow">Kommentar<input bind:value={comment} placeholder="optional" autocomplete="off" /></label>
      <button type="button" class="btn" disabled={busy} onclick={check}>Prüfen</button>
      <button type="button" class="btn primary" disabled={!canSave} onclick={save}>Speichern und aktivieren</button>
    </div>
    {#if saved}<p class="ok" role="status">{saved}</p>{/if}
    {#if checkError}<p class="err" role="alert">{checkError}</p>{/if}
    {#if issues.length > 0}
      <ul class="issues" role="alert">
        {#each issues as i, n (n)}<li>{describe(i)}</li>{/each}
      </ul>
    {/if}
    {#if preview}
      <section aria-labelledby="h-preview" class="preview">
        <h3 id="h-preview">Vorschau</h3>
        {#if preview.needs_sealer}<p class="err">Die Wortliste kann nicht gespeichert werden: kein Schlüssel verbunden.</p>{/if}
        <ul class="fixtures">
          {#each preview.fixtures as fx (fx.name)}
            <li>
              <div><span class="mono">{fx.name}</span> <span class="muted small">aktiv {fx.active.length} · Entwurf {fx.candidate.length} Treffer</span></div>
              {#if fx.error}<p class="err small">{fx.error}</p>{/if}
              <ul class="chips">
                {#each fx.candidate as h, n (n)}
                  <li class:new={isNew(h, fx.active)} title={`${h.rule} · ${h.path ?? ''} [${h.start}–${h.end}]`}>{h.type}</li>
                {/each}
              </ul>
            </li>
          {/each}
        </ul>
        <p class="muted small">Hervorgehoben: Treffer, die erst der Entwurf findet.</p>
      </section>
    {/if}
  </section>
</div>

<section aria-labelledby="h-test">
  <h2 id="h-test" class="section">Live-Test</h2>
  <form class="stack" onsubmit={runTest}>
    <label>Testtext<textarea bind:value={testText} rows="3" spellcheck="false"></textarea></label>
    <div><button type="submit" class="btn">Testen</button> <span class="muted small">prüft den aktuellen Entwurf</span></div>
  </form>
  {#if testError}<p class="err" role="alert">{testError}</p>{/if}
  {#if testResult}
    <pre class="out">{testResult.redacted}</pre>
    {#if testResult.hits.length === 0}
      <p class="muted small">Keine Treffer.</p>
    {:else}
      <ul class="chips">
        {#each testResult.hits as h, n (n)}<li title={`${h.rule} [${h.start}–${h.end}]`}>{h.type}</li>{/each}
      </ul>
    {/if}
  {/if}
</section>

<section aria-labelledby="h-versions">
  <h2 id="h-versions" class="section">Versionen</h2>
  {#if versionError}<p class="err" role="alert">{versionError}</p>{/if}
  <ul class="versions">
    {#each versions as v (v.id)}
      <li>
        <button type="button" class="row-btn" aria-expanded={openVersion === v.id} onclick={() => toggleVersion(v.id)}>
          <span class="vid">Version {v.id}</span>
          <span class="muted">{formatDateTime(v.created_at)}</span>
          <span>{v.author}{v.comment ? ` · ${v.comment}` : ''}</span>
          <span class="muted small">{v.active ? 'aktiv' : ''}</span>
        </button>
        {#if openVersion === v.id}
          <div class="vbody">
            {#if v.diff}<pre class="out">{v.diff}</pre>{:else}<p class="muted small">Kein Diff (erste Version).</p>{/if}
            <div class="row">
              {#if details[v.id]}
                {@const det = details[v.id]}
                <button type="button" class="btn" onclick={() => intoDraft(det)}>In den Entwurf laden</button>
              {/if}
              {#if !v.active}<button type="button" class="btn" onclick={() => rollback(v.id)}>Zurückrollen auf Version {v.id}</button>{/if}
            </div>
          </div>
        {/if}
      </li>
    {/each}
  </ul>
</section>

<section aria-labelledby="h-io">
  <h2 id="h-io" class="section">Import und Export</h2>
  {#if ioError}<p class="err" role="alert">{ioError}</p>{/if}
  <div class="row">
    <button type="button" class="btn" onclick={() => download('omit')}>Exportieren</button>
    <button type="button" class="btn" onclick={() => (confirmExport = true)}>Mit Wortliste exportieren …</button>
    <label class="btn file">YAML-Datei laden<input type="file" accept=".yaml,.yml,application/yaml,text/yaml" onchange={loadFile} /></label>
  </div>
  <p class="muted small">Eine geladene Datei landet im Entwurf und wird erst nach „Prüfen“ gespeichert.</p>
</section>

{#if confirmExport}
  <div class="backdrop">
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="dlg-t" aria-describedby="dlg-d" tabindex="-1" onkeydown={dialogKey}>
      <h3 id="dlg-t">Export mit Wortliste</h3>
      <p id="dlg-d">Die Datei enthält die vertrauliche Wortliste – nicht committen. Sie umfasst auch die Listen für Kunden, Vornamen und Nachnamen.</p>
      <div class="row end">
        <button type="button" class="btn" bind:this={cancelBtn} onclick={() => (confirmExport = false)}>Abbrechen</button>
        <button type="button" class="btn danger" onclick={() => download('plain')}>Trotzdem exportieren</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .first { margin-top: 0; }
  .small { font-size: 0.8rem; }
  .ok { color: var(--c-ok); font-size: 0.9rem; }
  h3 { font-size: 0.9rem; font-weight: 600; margin: 1.25rem 0 0.4rem; }
  .cols { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr); gap: 0 2.5rem; }
  ul { list-style: none; margin: 0; padding: 0; }
  label { display: flex; flex-direction: column; gap: 0.2rem; font-size: 0.8rem; color: var(--c-muted); }
  input, textarea { font: inherit; font-size: 0.9rem; color: var(--c-text); background: var(--c-surface); border: 1px solid var(--c-line); border-radius: var(--radius); padding: 0.4rem 0.6rem; }
  input:focus-visible, textarea:focus-visible { outline: 2px solid var(--c-accent); outline-offset: -1px; }
  .switches { display: grid; gap: 0.15rem; }
  .sw { all: unset; box-sizing: border-box; display: flex; align-items: center; gap: 0.6rem; width: 100%; padding: 0.35rem 0.3rem; border-radius: var(--radius); cursor: pointer; font-size: 0.92rem; }
  .sw:hover { background: var(--c-hover); }
  .sw:focus-visible { outline: 2px solid var(--c-accent); outline-offset: -2px; }
  .track { position: relative; flex: none; width: 1.9rem; height: 1.1rem; border-radius: 999px; background: var(--c-idle); transition: background var(--t); }
  .track::after { content: ''; position: absolute; top: 0.15rem; left: 0.15rem; width: 0.8rem; height: 0.8rem; border-radius: 50%; background: #fff; transition: transform var(--t); }
  .sw[aria-checked='true'] .track { background: var(--c-accent); }
  .sw[aria-checked='true'] .track::after { transform: translateX(0.8rem); }
  .rules li { display: flex; align-items: baseline; gap: 0.6rem; padding: 0.35rem 0; border-bottom: 1px solid var(--c-line); flex-wrap: wrap; }
  .rules .name { font-weight: 500; }
  code { font-family: var(--mono); font-size: 0.82rem; }
  .x { all: unset; cursor: pointer; margin-left: auto; color: var(--c-muted); padding: 0 0.35rem; border-radius: 4px; }
  .x:hover { color: var(--c-error); }
  .x:focus-visible, .link:focus-visible { outline: 2px solid var(--c-accent); }
  .link { all: unset; cursor: pointer; color: var(--c-accent); margin-left: 0.4rem; }
  .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.5rem; margin-top: 0.6rem; align-items: end; }
  .stack { display: grid; gap: 0.5rem; margin-top: 0.5rem; }
  .row { display: flex; gap: 0.5rem; align-items: end; flex-wrap: wrap; margin-top: 0.5rem; }
  .row.end { justify-content: flex-end; }
  .grow { flex: 1 1 12rem; }
  .chips { display: flex; flex-wrap: wrap; gap: 0.35rem; margin: 0.35rem 0; }
  .chips li { display: inline-flex; align-items: center; gap: 0.2rem; font-size: 0.82rem; padding: 0.1rem 0.6rem; border-radius: 999px; background: var(--c-hover); }
  .chips li.new { background: var(--c-accent-soft); color: var(--c-accent); }
  .chips .x { margin-left: 0.2rem; }
  .actions { display: flex; gap: 0.5rem; align-items: end; flex-wrap: wrap; margin-top: 0.75rem; }
  .issues { margin-top: 0.5rem; color: var(--c-error); font-size: 0.88rem; display: grid; gap: 0.2rem; }
  .preview { margin-top: 0.5rem; }
  .fixtures li { padding: 0.4rem 0; border-bottom: 1px solid var(--c-line); }
  .out { margin: 0.5rem 0; padding: 0.6rem 0.75rem; background: var(--c-sidebar); border-radius: var(--radius); font: 0.82rem/1.45 var(--mono); white-space: pre-wrap; overflow-wrap: anywhere; }
  .versions li { border-bottom: 1px solid var(--c-line); }
  .versions .row-btn { all: unset; box-sizing: border-box; width: 100%; display: grid; grid-template-columns: 6rem 10rem 1fr 3rem; gap: 0.75rem; padding: 0.6rem 0.4rem; cursor: pointer; border-radius: var(--radius); font-size: 0.9rem; }
  .versions .row-btn:hover { background: var(--c-hover); }
  .versions .row-btn > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .vid { font-weight: 500; }
  .vbody { padding: 0 0.4rem 0.9rem; animation: fade var(--t) ease-out; }
  .file { position: relative; display: inline-flex; flex-direction: row; color: var(--c-text); font-size: 0.9rem; }
  .file input { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
  .file:focus-within { outline: 2px solid var(--c-accent); outline-offset: 2px; }
  .backdrop { position: fixed; inset: 0; background: rgb(0 0 0 / 0.3); display: grid; place-items: center; padding: 1rem; z-index: 10; }
  .dialog { background: var(--c-surface); color: var(--c-text); border-radius: calc(var(--radius) * 1.5); padding: 1.25rem 1.5rem; max-width: 26rem; box-shadow: 0 10px 40px rgb(0 0 0 / 0.25); }
  .dialog h3 { margin-top: 0; font-size: 1.05rem; }
  @keyframes fade { from { opacity: 0; transform: translateY(-2px); } }
  @media (max-width: 900px) { .cols { grid-template-columns: 1fr; } .cols > section + section { margin-top: 1.5rem; } }
  @media (max-width: 720px) {
    .grid { grid-template-columns: 1fr; }
    .versions .row-btn { grid-template-columns: 5.5rem 1fr; }
    .versions .row-btn > span:nth-child(2), .versions .row-btn > span:nth-child(4) { display: none; }
  }
</style>
