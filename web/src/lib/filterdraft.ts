// The filter view keeps exactly one state: the YAML draft as text. The form
// edits it through the yaml document model, so comments and the compress
// section survive, and both ways send the same text to the same endpoints.
import { isSeq, parseDocument, type Document as YamlDocument } from 'yaml';
import type { Builtin, CustomRule, Document, NameLists } from './types/redactcfg';

export const builtinLabels: ReadonlyArray<readonly [keyof Builtin, string]> = [
  ['email', 'E-Mail'],
  ['api_keys', 'API-Keys und Tokens'],
  ['jwt', 'JWT'],
  ['connection_urls', 'Verbindungs-URLs mit Passwort'],
  ['ip_addresses', 'IP-Adressen'],
  ['private_keys', 'Private Keys'],
];

// Defaults of a missing switch, as in the schema.
const builtinDefaults: Builtin = { email: true, api_keys: true, jwt: true, connection_urls: true, ip_addresses: false, private_keys: true };

type Raw = Partial<Omit<Document, 'builtin' | 'words'>> & { builtin?: Partial<Builtin>; words?: Partial<Document['words']> };

/** Reads the draft for the form; error when the YAML does not parse. */
export function readDraft(yaml: string): { doc?: Document; error?: string } {
  const d = parseDocument(yaml);
  if (d.errors.length > 0) return { error: d.errors[0].message.split('\n')[0] };
  const raw = (d.toJS() ?? {}) as Raw;
  if (typeof raw !== 'object' || Array.isArray(raw)) return { error: 'Das Dokument ist kein YAML-Objekt.' };
  return {
    doc: {
      version: raw.version ?? 1,
      builtin: { ...builtinDefaults, ...(raw.builtin ?? {}) },
      custom: Array.isArray(raw.custom) ? raw.custom : [],
      words: { case_sensitive: false, whole_word: true, list: [], customers: [], first_names: [], last_names: [], ...(raw.words ?? {}) },
      allowlist: Array.isArray(raw.allowlist) ? raw.allowlist : [],
      compress: raw.compress,
    },
  };
}

function edit(yaml: string, fn: (d: YamlDocument) => void): string {
  const d = parseDocument(yaml);
  if (d.errors.length > 0) throw new Error('Der YAML-Entwurf ist ungültig; bitte zuerst im Editor korrigieren.');
  fn(d);
  return d.toString();
}

function append(d: YamlDocument, path: string[], value: unknown) {
  const node = d.getIn(path, true);
  if (isSeq(node)) {
    node.flow = false;
    node.add(d.createNode(value));
  } else {
    d.setIn(path, d.createNode([value]));
  }
}

export const setBuiltin = (yaml: string, key: keyof Builtin, on: boolean) => edit(yaml, (d) => d.setIn(['builtin', key], on));

export const setWordOption = (yaml: string, key: 'case_sensitive' | 'whole_word', on: boolean) =>
  edit(yaml, (d) => d.setIn(['words', key], on));

export const addCustom = (yaml: string, r: CustomRule) =>
  edit(yaml, (d) => append(d, ['custom'], { name: r.name, pattern: r.pattern, placeholder: r.placeholder, examples: r.examples }));

export const removeCustom = (yaml: string, i: number) => edit(yaml, (d) => d.deleteIn(['custom', i]));

export const addAllow = (yaml: string, v: string) => edit(yaml, (d) => append(d, ['allowlist'], v));

export const removeAllow = (yaml: string, i: number) => edit(yaml, (d) => d.deleteIn(['allowlist', i]));

/** Removes entry i of the word list in the draft (not of a words_from list). */
export const removeWord = (yaml: string, i: number) => edit(yaml, (d) => d.deleteIn(['words', 'list', i]));

/** Splits the "add words" input at line breaks and commas; trims, drops empty entries. */
export const splitWords = (text: string): string[] => text.split(/[\n,]/).map((w) => w.trim()).filter(Boolean);

/** Replaces the word list (empty = keep the stored one via words_from). */
export const setWords = (yaml: string, list: string[]) => edit(yaml, (d) => d.setIn(['words', 'list'], d.createNode(list)));

export type NameListKey = keyof NameLists;

/** The three name lists (#143) with their section title and placeholder type. */
export const nameListLabels: ReadonlyArray<{ key: NameListKey; title: string; type: string }> = [
  { key: 'customers', title: 'Kunden', type: 'kunde' },
  { key: 'first_names', title: 'Vornamen', type: 'vorname' },
  { key: 'last_names', title: 'Nachnamen', type: 'nachname' },
];

export const nameListKeys: ReadonlyArray<NameListKey> = nameListLabels.map((l) => l.key);

/**
 * Reads a textarea with one entry per line: trims, drops empty lines and
 * duplicates (case-insensitive unless case_sensitive); the server applies
 * the same rules again, so this only keeps the display tidy.
 */
export function parseLines(text: string, caseSensitive: boolean): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const line of text.split('\n')) {
    const w = line.trim();
    const k = caseSensitive ? w : w.toLowerCase();
    if (w === '' || seen.has(k)) continue;
    seen.add(k);
    out.push(w);
  }
  return out;
}

/** Replaces one name list in the draft. */
export const setNameList = (yaml: string, key: NameListKey, list: string[]) => edit(yaml, (d) => d.setIn(['words', key], d.createNode(list)));

/**
 * Turns the masked YAML of a stored version into a draft: header comments
 * go, the masked word lists are emptied and taken over via words_from.
 */
export function draftFromVersion(
  masked: string,
  id: number,
  hasWords: boolean,
): { yaml: string; wordsFrom: number; masked: string[]; names: NameLists } {
  let text = masked.replace(/^(#[^\n]*\n)+/, '');
  const words = readDraft(text).doc?.words;
  const list = words?.list ?? [];
  const names: NameLists = { customers: words?.customers ?? [], first_names: words?.first_names ?? [], last_names: words?.last_names ?? [] };
  if (list.length > 0) text = setWords(text, []);
  for (const key of nameListKeys) if (names[key].length > 0) text = setNameList(text, key, []);
  return { yaml: text, wordsFrom: hasWords ? id : 0, masked: list, names };
}

const MASK = '••••••••';

/** Masks a word-list entry like the server (redactcfg.Mask). */
export function mask(w: string): string {
  const r = [...w];
  return r.length < 6 ? MASK : r[0] + MASK + r[r.length - 1];
}
