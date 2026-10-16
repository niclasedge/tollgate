import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { EditorView } from '@codemirror/view';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import { json, mockFetch, type Handler } from './test-helpers';
import schemaText from '../../config/schema/tollgate-config.v1.schema.json?raw';
// Golden file shared with internal/redactcfg/equivalence_test.go: the YAML the
// form sends must import to the same configuration as the hand-written YAML.
import formConfig from './testdata/form-config.yaml?raw';
import type { DryRunResult, NameLists, VersionDetail, VersionInfo } from './lib/types/redactcfg';

const defaultYaml = `# Wortliste nicht enthalten (words.list: []); Export mit words=plain enthält sie.
version: 1
builtin:
  email: true
  api_keys: true
  jwt: true
  connection_urls: true
  ip_addresses: false
  private_keys: true
custom: []
words:
  case_sensitive: false
  whole_word: true
  list: []
allowlist: []
`;

const maskedYaml = `# Wortliste maskiert – nicht importierbar.
version: 1
builtin:
  email: true
  api_keys: true
  jwt: true
  connection_urls: true
  ip_addresses: false
  private_keys: true
custom:
  - name: kundennummer
    pattern: KD-\\d{6}
    placeholder: customer_id
    examples:
      - KD-123456
words:
  case_sensitive: false
  whole_word: true
  list:
    - P••••••••e
    - M••••••••H
allowlist:
  - noreply@github.com
`;

const info = (o: Partial<VersionInfo> = {}): VersionInfo => ({
  id: 1,
  created_at: '2026-10-04T10:00:00Z',
  author: 'system',
  comment: 'Standardkonfiguration',
  diff: '',
  active: true,
  has_words: false,
  ...o,
});

const dryRunOK: DryRunResult = {
  active_version: 1,
  needs_sealer: false,
  fixtures: [
    {
      name: 'redact-dry-run-messages.json',
      targets: 3,
      active: [{ path: 'messages[0].content[0].text', start: 0, end: 5, type: 'email', rule: 'email' }],
      candidate: [
        { path: 'messages[0].content[0].text', start: 0, end: 5, type: 'email', rule: 'email' },
        { path: 'messages[0].content[0].text', start: 9, end: 18, type: 'customer_id', rule: 'kundennummer' },
      ],
    },
  ],
};

type Body = Record<string, unknown>;
const bodyOf = (init: RequestInit | undefined): Body => JSON.parse(String(init?.body ?? '{}')) as Body;

/** Mocks the config API with one version (default) or two (v2 with words). */
function filters(withWords: boolean, over: Record<string, Handler> = {}) {
  auth.token = 'tok';
  location.hash = '#/filters';
  const v1 = info({ active: !withWords });
  const v2 = info({ id: 2, author: 'import', comment: 'Kunden', active: true, has_words: true, diff: '- custom: []\n+ custom:\n+   - name: kundennummer\n' });
  const versions = withWords ? [v2, v1] : [v1];
  const detail = (v: VersionInfo, yaml: string): VersionDetail => ({ ...v, yaml });
  const calls: Array<{ key: string; body: Body }> = [];
  const record = (key: string, h: Handler): Handler => (init, url) => {
    calls.push({ key, body: bodyOf(init) });
    return h(init, url);
  };
  const f = mockFetch({
    'GET /api/config/schema': () => new Response(schemaText, { headers: { 'Content-Type': 'application/schema+json' } }),
    'GET /api/config/versions': () => json({ active: withWords ? 2 : 1, versions }),
    'GET /api/config/versions/1': () => json(detail(v1, defaultYaml)),
    'GET /api/config/versions/2': () => json(detail(v2, maskedYaml)),
    'POST /api/config/dry-run': record('dry-run', () => json(dryRunOK)),
    'POST /api/config/import': record('import', () => json(info({ id: 3, author: 'import', active: true }), 201)),
    'POST /api/config/test': record('test', () => json({ version: -1, hits: [{ start: 8, end: 21, type: 'email', rule: 'email' }], redacted: 'mail an <redacted:email>' })),
    'POST /api/config/versions/1/rollback': record('rollback', () => json(info({ id: 3, comment: 'Rollback auf Version 1' }), 201)),
    'GET /api/config/export': () => new Response('# WARNUNG\nversion: 1\n', { headers: { 'Content-Type': 'application/yaml' } }),
    ...over,
  });
  return { f, calls };
}

function editor(): EditorView {
  const el = document.querySelector('.cm-editor');
  if (!(el instanceof HTMLElement)) throw new Error('no editor');
  const view = EditorView.findFromDOM(el);
  if (!view) throw new Error('no editor view');
  return view;
}

function setEditor(text: string) {
  const v = editor();
  v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: text } });
}

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('Filter', () => {
  it('lists the active filters with switches, custom rules, the masked word list and the allowlist', async () => {
    filters(true);
    render(App);
    expect(await screen.findByRole('heading', { level: 1, name: 'Filter' })).toBeInTheDocument();
    const list = await screen.findByRole('region', { name: 'Aktive Filter' });
    expect(await within(list).findByRole('switch', { name: 'E-Mail' })).toBeChecked();
    expect(within(list).getByRole('switch', { name: 'IP-Adressen' })).not.toBeChecked();
    expect(within(list).getByText('kundennummer')).toBeInTheDocument();
    expect(within(list).getByText('KD-\\d{6}')).toBeInTheDocument();
    expect(within(list).getByText('P••••••••e')).toBeInTheDocument();
    expect(within(list).getByText('M••••••••H')).toBeInTheDocument();
    expect(within(list).getByText('noreply@github.com')).toBeInTheDocument();
    expect(screen.getByText(/Version 2 aktiv/)).toBeInTheDocument();
  });

  it('builds the configuration in the form and saves exactly the YAML of the golden file', async () => {
    const { calls } = filters(false);
    render(App);
    const list = await screen.findByRole('region', { name: 'Aktive Filter' });
    await fireEvent.click(await within(list).findByRole('switch', { name: 'IP-Adressen' }));
    await fireEvent.input(within(list).getByLabelText('Name'), { target: { value: 'kundennummer' } });
    await fireEvent.input(within(list).getByLabelText('Muster (RE2)'), { target: { value: 'KD-\\d{6}' } });
    await fireEvent.input(within(list).getByLabelText('Platzhalter-Typ'), { target: { value: 'customer_id' } });
    await fireEvent.input(within(list).getByLabelText('Beispiel'), { target: { value: 'KD-123456' } });
    await fireEvent.click(within(list).getByRole('button', { name: 'Regel hinzufügen' }));
    await fireEvent.input(within(list).getByLabelText('Neuer Allowlist-Eintrag'), { target: { value: 'noreply@github.com' } });
    await fireEvent.click(within(list).getByRole('button', { name: 'Zur Allowlist' }));

    // The editor shows the same draft the form produced.
    await waitFor(() => expect(editor().state.doc.toString()).toBe(formConfig));

    const save = screen.getByRole('button', { name: 'Speichern und aktivieren' });
    expect(save).toBeDisabled(); // no dry run of this text yet
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    const preview = await screen.findByRole('region', { name: 'Vorschau' });
    expect(await within(preview).findByText('redact-dry-run-messages.json')).toBeInTheDocument();
    expect(within(preview).getByText('customer_id')).toBeInTheDocument();
    await waitFor(() => expect(save).toBeEnabled());
    await fireEvent.click(save);
    await waitFor(() => expect(calls.some((c) => c.key === 'import')).toBe(true));
    const imp = calls.find((c) => c.key === 'import')!.body;
    expect(imp.yaml).toBe(formConfig);
    expect(imp.activate).toBe(true);
    expect(imp.words_from).toBe(0);
    expect(calls.find((c) => c.key === 'dry-run')!.body.yaml).toBe(formConfig);
  });

  it('blocks saving and shows issues with line and column when the dry run rejects the draft', async () => {
    filters(false, {
      'POST /api/config/dry-run': () =>
        json({ error: 'Konfiguration ungültig', issues: [{ path: 'custom[0].pattern', line: 12, column: 14, message: 'Regex kompiliert nicht' }] }, 422),
    });
    render(App);
    await screen.findByRole('region', { name: 'Aktive Filter' });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    setEditor(defaultYaml.replace('custom: []', "custom:\n  - name: x\n    pattern: '('\n    placeholder: y\n    examples: [a]"));
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    expect(await screen.findByText('custom[0].pattern (Zeile 12, Spalte 14): Regex kompiliert nicht')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Speichern und aktivieren' })).toBeDisabled();
  });

  it('validates the editor live against the served schema', async () => {
    const { f } = filters(false);
    render(App);
    await screen.findByRole('region', { name: 'Aktive Filter' });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    expect(f.mock.calls.map((c) => String(c[0]))).toContain('/api/config/schema');
    setEditor('version: 1\nunbekannt: true\n');
    expect(await screen.findByText(/Schema: 1 Fehler/, {}, { timeout: 3000 })).toBeInTheDocument();
    setEditor('version: 1\n');
    expect(await screen.findByText('Schema: gültig', {}, { timeout: 3000 })).toBeInTheDocument();
  });

  it('keeps the stored word list via words_from and never sends masked entries', async () => {
    const { calls } = filters(true);
    render(App);
    await screen.findByText('P••••••••e');
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    const draft = editor().state.doc.toString();
    expect(draft).not.toContain('••••');
    expect(draft).toContain('list: []');
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    const dr = calls.find((c) => c.key === 'dry-run')!.body;
    expect(dr.words_from).toBe(2);
    expect(String(dr.yaml)).not.toContain('••••');
  });

  it('runs the live test on the draft and shows hits and the redacted text', async () => {
    const { calls } = filters(true);
    render(App);
    const test = await screen.findByRole('region', { name: 'Live-Test' });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    await fireEvent.input(within(test).getByLabelText('Testtext'), { target: { value: 'mail an jane@firma.de' } });
    await fireEvent.click(within(test).getByRole('button', { name: 'Testen' }));
    expect(await within(test).findByText('mail an <redacted:email>')).toBeInTheDocument();
    expect(within(test).getByText('email')).toBeInTheDocument();
    const body = calls.find((c) => c.key === 'test')!.body;
    expect(body.text).toBe('mail an jane@firma.de');
    expect(body.words_from).toBe(2);
    expect(String(body.yaml)).toContain('version: 1');
  });

  it('shows the version history with diff and rolls back as a new version', async () => {
    const { calls, f } = filters(true);
    render(App);
    const hist = await screen.findByRole('region', { name: 'Versionen' });
    const row = await within(hist).findByRole('button', { name: /Version 2/ });
    await fireEvent.click(row);
    expect(within(hist).getByText(/\+ custom:/)).toBeInTheDocument();
    await fireEvent.click(within(hist).getByRole('button', { name: /Version 1/ }));
    await fireEvent.click(within(hist).getByRole('button', { name: 'Zurückrollen auf Version 1' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'rollback')).toBe(true));
    await waitFor(() => expect(f.mock.calls.filter((c) => String(c[0]) === '/api/config/versions').length).toBeGreaterThan(1));
  });

  it('warns before exporting the word list and only then requests words=plain', async () => {
    const created = vi.fn(() => 'blob:x');
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: created, revokeObjectURL: vi.fn() }));
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    const { f } = filters(true);
    render(App);
    const io = await screen.findByRole('region', { name: 'Import und Export' });
    await fireEvent.click(within(io).getByRole('button', { name: 'Mit Wortliste exportieren …' }));
    const dlg = await screen.findByRole('alertdialog');
    expect(dlg).toHaveTextContent('enthält die vertrauliche Wortliste – nicht committen');
    const urls = () => f.mock.calls.map((c) => String(c[0]));
    expect(urls().some((u) => u.startsWith('/api/config/export'))).toBe(false);
    await fireEvent.click(within(dlg).getByRole('button', { name: 'Trotzdem exportieren' }));
    await waitFor(() => expect(urls()).toContain('/api/config/export?words=plain'));
    await waitFor(() => expect(created).toHaveBeenCalled());
    // The token travels as header, never in the URL.
    const init = f.mock.calls.find((c) => String(c[0]).startsWith('/api/config/export'))![1] as RequestInit;
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok');
  });

  it('adds words to the stored list as words_add and shows them masked only', async () => {
    const { calls } = filters(true);
    render(App);
    const list = await screen.findByRole('region', { name: 'Aktive Filter' });
    await within(list).findByText('P••••••••e');
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    const save = screen.getByRole('button', { name: 'Speichern und aktivieren' });
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(save).toBeEnabled());

    await fireEvent.input(within(list).getByLabelText(/Wörter hinzufügen/), { target: { value: 'Neu AG, Zweite KG\n\n' } });
    await fireEvent.click(within(list).getByRole('button', { name: 'Hinzufügen' }));
    // The operation invalidates the dry run.
    expect(save).toBeDisabled();
    expect(within(list).getByText('N••••••••G')).toBeInTheDocument();
    expect(within(list).getByText('Z••••••••G')).toBeInTheDocument();
    expect(list).not.toHaveTextContent('Neu AG');
    expect(editor().state.doc.toString()).not.toContain('Neu AG');
    expect(editor().state.doc.toString()).toContain('list: []');

    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(save).toBeEnabled());
    await fireEvent.click(save);
    await waitFor(() => expect(calls.some((c) => c.key === 'import')).toBe(true));
    for (const key of ['dry-run', 'import']) {
      const b = calls.filter((c) => c.key === key).at(-1)!.body;
      expect(b.words_from).toBe(2);
      expect(b.words_add).toEqual(['Neu AG', 'Zweite KG']);
      expect(b.words_remove).toBeUndefined();
      expect(String(b.yaml)).not.toContain('••••');
    }
  });

  it('removes a single stored entry by index as words_remove', async () => {
    const { calls } = filters(true);
    render(App);
    const list = await screen.findByRole('region', { name: 'Aktive Filter' });
    await within(list).findByText('M••••••••H');
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    await fireEvent.click(within(list).getByRole('button', { name: 'Eintrag 2 entfernen' }));
    expect(within(list).queryByText('M••••••••H')).not.toBeInTheDocument();
    expect(within(list).getByText('P••••••••e')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    const dr = calls.find((c) => c.key === 'dry-run')!.body;
    expect(dr.words_from).toBe(2);
    expect(dr.words_remove).toEqual([1]);
    expect(dr.words_add).toBeUndefined();
    const test = screen.getByRole('region', { name: 'Live-Test' });
    await fireEvent.input(within(test).getByLabelText('Testtext'), { target: { value: 'x' } });
    await fireEvent.click(within(test).getByRole('button', { name: 'Testen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'test')).toBe(true));
    expect(calls.find((c) => c.key === 'test')!.body.words_remove).toEqual([1]);
  });

  it('removes an entry of a new list directly in the draft and drops pending additions', async () => {
    const { calls } = filters(false);
    render(App);
    const list = await screen.findByRole('region', { name: 'Aktive Filter' });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    await fireEvent.input(await within(list).findByLabelText(/Neue Wortliste/), { target: { value: 'Alpha Beta\nGamma Delta' } });
    await fireEvent.click(within(list).getByRole('button', { name: 'Wortliste ersetzen' }));
    await fireEvent.input(within(list).getByLabelText(/Wörter hinzufügen/), { target: { value: 'Neu AG' } });
    await fireEvent.click(within(list).getByRole('button', { name: 'Hinzufügen' }));
    await fireEvent.click(within(list).getByRole('button', { name: 'Neuen Eintrag 1 entfernen' }));
    await fireEvent.click(within(list).getByRole('button', { name: 'Eintrag 1 entfernen' }));
    await waitFor(() => expect(editor().state.doc.toString()).not.toContain('Alpha Beta'));
    expect(editor().state.doc.toString()).toContain('Gamma Delta');
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    const dr = calls.find((c) => c.key === 'dry-run')!.body;
    expect(dr.words_from).toBe(0);
    expect(dr.words_remove).toBeUndefined();
    expect(dr.words_add).toBeUndefined();
  });

  it('loads an imported YAML file into the draft', async () => {
    filters(false);
    render(App);
    const io = await screen.findByRole('region', { name: 'Import und Export' });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    const file = new File(['version: 1\nallowlist: ["a@b.de"]\n'], 'c.yaml', { type: 'application/yaml' });
    await fireEvent.change(within(io).getByLabelText('YAML-Datei laden'), { target: { files: [file] } });
    await waitFor(() => expect(editor().state.doc.toString()).toBe('version: 1\nallowlist: ["a@b.de"]\n'));
  });
});

const maskedNamesYaml = maskedYaml.replace(
  '    - M••••••••H\nallowlist:',
  `    - M••••••••H
  customers:
    - M••••••••H
    - Z••••••••G
  first_names:
    - ••••••••
    - ••••••••
  last_names:
    - M••••••••r
    - S••••••••t
allowlist:`,
);

const plainNames: NameLists = { customers: ['Muster GmbH', 'Zander AG'], first_names: ['Anna', 'Bernd'], last_names: ['Müller', 'Schmidt'] };

/** Version 2 with the three name lists; GET /api/config/lists is counted. */
function withNames(over: Record<string, Handler> = {}) {
  const listCalls: string[] = [];
  const v2 = info({ id: 2, author: 'import', comment: 'Kunden', active: true, has_words: true });
  const r = filters(true, {
    'GET /api/config/versions/2': () => json({ ...v2, yaml: maskedNamesYaml }),
    'GET /api/config/lists': (_init, url) => {
      listCalls.push(url);
      return json(plainNames);
    },
    ...over,
  });
  return { ...r, listCalls };
}

const group = (name: string | RegExp) => screen.getByRole('group', { name });

describe('Filter: Kunden, Vornamen und Nachnamen', () => {
  it('shows the three lists masked with their count and never loads the plain text on its own', async () => {
    const { listCalls } = withNames();
    render(App);
    const customers = await screen.findByRole('group', { name: /^Kunden/ });
    expect(within(customers).getByText('M••••••••H')).toBeInTheDocument();
    expect(within(customers).getByText('Z••••••••G')).toBeInTheDocument();
    expect(customers).toHaveTextContent('2 Einträge');
    expect(group(/^Vornamen/)).toHaveTextContent('2 Einträge');
    expect(within(group(/^Nachnamen/)).getByText('S••••••••t')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Aktive Filter' })).not.toHaveTextContent('Muster GmbH');
    // The draft holds neither masked nor plain entries of the lists.
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    const draft = editor().state.doc.toString();
    expect(draft).not.toContain('••••');
    expect(draft).toContain('customers: []');
    expect(listCalls).toEqual([]);
  });

  it('shows empty lists without an entry', async () => {
    filters(false);
    render(App);
    expect(await screen.findByRole('group', { name: /^Kunden/ })).toHaveTextContent('Keine Einträge');
  });

  it('edits one list in a textarea, loads the plain text once and sends lists_set with words_from', async () => {
    const { calls, listCalls } = withNames();
    render(App);
    const customers = await screen.findByRole('group', { name: /^Kunden/ });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    const save = screen.getByRole('button', { name: 'Speichern und aktivieren' });
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(save).toBeEnabled());

    await fireEvent.click(within(customers).getByRole('button', { name: 'Kunden bearbeiten' }));
    const box = await within(customers).findByLabelText('Kunden (eine Zeile je Eintrag)');
    expect((box as HTMLTextAreaElement).value).toBe('Muster GmbH\nZander AG');
    expect(listCalls).toHaveLength(1);
    await fireEvent.input(box, { target: { value: 'Muster GmbH\n\n  Neu KG \nmuster gmbh\n' } });
    await fireEvent.click(within(customers).getByRole('button', { name: 'Übernehmen' }));

    // Masked again, the dry run is invalid, the draft stays free of entries.
    expect(within(customers).queryByLabelText('Kunden (eine Zeile je Eintrag)')).not.toBeInTheDocument();
    expect(within(customers).getByText('N••••••••G')).toBeInTheDocument();
    expect(within(customers).queryByText('Z••••••••G')).not.toBeInTheDocument();
    expect(customers).toHaveTextContent('2 Einträge');
    expect(screen.getByRole('region', { name: 'Aktive Filter' })).not.toHaveTextContent('Neu KG');
    expect(save).toBeDisabled();
    expect(editor().state.doc.toString()).not.toContain('Neu KG');

    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(save).toBeEnabled());
    await fireEvent.click(save);
    await waitFor(() => expect(calls.some((c) => c.key === 'import')).toBe(true));
    for (const key of ['dry-run', 'import']) {
      const b = calls.filter((c) => c.key === key).at(-1)!.body;
      expect(b.words_from).toBe(2);
      // Blank lines and the case-insensitive duplicate are gone; other lists are not sent.
      expect(b.lists_set).toEqual({ customers: ['Muster GmbH', 'Neu KG'] });
      expect(String(b.yaml)).not.toContain('Neu KG');
    }
    expect(listCalls).toHaveLength(1);
  });

  it('keeps case variants when case_sensitive is on', async () => {
    const { calls } = withNames({
      'GET /api/config/versions/2': () => json({ ...info({ id: 2, has_words: true }), yaml: maskedNamesYaml.replace('case_sensitive: false', 'case_sensitive: true') }),
    });
    render(App);
    const last = await screen.findByRole('group', { name: /^Nachnamen/ });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('case_sensitive: true'));
    await fireEvent.click(within(last).getByRole('button', { name: 'Nachnamen bearbeiten' }));
    await fireEvent.input(await within(last).findByLabelText('Nachnamen (eine Zeile je Eintrag)'), { target: { value: 'Müller\nMÜLLER\nMüller' } });
    await fireEvent.click(within(last).getByRole('button', { name: 'Übernehmen' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    expect(calls.find((c) => c.key === 'dry-run')!.body.lists_set).toEqual({ last_names: ['Müller', 'MÜLLER'] });
  });

  it('can empty a list, which sends an empty array', async () => {
    const { calls } = withNames();
    render(App);
    const first = await screen.findByRole('group', { name: /^Vornamen/ });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    await fireEvent.click(within(first).getByRole('button', { name: 'Vornamen bearbeiten' }));
    await fireEvent.input(await within(first).findByLabelText('Vornamen (eine Zeile je Eintrag)'), { target: { value: '\n  \n' } });
    await fireEvent.click(within(first).getByRole('button', { name: 'Übernehmen' }));
    expect(first).toHaveTextContent('Keine Einträge');
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    expect(calls.find((c) => c.key === 'dry-run')!.body.lists_set).toEqual({ first_names: [] });
  });

  it('discards the textarea on Abbrechen without changing the draft', async () => {
    const { calls } = withNames();
    render(App);
    const customers = await screen.findByRole('group', { name: /^Kunden/ });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    await fireEvent.click(within(customers).getByRole('button', { name: 'Kunden bearbeiten' }));
    await fireEvent.input(await within(customers).findByLabelText('Kunden (eine Zeile je Eintrag)'), { target: { value: 'Nur Test' } });
    await fireEvent.click(within(customers).getByRole('button', { name: 'Abbrechen' }));
    expect(within(customers).queryByRole('textbox')).not.toBeInTheDocument();
    expect(within(customers).getByText('M••••••••H')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    expect(calls.find((c) => c.key === 'dry-run')!.body.lists_set).toBeUndefined();
  });

  it('shows a failed plain-text request and stays masked', async () => {
    withNames({ 'GET /api/config/lists': () => json({ error: 'Wortliste braucht Verschlüsselung' }, 422) });
    render(App);
    const customers = await screen.findByRole('group', { name: /^Kunden/ });
    await fireEvent.click(within(customers).getByRole('button', { name: 'Kunden bearbeiten' }));
    expect(await within(customers).findByRole('alert')).toHaveTextContent('Wortliste braucht Verschlüsselung');
    expect(within(customers).queryByRole('textbox')).not.toBeInTheDocument();
    expect(within(customers).getByText('M••••••••H')).toBeInTheDocument();
  });

  it('takes the plain text of a pending replacement without asking the server again', async () => {
    const { listCalls } = withNames();
    render(App);
    const customers = await screen.findByRole('group', { name: /^Kunden/ });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    await fireEvent.click(within(customers).getByRole('button', { name: 'Kunden bearbeiten' }));
    await fireEvent.input(await within(customers).findByLabelText('Kunden (eine Zeile je Eintrag)'), { target: { value: 'Neu KG' } });
    await fireEvent.click(within(customers).getByRole('button', { name: 'Übernehmen' }));
    await fireEvent.click(within(customers).getByRole('button', { name: 'Kunden bearbeiten' }));
    expect(((await within(customers).findByLabelText('Kunden (eine Zeile je Eintrag)')) as HTMLTextAreaElement).value).toBe('Neu KG');
    expect(listCalls).toHaveLength(1);
  });

  it('keeps the name lists when the general word list is replaced', async () => {
    const { calls } = withNames();
    render(App);
    const list = await screen.findByRole('region', { name: 'Aktive Filter' });
    await within(list).findByText('P••••••••e');
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    await fireEvent.input(within(list).getByLabelText(/Neue Wortliste/), { target: { value: 'Alpha Beta' } });
    await fireEvent.click(within(list).getByRole('button', { name: 'Wortliste ersetzen' }));
    expect(within(group(/^Kunden/)).getByText('M••••••••H')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    const dr = calls.find((c) => c.key === 'dry-run')!.body;
    expect(dr.words_from).toBe(2);
    expect(dr.words_remove).toEqual([0, 1]);
    expect(dr.words_add).toEqual(['Alpha Beta']);
    expect(String(dr.yaml)).not.toContain('Alpha Beta');
  });

  it('keeps the name lists when the general word list is removed', async () => {
    const { calls } = withNames();
    render(App);
    const list = await screen.findByRole('region', { name: 'Aktive Filter' });
    await within(list).findByText('P••••••••e');
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    await fireEvent.click(within(list).getByRole('button', { name: 'Wortliste entfernen' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    const dr = calls.find((c) => c.key === 'dry-run')!.body;
    expect(dr.words_from).toBe(2);
    expect(dr.words_remove).toEqual([0, 1]);
  });

  it('edits the lists of a draft without words_from directly in the YAML, without asking the server', async () => {
    const { calls, listCalls } = withNames();
    render(App);
    await screen.findByRole('group', { name: /^Kunden/ });
    await waitFor(() => expect(editor().state.doc.toString()).toContain('version: 1'));
    const io = screen.getByRole('region', { name: 'Import und Export' });
    const file = new File(['version: 1\nwords:\n  customers: ["Alt AG"]\n'], 'c.yaml', { type: 'application/yaml' });
    await fireEvent.change(within(io).getByLabelText('YAML-Datei laden'), { target: { files: [file] } });
    const customers = await screen.findByRole('group', { name: /^Kunden/ });
    expect(await within(customers).findByText('A••••••••G')).toBeInTheDocument();
    await fireEvent.click(within(customers).getByRole('button', { name: 'Kunden bearbeiten' }));
    const box = (await within(customers).findByLabelText('Kunden (eine Zeile je Eintrag)')) as HTMLTextAreaElement;
    expect(box.value).toBe('Alt AG');
    await fireEvent.input(box, { target: { value: 'Alt AG\nNeu KG' } });
    await fireEvent.click(within(customers).getByRole('button', { name: 'Übernehmen' }));
    await waitFor(() => expect(editor().state.doc.toString()).toContain('Neu KG'));
    await fireEvent.click(screen.getByRole('button', { name: 'Prüfen' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'dry-run')).toBe(true));
    const dr = calls.find((c) => c.key === 'dry-run')!.body;
    expect(dr.words_from).toBe(0);
    expect(dr.lists_set).toBeUndefined();
    expect(String(dr.yaml)).toContain('Neu KG');
    expect(listCalls).toEqual([]);
  });

  it('has keyboard-reachable controls and a labelled textarea', async () => {
    withNames();
    render(App);
    const customers = await screen.findByRole('group', { name: /^Kunden/ });
    const btn = within(customers).getByRole('button', { name: 'Kunden bearbeiten' });
    btn.focus();
    expect(btn).toHaveFocus();
    await fireEvent.click(btn);
    const box = await within(customers).findByLabelText('Kunden (eine Zeile je Eintrag)');
    expect(box.tagName).toBe('TEXTAREA');
    expect(box).toHaveFocus();
  });
});
