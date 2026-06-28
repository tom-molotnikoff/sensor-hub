import { render, screen, waitForElementToBeRemoved } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../../gen/aliases';
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
  });

  it('shows skeleton tiles (not "No data available") while loading', () => {
    scheduleMock.mockReturnValue(new Promise(() => {}));
    render(<MinMaxAvgWidget id="w" isEditing={false} config={config} />);
    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.getAllByTestId('stat-tile')).toHaveLength(3);
    expect(screen.queryByText('No data available')).not.toBeInTheDocument();
  });

  it('shows the empty state once loaded with no readings', async () => {
    scheduleMock.mockResolvedValue({ data: { readings: [] } });
    render(<MinMaxAvgWidget id="w" isEditing={false} config={config} />);
    await waitForElementToBeRemoved(() => screen.queryByTestId('widget-loader'));
    expect(screen.getByText('No data available')).toBeInTheDocument();
  });
});
