import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Automation } from '../../gen/aliases';
import { theme } from '../../ui/theme';
import AutomationsPage from './AutomationsPage';

const api = vi.hoisted(() => ({ GET: vi.fn(), PUT: vi.fn() }));
const authState = vi.hoisted(() => ({ user: { id: 1, username: 'tom', roles: [] as string[], permissions: [] as string[] } }));

vi.mock('../../gen/client', () => ({ apiClient: api }));
vi.mock('../../providers/AuthContext', () => ({ useAuth: () => authState }));
vi.mock('../../hooks/useSensorContext', () => ({
  useSensorContext: () => ({ sensors: [{ id: 6, name: 'office-plug' }], loaded: true }),
}));
vi.mock('../../navigation/AppNav', () => ({ default: () => <nav>sidebar</nav> }));
vi.mock('../../navigation/TopAppBar', () => ({ default: ({ pageTitle }: { pageTitle: string }) => <header>{pageTitle}</header> }));

function automation(id: number, name: string, overrides: Partial<Automation>): Automation {
  return {
    id,
    name,
    enabled: true,
    triggers: [{ id, type: 'schedule', at: '23:30', days: ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] }],
    steps: [{ type: 'set', sensor_id: 6, property: 'state', value: 'OFF' }],
    status: 'armed',
    last_run_failed: false,
    next_fire_at: '2026-10-06T22:30:00Z',
    hub_timezone: 'Europe/London',
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...overrides,
  };
}

const automations = [
  automation(1, 'Office plug off at night', {}),
  automation(2, 'Pump cycle', { last_run_failed: true }),
  automation(3, 'Christmas lights', { enabled: false, status: 'off', next_fire_at: null, last_run_failed: true }),
  automation(4, 'Evening lights', { status: 'running' }),
];

function atWidth(width: number) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: Number(/\(min-width:\s*(\d+)px\)/.exec(query)?.[1] ?? 0) <= width,
    media: query,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

function renderList(width: number, permissions = ['view_automations', 'manage_automations', 'control_sensors']) {
  atWidth(width);
  authState.user.permissions = permissions;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <ThemeProvider theme={theme}>
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <AutomationsPage />
        </MemoryRouter>
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

const rowOf = async (name: string) => (await screen.findByText(name)).closest<HTMLElement>('[role=row]')!;

describe('AutomationsPage', () => {
  beforeEach(() => {
    api.GET.mockResolvedValue({ data: automations, response: new Response() });
    api.PUT.mockReset();
  });
  afterEach(() => vi.unstubAllGlobals());

  it('shows the enabled switch, then the automation, Status and Next columns in that order', async () => {
    renderList(1280);

    await screen.findByText('Office plug off at night');
    expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual(['', 'Automation', 'Status', 'Next']);
    const row = await rowOf('Office plug off at night');
    expect(row).toHaveTextContent('At 23:30 every day, set office-plug state to OFF.');
    expect(within(row).getByRole('switch', { name: 'Office plug off at night enabled' })).toBeChecked();
    expect(row).toHaveTextContent('Armed');
    expect(row).toHaveTextContent('Tue 6 Oct, 23:30');
    expect(screen.getByRole('button', { name: 'New automation' })).toBeInTheDocument();
  });

  it('flags a failed last run beside Armed, and shows Off with a dash for Next', async () => {
    renderList(1280);

    expect(await rowOf('Pump cycle')).toHaveTextContent('Armedlast run failed');
    expect(await rowOf('Office plug off at night')).not.toHaveTextContent('last run failed');
    const off = await rowOf('Christmas lights');
    expect(off).toHaveTextContent('Off');
    expect(off).not.toHaveTextContent('last run failed');
    expect(within(off).getAllByRole('gridcell').at(-1)).toHaveTextContent(/^-$/);
    expect(await rowOf('Evening lights')).toHaveTextContent('Running');
  });

  it('switches an automation on and off from its row', async () => {
    api.PUT.mockResolvedValue({ data: automation(1, 'Office plug off at night', { enabled: false, status: 'off' }), response: new Response() });
    renderList(1280);

    fireEvent.click(within(await rowOf('Office plug off at night')).getByRole('switch'));

    await waitFor(() =>
      expect(api.PUT).toHaveBeenCalledWith('/automations/{id}/enabled', { params: { path: { id: 1 } }, body: { enabled: false } }),
    );
  });

  it('gives viewers the list without enabled switches or New automation', async () => {
    renderList(1280, ['view_automations']);

    await screen.findByText('Office plug off at night');
    expect(screen.queryByRole('switch')).toBeNull();
    expect(screen.queryByRole('button', { name: 'New automation' })).toBeNull();
  });

  it('shows each automation on phones as its name, summary and status', async () => {
    renderList(390);

    const item = (await screen.findByText('Pump cycle')).closest<HTMLElement>('[data-ui=data-table-row]')!;
    expect(within(item).getByText('At 23:30 every day, set office-plug state to OFF. · last run failed')).toHaveAttribute('data-ui', 'data-table-meta');
    expect(item.querySelector('[data-ui=status-pill]')).toHaveTextContent('Armed');
  });
});
