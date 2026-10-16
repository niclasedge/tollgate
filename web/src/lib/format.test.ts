import { describe, expect, it } from 'vitest';
import { cacheHitRate, formatCost, formatPercent, formatSavedDelta, formatUntil, totalTokens } from './format';
import { aggregate, costLabel, daysBetween, total } from './costs';
import { costRow } from '../test-helpers';

describe('format', () => {
  it('formats nano-USD and shows a dash for null', () => {
    expect(formatCost(1_500_000_000)).toBe('$1.50');
    expect(formatCost(12_300_000)).toBe('$0.0123');
    expect(formatCost(0)).toBe('$0.00');
    expect(formatCost(null)).toBe('–');
  });
  it('computes the cache hit rate as read / (input + read + creation)', () => {
    const u = { input: 100, output: 999, cache_read: 800, cache_creation: 100 };
    expect(cacheHitRate(u)).toBe(0.8);
    expect(formatPercent(0.8)).toBe('80,0 %');
    expect(cacheHitRate({ input: 0, output: 5, cache_read: 0, cache_creation: 0 })).toBeNull();
    expect(totalTokens(u)).toBe(1999);
  });
});

describe('costs aggregation', () => {
  it('sums by key and keeps null only when no row has a price', () => {
    const rows = [costRow(), costRow({ model: 'x', cost_nano_usd: null }), costRow({ day: '2026-10-03', cost_nano_usd: 500 })];
    const byModel = aggregate(rows, (r) => r.model);
    expect(byModel.find((a) => a.key === 'x')!.cost_nano_usd).toBeNull();
    expect(byModel.find((a) => a.key === 'claude-opus-5-5')!.requests).toBe(4);
    expect(total(rows).cost_nano_usd).toBe(1_500_000_500);
    expect(total([]).requests).toBe(0);
    expect(costLabel(total(rows))).toBe('≥ $1.50');
    expect(costLabel(byModel.find((a) => a.key === 'x'))).toBe('–');
    expect(costLabel(total([costRow()]))).toBe('$1.50');
  });
  it('lists every UTC day in the range', () => {
    expect(daysBetween('2026-09-29', '2026-10-02')).toEqual(['2026-09-29', '2026-09-30', '2026-10-01', '2026-10-02']);
  });
});

describe('formatUntil', () => {
  const now = Date.parse('2026-10-04T18:00:00Z');
  it('formats minutes, hours and days and hides unknown or past times', () => {
    expect(formatUntil('2026-10-04T18:04:30Z', now)).toBe('5 min');
    expect(formatUntil('2026-10-04T20:13:00Z', now)).toBe('2 h 13 min');
    expect(formatUntil('2026-10-04T21:00:00Z', now)).toBe('3 h');
    expect(formatUntil('2026-10-07T22:00:00Z', now)).toBe('3 T 4 h');
    expect(formatUntil('2026-10-04T17:59:00Z', now)).toBe('');
    expect(formatUntil('', now)).toBe('');
    expect(formatUntil('soon', now)).toBe('');
  });
});

describe('formatSavedDelta', () => {
  it.each([
    [12, '−12'],
    [1234, '−1.234'],
    [-12, '+12'],
    [-1234, '+1.234'],
    [0, '–'],
    [null, '–'],
    [undefined, '–'],
  ])('formats %s as %s with exactly one sign', (saved, want) => {
    expect(formatSavedDelta(saved)).toBe(want);
  });
});
