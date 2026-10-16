// Einschätzung (#136): weekly need of full 5-hour windows against what the
// accounts' weekly limits hold.
import type { Capacity } from './types/dashboard';

export type Verdict = 'reicht' | 'knapp' | 'reicht nicht';

export interface Estimate {
  needPerWeek: number;
  capacityPerWeek: number;
  // Need as a share of capacity; above 1 when the accounts don't suffice.
  share: number;
  verdict: Verdict;
}

// From this share on the accounts count as tight.
const TIGHT = 0.8;

// estimate returns null while a measurement is missing.
export function estimate(cap: Capacity): Estimate | null {
  const wpw = cap.windows_per_week;
  const fills = cap.fills_per_day;
  if (wpw === null || fills === null || wpw <= 0 || cap.accounts === 0) return null;
  const needPerWeek = fills * 7;
  const capacityPerWeek = cap.accounts * wpw;
  const share = needPerWeek / capacityPerWeek;
  const verdict: Verdict = share > 1 ? 'reicht nicht' : share >= TIGHT ? 'knapp' : 'reicht';
  return { needPerWeek, capacityPerWeek, share, verdict };
}

// missing names the measurement the estimate still waits for.
export function missing(cap: Capacity): string {
  if (cap.window_cost_nano_usd === null) return 'Noch kein 5-h-Fenster mit 30 Punkten Anstieg gemessen';
  if (cap.windows_per_week === null) return 'Noch kein Account mit mindestens 25 % Wochenverbrauch und vollständig gemessener Woche';
  if (cap.fills_per_day === null) return 'Noch unter einer Stunde 5-h-Werte gemessen';
  return 'Noch keine Accounts angelegt';
}

// formatCount shows a window count with at most one decimal, German style.
export function formatCount(n: number): string {
  return n.toLocaleString('de-DE', { maximumFractionDigits: 1 });
}
