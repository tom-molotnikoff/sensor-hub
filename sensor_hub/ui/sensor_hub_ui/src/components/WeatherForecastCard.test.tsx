import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import WeatherForecastCard from './WeatherForecastCard';

const { weatherMock } = vi.hoisted(() => ({ weatherMock: vi.fn() }));

const properties: Record<string, string> = {};

vi.mock('../hooks/useProperties', () => ({ useProperties: () => properties }));
vi.mock('../hooks/useWeatherApi', () => ({ useWeatherApi: () => weatherMock() }));
vi.mock('../hooks/useMobile', () => ({ useIsMobile: () => false }));
vi.mock('./DayForecastCard', () => ({ default: () => <div data-testid="day-card" /> }));
vi.mock('./HourlyForecastDetail', () => ({ default: () => <div data-testid="hourly" /> }));
vi.mock('../theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

describe('WeatherForecastCard loading state', () => {
  beforeEach(() => {
    weatherMock.mockReset();
    properties['weather.latitude'] = '51.5';
    properties['weather.longitude'] = '-0.1';
    properties['weather.location.name'] = 'Home';
  });

  it('shows the cascading weather loader (not a spinner/text) while loading', () => {
    weatherMock.mockReturnValue({ data: null, loading: true, error: null });
    render(<WeatherForecastCard />);
    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.getAllByTestId('wx-day').length).toBe(6);
    expect(screen.queryByText('Loading forecast…')).not.toBeInTheDocument();
  });
});
