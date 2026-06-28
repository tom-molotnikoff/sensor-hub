import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../../gen/aliases';
import GaugeWidget from './GaugeWidget';

const { readingsMock, readyMock, reportUpdateMock } = vi.hoisted(() => ({
  readingsMock: vi.fn(),
  readyMock: vi.fn(),
  reportUpdateMock: vi.fn(),
}));

const sensors: Sensor[] = [];

vi.mock('../../hooks/useSensorContext', () => ({ useSensorContext: () => ({ sensors }) }));
vi.mock('../../hooks/useCurrentReadings', () => ({
  useCurrentReadings: () => readingsMock(),
  useCurrentReadingsReady: () => readyMock(),
}));
vi.mock('../WidgetUpdateContext', () => ({ useReportWidgetUpdate: () => reportUpdateMock }));
vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 7, name: 'boiler', external_id: 'b', sensor_driver: 'z', config: {}, metadata: {},
    health_status: 'good', health_reason: 'ok', enabled: true, status: 'active', retention_hours: null, ...overrides,
  };
}

const config = { sensorId: 7, measurementType: 'temperature' };

describe('GaugeWidget loading states', () => {
  beforeEach(() => {
    sensors.splice(0, sensors.length, makeSensor());
    readingsMock.mockReset();
    readyMock.mockReset();
  });

  it('shows the circular-draw loader while loading (not the empty dash)', () => {
    readingsMock.mockReturnValue({});
    readyMock.mockReturnValue(false);
    render(<GaugeWidget id="w" isEditing={false} config={config} />);
    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByText('—')).not.toBeInTheDocument();
  });

  it('shows the gauge value once data has arrived', () => {
    readingsMock.mockReturnValue({ boiler: { temperature: { numeric_value: 21.4, unit: '°C', time: '2026-05-09T10:00:00Z' } } });
    readyMock.mockReturnValue(true);
    render(<GaugeWidget id="w" isEditing={false} config={config} />);
    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.getByText(/21\.4/)).toBeInTheDocument();
  });

  it('shows the empty dash once loaded with no reading', () => {
    readingsMock.mockReturnValue({});
    readyMock.mockReturnValue(true);
    render(<GaugeWidget id="w" isEditing={false} config={config} />);
    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.getByText('—')).toBeInTheDocument();
  });
});
