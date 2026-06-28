import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import CascadeRowsLoader from './CascadeRowsLoader';

vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

describe('CascadeRowsLoader', () => {
  it('renders skeleton rows inside an aria-busy loader shell', () => {
    render(<CascadeRowsLoader rows={4} />);
    const loader = screen.getByTestId('widget-loader');
    expect(loader).toBeInTheDocument();
    expect(loader).toHaveAttribute('aria-busy', 'true');
    expect(screen.getAllByTestId('cascade-row')).toHaveLength(4);
  });
});
