import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Automation, AutomationRun, Sensor } from '../../gen/aliases';
import { theme } from '../../ui/theme';
import AutomationEditorPage from './AutomationEditorPage';

const api = vi.hoisted(() => ({ GET: vi.fn(), POST: vi.fn(), PUT: vi.fn(), DELETE: vi.fn() }));
const authState = vi.hoisted(() => ({ user: { id: 1, username: 'tom', roles: [] as string[], permissions: [] as string[] } }));
const sensorState = vi.hoisted(() => ({ sensors: [] as unknown[], loaded: true }));

vi.mock('../../gen/client', () => ({ apiClient: api }));
vi.mock('../../providers/AuthContext', () => ({ useAuth: () => authState }));
vi.mock('../../hooks/useSensorContext', () => ({ useSensorContext: () => sensorState }));
vi.mock('../../navigation/AppNav', () => ({ default: () => <nav>sidebar</nav> }));
vi.mock('../../navigation/TopAppBar', () => ({ default: ({ pageTitle }: { pageTitle: string }) => <header>{pageTitle}</header> }));

const editor = ['view_automations', 'manage_automations', 'control_sensors'];

const lamp = {
  id: 14,
  name: 'hallway-lamp',
  capabilities: [
    { property: 'state', type: 'binary', value_on: 'ON', value_off: 'OFF' },
    { property: 'brightness', type: 'numeric', min: 0, max: 254 },
    { property: 'color_temp_preset', type: 'enum', values: ['warm', 'neutral', 'cool'] },
  ],
} as unknown as Sensor;

function automation(overrides: Partial<Automation> = {}): Automation {
  return {
    id: 3,
    name: 'Evening lights',
    enabled: true,
    triggers: [{ id: 1, type: 'schedule', at: '19:00', days: ['mon', 'tue', 'wed', 'thu', 'fri'] }],
    steps: [
      { type: 'set', sensor_id: 14, property: 'state', value: 'ON' },
      { type: 'set', sensor_id: 14, property: 'brightness', value: '150' },
    ],
    status: 'armed',
    last_run_failed: false,
    next_fire_at: '2026-10-06T18:00:00Z',
    hub_timezone: 'Europe/London',
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...overrides,
  };
}

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

function serve(saved: Automation, runs: AutomationRun[] = []) {
  api.GET.mockImplementation(async (path: string) => ({
    data: path.endsWith('/runs') ? runs : saved,
    response: new Response(),
  }));
}

function Location() {
  return <output aria-label="location">{useLocation().pathname}</output>;
}

function renderEditor(at: string, permissions = editor, width = 1280) {
  atWidth(width);
  authState.user.permissions = permissions;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <ThemeProvider theme={theme}>
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[at]}>
          <Routes>
            <Route path="/automations/:id" element={<AutomationEditorPage />} />
            <Route path="/automations" element={<p>list</p>} />
          </Routes>
          <Location />
        </MemoryRouter>
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

const savedBody = (mock: typeof api.PUT) => mock.mock.calls.at(-1)![1].body;

describe('AutomationEditorPage', () => {
  beforeEach(() => {
    sensorState.sensors = [lamp];
    Object.values(api).forEach((mock) => mock.mockReset());
  });
  afterEach(() => vi.unstubAllGlobals());

  it('matches each set step value control to its capability type', async () => {
    serve(automation({ steps: [
      { type: 'set', sensor_id: 14, property: 'state', value: 'ON' },
      { type: 'set', sensor_id: 14, property: 'brightness', value: '150' },
      { type: 'set', sensor_id: 14, property: 'color_temp_preset', value: 'cool' },
    ] }));
    renderEditor('/automations/3');

    const toggle = await screen.findByRole('group', { name: 'Value' });
    expect(within(toggle).getByRole('button', { name: 'ON' })).toHaveAttribute('aria-pressed', 'true');
    expect(within(toggle).getByRole('button', { name: 'OFF' })).toHaveAttribute('aria-pressed', 'false');
    const slider = screen.getByRole('slider', { name: 'Value' });
    expect(slider).toHaveAttribute('aria-valuemin', '0');
    expect(slider).toHaveAttribute('aria-valuemax', '254');
    expect(slider).toHaveValue('150');
    expect(screen.getByRole('spinbutton', { name: 'Value' })).toHaveAttribute('max', '254');
    expect(screen.getAllByRole('combobox', { name: 'Value' })[0]).toHaveTextContent('cool');
  });

  it("shows the API's message when it rejects a save", async () => {
    serve(automation());
    api.PUT.mockResolvedValue({ error: { message: 'triggers[0].days: choose at least one weekday' }, response: new Response(null, { status: 400 }) });
    renderEditor('/automations/3');

    fireEvent.click(await screen.findByRole('button', { name: 'Save' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('triggers[0].days: choose at least one weekday');
  });

  it('saves the steps in the order the drag handles put them in', async () => {
    serve(automation());
    api.PUT.mockResolvedValue({ data: automation(), response: new Response() });
    renderEditor('/automations/3');

    fireEvent.keyDown(await screen.findByRole('button', { name: 'Move step 2' }), { key: 'ArrowUp' });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(api.PUT).toHaveBeenCalled());
    expect(savedBody(api.PUT).steps.map((step: { property: string }) => step.property)).toEqual(['brightness', 'state']);
  });

  it('creates a new automation and opens it', async () => {
    api.POST.mockResolvedValue({ data: automation({ id: 9 }), response: new Response() });
    serve(automation({ id: 9 }));
    renderEditor('/automations/new');

    expect(screen.getByRole('switch', { name: 'Enabled' })).toBeChecked();
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'Evening lights' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(screen.getByRole('status', { name: 'location' })).toHaveTextContent('/automations/9'));
    expect(savedBody(api.POST)).toMatchObject({ name: 'Evening lights', enabled: true, triggers: [{ type: 'schedule', at: '19:00' }] });
  });

  it('deletes only after the confirmation', async () => {
    serve(automation());
    api.DELETE.mockResolvedValue({ data: { message: 'Automation deleted' }, response: new Response() });
    renderEditor('/automations/3');

    fireEvent.click(await screen.findByRole('button', { name: 'Delete' }));
    expect(api.DELETE).not.toHaveBeenCalled();
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(screen.getByRole('status', { name: 'location' })).toHaveTextContent(/^\/automations$/));
    expect(api.DELETE).toHaveBeenCalledWith('/automations/{id}', { params: { path: { id: 3 } } });
  });

  it('gives viewers every control read-only, with no Enabled switch, Save or Delete', async () => {
    serve(automation());
    renderEditor('/automations/3', ['view_automations']);

    expect(await screen.findByRole('slider', { name: 'Value' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Save' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Delete' })).toBeNull();
    expect(screen.queryByRole('switch', { name: 'Enabled' })).toBeNull();
    expect(screen.queryByRole('button', { name: '+ Add trigger' })).toBeNull();
    expect(screen.queryByRole('button', { name: /^Move step/ })).toBeNull();
    expect(screen.getByRole('button', { name: 'Mon' })).toBeDisabled();
  });

  it('lists the last 30 days of runs newest first with what each one did', async () => {
    const run = (id: number, daysAgo: number, overrides: Partial<AutomationRun>): AutomationRun => ({
      id,
      automation_id: 3,
      trigger_kind: 'schedule',
      status: 'succeeded',
      current_step: 2,
      steps: automation().steps,
      step_outcomes: [],
      started_at: new Date(Date.now() - daysAgo * 86_400_000).toISOString(),
      ...overrides,
    });
    serve(automation(), [
      run(3, 1, {}),
      run(2, 2, { status: 'failed', current_step: 1, error: 'step 1 (set hallway-lamp state to ON) failed: hallway-lamp did not acknowledge the command within 10s' }),
      run(1, 31, {}),
    ]);
    renderEditor('/automations/3');

    await waitFor(() => expect(document.querySelectorAll('[data-ui=automation-run]')).toHaveLength(2));
    const [succeeded, failed] = document.querySelectorAll<HTMLElement>('[data-ui=automation-run]');
    expect(succeeded).toHaveTextContent('succeeded');
    expect(succeeded).toHaveTextContent('2 steps · all succeeded');
    expect(failed).toHaveTextContent('failed');
    expect(failed).toHaveTextContent('step 1 (set hallway-lamp state to ON) failed: hallway-lamp did not acknowledge');
  });

  it('stacks the summary, When, Then and Recent runs on phones with Save in the bottom bar', async () => {
    serve(automation());
    renderEditor('/automations/3', editor, 390);

    expect((await screen.findByRole('button', { name: 'Save' })).closest('[data-ui=sticky-footer]')).not.toBeNull();
    const titles = Array.from(document.querySelectorAll('[data-ui=card-header] h2'), (heading) => heading.textContent);
    expect(titles).toEqual(['In plain words', 'When any of these happens', 'Then in this order', 'Recent runs']);
  });
});
