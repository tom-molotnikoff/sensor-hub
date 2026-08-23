import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor, SensorHealthHistory } from '../gen/aliases';
import SensorHealthHistoryChart from './SensorHealthHistoryChart';

const { useSensorHealthHistoryMock } = vi.hoisted(() => ({
  useSensorHealthHistoryMock: vi.fn(),
}));

const properties: Record<string, string> = {};

vi.mock('../hooks/useSensorHealthHistory.ts', () => ({
  default: useSensorHealthHistoryMock,
}));

vi.mock('../hooks/useProperties.ts', () => ({
  useProperties: () => properties,
}));

vi.mock('../hooks/useMobile', () => ({
  useIsMobile: () => false,
}));

vi.mock('../theme/chartColours', () => ({
  useChartColours: () => ({
    categorical: ['#D4451A'],
    grid: '#ccc',
    health: ['#0f0', '#f00', '#999'],
    stat: ['', '', ''],
    axisText: '#000',
    noData: '#E0D8D0',
  }),
}));

vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AreaChart: ({ data, children }: { data: Array<{ recorded_at: string }>; children: React.ReactNode }) => (
    <div
      data-testid="area-chart"
      data-point-count={String(data.length)}
      data-last-recorded-at={data[data.length - 1]?.recorded_at ?? ''}
    >
      {children}
    </div>
  ),
  CartesianGrid: () => null,
  Legend: () => null,
  Line: () => null,
  Tooltip: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Area: () => null,
  ReferenceArea: () => null,
}));

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 1,
    name: 'hallway',
    external_id: 'hallway',
    sensor_driver: 'zigbee2mqtt',
    config: {},
    metadata: {},
    health_status: 'good',
    health_reason: 'ok',
    enabled: true,
    status: 'active',
    retention_hours: null,
    ...overrides,
  };
}

function makeHistory(overrides: Partial<SensorHealthHistory> = {}): SensorHealthHistory {
  return {
    id: 1,
    sensor_id: '1',
    health_status: 'good',
    recorded_at: '2026-05-09T10:00:00Z',
    ...overrides,
  };
}

describe('SensorHealthHistoryChart', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-09T12:00:00Z'));
    Object.keys(properties).forEach((key) => delete properties[key]);
    useSensorHealthHistoryMock.mockReset();
  });

  it('shows the signal-trace loader while history is loading with no data yet', () => {
    useSensorHealthHistoryMock.mockReturnValue([[], vi.fn(), true]);

    render(<SensorHealthHistoryChart sensor={makeSensor()} />);

    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByTestId('area-chart')).not.toBeInTheDocument();
  });

  it('extends the latest health state to now before rendering the step chart', () => {
    useSensorHealthHistoryMock.mockReturnValue([[makeHistory()], vi.fn()]);

    render(<SensorHealthHistoryChart sensor={makeSensor()} />);

    expect(screen.getByTestId('area-chart')).toHaveAttribute('data-point-count', '2');
    expect(screen.getByTestId('area-chart')).toHaveAttribute('data-last-recorded-at', '2026-05-09T12:00:00.000Z');
  });

  it('shows retained-window context and duration summary above the chart', () => {
    properties['health.history.retention.days'] = '1';
    useSensorHealthHistoryMock.mockReturnValue([[
      makeHistory({
        recorded_at: '2026-05-09T10:00:00Z',
        health_status: 'good',
      }),
      makeHistory({
        id: 2,
        recorded_at: '2026-05-09T11:00:00Z',
        health_status: 'bad',
      }),
    ], vi.fn()]);

    render(<SensorHealthHistoryChart sensor={makeSensor()} />);

    expect(screen.getByText('Window 24h')).toBeInTheDocument();
    expect(screen.getByText('Current bad')).toBeInTheDocument();
    expect(screen.getByText('Last change 1h ago')).toBeInTheDocument();
    expect(screen.getByText('Good 1h · Bad 1h · Unknown 22h')).toBeInTheDocument();
  });
});
