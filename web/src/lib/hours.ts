// Hourly time axis shared by the cost and tokens/s charts (#116, #121).

export const HOUR_MS = 3_600_000;
/** The axis never spans more than 7 days. */
export const MAX_HOURS = 7 * 24;

/** Start of the UTC hour of t as YYYY-MM-DDTHH:00:00Z. */
export const hourKey = (t: number): string => `${new Date(t).toISOString().slice(0, 13)}:00:00Z`;

/**
 * Every UTC hour from the first hour with data up to the current hour, at
 * most the last MAX_HOURS. A few hours of data give a few hours of axis;
 * without data the axis is empty.
 */
export function hourAxis(keys: Iterable<string>, now: number): string[] {
  let first = Infinity;
  let last = -Infinity;
  for (const k of keys) {
    const t = Date.parse(k);
    first = Math.min(first, t);
    last = Math.max(last, t);
  }
  if (first === Infinity) return [];
  const end = Math.max(last, Math.floor(now / HOUR_MS) * HOUR_MS);
  const out: string[] = [];
  for (let t = Math.max(first, end - (MAX_HOURS - 1) * HOUR_MS); t <= end; t += HOUR_MS) out.push(hourKey(t));
  return out;
}

/** Index ranges of the hour keys per UTC day in axis order; partial days included (#123). */
export function daySpans(buckets: string[]): { day: string; from: number; to: number }[] {
  const out: { day: string; from: number; to: number }[] = [];
  buckets.forEach((b, i) => {
    const day = b.slice(0, 10);
    const last = out[out.length - 1];
    if (last?.day === day) last.to = i;
    else out.push({ day, from: i, to: i });
  });
  return out;
}

export type DayBand = { day: string; /** x of the separator before the day */ sep: number | null; /** label center; null when it does not fit */ at: number | null };

/**
 * Day bands of an hour axis drawn by x (#123): the separator before each
 * day and the center of its label over the day's visible hours. Labels of
 * width(day) are placed newest first and dropped where they would overlap.
 */
export function dayBands(buckets: string[], x: (i: number) => number, width: (day: string) => number): DayBand[] {
  const n = buckets.length;
  if (n === 0) return [];
  const lo = x(0);
  const hi = x(n - 1);
  const out: DayBand[] = daySpans(buckets).map((s) => ({ day: s.day, sep: s.from > 0 ? x(s.from - 0.5) : null, at: null }));
  let limit = Infinity;
  daySpans(buckets).reverse().forEach((s, ri) => {
    const w = width(s.day);
    if (w <= 0) return;
    const center = (Math.max(x(s.from - 0.5), lo) + Math.min(x(s.to + 0.5), hi)) / 2;
    const at = hi - lo < w ? (lo + hi) / 2 : Math.min(Math.max(center, lo + w / 2), hi - w / 2);
    if (at + w / 2 + 6 > limit) return;
    out[out.length - 1 - ri].at = at;
    limit = at - w / 2;
  });
  return out;
}

/** Rough width of a chart label at 11px in viewBox units. */
export const labelWidth = (text: string): number => text.length * 6.6;

const date = (k: string) => `${k.slice(8, 10)}.${k.slice(5, 7)}.`;

/**
 * Axis labels (UTC) for hour keys: up to 36 hours every 1, 2, 3 or 6 hours
 * as "HH:00" with the date at midnight, beyond that the date at midnight.
 */
export function hourTicks(buckets: string[]): { i: number; text: string }[] {
  const n = buckets.length;
  if (n === 0) return [];
  if (n > 36) {
    return buckets.flatMap((b, i) => (b.slice(11, 13) === '00' ? [{ i, text: date(b) }] : []));
  }
  const step = [1, 2, 3, 6].find((s) => n / s <= 8) ?? 6;
  const out = buckets.flatMap((b, i) => {
    const h = Number(b.slice(11, 13));
    if (h % step !== 0) return [];
    return [{ i, text: h === 0 && i > 0 ? date(b) : `${b.slice(11, 13)}:00` }];
  });
  // A single hour that is off the step still gets its label.
  return out.length ? out : [{ i: 0, text: `${buckets[0].slice(11, 13)}:00` }];
}
