import { render, screen, waitForElementToBeRemoved } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../gen/aliases';
import SensorDetailCard from './SensorDetailCard';

const { getMock, readingsMock } = vi.hoisted(() => ({ getMock: vi.fn(), readingsMock: vi.fn() }));

vi.mock('../gen/client', () => ({ apiClient: { GET: getMock } }));
vi.mock('../hooks/useCurrentReadings', () => ({ useCurrentReadings: () => readingsMock() }));
vi.mock('../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

function makeSensor(): Sensor {
  return {
    id: 7, name: 'greenhouse', external_id: 'g', sensor_driver: 'z', config: {}, metadata: {},
    health_status: 'good', health_reason: 'ok', enabled: true, status: 'active', retention_hours: null,
  };
}

describe('SensorDetailCard loading state', () => {
  beforeEach(() => {
    getMock.mockReset();
    readingsMock.mockReset();
    readingsMock.mockReturnValue({});
  });

  it('shows cascading tiles while measurement types are loading', () => {
    getMock.mockReturnValue(new Promise(() => {}));
    render(<SensorDetailCard sensor={makeSensor()} />);
    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.getAllByTestId('detail-tile').length).toBe(6);
  });

  it('shows the detail grid once measurement types load', async () => {
    getMock.mockResolvedValue({ data: [{ name: 'temperature', display_name: 'Temperature', unit: '°C' }] });
    render(<SensorDetailCard sensor={makeSensor()} />);
    await waitForElementToBeRemoved(() => screen.queryByTestId('widget-loader'));
    expect(screen.getByText('Temperature')).toBeInTheDocument();
  });
});
