import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import SensorDetailTilesLoader from './SensorDetailTilesLoader';

vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

describe('SensorDetailTilesLoader', () => {
  it('renders cascading skeleton tiles inside an aria-busy shell', () => {
    render(<SensorDetailTilesLoader tiles={6} />);
    const loader = screen.getByTestId('widget-loader');
    expect(loader).toBeInTheDocument();
    expect(loader).toHaveAttribute('aria-busy', 'true');
    expect(screen.getAllByTestId('detail-tile')).toHaveLength(6);
  });
});
