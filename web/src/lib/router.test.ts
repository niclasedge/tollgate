import { describe, expect, it } from 'vitest';
import { parseHash, routes, hrefFor, groups } from './router.svelte';

describe('parseHash', () => {
  it.each([
    ['', 'home'],
    ['#', 'home'],
    ['#/', 'home'],
    ['#/home', 'home'],
    ['#/live', 'live'],
    ['#/requests', 'requests'],
    ['#/costs', 'costs'],
    ['#/performance', 'costs'],
    ['#/filters', 'filters'],
    ['#/compression', 'compression'],
    ['#/clients', 'clients'],
    ['#/accounts', 'accounts'],
    ['#/unknown', 'home'],
    ['#/requests/abc123', 'requests'],
  ])('%j -> %s', (hash, id) => {
    expect(parseHash(hash).id).toBe(id);
  });

  it('exposes the detail segment for inline views', () => {
    expect(parseHash('#/requests/abc123').detail).toBe('abc123');
    expect(parseHash('#/requests').detail).toBeUndefined();
  });

  it('ignores a malformed detail segment instead of throwing', () => {
    expect(parseHash('#/requests/%E0%A4%A')).toEqual({ id: 'requests' });
    expect(parseHash('#/requests/a%20b').detail).toBe('a b');
  });

  it('lists the eight areas in a fixed order with unique keys, home on 0', () => {
    expect(routes.map((r) => r.id)).toEqual([
      'home', 'live', 'requests', 'costs', 'filters', 'compression', 'clients', 'accounts',
    ]);
    expect(routes.map((r) => r.key)).toEqual(['0', '1', '2', '3', '4', '5', '6', '7']);
    expect(routes.find((r) => r.id === 'costs')?.label).toBe('Kosten & Performance');
    expect(hrefFor('costs')).toBe('#/costs');
    expect(hrefFor('live', 'a/b')).toBe('#/live/a%2Fb');
  });

  it('groups the areas for the sidebar', () => {
    expect(groups.map((g) => [g.label, g.routes.map((r) => r.id)])).toEqual([
      ['Übersicht', ['home']],
      ['Beobachten', ['live', 'requests', 'costs']],
      ['Steuern', ['filters', 'compression']],
      ['Verwalten', ['clients', 'accounts']],
    ]);
  });
});
