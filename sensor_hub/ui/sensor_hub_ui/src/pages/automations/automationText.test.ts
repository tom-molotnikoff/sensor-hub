import { describe, expect, it } from 'vitest';
import { describeAutomation, describeDays, formatHubTime } from './automationText';

describe('describeDays', () => {
  it.each([
    [['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'], 'every day'],
    [['fri', 'mon', 'tue', 'wed', 'thu'], 'on weekdays'],
    [['sun', 'sat'], 'at weekends'],
    [['wed', 'mon'], 'on Mon, Wed'],
  ] as const)('describes %j as %s', (days, text) => {
    expect(describeDays(days)).toBe(text);
  });
});

describe('describeAutomation', () => {
  it('joins the triggers with or and the steps in order', () => {
    const text = describeAutomation(
      {
        triggers: [
          { type: 'schedule', at: '19:00', days: ['mon', 'tue', 'wed', 'thu', 'fri'] },
          { type: 'schedule', at: '18:00', days: ['sat', 'sun'] },
        ],
        steps: [
          { type: 'set', sensor_id: 14, property: 'state', value: 'ON' },
          { type: 'set', sensor_id: 14, property: 'brightness', value: '150' },
        ],
      },
      () => 'hallway-lamp',
    );

    expect(text).toBe('At 19:00 on weekdays or 18:00 at weekends, set hallway-lamp state to ON, then set hallway-lamp brightness to 150.');
  });
});

describe('formatHubTime', () => {
  it('shows a UTC instant in the hub zone on both sides of the clock change', () => {
    expect(formatHubTime('2026-10-23T18:00:00Z', 'Europe/London')).toBe('Fri 23 Oct, 19:00');
    expect(formatHubTime('2026-10-26T19:00:00Z', 'Europe/London')).toBe('Mon 26 Oct, 19:00');
  });
});
