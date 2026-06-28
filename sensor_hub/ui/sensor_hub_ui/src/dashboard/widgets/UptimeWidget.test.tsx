import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor, SensorHealthHistory } from '../../gen/aliases';
import UptimeWidget from './UptimeWidget';

const { reportUpdateMock, useSensorHealthHistoryMock } = vi.hoisted(() => ({
  reportUpdateMock: vi.fn(),
  useSensorHealthHistoryMock: vi.fn(),
}));

const sensors: Sensor[] = [];
const properties: Record<string, string> = {};

vi.mock('../../hooks/useSensorContext', () => ({
  useSensorContext: () => ({
    sensors,
  }),
}));

vi.mock('../../hooks/useSensorHealthHistory', () => ({
  default: useSensorHealthHistoryMock,
}));

vi.mock('../../hooks/useProperties', () => ({
  useProperties: () => properties,
}));

vi.mock('../WidgetUpdateContext', () => ({
  useReportWidgetUpdate: () => reportUpdateMock,
}));

vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 7,
    name: 'boiler',
    external_id: 'boiler',
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
    sensor_id: '7',
    health_status: 'good',
    recorded_at: '2026-05-09T10:00:00Z',
    ...overrides,
  };
}

describe('UptimeWidget', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-09T13:00:00Z'));
    sensors.splice(0, sensors.length, makeSensor());
    Object.keys(properties).forEach((key) => delete properties[key]);
    reportUpdateMock.mockReset();
    useSensorHealthHistoryMock.mockReset();
  });

  it('shows the indeterminate-bar loader while health history is loading', () => {
    useSensorHealthHistoryMock.mockReturnValue([[], vi.fn(), true]);

    render(<UptimeWidget id="widget-1" isEditing={false} config={{ sensorId: 7 }} />);

    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByText('—')).not.toBeInTheDocument();
  });

  it('shows a time-weighted uptime percentage instead of transition-count uptime', () => {
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

    render(<UptimeWidget id="widget-1" isEditing={false} config={{ sensorId: 7 }} />);

    expect(screen.getByText('33.3%')).toBeInTheDocument();
  });

  it('uses the retained window and shows the good/bad/unknown time breakdown when available', () => {
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

    render(<UptimeWidget id="widget-1" isEditing={false} config={{ sensorId: 7 }} />);

    expect(screen.getByText('4.2%')).toBeInTheDocument();
    expect(screen.getByText('Good for 1h of last 24h')).toBeInTheDocument();
    expect(screen.getByText('Bad 2h · Unknown 21h')).toBeInTheDocument();
  });
});
