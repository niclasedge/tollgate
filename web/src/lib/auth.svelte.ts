// Admin token: kept in localStorage (so it survives on the iPhone), sent as
// Bearer on every API request. A 401 discards it and shows the prompt again.

const KEY = 'tollgate.admin_token';

function read(): string | undefined {
  try {
    return localStorage.getItem(KEY) || undefined;
  } catch {
    return undefined;
  }
}

export const auth = $state<{ token: string | undefined; rejected: boolean }>({ token: read(), rejected: false });

export function login(token: string): void {
  const t = token.trim();
  if (!t) return;
  auth.token = t;
  auth.rejected = false;
  try {
    localStorage.setItem(KEY, t);
  } catch {
    // storage unavailable: the token then only lasts for this page view
  }
}

/** Discards the token. `rejected` marks a 401 so the prompt can say so. */
export function logout(rejected = false): void {
  auth.token = undefined;
  auth.rejected = rejected;
  try {
    localStorage.removeItem(KEY);
  } catch {
    // ignore
  }
}
