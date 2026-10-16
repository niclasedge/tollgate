import type { Usage } from './types/dashboard';

const NANO = 1e9;

/** Nano-USD to a USD string; null (unknown price / no usage) becomes a dash. */
export function formatCost(nano: number | null | undefined): string {
  if (nano === null || nano === undefined) return '–';
  const usd = nano / NANO;
  return `$${usd.toFixed(usd !== 0 && Math.abs(usd) < 1 ? 4 : 2)}`;
}

export function formatTokens(n: number): string {
  return new Intl.NumberFormat('de-DE').format(n);
}

/** Saved tokens as a delta with exactly one sign: 12 -> "−12", -12 -> "+12", 0/missing -> "–". */
export function formatSavedDelta(saved: number | null | undefined): string {
  if (!saved) return '–';
  return `${saved > 0 ? '−' : '+'}${formatTokens(Math.abs(saved))}`;
}

export function totalTokens(u: Usage | null | undefined): number {
  return u ? u.input + u.output + u.cache_read + u.cache_creation : 0;
}

/** cache_read / (input + cache_read + cache_creation); null without input. */
export function cacheHitRate(u: Usage): number | null {
  const denom = u.input + u.cache_read + u.cache_creation;
  return denom > 0 ? u.cache_read / denom : null;
}

export function formatPercent(r: number | null): string {
  return r === null ? '–' : `${(r * 100).toFixed(1).replace('.', ',')} %`;
}

export function formatTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '–';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' });
}

/** Time until iso as "2 h 13 min", "5 min" or "3 T 4 h"; "" if unknown or past. */
export function formatUntil(iso: string, now: number = Date.now()): string {
  const t = new Date(iso).getTime();
  if (!iso || Number.isNaN(t) || t <= now) return '';
  const min = Math.ceil((t - now) / 60000);
  const d = Math.floor(min / 1440);
  const h = Math.floor((min % 1440) / 60);
  const m = min % 60;
  if (d > 0) return h > 0 ? `${d} T ${h} h` : `${d} T`;
  if (h > 0) return m > 0 ? `${h} h ${m} min` : `${h} h`;
  return `${m} min`;
}
