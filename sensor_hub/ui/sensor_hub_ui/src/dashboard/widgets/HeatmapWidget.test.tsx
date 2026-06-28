import { render, screen, waitForElementToBeRemoved } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../../gen/aliases';
import HeatmapWidget from './HeatmapWidget';

const { scheduleMock, reportUpdateMock } = vi.hoisted(() => ({
  scheduleMock: vi.fn(),
  reportUpdateMock: vi.fn(),
}));

const sensors: Sensor[] = [];

vi.mock('../../hooks/useSensorContext', () => ({ useSensorContext: () => ({ sensors }) }));
vi.mock('../WidgetUpdateContext', () => ({ useReportWidgetUpdate: () => reportUpdateMock }));
vi.mock('../../theme/useIsDark', () => ({ useIsDark: () => false }));
vi.mock('../../scheduler/requestScheduler', () => ({ requestScheduler: { schedule: scheduleMock } }));
vi.mock('../../gen/client', () => ({ apiClient: { GET: vi.fn() } }));
vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

function makeSensor(): Sensor {
  return {
    id: 7, name: 'greenhouse', external_id: 'g', sensor_driver: 'z', config: {}, metadata: {},
    health_status: 'good', health_reason: 'ok', enabled: true, status: 'active', retention_hours: null,
  };
}

const config = { sensorId: 7, measurementType: 'temperature' };

describe('HeatmapWidget loading state', () => {
  beforeEach(() => {
    sensors.splice(0, sensors.length, makeSensor());
    scheduleMock.mockReset();
    reportUpdateMock.mockReset();
  });

  it('shows the ripple loader while the fetch is in flight', () => {
    scheduleMock.mockReturnValue(new Promise(() => {})); // never resolves
    render(<HeatmapWidget id="w" isEditing={false} config={config} />);
    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.getAllByTestId('heatmap-cell').length).toBe(30);
  });

  it('replaces the loader with the day grid once data has loaded', async () => {
    scheduleMock.mockResolvedValue({ data: { readings: [] } });
    render(<HeatmapWidget id="w" isEditing={false} config={config} />);
    await waitForElementToBeRemoved(() => screen.queryByTestId('widget-loader'));
    // Loader gone; the real grid (no animated loader cells) is shown.
    expect(screen.queryByTestId('heatmap-cell')).not.toBeInTheDocument();
  });
});
