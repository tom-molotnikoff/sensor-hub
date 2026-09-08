import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../../gen/aliases';
import { apiClient } from '../../gen/client';
import MinMaxAvgWidget from './MinMaxAvgWidget';

const { scheduleMock, reportUpdateMock } = vi.hoisted(() => ({
  scheduleMock: vi.fn(),
  reportUpdateMock: vi.fn(),
}));

const sensors: Sensor[] = [];

vi.mock('../../hooks/useSensorContext', () => ({ useSensorContext: () => ({ sensors }) }));
vi.mock('../WidgetUpdateContext', () => ({ useReportWidgetUpdate: () => reportUpdateMock }));
vi.mock('../../scheduler/requestScheduler', () => ({ requestScheduler: { schedule: scheduleMock } }));
vi.mock('../../gen/client', () => ({ apiClient: { GET: vi.fn() } }));
vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['#0288D1', '#5C5C5C', '#C62828'], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

function makeSensor(): Sensor {
  return {
    id: 7, name: 'fridge', external_id: 'f', sensor_driver: 'z', config: {}, metadata: {},
    health_status: 'good', health_reason: 'ok', enabled: true, status: 'active', retention_hours: null,
  };
}

const config = { sensorId: 7, measurementType: 'temperature' };

describe('MinMaxAvgWidget loading state', () => {
  beforeEach(() => {
    sensors.splice(0, sensors.length, makeSensor());
    scheduleMock.mockReset();
    reportUpdateMock.mockReset();
    vi.mocked(apiClient.GET).mockReset();
  });

  it('shows skeleton tiles (not "No data available") while loading', async () => {
    scheduleMock.mockReturnValue(new Promise(() => {}));
    render(<MinMaxAvgWidget id="w" isEditing={false} config={config} />);
    expect(await screen.findByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.getAllByTestId('stat-tile')).toHaveLength(3);
    expect(screen.queryByText('No data available')).not.toBeInTheDocument();
  });

  it('shows the empty state once loaded with no readings', async () => {
    vi.mocked(apiClient.GET).mockResolvedValue({ data: { readings: [] } });
    scheduleMock.mockImplementation((_priority: string, fetcher: () => Promise<unknown>) => fetcher());
    render(<MinMaxAvgWidget id="w" isEditing={false} config={config} />);
    expect(await screen.findByText('No data available', {}, { timeout: 3000 })).toBeInTheDocument();
  });
});

describe('MinMaxAvgWidget request', () => {
  beforeEach(() => {
    sensors.splice(0, sensors.length, makeSensor());
    scheduleMock.mockReset();
    reportUpdateMock.mockReset();
    vi.mocked(apiClient.GET).mockReset();
  });

  it('asks for its own sensor and measurement type only', async () => {
    vi.mocked(apiClient.GET).mockResolvedValue({ data: { readings: [] } });
    scheduleMock.mockImplementation((_priority: string, fetcher: () => Promise<unknown>) => fetcher());

    render(<MinMaxAvgWidget id="w" isEditing={false} config={config} />);

    await waitFor(() => expect(apiClient.GET).toHaveBeenCalledWith('/readings/between', expect.objectContaining({
      params: { query: expect.objectContaining({ type: 'temperature', sensor: 'fridge' }) },
    })));
  });
});
