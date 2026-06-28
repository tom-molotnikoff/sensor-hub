import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import IndeterminateBarLoader from './IndeterminateBarLoader';

vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

describe('IndeterminateBarLoader', () => {
  it('renders the sliding bar inside an aria-busy loader shell', () => {
    render(<IndeterminateBarLoader />);
    const loader = screen.getByTestId('widget-loader');
    expect(loader).toBeInTheDocument();
    expect(loader).toHaveAttribute('aria-busy', 'true');
    expect(screen.getByTestId('indeterminate-bar')).toBeInTheDocument();
  });
});
