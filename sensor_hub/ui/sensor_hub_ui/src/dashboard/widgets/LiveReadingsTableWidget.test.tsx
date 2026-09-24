import { ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { theme } from '../../ui/theme';
import LiveReadingsTableWidget from './LiveReadingsTableWidget';

const { readingsMock, loadedMock } = vi.hoisted(() => ({ readingsMock: vi.fn(), loadedMock: vi.fn() }));

vi.mock('../../hooks/useCurrentReadings', () => ({
  useCurrentReadings: () => readingsMock(),
  useCurrentReadingsReady: () => true,
}));
vi.mock('../../hooks/useSensorContext', () => ({ useSensorContext: () => ({ loaded: loadedMock() }) }));
vi.mock('../WidgetUpdateContext', () => ({ useReportWidgetUpdate: () => vi.fn() }));

function reading(sensor: string, type: string, value: number | null, unit: string) {
  return { sensor_name: sensor, measurement_type: type, numeric_value: value, unit, time: '2026-09-24T10:00:00Z' };
}

function renderWidget() {
  return render(
    <ThemeProvider theme={theme}>
      <MemoryRouter>
        <LiveReadingsTableWidget id="w" isEditing={false} config={{}} />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

function stubWide() {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: true,
    media: query,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

describe('LiveReadingsTableWidget', () => {
  beforeEach(() => {
    loadedMock.mockReturnValue(true);
    readingsMock.mockReturnValue({
      office: { temperature: reading('office', 'temperature', 21.5, '°C') },
      attic: { humidity: reading('attic', 'humidity', null, '%') },
    });
  });

  afterEach(() => vi.unstubAllGlobals());

  it('lists each reading sorted by sensor on compact, with its measurement and value', () => {
    const { container } = renderWidget();

    expect(container.querySelector('.MuiDataGrid-root')).toBeNull();
    const titles = Array.from(container.querySelectorAll('[data-ui=data-table-title]'), (title) => title.textContent);
    expect(titles).toEqual(['attic', 'office']);
    expect(screen.getByText('humidity · —')).toBeInTheDocument();
    expect(screen.getByText('temperature · 21.5 °C')).toBeInTheDocument();
  });

  it('shows every column in a data grid on wide', async () => {
    stubWide();
    const { container } = renderWidget();

    expect(container.querySelector('.MuiDataGrid-root')).not.toBeNull();
    expect(await screen.findByRole('columnheader', { name: 'Time' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Measurement' })).toBeInTheDocument();
  });

  it('shows the empty state once sensors load with no readings', () => {
    readingsMock.mockReturnValue({});
    renderWidget();

    expect(screen.getByText('No live temperature data')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Go to Sensors' })).toHaveAttribute('href', '/sensors-overview');
  });

  it('shows the loader while sensors are loading', async () => {
    loadedMock.mockReturnValue(false);
    renderWidget();

    expect(await screen.findByTestId('widget-loader', {}, { timeout: 3000 })).toBeInTheDocument();
    expect(screen.queryByText('No live temperature data')).toBeNull();
  });
});
