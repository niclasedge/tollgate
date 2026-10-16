// Display helpers for the performance values of #87 and #116. The API sends
// numbers only; all sentences are formed here.
import type { PerfMetric, PerfReport, TPSJudgement, TraceSummary, Insight } from './types/dashboard';
import { hourAxis } from './hours';

export type DotState = 'ok' | 'error' | 'busy' | 'slow';

const num1 = new Intl.NumberFormat('de-DE', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const num0 = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 0 });

/** Milliseconds as "850 ms" or "4,2 s". */
export function formatMs(ms: number | null | undefined): string {
  if (ms === null || ms === undefined) return '–';
  return ms < 1000 ? `${num0.format(Math.round(ms))} ms` : `${num1.format(ms / 1000)} s`;
}

/** Microseconds as formatMs. */
export const formatUs = (us: number | null | undefined): string => (us === null || us === undefined ? '–' : formatMs(us / 1000));

export const formatTps = (v: number | null | undefined): string => (v === null || v === undefined ? '–' : num1.format(v));

const metricName = { ttft: 'TTFT', duration: 'Gesamtdauer' } as const;

/** Explanation of one metric verdict. */
export function explainMetric(name: 'ttft' | 'duration', m: PerfMetric): string {
  const label = metricName[name];
  if (m.verdict === 'unbekannt') return `${label}: noch zu wenig Vergleichswerte (${m.samples} von 20)`;
  if (m.verdict === 'langsam')
    return `${label}: erwartet ~${formatMs(m.median_ms)}, gebraucht ${formatMs(m.value_ms)} (p90 ${formatMs(m.p90_ms)}, ${m.samples} Vergleichswerte)`;
  return `${label}: im üblichen Rahmen (Median ${formatMs(m.median_ms)})`;
}

/** All verdict sentences of a trace; empty without a verdict. */
export function explainPerf(t: TraceSummary): string[] {
  if (!t.perf) return [];
  const out: string[] = [];
  if (t.perf.ttft) out.push(explainMetric('ttft', t.perf.ttft));
  if (t.perf.duration) out.push(explainMetric('duration', t.perf.duration));
  return out;
}

/** Status dot with precedence running > error > slow > ok. */
export function traceDot(t: TraceSummary): { state: DotState; label: string } {
  if (t.status === 0) return { state: 'busy', label: 'läuft' };
  if (t.status >= 400) return { state: 'error', label: `Fehler (${t.status})` };
  if (t.perf?.slow) return { state: 'slow', label: `langsam: ${explainPerf(t).join('; ')}` };
  return { state: 'ok', label: `OK (${t.status})` };
}

const inputLabel: Record<string, string> = { i0: '< 8k', i1: '8k–32k', i2: '32k–128k', i3: '≥ 128k', '*': 'alle Größen' };
const cacheLabel: Record<string, string> = { c0: '< 50 %', c1: '50–89 %', c2: '≥ 90 %' };
export const inputClassLabel = (c: string): string => inputLabel[c] ?? c;
export const cacheClassLabel = (c: string): string => cacheLabel[c] ?? c;

/** Deviation in permille as "−23 %" / "+5 %". */
export function formatDeviation(permille: number | null | undefined): string {
  if (permille === null || permille === undefined) return '';
  const p = Math.round(permille / 10);
  return `${p < 0 ? '−' : p > 0 ? '+' : '±'}${Math.abs(p)} %`;
}

/** The tokens/s cell of the live list (#116). */
export function tpsCell(t: TraceSummary): { text: string; sub: string; tone: 'slower' | 'faster' | 'usual' | 'plain'; title: string } {
  if (t.status === 0 || !t.stream || t.output_tps === null || t.output_tps === undefined)
    return { text: '–', sub: '', tone: 'plain', title: '' };
  const text = num0.format(Math.round(t.output_tps));
  const j = t.tps;
  if (!j || j.verdict === 'unbekannt') return { text, sub: '', tone: 'plain', title: tpsSentence(t.output_tps, j) };
  const tone = j.verdict === 'langsamer' ? 'slower' : j.verdict === 'schneller' ? 'faster' : 'usual';
  return { text, sub: formatDeviation(j.deviation_permille), tone, title: tpsSentence(t.output_tps, j) };
}

/** "48,2 t/s · üblich 62,0 (p10 51,3 – p90 70,1), 87 Vergleichswerte, Klasse 32k–128k". */
export function tpsSentence(tps: number, j: TPSJudgement | null | undefined): string {
  const head = `${num1.format(tps)} t/s`;
  if (!j) return head;
  if (j.verdict === 'unbekannt') return `${head} · noch zu wenig Vergleichswerte (${j.samples} von 20)`;
  return `${head} · üblich ${formatTps(j.median)} (p10 ${formatTps(j.p10)} – p90 ${formatTps(j.p90)}), ${j.samples} Vergleichswerte, Klasse ${inputClassLabel(j.class)}`;
}

/** Sentence of one insight of the influence matrix. */
export function insightSentence(i: Insight): string {
  const pct = Math.round(Math.abs(i.delta_permille) / 10);
  if (i.kind === 'cache_ttft') {
    const dir = i.delta_permille <= 0 ? 'senkt' : 'erhöht';
    return `Warmer Cache (≥ 90 %) ${dir} TTFT bei ${inputClassLabel(i.input)} Eingabe im Median um ${pct} %.`;
  }
  const dir = i.delta_permille <= 0 ? 'langsamer' : 'schneller';
  return `Bei ${inputClassLabel(i.to)} Eingabe generiert das Modell ${pct} % ${dir} als bei ${inputClassLabel(i.from)}.`;
}

export type TpsLine = {
  model: string;
  values: (number | null)[];
  last: number | null;
  /** Median tokens/s per UTC day YYYY-MM-DD; days without a value are absent (#123). */
  days: ReadonlyMap<string, number>;
};

/**
 * Median tokens/s per model and UTC hour on the axis of hourAxis (at most
 * the last 7 days); hours without values are null.
 */
export function tpsByHour(r: PerfReport, now: number = Date.now()): { hours: string[]; lines: TpsLine[] } {
  const hours = hourAxis(
    r.models.flatMap((m) => (m.hours ?? []).map((h) => h.hour)),
    now,
  );
  const lines = r.models
    .map((m) => {
      const byHour = new Map((m.hours ?? []).map((h) => [h.hour, h.tps_median_milli / 1000]));
      const values = hours.map((h) => byHour.get(h) ?? null);
      const known = values.filter((v): v is number => v !== null);
      const days = new Map(m.days.flatMap((d) => (d.tps_median_milli === null ? [] : [[d.day, d.tps_median_milli / 1000] as const])));
      return { model: m.model, values, last: known.length ? known[known.length - 1] : null, days };
    })
    .filter((l) => l.last !== null);
  return { hours, lines };
}
