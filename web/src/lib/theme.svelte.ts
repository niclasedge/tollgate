// Light/dark theme. Without an explicit choice the system preference applies
// (pure CSS); an explicit choice is stored and set as data-theme on <html>.

const KEY = 'tollgate.theme';

export type Theme = 'light' | 'dark';

function systemPrefersDark(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches;
}

function stored(): Theme | undefined {
  try {
    const v = localStorage.getItem(KEY);
    return v === 'light' || v === 'dark' ? v : undefined;
  } catch {
    return undefined;
  }
}

export const theme = $state<{ value: Theme }>({ value: stored() ?? (systemPrefersDark() ? 'dark' : 'light') });

export function setTheme(value: Theme): void {
  theme.value = value;
  document.documentElement.dataset.theme = value;
  try {
    localStorage.setItem(KEY, value);
  } catch {
    // storage unavailable: the choice then only lasts for this page view
  }
}

export function toggleTheme(): void {
  setTheme(theme.value === 'dark' ? 'light' : 'dark');
}
