import { describe, expect, it } from 'vitest';
import { wideMediaQuery } from '../tiers';
import type { Palette } from '@mui/material/styles';
import { theme } from '.';

describe('theme', () => {
  it('defines the density tokens', () => {
    expect(theme.density).toEqual({
      page: { compact: 12, wide: 24 },
      card: { compact: 12, wide: 20 },
      gap: { compact: 12, wide: 16 },
    });
  });

  it.each([
    ['pageTitle', ['18px', 500], ['24px', 600]],
    ['cardTitle', ['18px', 600], ['20px', 600]],
  ] as const)('sizes %s by tier', (variant, [compactSize, compactWeight], [wideSize, wideWeight]) => {
    expect(theme.typography[variant]).toMatchObject({
      fontSize: compactSize,
      fontWeight: compactWeight,
      [wideMediaQuery]: { fontSize: wideSize, fontWeight: wideWeight },
    });
  });

  it.each([
    ['sectionTitle', '15px', 600],
    ['body', '15px', 400],
    ['bodySmall', '13px', 400],
    ['caption', '12px', 400],
  ] as const)('sizes %s the same at every tier', (variant, size, weight) => {
    expect(theme.typography[variant]).toMatchObject({ fontSize: size, fontWeight: weight });
    expect(theme.typography[variant]).not.toHaveProperty(wideMediaQuery);
  });

  it.each([
    ['metricSm', '24px'],
    ['metricMd', '36px'],
    ['metricLg', '56px'],
  ] as const)('sizes %s with tabular numbers', (variant, size) => {
    expect(theme.typography[variant]).toMatchObject({
      fontSize: size,
      fontWeight: 700,
      fontVariantNumeric: 'tabular-nums',
    });
  });

  it.each(['light', 'dark'] as const)('gives every status a strong and a soft colour in %s', (scheme) => {
    const { palette } = (theme as unknown as { colorSchemes: Record<string, { palette: Palette }> }).colorSchemes[scheme];
    expect(Object.keys(palette.status).sort()).toEqual(['bad', 'info', 'ok', 'unknown', 'warn']);
    for (const colour of Object.values(palette.status)) {
      expect(colour).toEqual({ strong: expect.any(String), soft: expect.any(String) });
    }
    expect(palette.chart.categorical).toHaveLength(8);
  });

  it.each([
    ['light', '#211E1B'],
    ['dark', '#121212'],
  ] as const)('gives the nav its charcoal colours in %s', (scheme, bg) => {
    const { palette } = (theme as unknown as { colorSchemes: Record<string, { palette: Palette }> }).colorSchemes[scheme];
    expect(palette.nav).toEqual({
      bg,
      text: '#D9D3CC',
      muted: '#8F867D',
      hover: 'rgba(255,255,255,0.06)',
      activeBg: 'rgba(237,81,37,0.18)',
      activeText: '#FFFFFF',
      indicator: '#ED5125',
    });
  });
});
