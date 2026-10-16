// Order of the subscription cards and compact cards on the overview (#124).
import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import AccountCard from './lib/components/AccountCard.svelte';
import AccountUsage from './lib/components/AccountUsage.svelte';
import { orderSubscriptions } from './lib/subscriptions';
import type { Account, UsageSnapshot } from './lib/types/accounts';

const NOW = Date.parse('2026-10-06T12:00:00Z');
const at = (h: number) => new Date(NOW + h * 3_600_000).toISOString();
const usage = (five: number | null, fiveReset: string, week: number | null, weekReset = at(72)): UsageSnapshot => ({
  five_hour: { utilization: five, reset: fiveReset },
  seven_day: { utilization: week, reset: weekReset },
  observed_at: at(0),
  error: '',
});
const account = (id: string, o: Partial<Account> = {}): Account => ({
  id, name: `Abo ${id}`, email: `${id}@example.com`, active: false, status: 'gespeichert', expires_at: '', last_refresh: '', updated_at: '', ...o,
});

describe('orderSubscriptions', () => {
  it('puts the active one first, then who can take over next', () => {
    const list = [
      account('weekdone', { usage: usage(10, at(1), 100, at(30)) }),
      account('unknown'),
      account('full-late', { usage: usage(100, at(4), 40) }),
      account('free-late', { usage: usage(30, at(3), 40) }),
      account('active', { active: true, usage: usage(100, at(2), 50) }),
      account('full-soon', { usage: usage(100, at(1), 90) }),
      account('fresh', { usage: usage(null, '', 20) }),
      account('free-soon', { usage: usage(60, at(1), 70) }),
    ];
    // free ones by lowest weekly use (#127), full 5-hour windows by reset
    expect(orderSubscriptions(list, NOW).map((a) => a.id)).toEqual([
      'active', 'fresh', 'free-late', 'free-soon', 'full-soon', 'full-late', 'weekdone', 'unknown',
    ]);
  });

  it('puts a week at 99 % behind every account with weekly quota (#127)', () => {
    const list = [
      account('week99', { usage: usage(0, at(1), 99, at(30)) }),
      account('fivefull', { usage: usage(100, at(2), 50) }),
      account('week100', { usage: usage(0, at(1), 100, at(20)) }),
      account('free', { usage: usage(50, at(1), 80) }),
    ];
    expect(orderSubscriptions(list, NOW).map((a) => a.id)).toEqual(['free', 'fivefull', 'week100', 'week99']);
  });

  it('breaks a weekly tie by the lower 5-hour use', () => {
    const list = [account('busy', { usage: usage(70, at(1), 30) }), account('calm', { usage: usage(10, at(4), 30) })];
    expect(orderSubscriptions(list, NOW).map((a) => a.id)).toEqual(['calm', 'busy']);
  });

  it('treats a passed reset as a fresh window', () => {
    const list = [
      account('full', { usage: usage(100, at(1), 10) }),
      account('reset', { usage: usage(100, at(-1), 10) }),
      account('weekreset', { usage: usage(10, at(1), 100, at(-2)) }),
    ];
    expect(orderSubscriptions(list, NOW).map((a) => a.id)).toEqual(['weekreset', 'reset', 'full']);
  });

  it('does not change the input', () => {
    const list = [account('b'), account('a', { active: true })];
    orderSubscriptions(list, NOW);
    expect(list.map((a) => a.id)).toEqual(['b', 'a']);
  });
});

describe('compact card', () => {
  it('uses the email as title and keeps the stored name as tooltip', () => {
    render(AccountCard, { account: account('a', { name: 'Hauptabo' }) });
    const card = screen.getByRole('article', { name: 'a@example.com' });
    expect(within(card).getByRole('heading')).toHaveAttribute('title', 'Hauptabo');
    expect(within(card).queryByText('Hauptabo')).toBeNull();
    expect(within(card).getAllByText('a@example.com')).toHaveLength(1);
  });

  it('falls back to the name without email', () => {
    render(AccountCard, { account: account('a', { name: 'Hauptabo', email: '' }) });
    expect(screen.getByRole('article', { name: 'Hauptabo' })).toBeInTheDocument();
  });

  it('shows the reset next to the percentage, without the mark caption', () => {
    render(AccountUsage, { usage: usage(40, new Date(Date.now() + 2 * 3_600_000).toISOString(), 50, new Date(Date.now() + 3 * 86_400_000).toISOString()) });
    for (const row of screen.getAllByTestId('pct-row')) {
      expect(row.querySelector('.pct')).not.toBeNull();
      expect(row.querySelector('.reset')?.textContent).toMatch(/^Reset in /);
    }
    expect(screen.getAllByTestId('pct-row')).toHaveLength(2);
    expect(screen.queryByText(/^Strich = jetzt/)).toBeNull();
  });
});
