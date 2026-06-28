import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../gen/aliases';
import SensorHealthCard from './SensorHealthCard';

const { contextMock } = vi.hoisted(() => ({ contextMock: vi.fn() }));

vi.mock('../hooks/useSensorContext', () => ({ useSensorContext: () => contextMock() }));
vi.mock('./SensorHealthPieChart', () => ({ default: () => <div data-testid="health-pie" /> }));
vi.mock('../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

function makeSensor(): Sensor {
  return {
    id: 1, name: 'living-room', external_id: 'lr', sensor_driver: 'z', config: {}, metadata: {},
    health_status: 'good', health_reason: 'ok', enabled: true, status: 'active', retention_hours: null,
  };
}

describe('SensorHealthCard loading state', () => {
  beforeEach(() => contextMock.mockReset());

  it('shows the circular-draw loader while sensors are still loading', () => {
    contextMock.mockReturnValue({ sensors: [], loaded: false });
    render(<SensorHealthCard />);
    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByTestId('health-pie')).not.toBeInTheDocument();
  });

  it('shows the pie chart once loaded with sensors', () => {
    contextMock.mockReturnValue({ sensors: [makeSensor()], loaded: true });
    render(<SensorHealthCard />);
    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.getByTestId('health-pie')).toBeInTheDocument();
  });
});
