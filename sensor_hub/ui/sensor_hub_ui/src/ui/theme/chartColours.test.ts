import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { heatColour, useChartColours } from './chartColours';
import { chartPalettes, statusPalettes } from './palette';

const scheme = vi.hoisted(() => ({ dark: false }));

vi.mock('./useIsDark', () => ({ useIsDark: () => scheme.dark }));

describe('useChartColours', () => {
  it.each([
    ['light', false],
    ['dark', true],
  ] as const)('derives health and categorical colours from the %s palette', (name, dark) => {
    scheme.dark = dark;
    const { result } = renderHook(() => useChartColours());

    const status = statusPalettes[name];
    expect(result.current.health).toEqual([status.ok.strong, status.bad.strong, status.unknown.strong]);
    expect(result.current.categorical).toBe(chartPalettes[name].categorical);
  });
});

describe('heatColour', () => {
  it('runs from blue at the bottom of the scale to red at the top, clamping outside it', () => {
    expect(heatColour(-1)).toBe('rgb(33,102,172)');
    expect(heatColour(0)).toBe('rgb(33,102,172)');
    expect(heatColour(0.5)).toBe('rgb(68,179,96)');
    expect(heatColour(1)).toBe('rgb(215,48,39)');
    expect(heatColour(2)).toBe('rgb(215,48,39)');
  });
});
