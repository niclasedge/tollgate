import { describe, expect, it } from 'vitest';
import { planLabel, subscriptionLabel, tokenState } from './lib/license';
import type { License } from './lib/types/accounts';

const lic = (o: Partial<License> = {}): License => ({ token: 'valid', checked_at: '2026-10-04T10:00:00Z', organization_type: '', rate_limit_tier: '', subscription_status: '', profile_observed_at: '', ...o });

describe('license labels', () => {
  it('maps known plans and tiers and keeps unknown identifiers', () => {
    expect(planLabel(lic({ organization_type: 'claude_max', rate_limit_tier: 'default_claude_max_20x' }))).toBe('Max 20x');
    expect(planLabel(lic({ organization_type: 'claude_max', rate_limit_tier: 'default_claude_max_5x' }))).toBe('Max 5x');
    expect(planLabel(lic({ organization_type: 'claude_pro' }))).toBe('Pro');
    expect(planLabel(lic({ organization_type: 'claude_team' }))).toBe('Team');
    expect(planLabel(lic({ organization_type: 'claude_future_plan' }))).toBe('claude_future_plan');
    expect(planLabel(lic())).toBe('Plan unbekannt');
    expect(planLabel(undefined)).toBe('Plan unbekannt');
  });
  it('labels subscription status without guessing', () => {
    expect(subscriptionLabel(lic({ subscription_status: 'active' }))).toBe('Abo aktiv');
    expect(subscriptionLabel(lic({ subscription_status: 'past_due' }))).toBe('Zahlung offen');
    expect(subscriptionLabel(lic({ subscription_status: 'canceled' }))).toBe('gekündigt');
    expect(subscriptionLabel(lic({ subscription_status: 'paused_x' }))).toBe('paused_x');
    expect(subscriptionLabel(lic())).toBe('');
  });
  it('derives a round status dot and text from the token state', () => {
    expect(tokenState(lic(), false)).toEqual({ state: 'ok', label: 'Token gültig' });
    expect(tokenState(lic({ token: 'relogin' }), false)).toEqual({ state: 'warn', label: 'Neu einloggen nötig' });
    expect(tokenState(lic({ token: 'expired' }), false)).toEqual({ state: 'idle', label: 'Zugang abgelaufen · wird beim Aktivieren erneuert' });
    expect(tokenState(lic({ token: 'expired' }), true)).toEqual({ state: 'idle', label: 'Zugang abgelaufen · wird von CLIProxyAPI erneuert' });
    expect(tokenState(lic({ token: 'unknown' }), true)).toEqual({ state: 'idle', label: 'Token-Status unbekannt' });
    expect(tokenState(undefined, true)).toEqual({ state: 'idle', label: 'Token-Status unbekannt' });
  });
});
