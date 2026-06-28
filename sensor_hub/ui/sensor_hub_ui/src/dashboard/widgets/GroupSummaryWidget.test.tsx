import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import GroupSummaryWidget from './GroupSummaryWidget';

const { readingsMock, readyMock, reportUpdateMock } = vi.hoisted(() => ({
  readingsMock: vi.fn(),
  readyMock: vi.fn(),
  reportUpdateMock: vi.fn(),
}));

vi.mock('../../hooks/useCurrentReadings', () => ({
  useCurrentReadings: () => readingsMock(),
  useCurrentReadingsReady: () => readyMock(),
}));
vi.mock('../WidgetUpdateContext', () => ({ useReportWidgetUpdate: () => reportUpdateMock }));
vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

const config = { measurementType: 'temperature' };

describe('GroupSummaryWidget loading state', () => {
  beforeEach(() => {
    readingsMock.mockReset();
    readyMock.mockReset();
  });

  it('shows the loader while readings are still loading', () => {
    readingsMock.mockReturnValue({});
    readyMock.mockReturnValue(false);
    render(<GroupSummaryWidget id="w" isEditing={false} config={config} />);
    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByText('No temperature readings available')).not.toBeInTheDocument();
  });

  it('shows the empty message once loaded with no readings', () => {
    readingsMock.mockReturnValue({});
    readyMock.mockReturnValue(true);
    render(<GroupSummaryWidget id="w" isEditing={false} config={config} />);
    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.getByText('No temperature readings available')).toBeInTheDocument();
  });

  it('shows the group average once data arrives', () => {
    readingsMock.mockReturnValue({ 'living-room': { temperature: { numeric_value: 20, unit: '°C', time: 't' } } });
    readyMock.mockReturnValue(true);
    render(<GroupSummaryWidget id="w" isEditing={false} config={config} />);
    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.getByText('Group Average')).toBeInTheDocument();
    expect(screen.getAllByText(/20\.0/).length).toBeGreaterThan(0);
  });
});
