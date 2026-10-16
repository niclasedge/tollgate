// Costs per subscription window and the week comparison on the overview (#125).
import { aggregate, type Agg } from './costs';
import type { CostHour, CostRow } from './types/dashboard';

const HOUR_MS = 3_600_000;
const DAY_MS = 86_400_000;

export interface WindowCost {
  requests: number;
  /** Sum of the known costs; null when no hour has a price. */
  cost_nano_usd: number | null;
  /** True when some hours have no price, so the sum is a lower bound. */
  cost_partial: boolean;
}

/**
 * Costs of one account inside the running window that ends at reset and
 * lasts spanMs. Hour buckets count from the hour the window starts in, so the
 * sum may include up to one hour before the start. Null when the reset is
 * unknown, passed or further away than the window length.
 */
export function windowCost(
  hours: readonly CostHour[] | undefined,
  accountId: string,
  reset: string | null | undefined,
  spanMs: number,
  now: number,
): WindowCost | null {
  const end = reset ? Date.parse(reset) : NaN;
  if (!Number.isFinite(end) || end <= now || end - now > spanMs) return null;
  const from = Math.floor((end - spanMs) / HOUR_MS) * HOUR_MS;
  const out: WindowCost = { requests: 0, cost_nano_usd: null, cost_partial: false };
  for (const h of hours ?? []) {
    if (h.account_id !== accountId || Date.parse(h.hour) < from) continue;
    out.requests += h.requests;
    if (h.cost_nano_usd !== null) out.cost_nano_usd = (out.cost_nano_usd ?? 0) + h.cost_nano_usd;
    else out.cost_partial = true;
  }
  return out;
}

export interface WeekDay {
  /** Short weekday name, Monday first. */
  weekday: string;
  /** Date in this week and the same weekday last week (YYYY-MM-DD, UTC). */
  day: string;
  prev_day: string;
  /** Null for days after today. */
  current: Agg | null;
  previous: Agg;
}

export interface WeekComparison {
  /** This calendar week (UTC, Monday first) up to and including today. */
  current: Agg;
  /** Last week over the same weekdays, so the comparison is fair. */
  previous: Agg;
  /** All seven days of last week. */
  previous_full: Agg;
  days: WeekDay[];
}

const WEEKDAYS = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'];
const iso = (t: number) => new Date(t).toISOString().slice(0, 10);

/** Compares this calendar week with last week; today is the report's last day. */
export function weekComparison(rows: readonly CostRow[], today: string): WeekComparison {
  const t = Date.parse(`${today}T00:00:00Z`);
  const offset = (new Date(t).getUTCDay() + 6) % 7;
  const monday = t - offset * DAY_MS;
  const byDay = new Map(aggregate([...rows], (r) => r.day).map((a) => [a.key, a]));
  const sum = (days: string[]) => aggregate(rows.filter((r) => days.includes(r.day)), () => 'all')[0] ?? emptyAgg();
  const days: WeekDay[] = WEEKDAYS.map((weekday, i) => {
    const day = iso(monday + i * DAY_MS);
    const prev_day = iso(monday + (i - 7) * DAY_MS);
    return { weekday, day, prev_day, current: i <= offset ? (byDay.get(day) ?? emptyAgg(day)) : null, previous: byDay.get(prev_day) ?? emptyAgg(prev_day) };
  });
  const upToToday = days.slice(0, offset + 1);
  return {
    current: sum(upToToday.map((d) => d.day)),
    previous: sum(upToToday.map((d) => d.prev_day)),
    previous_full: sum(days.map((d) => d.prev_day)),
    days,
  };
}

function emptyAgg(key = 'all'): Agg {
  return { key, label: key, requests: 0, usage: { input: 0, output: 0, cache_read: 0, cache_creation: 0 }, saved_tokens: 0, cost_nano_usd: null, cost_partial: false, cache_savings_nano_usd: null };
}

/** Relative change as text ("+12 %", "−5 %"); null without a base to compare with. */
export function deltaLabel(current: number | null, previous: number | null): string | null {
  if (current === null || previous === null || previous === 0) return null;
  const pct = Math.round(((current - previous) / previous) * 100);
  return pct === 0 ? '±0 %' : `${pct > 0 ? '+' : '−'}${Math.abs(pct)} %`;
}

/** Change of a rate (0..1) in percentage points ("+3,1 Pp."). */
export function pointsLabel(current: number | null, previous: number | null): string | null {
  if (current === null || previous === null) return null;
  const pp = (current - previous) * 100;
  const text = Math.abs(pp).toFixed(1).replace('.', ',');
  return text === '0,0' ? '±0 Pp.' : `${pp > 0 ? '+' : '−'}${text} Pp.`;
}
