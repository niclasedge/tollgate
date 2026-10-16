import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/svelte';
import App from './App.svelte';
import { auth, logout } from './lib/auth.svelte';
import { json, mockFetch, okStatus } from './test-helpers';

const labels = ['Übersicht', 'Live', 'Anfragen', 'Kosten & Performance', 'Filter', 'Kompression', 'Clients', 'Accounts'];

afterEach(() => {
  logout();
  vi.unstubAllGlobals();
});

describe('App shell', () => {
  it('renders navigation for all eight areas', () => {
    render(App);
    const nav = screen.getByRole('navigation', { name: 'Bereiche' });
    for (const l of labels) expect(nav).toHaveTextContent(l);
    expect(nav.querySelectorAll('a')).toHaveLength(8);
  });

  it('opens the overview by default and with 0', async () => {
    render(App);
    expect(screen.getByRole('heading', { level: 1, name: 'Übersicht' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Übersicht' })).toHaveAttribute('aria-current', 'page');
    await fireEvent.keyDown(window, { key: '1' });
    expect(location.hash).toBe('#/live');
    await fireEvent.keyDown(window, { key: '0' });
    expect(location.hash).toBe('#/home');
    expect(await screen.findByRole('heading', { level: 1, name: 'Übersicht' })).toBeInTheDocument();
  });

  it('groups the sidebar under small headings', () => {
    render(App);
    const nav = screen.getByRole('navigation', { name: 'Bereiche' });
    const groups = within(nav).getAllByRole('group');
    expect(groups.map((g) => g.getAttribute('aria-label'))).toEqual(['Übersicht', 'Beobachten', 'Steuern', 'Verwalten']);
    expect(within(groups[1]).getAllByRole('link').map((a) => a.getAttribute('aria-label'))).toEqual(['Live', 'Anfragen', 'Kosten & Performance']);
    expect(within(groups[3]).getAllByRole('link').map((a) => a.getAttribute('aria-label'))).toEqual(['Clients', 'Accounts']);
  });

  it('collapses the sidebar to icons and remembers it', async () => {
    const { unmount } = render(App);
    const btn = screen.getByRole('button', { name: 'Seitenleiste einklappen' });
    expect(btn).toHaveAttribute('aria-expanded', 'true');
    await fireEvent.click(btn);
    expect(localStorage.getItem('tollgate.sidebar')).toBe('collapsed');
    unmount();
    render(App);
    const again = screen.getByRole('button', { name: 'Seitenleiste ausklappen' });
    expect(again).toHaveAttribute('aria-expanded', 'false');
    expect(again.closest('.shell')).toHaveClass('collapsed');
    await fireEvent.click(again);
    expect(localStorage.getItem('tollgate.sidebar')).toBe('expanded');
  });

  it('shows quiet counters for clients and accounts once signed in', async () => {
    auth.token = 'tok';
    mockFetch({
      'GET /api/clients': () => json({ clients: [{ id: 'c1' }, { id: 'c2' }] }),
      'GET /api/status': () => json(okStatus()),
      'GET /api/costs': () => json({ from: '2026-10-04', to: '2026-10-04', rows: [] }),
      'GET /api/traces': () => json({ traces: [], next_before: '', next_before_id: '' }),
    });
    render(App);
    const clients = screen.getByRole('link', { name: 'Clients' });
    expect(await within(clients).findByText('2')).toHaveClass('count');
    expect(await within(screen.getByRole('link', { name: 'Accounts' })).findByText('1')).toHaveClass('count');
  });

  it('marks the current area and shows its title', async () => {
    location.hash = '#/costs';
    render(App);
    expect(await screen.findByRole('heading', { level: 1, name: 'Kosten & Performance' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Kosten & Performance' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Live' })).not.toHaveAttribute('aria-current');
  });

  it('navigates with number keys', async () => {
    render(App);
    await fireEvent.keyDown(window, { key: '6' });
    expect(location.hash).toBe('#/clients');
    expect(await screen.findByRole('heading', { level: 1, name: 'Clients' })).toBeInTheDocument();
  });

  it('ignores shortcuts while typing and with modifiers', async () => {
    render(App);
    const input = document.createElement('input');
    document.body.appendChild(input);
    await fireEvent.keyDown(input, { key: '3' });
    await fireEvent.keyDown(window, { key: '3', metaKey: true });
    expect(location.hash).toBe('');
    input.remove();
  });

  it('toggles dark mode with d and persists it', async () => {
    render(App);
    await fireEvent.keyDown(window, { key: 'd' });
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('tollgate.theme')).toBe('dark');
    await fireEvent.click(screen.getByRole('button', { name: /Darstellung/ }));
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('has a skip link and a main landmark', () => {
    render(App);
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Zum Inhalt springen' })).toBeInTheDocument();
  });

  it('skip link focuses main without changing the route', async () => {
    location.hash = '#/costs';
    render(App);
    await fireEvent.click(screen.getByRole('link', { name: 'Zum Inhalt springen' }));
    expect(location.hash).toBe('#/costs');
    expect(document.activeElement).toBe(screen.getByRole('main'));
  });
});
