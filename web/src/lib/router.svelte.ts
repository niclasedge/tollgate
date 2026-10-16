// Tiny hash router. Detail views unfold inline, so there are only the eight
// top-level areas plus an optional detail segment (#/live/<id>).

export type RouteId = 'home' | 'live' | 'requests' | 'costs' | 'filters' | 'compression' | 'clients' | 'accounts';

export type GroupLabel = 'Übersicht' | 'Beobachten' | 'Steuern' | 'Verwalten';

export interface Route {
  id: RouteId;
  label: string;
  /** Keyboard shortcut (single key, no modifiers). */
  key: string;
  /** Sidebar group heading. */
  group: GroupLabel;
}

export const routes: readonly Route[] = [
  { id: 'home', label: 'Übersicht', key: '0', group: 'Übersicht' },
  { id: 'live', label: 'Live', key: '1', group: 'Beobachten' },
  { id: 'requests', label: 'Anfragen', key: '2', group: 'Beobachten' },
  // Costs and performance share one page (#121).
  { id: 'costs', label: 'Kosten & Performance', key: '3', group: 'Beobachten' },
  { id: 'filters', label: 'Filter', key: '4', group: 'Steuern' },
  { id: 'compression', label: 'Kompression', key: '5', group: 'Steuern' },
  { id: 'clients', label: 'Clients', key: '6', group: 'Verwalten' },
  { id: 'accounts', label: 'Accounts', key: '7', group: 'Verwalten' },
];

/** Routes grouped in sidebar order. */
export const groups: ReadonlyArray<{ label: GroupLabel; routes: Route[] }> = [...new Set(routes.map((r) => r.group))].map(
  (label) => ({ label, routes: routes.filter((r) => r.group === label) }),
);

export interface Location {
  id: RouteId;
  detail?: string;
}

export function hrefFor(id: RouteId, detail?: string): string {
  return detail ? `#/${id}/${encodeURIComponent(detail)}` : `#/${id}`;
}

/** Former areas that now live on another page (#121). */
const aliases: Readonly<Record<string, RouteId>> = { performance: 'costs' };

export function parseHash(hash: string): Location {
  const [raw, detail] = hash.replace(/^#\/?/, '').split('/');
  const area = aliases[raw] ?? raw;
  const route = routes.find((r) => r.id === area);
  if (!route) return { id: 'home' };
  if (!detail) return { id: route.id };
  try {
    return { id: route.id, detail: decodeURIComponent(detail) };
  } catch {
    // Malformed escape (e.g. "%E0%A4%A"): stay in the area, drop the detail.
    return { id: route.id };
  }
}

export const router = $state<{ current: Location }>({ current: parseHash(globalThis.location?.hash ?? '') });

/** Subscribes to hash changes; returns the unsubscribe function. */
export function startRouter(): () => void {
  const sync = () => {
    router.current = parseHash(location.hash);
    // An alias is replaced in place, so the old address leaves no history entry.
    const area = location.hash.replace(/^#\/?/, '').split('/')[0];
    if (Object.hasOwn(aliases, area)) history.replaceState(history.state, '', hrefFor(router.current.id, router.current.detail));
  };
  sync();
  window.addEventListener('hashchange', sync);
  return () => window.removeEventListener('hashchange', sync);
}

export function navigate(id: RouteId): void {
  location.hash = hrefFor(id);
}
