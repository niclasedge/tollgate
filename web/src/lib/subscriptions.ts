// Order of the subscription cards on the overview (#124): the active one
// first, then the others by who can take over when its window is full.
import type { Account, UsageWindow } from './types/accounts';

// From this weekly use on an account counts as used up: 99 % leaves no room
// for a session (#127).
const WEEK_FULL = 99;

// Reset time of a window that is still running, else null.
function running(w: UsageWindow | undefined, now: number): number | null {
  const t = w?.reset ? Date.parse(w.reset) : NaN;
  return Number.isFinite(t) && t > now ? t : null;
}

// The limit used up in a running window: the week (from WEEK_FULL) before the 5-hour window.
function reached(a: Account, now: number): { limit: 'week' | 'five'; reset: number } | null {
  const u = a.usage;
  if (!u) return null;
  const week = running(u.seven_day, now);
  if (week !== null && (u.seven_day.utilization ?? 0) >= WEEK_FULL) return { limit: 'week', reset: week };
  const five = running(u.five_hour, now);
  if (five !== null && (u.five_hour.utilization ?? 0) >= 100) return { limit: 'five', reset: five };
  return null;
}

// Group and keys within the group:
//   0 active
//   1 weekly quota left, 5-hour window free; keys: weekly use, then 5-hour use (#127)
//   2 weekly quota left, 5-hour window full; key: its reset
//   3 weekly quota used up; key: weekly reset
//   4 usage unknown
// A passed window counts as fresh, i.e. 0 % used.
function rank(a: Account, now: number): [number, number, number] {
  if (a.active) return [0, 0, 0];
  const u = a.usage;
  if (!u) return [4, 0, 0];
  const r = reached(a, now);
  if (r) return [r.limit === 'week' ? 3 : 2, r.reset, 0];
  const use = (w: UsageWindow) => (running(w, now) === null ? 0 : (w.utilization ?? 0));
  return [1, use(u.seven_day), use(u.five_hour)];
}

/** True when the account has used up its 5-hour or weekly limit in a running window. */
export function isFull(a: Account, now: number): boolean {
  return reached(a, now) !== null;
}

/**
 * The account to switch to by hand once the active one is full (#126): the
 * first free account in overview order. tollgate does not switch on its own,
 * so the subscription can be adjusted first.
 */
export function switchCandidate(list: readonly Account[], now: number): Account | undefined {
  const active = list.find((a) => a.active);
  if (!active || !isFull(active, now)) return undefined;
  return orderSubscriptions(list, now).find((a) => !a.active && a.usage && !isFull(a, now));
}

export function orderSubscriptions(list: readonly Account[], now: number): Account[] {
  const keyed = list.map((a) => ({ a, r: rank(a, now), label: a.email || a.name }));
  keyed.sort((x, y) => x.r[0] - y.r[0] || x.r[1] - y.r[1] || x.r[2] - y.r[2] || (x.label < y.label ? -1 : x.label > y.label ? 1 : 0));
  return keyed.map((k) => k.a);
}
