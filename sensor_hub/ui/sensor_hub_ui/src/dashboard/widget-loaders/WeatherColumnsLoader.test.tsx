import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import WeatherColumnsLoader from './WeatherColumnsLoader';

vi.mock('../../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

describe('WeatherColumnsLoader', () => {
  it('renders skeleton day columns inside an aria-busy loader shell', () => {
    render(<WeatherColumnsLoader days={6} />);
    const loader = screen.getByTestId('widget-loader');
    expect(loader).toBeInTheDocument();
    expect(loader).toHaveAttribute('aria-busy', 'true');
    expect(screen.getAllByTestId('wx-day')).toHaveLength(6);
  });
});
