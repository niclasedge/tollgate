import { formatCost, totalTokens } from './format';
import { hourAxis } from './hours';
import type { CostReport, CostRow, Usage } from './types/dashboard';

export interface Agg {
  key: string;
  label: string;
  requests: number;
  usage: Usage;
  saved_tokens: number;
  /** Sum of the known costs; null when no row has a price. */
  cost_nano_usd: number | null;
  /** True when some rows have no price, so the cost sum is a lower bound. */
  cost_partial: boolean;
  /** Sum of the known cache savings (#121); null when no row has a price. */
  cache_savings_nano_usd: number | null;
}

const zero = (): Usage => ({ input: 0, output: 0, cache_read: 0, cache_creation: 0 });
const empty = (key: string, label: string): Agg => ({
  key, label, requests: 0, usage: zero(), saved_tokens: 0, cost_nano_usd: null, cost_partial: false, cache_savings_nano_usd: null,
});

export function aggregate(rows: CostRow[], key: (r: CostRow) => string, label: (r: CostRow) => string = key): Agg[] {
  const m = new Map<string, Agg>();
  for (const r of rows) {
    const k = key(r);
    let a = m.get(k);
    if (!a) {
      a = empty(k, label(r));
      m.set(k, a);
    }
    a.requests += r.requests;
    a.usage.input += r.usage.input;
    a.usage.output += r.usage.output;
    a.usage.cache_read += r.usage.cache_read;
    a.usage.cache_creation += r.usage.cache_creation;
    a.saved_tokens += r.saved_tokens;
    if (r.cost_nano_usd !== null) a.cost_nano_usd = (a.cost_nano_usd ?? 0) + r.cost_nano_usd;
    else a.cost_partial = true;
    // Older reports have no savings field; treat it like an unknown price.
    if (r.cache_savings_nano_usd != null) a.cache_savings_nano_usd = (a.cache_savings_nano_usd ?? 0) + r.cache_savings_nano_usd;
  }
  return [...m.values()];
}

export function total(rows: CostRow[]): Agg {
  return aggregate(rows, () => 'all')[0] ?? empty('all', '');
}

/** Cost label; a sum with unpriced rows is shown as a lower bound. */
export function costLabel(a: Pick<Agg, 'cost_nano_usd' | 'cost_partial'> | undefined): string {
  const c = formatCost(a?.cost_nano_usd);
  return a?.cost_partial && a.cost_nano_usd !== null ? `≥ ${c}` : c;
}

/** Cost label per UTC day for the day sums above the cost curve (#123). */
export function dayCosts(rows: CostRow[]): Map<string, string> {
  return new Map(aggregate(rows, (r) => r.day).map((a) => [a.key, costLabel(a)]));
}

/** All UTC days from..to (inclusive), so gaps show as empty bars. */
export function daysBetween(from: string, to: string): string[] {
  const out: string[] = [];
  const end = Date.parse(`${to}T00:00:00Z`);
  for (let t = Date.parse(`${from}T00:00:00Z`); t <= end && out.length < 400; t += 86400000) {
    out.push(new Date(t).toISOString().slice(0, 10));
  }
  return out;
}

/** Highest cost first, then most tokens. */
export function byCost(a: Agg, b: Agg): number {
  return (b.cost_nano_usd ?? -1) - (a.cost_nano_usd ?? -1) || totalTokens(b.usage) - totalTokens(a.usage);
}

/** Account label as in the Accounts area (#121); names maps ID to name. */
export function accountLabel(id: string, names: ReadonlyMap<string, string>): string {
  if (!id) return 'Nicht zugeordnet';
  return names.get(id) || `Account ${id.slice(0, 12)}`;
}

export type Shared = Agg & { /** Share of the known total cost (0..1); null without a known cost. */ share: number | null };

/** Adds each group's share of the known total cost. */
export function withShares(list: Agg[]): Shared[] {
  const sum = list.reduce((s, a) => s + (a.cost_nano_usd ?? 0), 0);
  return list.map((a) => ({ ...a, share: sum > 0 && a.cost_nano_usd !== null ? a.cost_nano_usd / sum : null }));
}

export type BreakdownBy = 'account' | 'model' | 'client' | 'day';

/** The breakdown table: groups by cost, days newest first; only groups with rows exist, so empty days are hidden. */
export function breakdown(rows: CostRow[], by: BreakdownBy, account: (id: string) => string): Shared[] {
  const groups =
    by === 'account'
      ? aggregate(rows, (r) => r.account_id ?? '', (r) => account(r.account_id ?? ''))
      : by === 'model'
        ? aggregate(rows, (r) => r.model, (r) => r.model || '–')
        : by === 'client'
          ? aggregate(rows, (r) => r.client_id, (r) => r.client_name || r.client_id)
          : aggregate(rows, (r) => r.day);
  const kept = groups.filter((a) => a.requests > 0);
  return withShares(by === 'day' ? kept.sort((a, b) => b.key.localeCompare(a.key)) : kept.sort(byCost));
}

export interface Metrics {
  processed: number;
  cache_read: number;
  /** Input without a cache hit: plain input plus cache writes. */
  uncached_input: number;
  output: number;
  cache_savings_nano_usd: number | null;
  /** True when some rows have no price, so the savings are a lower bound. */
  cache_savings_partial: boolean;
  saved_tokens: number;
}

/** The key figures row of the dashboard. */
export function metrics(rows: CostRow[]): Metrics {
  const t = total(rows);
  return {
    processed: totalTokens(t.usage),
    cache_read: t.usage.cache_read,
    uncached_input: t.usage.input + t.usage.cache_creation,
    output: t.usage.output,
    cache_savings_nano_usd: t.cache_savings_nano_usd,
    cache_savings_partial: rows.some((r) => r.cache_savings_nano_usd == null),
    saved_tokens: t.saved_tokens,
  };
}

export interface CostSeries {
  /** Hour starts YYYY-MM-DDTHH:00:00Z (UTC) without gaps, see hourAxis. */
  buckets: string[];
  /** One series per account in the given order; known costs in nano-USD. */
  series: { key: string; values: number[] }[];
}

/**
 * The hourly cost curve per account (#121): from the first hour with costs
 * up to the current hour, at most the last 7 days. accounts fixes the order
 * (and thereby the colors).
 */
export function costSeries(r: CostReport, accounts: string[], now: number = Date.now()): CostSeries {
  const hours = r.hours ?? [];
  const buckets = hourAxis(
    hours.map((h) => h.hour),
    now,
  );
  const index = new Map(buckets.map((b, i) => [b, i]));
  const m = new Map(accounts.map((a) => [a, new Array<number>(buckets.length).fill(0)]));
  for (const h of hours) {
    const v = m.get(h.account_id);
    const i = index.get(h.hour);
    if (v && i !== undefined && h.cost_nano_usd !== null) v[i] += h.cost_nano_usd;
  }
  return { buckets, series: accounts.map((key) => ({ key, values: m.get(key)! })) };
}

/**
 * Color per account in fixed order (#128): five categorical colors from
 * app.css, checked for color vision deficiency in light and dark; further
 * accounts are gray instead of reusing a color.
 */
export const SERIES_COLORS = 5;
export const seriesColor = (i: number): string => (i < SERIES_COLORS ? `var(--c-series-${i + 1})` : 'var(--c-muted)');
