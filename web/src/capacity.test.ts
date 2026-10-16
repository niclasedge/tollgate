import { describe, expect, it } from 'vitest';
import { estimate, missing } from './lib/capacity';
import type { Capacity } from './lib/types/dashboard';

const cap = (over: Partial<Capacity> = {}): Capacity => ({
  windows_per_week: 8, fills_per_day: 5, measured_hours: 48, sample_windows: 3,
  window_cost_nano_usd: 100e9, sample_accounts: 2, accounts: 4, free_this_week: null, ...over,
});

describe('estimate', () => {
  it('flags four accounts as not enough for 35 windows a week', () => {
    const e = estimate(cap())!;
    expect(e.needPerWeek).toBe(35);
    expect(e.capacityPerWeek).toBe(32);
    expect(e.verdict).toBe('reicht nicht');
  });
  it('judges by share of capacity', () => {
    expect(estimate(cap({ fills_per_day: 3 }))!.verdict).toBe('reicht');
    expect(estimate(cap({ fills_per_day: 4 }))!.verdict).toBe('knapp');
  });
  it('waits for missing measurements', () => {
    expect(estimate(cap({ windows_per_week: null }))).toBeNull();
    expect(estimate(cap({ fills_per_day: null }))).toBeNull();
    expect(estimate(cap({ accounts: 0 }))).toBeNull();
    expect(missing(cap({ window_cost_nano_usd: null, windows_per_week: null }))).toMatch(/30 Punkten Anstieg/);
    expect(missing(cap({ windows_per_week: null }))).toMatch(/25 % Wochenverbrauch/);
  });
});
