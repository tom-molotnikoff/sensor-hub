import { describe, expect, it } from 'vitest';
import { formatHubTime } from './automationText';

describe('formatHubTime', () => {
  it('shows a UTC instant in the hub zone on both sides of the clock change', () => {
    expect(formatHubTime('2026-10-23T18:00:00Z', 'Europe/London')).toBe('Fri 23 Oct, 19:00');
    expect(formatHubTime('2026-10-26T19:00:00Z', 'Europe/London')).toBe('Mon 26 Oct, 19:00');
  });
});
