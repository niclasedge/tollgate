import { describe, expect, it } from 'vitest';
import shell from './lib/components/Shell.svelte?raw';
import home from './lib/views/Home.svelte?raw';

// jsdom computes no layout, so these tests check the style rules in the
// sources (issue #115): full-width content, auto-fit tiles, readable prose.

// Declarations of the first top-level rule whose selector is exactly `selector`.
function rule(src: string, selector: string): string {
  const esc = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const m = new RegExp(`(?:^|[\\s}])${esc}\\s*\\{([^}]*)\\}`).exec(src);
  if (!m) throw new Error(`rule ${selector} not found`);
  return m[1];
}

describe('layout (issue #115)', () => {
  it('content area has no fixed max-width', () => {
    expect(rule(shell, 'main')).not.toMatch(/max-width/);
  });

  it('subscription tiles fit three in a row at 1440px', () => {
    const decl = rule(home, '.subscriptions');
    expect(decl).toMatch(/repeat\(\s*auto-fit/);
    const m = /minmax\(\s*min\(\s*100%\s*,\s*([\d.]+)rem\s*\)/.exec(decl);
    expect(m).not.toBeNull();
    // 1440px − wide sidebar (13.5rem) − padding (2 × 4rem) = 68.5rem; gaps 2 × 0.9rem.
    const min = Number(m![1]);
    expect(3 * min + 2 * 0.9).toBeLessThanOrEqual(68.5);
  });

  it('prose paragraphs stay readable', () => {
    expect(rule(shell, 'main :global(p)')).toMatch(/max-width:\s*75ch/);
  });
});
