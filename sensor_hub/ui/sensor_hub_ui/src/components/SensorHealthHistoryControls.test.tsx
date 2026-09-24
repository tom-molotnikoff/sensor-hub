import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../gen/aliases';
import SensorHealthHistory from './SensorHealthHistory';
import SensorHealthHistoryChartCard from './SensorHealthHistoryChartCard';

const { refreshMock, useSensorHealthHistoryMock } = vi.hoisted(() => ({
  refreshMock: vi.fn(),
  useSensorHealthHistoryMock: vi.fn(),
}));

vi.mock('../hooks/useSensorHealthHistory.ts', () => ({
  default: useSensorHealthHistoryMock,
}));

vi.mock('./SensorHealthHistoryChart', () => ({
  default: () => <div data-testid="sensor-health-history-chart" />,
}));

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 1,
    name: 'garage-door',
    external_id: 'garage-door',
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

describe('sensor health history controls', () => {
  beforeEach(() => {
    refreshMock.mockReset();
    useSensorHealthHistoryMock.mockReset();
    useSensorHealthHistoryMock.mockReturnValue([[], refreshMock]);
  });

  it('does not render a limit input on the sensor health history table card', () => {
    render(<SensorHealthHistory sensor={makeSensor()} />);

    expect(screen.getByRole('button', { name: 'Refresh' })).toBeInTheDocument();
    expect(screen.queryByLabelText('Limit History Entries')).not.toBeInTheDocument();
    expect(screen.queryByText('Limit History Entries')).not.toBeInTheDocument();
  });

  it('does not render settings for history limits on the chart card', () => {
    render(<SensorHealthHistoryChartCard sensor={makeSensor()} />);

    expect(screen.getByText('Sensor Health History')).toBeInTheDocument();
    expect(screen.queryByTitle('Settings')).not.toBeInTheDocument();
    expect(screen.queryByText('Health Timeline Settings')).not.toBeInTheDocument();
  });
});
