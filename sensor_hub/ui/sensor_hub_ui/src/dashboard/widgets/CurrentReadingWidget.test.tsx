import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../../gen/aliases';
import CurrentReadingWidget from './CurrentReadingWidget';

const { readingsMock, readyMock, reportUpdateMock } = vi.hoisted(() => ({
  readingsMock: vi.fn(),
  readyMock: vi.fn(),
  reportUpdateMock: vi.fn(),
}));

const sensors: Sensor[] = [];

vi.mock('../../hooks/useSensorContext', () => ({
  useSensorContext: () => ({ sensors }),
}));

vi.mock('../../hooks/useCurrentReadings', () => ({
  useCurrentReadings: () => readingsMock(),
  useCurrentReadingsReady: () => readyMock(),
}));

vi.mock('../WidgetUpdateContext', () => ({
  useReportWidgetUpdate: () => reportUpdateMock,
}));

vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({
    categorical: [],
    health: ['', '', ''],
    stat: ['', '', ''],
    grid: '#000',
    axisText: '#000',
    noData: '#E0D8D0',
  }),
}));

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 7,
    name: 'living-room',
    external_id: 'lr',
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

const config = { sensorId: 7, measurementType: 'temperature' };

describe('CurrentReadingWidget', () => {
  beforeEach(() => {
    sensors.splice(0, sensors.length, makeSensor());
    readingsMock.mockReset();
    readyMock.mockReset();
  });

  it('shows the loader (not the empty dash) while the first snapshot is still loading', () => {
    readingsMock.mockReturnValue({});
    readyMock.mockReturnValue(false);

    render(<CurrentReadingWidget id="w" isEditing={false} config={config} />);

    const loader = screen.getByTestId('widget-loader');
    expect(loader).toBeInTheDocument();
    expect(loader).toHaveAttribute('aria-busy', 'true');
    expect(screen.queryByText('—')).not.toBeInTheDocument();
  });

  it('shows the reading once data has arrived', () => {
    readingsMock.mockReturnValue({
      'living-room': { temperature: { numeric_value: 21.4, unit: '°C', time: '2026-05-09T10:00:00Z' } },
    });
    readyMock.mockReturnValue(true);

    render(<CurrentReadingWidget id="w" isEditing={false} config={config} />);

    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.getByText(/21\.4/)).toBeInTheDocument();
  });

  it('shows the empty dash once the snapshot is in but this sensor has no reading', () => {
    readingsMock.mockReturnValue({});
    readyMock.mockReturnValue(true);

    render(<CurrentReadingWidget id="w" isEditing={false} config={config} />);

    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.getByText('—')).toBeInTheDocument();
  });
});
