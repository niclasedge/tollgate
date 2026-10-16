// Fastest-filling 5-hour window (#134): text for the tile under
// Kosten & Performance. The numbers come from GET /api/performance.
import type { WindowFill } from './types/dashboard';

/** A span in µs as "1 h 40 min" or "45 min", rounded to whole minutes (at least 1). */
export function formatSpan(us: number): string {
  const min = Math.max(1, Math.round(us / 60e6));
  const h = Math.floor(min / 60);
  const m = min % 60;
  if (h === 0) return `${m} min`;
  return m > 0 ? `${h} h ${m} min` : `${h} h`;
}

const pct = (permille: number) => Math.round(permille / 10);

/** The small line under the tile: rise, observed span and requests per minute. */
export function fillLine(f: WindowFill): string {
  const rate = f.per_minute.toFixed(1).replace('.', ',');
  return `${pct(f.from_permille)} → ${pct(f.to_permille)} % in ${formatSpan(f.observed_us)} · Ø ${rate} Anfragen/min`;
}
