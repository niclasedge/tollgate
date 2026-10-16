<script lang="ts">
  // CodeMirror YAML editor, linted live against the JSON schema the server
  // embeds (GET /api/config/schema). The parent owns the text.
  import { onMount } from 'svelte';
  import { basicSetup } from 'codemirror';
  import { EditorView, keymap } from '@codemirror/view';
  import { Prec } from '@codemirror/state';
  import { yaml } from '@codemirror/lang-yaml';
  import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
  import { linter, lintGutter, type Diagnostic } from '@codemirror/lint';
  import { tags as t } from '@lezer/highlight';
  import { handleRefresh, stateExtensions, updateSchema } from 'codemirror-json-schema';
  import { yamlSchemaLinter } from 'codemirror-json-schema/yaml';
  type JSONSchema7 = Parameters<typeof updateSchema>[1];

  let {
    value,
    schema,
    label,
    onchange,
    oncheck,
  }: { value: string; schema?: unknown; label: string; onchange: (v: string) => void; oncheck?: () => void } = $props();

  let host: HTMLDivElement;
  let view: EditorView | undefined;
  // -1 = not linted yet (no schema or first run pending).
  let issues = $state(-1);

  const highlight = HighlightStyle.define([
    { tag: [t.propertyName, t.definition(t.propertyName)], color: 'var(--c-accent)' },
    { tag: t.comment, color: 'var(--c-muted)', fontStyle: 'italic' },
    { tag: [t.number, t.bool, t.null, t.atom], color: 'var(--c-ok)' },
    { tag: [t.string, t.content], color: 'var(--c-text)' },
    { tag: [t.punctuation, t.separator], color: 'var(--c-muted)' },
  ]);

  const theme = EditorView.theme({
    '&': { color: 'var(--c-text)', backgroundColor: 'var(--c-surface)', fontSize: '0.85rem' },
    '.cm-content': { fontFamily: 'var(--mono)', caretColor: 'var(--c-text)' },
    '.cm-gutters': { backgroundColor: 'var(--c-sidebar)', color: 'var(--c-muted)', border: 'none' },
    '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'var(--c-hover)' },
    '&.cm-focused': { outline: 'none' },
    '&.cm-focused .cm-cursor': { borderLeftColor: 'var(--c-text)' },
    '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection': { backgroundColor: 'var(--c-accent-soft) !important' },
    '.cm-tooltip': { backgroundColor: 'var(--c-surface)', color: 'var(--c-text)', border: '1px solid var(--c-line)' },
  });

  const schemaLint = yamlSchemaLinter();
  const counted = (v: EditorView): Diagnostic[] => {
    const d = schemaLint(v) as Diagnostic[];
    issues = d.length;
    return d;
  };

  onMount(() => {
    view = new EditorView({
      parent: host,
      doc: value,
      extensions: [
        Prec.highest(keymap.of([{ key: 'Mod-Enter', run: () => (oncheck?.(), true) }])),
        basicSetup,
        yaml(),
        syntaxHighlighting(highlight),
        theme,
        lintGutter(),
        linter(counted, { delay: 300, needsRefresh: handleRefresh }),
        stateExtensions(schema as JSONSchema7 | undefined),
        EditorView.contentAttributes.of({ 'aria-label': label }),
        EditorView.updateListener.of((u) => {
          if (u.docChanged) onchange(u.state.doc.toString());
        }),
      ],
    });
    return () => view?.destroy();
  });

  // External changes (form, file import, version load) replace the text.
  $effect(() => {
    const v = value;
    if (view && view.state.doc.toString() !== v) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: v } });
  });

  $effect(() => {
    if (view && schema) updateSchema(view, schema as JSONSchema7);
  });
</script>

<div class="ed" bind:this={host}></div>
<p class="status" aria-live="polite">
  {#if !schema}Schema wird geladen …{:else if issues < 0}Schema: prüfe …{:else if issues === 0}Schema: gültig{:else}Schema: {issues} Fehler{/if}
  <span class="muted">· Mod+Enter prüft den Entwurf</span>
</p>

<style>
  .ed { border: 1px solid var(--c-line); border-radius: var(--radius); overflow: hidden; }
  .ed :global(.cm-editor) { max-height: 32rem; }
  .ed :global(.cm-scroller) { overflow: auto; }
  .status { margin: 0.35rem 0 0; font-size: 0.8rem; }
</style>
