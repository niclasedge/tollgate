// Cost curve: one colored line per account in one plot, not stacked (#128).
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import CostChart from './lib/components/CostChart.svelte';
import { seriesColor } from './lib/costs';

const data = {
  buckets: ['2026-10-05T10:00:00Z', '2026-10-05T11:00:00Z', '2026-10-05T12:00:00Z'],
  series: [
    { key: 'a', values: [1_000_000_000, 4_000_000_000, 0] },
    { key: 'b', values: [1_000_000_000, 2_000_000_000, 4_000_000_000] },
  ],
};

// y coordinates of a line path "Mx,yLx,y…".
const ys = (g: Element) => [...(g.querySelector('path')?.getAttribute('d') ?? '').matchAll(/,(-?\d+(?:\.\d+)?)/g)].map((m) => Number(m[1]));

describe('CostChart lines', () => {
  it('draws one line per account in its own color', () => {
    render(CostChart, { data, labels: ['Hauptabo', 'Zweitabo'], label: 'Kosten' });
    const lines = screen.getAllByTestId('cost-line');
    expect(lines).toHaveLength(2);
    expect(lines[0]).toHaveStyle({ color: seriesColor(0) });
    expect(lines[1]).toHaveStyle({ color: seriesColor(1) });
  });

  it('overlays the lines on one scale instead of stacking them', () => {
    render(CostChart, { data, labels: ['Hauptabo', 'Zweitabo'], label: 'Kosten' });
    const [a, b] = screen.getAllByTestId('cost-line').map(ys);
    expect(a[0]).toBe(b[0]); // same value, same height
    expect(b[2]).toBe(a[1]); // $4 is $4 for both accounts
  });

  it('uses fixed colors for five accounts and gray after that', () => {
    expect(new Set([0, 1, 2, 3, 4].map(seriesColor)).size).toBe(5);
    expect(seriesColor(5)).toBe('var(--c-muted)');
  });
});
