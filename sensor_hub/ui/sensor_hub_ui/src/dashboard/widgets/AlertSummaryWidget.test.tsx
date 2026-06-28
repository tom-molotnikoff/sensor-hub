import { render, screen, waitForElementToBeRemoved } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AlertSummaryWidget from './AlertSummaryWidget';

const { scheduleMock, reportUpdateMock } = vi.hoisted(() => ({
  scheduleMock: vi.fn(),
  reportUpdateMock: vi.fn(),
}));

vi.mock('../WidgetUpdateContext', () => ({ useReportWidgetUpdate: () => reportUpdateMock }));
vi.mock('../../scheduler/requestScheduler', () => ({ requestScheduler: { schedule: scheduleMock } }));
vi.mock('../../gen/client', () => ({ apiClient: { GET: vi.fn() } }));
vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

const props = { id: 'w', isEditing: false, config: {} };

describe('AlertSummaryWidget loading state', () => {
  beforeEach(() => {
    scheduleMock.mockReset();
    reportUpdateMock.mockReset();
  });

  it('shows the cascade loader (not a "Loading…" label) while fetching', () => {
    scheduleMock.mockReturnValue(new Promise(() => {}));
    render(<AlertSummaryWidget {...props} />);
    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByText('Loading…')).not.toBeInTheDocument();
  });

  it('shows the empty state once loaded with no rules', async () => {
    scheduleMock.mockResolvedValue({ data: [] });
    render(<AlertSummaryWidget {...props} />);
    await waitForElementToBeRemoved(() => screen.queryByTestId('widget-loader'));
    expect(screen.getByText('No alert rules configured')).toBeInTheDocument();
  });
});
