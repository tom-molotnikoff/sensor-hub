import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import RippleHeatmapLoader from './RippleHeatmapLoader';

vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

describe('RippleHeatmapLoader', () => {
  it('renders the loader shell with a 30-cell grid matching the real heatmap', () => {
    render(<RippleHeatmapLoader />);
    const loader = screen.getByTestId('widget-loader');
    expect(loader).toBeInTheDocument();
    expect(loader).toHaveAttribute('aria-busy', 'true');
    expect(screen.getAllByTestId('heatmap-cell')).toHaveLength(30);
  });
});
