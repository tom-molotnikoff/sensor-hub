import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AlertSummaryWidget from './AlertSummaryWidget';

const { scheduleMock, getMock, reportUpdateMock } = vi.hoisted(() => ({
  scheduleMock: vi.fn(),
  getMock: vi.fn(),
  reportUpdateMock: vi.fn(),
}));

vi.mock('../WidgetUpdateContext', () => ({ useReportWidgetUpdate: () => reportUpdateMock }));
vi.mock('../../scheduler/requestScheduler', () => ({ requestScheduler: { schedule: scheduleMock } }));
vi.mock('../../gen/client', () => ({ apiClient: { GET: getMock } }));
vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

const props = { id: 'w', isEditing: false, config: {} };

describe('AlertSummaryWidget loading state', () => {
  beforeEach(() => {
    scheduleMock.mockReset();
    getMock.mockReset();
    reportUpdateMock.mockReset();
  });

  it('shows the cascade loader (not a "Loading…" label) while fetching', async () => {
    scheduleMock.mockReturnValue(new Promise(() => {}));
    render(<AlertSummaryWidget {...props} />);
    expect(await screen.findByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByText('Loading…')).not.toBeInTheDocument();
  });

  it('shows the empty state once loaded with no rules', async () => {
    getMock.mockResolvedValue({ data: [] });
    scheduleMock.mockImplementation((_priority: string, fetcher: () => Promise<unknown>) => fetcher());
    render(<AlertSummaryWidget {...props} />);
    expect(await screen.findByText('No alert rules configured', {}, { timeout: 3000 })).toBeInTheDocument();
  });
});
