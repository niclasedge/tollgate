import { routes, navigate } from './router.svelte';
import { toggleTheme } from './theme.svelte';

function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);
}

/** Global single-key shortcuts: 1-7 switch the area, d toggles dark mode. */
export function handleKey(e: KeyboardEvent): void {
  if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey || isTyping(e.target)) return;
  const route = routes.find((r) => r.key === e.key);
  if (route) {
    e.preventDefault();
    navigate(route.id);
  } else if (e.key === 'd') {
    e.preventDefault();
    toggleTheme();
  }
}
