import { describe, expect, it } from 'vitest';
import type { ChartEntry, Reading, Sensor } from '../gen/aliases';
import { mergeReadings, readingsFingerprint } from './mergeReadings';
import fixtures from './__fixtures__/mergeReadings.json';

interface Fixture {
  sensors: string[];
  readings: Partial<Reading>[];
  expected: ChartEntry[];
}

function sensorsNamed(names: string[]): Sensor[] {
  return names.map((name, index) => ({ id: index + 1, name })) as Sensor[];
}

describe('mergeReadings', () => {
  const cases = Object.entries(fixtures as Record<string, Fixture>);

  it.each(cases)('matches the recorded chart rows for %s', (_name, fixture) => {
    const rows = mergeReadings(fixture.readings as Reading[], sensorsNamed(fixture.sensors));
    expect(rows).toEqual(fixture.expected);
  });
});

describe('readingsFingerprint', () => {
  const sensors = sensorsNamed(['living-room', 'kitchen']);

  it('is stable for identical rows', () => {
    const rows: ChartEntry[] = [
      { time: '2026-09-01T10:00:00Z', 'living-room': 21.4, kitchen: 19.8 },
      { time: '2026-09-01T10:05:00Z', 'living-room': 21.6, kitchen: 19.9 },
    ];
    expect(readingsFingerprint(rows, sensors)).toBe(readingsFingerprint(structuredClone(rows), sensors));
  });

  it('changes when a new row arrives', () => {
    const rows: ChartEntry[] = [{ time: '2026-09-01T10:00:00Z', 'living-room': 21.4, kitchen: 19.8 }];
    const withNewRow = [...rows, { time: '2026-09-01T10:05:00Z', 'living-room': 21.6, kitchen: 19.9 }];
    expect(readingsFingerprint(withNewRow, sensors)).not.toBe(readingsFingerprint(rows, sensors));
  });

  it('changes when the latest value moves', () => {
    const rows: ChartEntry[] = [{ time: '2026-09-01T10:00:00Z', 'living-room': 21.4, kitchen: 19.8 }];
    const moved: ChartEntry[] = [{ time: '2026-09-01T10:00:00Z', 'living-room': 22.0, kitchen: 19.8 }];
    expect(readingsFingerprint(moved, sensors)).not.toBe(readingsFingerprint(rows, sensors));
  });

  it('changes when the sensor set changes', () => {
    const rows: ChartEntry[] = [{ time: '2026-09-01T10:00:00Z', 'living-room': 21.4, kitchen: 19.8 }];
    expect(readingsFingerprint(rows, sensorsNamed(['living-room']))).not.toBe(readingsFingerprint(rows, sensors));
  });
});
