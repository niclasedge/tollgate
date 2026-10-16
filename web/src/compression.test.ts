// Compress view: rules per command and the step-by-step playground (#21).
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import type { Explanation, RuleInfo, RuleList } from './lib/types/compress';
import { json, mockFetch } from './test-helpers';

const rule = (o: Partial<RuleInfo>): RuleInfo => ({
  name: 'go_test', match: ['go test'], strategy: 'test_log', max_errors: 0, context_lines: 0, max_files: 0, max_hunks: 0, builtin: true, ...o,
});
const rules = (o: Partial<RuleList> = {}): RuleList => ({
  version: 3,
  enabled: true,
  min_bytes: 2048,
  rules: [rule({ name: 'own_diff', match: ['git diff'], strategy: 'diff', context_lines: 1, builtin: false }), rule({})],
  ...o,
});
const step = (name: string, text: string) => ({ name, text, bytes: text.length, lines: text.split('\n').length });

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
});

function open(list = rules(), explain?: (body: unknown) => Explanation) {
  auth.token = 'tok';
  location.hash = '#/compression';
  const f = mockFetch({
    'GET /api/compress/rules': () => json(list),
    'POST /api/compress/playground': (init) => json(explain!(JSON.parse(String(init?.body)))),
  });
  render(App);
  return f;
}

describe('Kompression', () => {
  it('lists the rules in match order with their commands', async () => {
    open();
    const items = await screen.findAllByTestId('compress-rule');
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent('own_diff');
    expect(items[0]).toHaveTextContent('diff · eigene');
    expect(items[0]).toHaveTextContent('max. 1 Kontextzeilen');
    expect(within(items[1]).getByText('go test')).toBeInTheDocument();
    expect(screen.getByText(/Läuft · Version 3/)).toBeInTheDocument();
    expect(screen.getByText(/ab 2\.048 Bytes/)).toBeInTheDocument();
  });

  it('says when the stage is off', async () => {
    open(rules({ enabled: false }));
    expect(await screen.findByText(/Aus · Version 3/)).toBeInTheDocument();
    expect(screen.getByText('TOLLGATE_COMPRESS_ENABLED')).toBeInTheDocument();
  });

  it('runs the playground and shows every step', async () => {
    const f = open(rules(), () => ({
      version: 3, normalized: 'go test ./...', rule: 'go_test', strategy: 'test_log',
      steps: [step('input', 'a\nb\nc\nd'), step('dedupe', 'a\nb'), step('result', 'a\n[tollgate: go_test · 3 Zeilen ausgelassen]')],
      output: 'a\n[tollgate: go_test · 3 Zeilen ausgelassen]', bytes_before: 4000, bytes_after: 1000, omitted: 3, applied: true, reason: '',
    }));
    await screen.findAllByTestId('compress-rule');
    await fireEvent.input(screen.getByLabelText('Befehl'), { target: { value: 'cd x && go test ./...' } });
    await fireEvent.input(screen.getByLabelText('Ausgabe'), { target: { value: 'a\nb\nc\nd' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Ausführen' }));

    const summary = await screen.findByTestId('compress-summary');
    expect(summary).toHaveTextContent('Regel go_test (test_log) · 4.000 Bytes → 1.000 Bytes (−75 %) · 3 Zeilen ausgelassen');
    expect(summary).toHaveTextContent('Geprüfter Befehl: go test ./...');
    const steps = screen.getAllByTestId('compress-step');
    expect(steps.map((s) => s.querySelector('button')?.textContent)).toEqual([
      '›1. Eingabe · 4 Zeilen · 7 Bytes',
      '›2. dedupe · 2 Zeilen · 3 Bytes',
      expect.stringMatching(/^›3\. Ergebnis mit Markerzeile/),
    ]);
    // Only the result is open; the other steps unfold inline.
    expect(within(steps[2]).getByText(/\[tollgate: go_test/)).toBeInTheDocument();
    expect(within(steps[1]).queryByText('a\nb')).toBeNull();
    await fireEvent.click(within(steps[1]).getByRole('button'));
    expect(steps[1].querySelector('pre')?.textContent).toBe('a\nb');

    const post = f.mock.calls.find(([, init]) => init?.method === 'POST')!;
    expect(JSON.parse(String(post[1]?.body))).toEqual({ command: 'cd x && go test ./...', output: 'a\nb\nc\nd', version: 0 });
  });

  it('explains why an output stays unchanged', async () => {
    open(rules(), () => ({
      version: 3, normalized: '', rule: '', strategy: '', steps: [step('input', 'x')], output: 'x',
      bytes_before: 1, bytes_after: 1, omitted: 0, applied: false, reason: 'Kein einzelner Befehl: Pipes, Ketten, Heredocs und Ersetzungen bleiben unverändert.',
    }));
    await screen.findAllByTestId('compress-rule');
    await fireEvent.input(screen.getByLabelText('Befehl'), { target: { value: 'go test | tail' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Ausführen' }));
    const summary = await screen.findByTestId('compress-summary');
    expect(summary).toHaveTextContent('Kein einzelner Befehl: Pipes');
    expect(summary).toHaveTextContent('Kein einzelner Befehl erkannt.');
    expect(screen.getAllByTestId('compress-step')).toHaveLength(1);
  });
});
