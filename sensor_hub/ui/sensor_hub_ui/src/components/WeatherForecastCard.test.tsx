import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import WeatherForecastCard from './WeatherForecastCard';

const { weatherMock } = vi.hoisted(() => ({ weatherMock: vi.fn() }));

const properties: Record<string, string> = {};

vi.mock('../hooks/useProperties', () => ({ useProperties: () => properties }));
vi.mock('../hooks/useWeatherApi', () => ({ useWeatherApi: () => weatherMock() }));
vi.mock('../ui/theme/chartColours', () => ({
  useChartColours: () => ({ categorical: ['#D4451A'], health: ['', '', ''], stat: ['', '', ''], grid: '#000', axisText: '#000', noData: '#E0D8D0' }),
}));

const today = new Date().toISOString().slice(0, 10);

const forecast = {
  daily: [
    { date: today, weatherCode: 0, tempMax: 21.4, tempMin: 11.6, precipitationProbability: 10, windSpeedMax: 14.2 },
    { date: '2099-01-02', weatherCode: 3, tempMax: 18, tempMin: 9, precipitationProbability: 60, windSpeedMax: 22 },
  ],
  hourly: [
    { time: `${today}T09:00`, temperature: 15.2, apparentTemperature: 13.8, precipitationProbability: 5, weatherCode: 0, windSpeed: 8 },
    { time: `${today}T10:00`, temperature: 16.7, apparentTemperature: 15.1, precipitationProbability: 20, weatherCode: 2, windSpeed: 11 },
  ],
};

describe('WeatherForecastCard', () => {
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

  it('asks for a location when none is configured', () => {
    delete properties['weather.latitude'];
    weatherMock.mockReturnValue({ data: null, loading: false, error: null });
    render(
      <MemoryRouter>
        <WeatherForecastCard />
      </MemoryRouter>,
    );
    expect(screen.getByText('Location not configured')).toBeInTheDocument();
  });

  it('lists each day in its own scrollable region and marks today', () => {
    weatherMock.mockReturnValue({ data: forecast, loading: false, error: null });
    render(<WeatherForecastCard />);

    const days = screen.getByRole('region', { name: 'Daily forecast' });
    const cells = days.querySelectorAll('[data-ui=strip-cell]');
    expect(cells).toHaveLength(2);
    expect(cells[0]).toHaveAttribute('aria-current', 'true');
    expect(cells[1]).not.toHaveAttribute('aria-current');
    expect(within(days).getByText('Today')).toBeInTheDocument();
    expect(within(days).getByText('21° / 12°')).toBeInTheDocument();
    expect(within(days).getByText('14 km/h', { exact: false })).toBeInTheDocument();
  });

  it('toggles the hourly forecast', () => {
    weatherMock.mockReturnValue({ data: forecast, loading: false, error: null });
    render(<WeatherForecastCard />);

    const hourly = screen.getByRole('region', { name: 'Hourly forecast' });
    expect(within(hourly).getByText('17°')).toBeInTheDocument();
    expect(within(hourly).getByText('Feels 15°')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Hide hourly detail' }));
    expect(screen.queryByRole('region', { name: 'Hourly forecast' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: "Show today's hourly forecast" }));
    expect(screen.getByRole('region', { name: 'Hourly forecast' })).toBeInTheDocument();
  });

  it('warns when the forecast fails to load', () => {
    weatherMock.mockReturnValue({ data: null, loading: false, error: 'timeout' });
    render(<WeatherForecastCard />);
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load weather data: timeout');
  });
});
