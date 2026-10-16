import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import AccountCard from './lib/components/AccountCard.svelte';
import AccountUsage from './lib/components/AccountUsage.svelte';
import LimitCard from './lib/components/LimitCard.svelte';
import { elapsedPercent, FIVE_HOURS_MS, WEEK_MS } from './lib/limits';
import type { Account } from './lib/types/accounts';
import type { RateLimits } from './lib/types/dashboard';
import accountCardSrc from './lib/components/AccountCard.svelte?raw';
import limitCardSrc from './lib/components/LimitCard.svelte?raw';

// jsdom applies no component CSS, so the outline colour is checked in the
// style rule of the active class (same approach as layout.test.ts).
function rule(src: string, selector: string): string {
  const esc = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const m = new RegExp(`(?:^|[\\s}])${esc}\\s*\\{([^}]*)\\}`).exec(src);
  if (!m) throw new Error(`rule ${selector} not found`);
  return m[1];
}

const NOW = '2026-10-04T12:00:00Z';
const now = Date.parse(NOW);

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date(NOW));
});
afterEach(() => vi.useRealTimers());

describe('elapsedPercent', () => {
  it.each([
    ['five hours, one hour in', '2026-10-04T16:00:00Z', FIVE_HOURS_MS, 20],
    ['five hours, just started', '2026-10-04T17:00:00Z', FIVE_HOURS_MS, 0],
    ['week, half way', '2026-10-08T00:00:00Z', WEEK_MS, 50],
  ])('%s', (_, reset, windowMs, want) => {
    expect(elapsedPercent(reset, windowMs, now)).toBeCloseTo(want, 6);
  });

  it.each([
    ['empty reset', ''],
    ['missing reset', undefined],
    ['invalid reset', 'kein Datum'],
    ['passed reset', '2026-10-04T11:00:00Z'],
    ['reset right now', NOW],
    ['reset further away than the window', '2026-10-04T18:00:00Z'],
  ])('no mark for %s', (_, reset) => {
    expect(elapsedPercent(reset, FIVE_HOURS_MS, now)).toBeNull();
  });
});

const limits = (o: Partial<RateLimits> = {}): RateLimits => ({
  status: 'allowed',
  reset: '',
  representative_claim: 'five_hour',
  observed_at: '2026-10-04T11:59:00Z',
  five_hour: { utilization: 0.1, reset: '2026-10-04T16:00:00Z', status: 'allowed' },
  seven_day: { utilization: 0.8, reset: '2026-10-08T00:00:00Z', status: 'allowed_warning' },
  ...o,
});

describe('LimitCard Zeitmarke', () => {
  it('marks now in the five-hour and the weekly bar', () => {
    render(LimitCard, { title: 'x', limits: limits() });
    const five = screen.getByLabelText('20 % des 5-Stunden-Fensters vergangen');
    expect(five.style.left).toBe('20%');
    const week = screen.getByLabelText('50 % der Woche vergangen');
    expect(week.style.left).toBe('50%');
  });

  it('shows no mark without, with a passed or a too distant reset', () => {
    render(LimitCard, {
      title: 'x',
      limits: limits({
        five_hour: { utilization: 0.1, reset: '', status: 'allowed' },
        seven_day: { utilization: 0.8, reset: '2026-10-04T11:00:00Z', status: 'allowed' },
      }),
    });
    expect(screen.queryByText(/^Strich = jetzt/)).toBeNull();
    expect(screen.queryByLabelText(/vergangen$/)).toBeNull();
  });

  it('shows no five-hour mark when the reset is more than five hours away', () => {
    render(LimitCard, { title: 'x', limits: limits({ five_hour: { utilization: 0.1, reset: '2026-10-04T18:00:00Z', status: 'allowed' } }) });
    expect(screen.queryByLabelText(/5-Stunden-Fensters vergangen$/)).toBeNull();
    expect(screen.getByLabelText('50 % der Woche vergangen')).toBeInTheDocument();
  });

  it('outlines the active card in the ok colour, not the inactive one', () => {
    render(LimitCard, { title: 'aktiv', limits: limits(), active: true });
    render(LimitCard, { title: 'inaktiv', limits: limits() });
    const active = screen.getByRole('region', { name: 'aktiv' });
    const inactive = screen.getByRole('region', { name: 'inaktiv' });
    expect(active).toHaveClass('active');
    expect(within(active).getByRole('img', { name: 'aktiv' })).toBeInTheDocument();
    expect(inactive).not.toHaveClass('active');
    expect(rule(limitCardSrc, '.card.active')).toMatch(/box-shadow:[^;]*var\(--c-ok\)/);
    expect(rule(limitCardSrc, '.card')).not.toMatch(/--c-ok/);
  });
});

describe('AccountUsage 5-Stunden-Zeitmarke', () => {
  it('marks now in the five-hour bar next to the weekly mark', () => {
    render(AccountUsage, { usage: { five_hour: { utilization: 10, reset: '2026-10-04T16:00:00Z' }, seven_day: { utilization: 80, reset: '2026-10-08T00:00:00Z' }, observed_at: '2026-10-04T11:59:00Z', error: '' } });
    expect(screen.getByLabelText('20 % des 5-Stunden-Fensters vergangen').style.left).toBe('20%');
    expect(screen.getByLabelText('50 % der Woche vergangen').style.left).toBe('50%');
  });

  it('shows no five-hour mark after the reset or without a reset', () => {
    const view = render(AccountUsage, { usage: { five_hour: { utilization: 10, reset: '2026-10-04T11:00:00Z' }, seven_day: { utilization: 80, reset: '2026-10-08T00:00:00Z' }, observed_at: '2026-10-04T11:59:00Z', error: '' } });
    expect(screen.queryByLabelText(/5-Stunden-Fensters vergangen$/)).toBeNull();
    expect(screen.queryByText(/des Fensters vergangen$/)).toBeNull();
    view.unmount();
    render(AccountUsage, { usage: { five_hour: { utilization: 10, reset: '' }, seven_day: { utilization: 80, reset: '2026-10-08T00:00:00Z' }, observed_at: '2026-10-04T11:59:00Z', error: '' } });
    expect(screen.queryByLabelText(/5-Stunden-Fensters vergangen$/)).toBeNull();
  });
});

describe('AccountCard aktives Abo', () => {
  const account = (o: Partial<Account> = {}): Account => ({ id: 'a', name: 'Abo', email: 'a@example.com', active: true, status: 'active', expires_at: '', last_refresh: '', updated_at: '', ...o });

  it('outlines the active subscription in the ok colour', () => {
    render(AccountCard, { account: account() });
    render(AccountCard, { account: account({ id: 'b', name: 'Zweites', active: false }) });
    const [active, inactive] = screen.getAllByRole('article');
    expect(within(active).getByRole('heading', { name: 'a@example.com' })).toBeInTheDocument();
    expect(active).toHaveClass('active');
    expect(inactive).not.toHaveClass('active');
    expect(rule(accountCardSrc, '.account.active')).toMatch(/box-shadow:[^;]*var\(--c-ok\)/);
    expect(rule(accountCardSrc, '.account')).not.toMatch(/--c-ok/);
  });
});
