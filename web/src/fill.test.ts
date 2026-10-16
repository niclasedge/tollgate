import { describe, expect, it } from 'vitest';
import { fillLine, formatSpan } from './lib/fill';
import type { WindowFill } from './lib/types/dashboard';

const fill = (over: Partial<WindowFill> = {}): WindowFill => ({
  account_id: 'a', reset: '2026-10-07T17:00:00Z', from_permille: 0, to_permille: 1000,
  observed_us: 100 * 60e6, full_us: 100 * 60e6, requests: 250, per_minute: 2.5, ...over,
});

describe('formatSpan', () => {
  it('formats hours and minutes', () => {
    expect(formatSpan(100 * 60e6)).toBe('1 h 40 min');
    expect(formatSpan(120 * 60e6)).toBe('2 h');
    expect(formatSpan(45 * 60e6)).toBe('45 min');
  });
  it('rounds to at least one minute', () => {
    expect(formatSpan(10e6)).toBe('1 min');
    expect(formatSpan(89.6 * 60e6)).toBe('1 h 30 min');
  });
});

describe('fillLine', () => {
  it('shows rise, span and requests per minute', () => {
    expect(fillLine(fill())).toBe('0 → 100 % in 1 h 40 min · Ø 2,5 Anfragen/min');
  });
  it('rounds the percentages', () => {
    expect(fillLine(fill({ from_permille: 204, to_permille: 696, observed_us: 60 * 60e6, per_minute: 3 }))).toBe('20 → 70 % in 1 h · Ø 3,0 Anfragen/min');
  });
});
