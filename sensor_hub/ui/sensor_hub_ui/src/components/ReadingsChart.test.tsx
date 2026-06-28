import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../gen/aliases';
import ReadingsChart from './ReadingsChart';

const { readingsDataMock } = vi.hoisted(() => ({ readingsDataMock: vi.fn() }));

vi.mock('../hooks/useReadingsData', () => ({
  useReadingsData: () => readingsDataMock(),
}));

vi.mock('../theme/chartColours', () => ({
  useChartColours: () => ({
    categorical: ['#D4451A', '#0288D1'],
    health: ['', '', ''],
    stat: ['', '', ''],
    grid: '#D9D0C7',
    axisText: '#5C5C5C',
    noData: '#E0D8D0',
  }),
}));

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 1,
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

function renderChart() {
  return render(
    <MemoryRouter>
      <ReadingsChart sensors={[makeSensor()]} startDate={null} endDate={null} measurementType="temperature" />
    </MemoryRouter>,
  );
}

describe('ReadingsChart loading states', () => {
  beforeEach(() => {
    readingsDataMock.mockReset();
  });

  it('shows the signal-trace loader while the first fetch is in flight', () => {
    readingsDataMock.mockReturnValue({ mergedData: [], aggregation: { interval: 'raw', function: 'none' }, isLoading: true, error: null });

    renderChart();

    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByText('No readings in selected date range')).not.toBeInTheDocument();
  });

  it('shows the chart (no loader, no empty state) once data has arrived', () => {
    readingsDataMock.mockReturnValue({
      mergedData: [{ time: '2026-05-09T10:00:00Z', 'living-room': 21 }],
      aggregation: { interval: 'raw', function: 'none' },
      isLoading: false,
      error: null,
    });

    renderChart();

    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.queryByText('No readings in selected date range')).not.toBeInTheDocument();
  });

  it('shows the empty state once loaded with no readings', () => {
    readingsDataMock.mockReturnValue({ mergedData: [], aggregation: { interval: 'raw', function: 'none' }, isLoading: false, error: null });

    renderChart();

    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.getByText('No readings in selected date range')).toBeInTheDocument();
  });

  it('shows an error empty state when the fetch failed with no data', () => {
    readingsDataMock.mockReturnValue({ mergedData: [], aggregation: { interval: 'raw', function: 'none' }, isLoading: false, error: 'boom' });

    renderChart();

    expect(screen.getByText("Couldn't load readings")).toBeInTheDocument();
  });
});
