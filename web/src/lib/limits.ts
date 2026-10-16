// Time position inside a subscription limit window that ends at its reset.

export const FIVE_HOURS_MS = 5 * 3_600_000;
export const WEEK_MS = 7 * 86_400_000;

/**
 * Elapsed share of the window ending at reset, in percent 0..100:
 * (now - (reset - window)) / window. Null if reset is unknown or invalid,
 * already passed, or further away than the window length.
 */
export function elapsedPercent(reset: string | null | undefined, windowMs: number, now: number = Date.now()): number | null {
  const end = reset ? Date.parse(reset) : NaN;
  if (!Number.isFinite(end) || end <= now || end - now > windowMs) return null;
  return (100 * (now - (end - windowMs))) / windowMs;
}
