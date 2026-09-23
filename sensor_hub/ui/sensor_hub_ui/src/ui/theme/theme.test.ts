import { describe, expect, it } from 'vitest';
import { wideMediaQuery } from '../tiers';
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
    ['pageTitle', '18px', '20px', 500],
    ['cardTitle', '18px', '20px', 600],
  ] as const)('sizes %s by tier', (variant, compact, wide, weight) => {
    expect(theme.typography[variant]).toMatchObject({
      fontSize: compact,
      fontWeight: weight,
      [wideMediaQuery]: { fontSize: wide },
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
});
