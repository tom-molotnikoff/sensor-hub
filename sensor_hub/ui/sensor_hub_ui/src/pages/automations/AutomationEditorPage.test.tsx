import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { useLocation } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Automation, AutomationRun, Sensor } from '../../gen/aliases';
import { installFakeWebSocket } from '../../test/fakeWebSocket';
import AutomationEditorPage from './AutomationEditorPage';
import { editorPermissions, renderAutomationPages, serveGets } from './automationPageHarness';

const api = vi.hoisted(() => ({ GET: vi.fn(), POST: vi.fn(), PUT: vi.fn(), DELETE: vi.fn() }));

vi.mock('../../gen/client', () => ({ apiClient: api }));

const lamp: Sensor = {
  id: 14,
  name: 'hallway-lamp',
  sensor_driver: 'zigbee2mqtt',
  config: {},
  health_status: 'good',
  health_reason: '',
  enabled: true,
  status: 'active',
  capabilities: [
    { property: 'state', type: 'binary', value_on: 'ON', value_off: 'OFF' },
    { property: 'brightness', type: 'numeric', min: 0, max: 254 },
    { property: 'color_temp_preset', type: 'enum', values: ['warm', 'neutral', 'cool'] },
  ],
};

function automation(overrides: Partial<Automation> = {}): Automation {
  return {
    id: 3,
    name: 'Evening lights',
    enabled: true,
    mode: 'single',
    triggers: [{ id: 1, type: 'schedule', at: '19:00', days: ['mon', 'tue', 'wed', 'thu', 'fri'] }],
    steps: [
      { type: 'set', sensor_id: lamp.id, property: 'state', value: 'ON' },
      { type: 'set', sensor_id: lamp.id, property: 'brightness', value: '150' },
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

function Location() {
  return <output aria-label="location">{useLocation().pathname}</output>;
}

let responses: Record<string, unknown> = {};

function serve(saved: Automation, runs: AutomationRun[] = []) {
  responses = { '/automations/{id}': saved, '/automations/{id}/runs': runs };
}

function renderEditor(at: string, permissions = editorPermissions, width = 1280) {
  serveGets(api.GET, permissions, responses);
  return renderAutomationPages({
    routes: { '/automations/:id': <AutomationEditorPage />, '/automations': <p>list</p> },
    at,
    width,
    sensors: [lamp],
    alongside: <Location />,
  });
}

const savedBody = (mock: typeof api.PUT) => mock.mock.calls.at(-1)![1].body;

function run(id: number, overrides: Partial<AutomationRun>): AutomationRun {
  return {
    id,
    automation_id: 3,
    trigger_kind: 'schedule',
    status: 'succeeded',
    current_step: 0,
    steps: [...automation().steps, { type: 'wait', seconds: 14_400 }, { type: 'set', sensor_id: lamp.id, property: 'state', value: 'OFF' }],
    step_outcomes: [],
    started_at: new Date(Date.now() - 60_000).toISOString(),
    ...overrides,
  };
}

describe('AutomationEditorPage', () => {
  let restoreWebSocket: () => void;

  beforeEach(() => {
    restoreWebSocket = installFakeWebSocket();
    Object.values(api).forEach((mock) => mock.mockReset());
  });
  afterEach(() => {
    restoreWebSocket();
    vi.unstubAllGlobals();
  });

  it('matches each set step value control to its capability type', async () => {
    serve(automation({ steps: [
      { type: 'set', sensor_id: lamp.id, property: 'state', value: 'ON' },
      { type: 'set', sensor_id: lamp.id, property: 'brightness', value: '150' },
      { type: 'set', sensor_id: lamp.id, property: 'color_temp_preset', value: 'cool' },
    ] }));
    await renderEditor('/automations/3');

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

  it("shows the API's message when it rejects a save, naming the trigger or step as the editor numbers it", async () => {
    serve(automation());
    api.PUT.mockResolvedValue({ error: { message: 'steps[1].value: 300 is above the maximum of 254' }, response: new Response(null, { status: 400 }) });
    await renderEditor('/automations/3');

    fireEvent.click(await screen.findByRole('button', { name: 'Save' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Step 2 value: 300 is above the maximum of 254');
  });

  it('saves the steps in the order the drag handles put them in', async () => {
    serve(automation());
    api.PUT.mockResolvedValue({ data: automation(), response: new Response() });
    await renderEditor('/automations/3');

    fireEvent.keyDown(await screen.findByRole('button', { name: 'Move step 2' }), { key: 'ArrowUp' });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(api.PUT).toHaveBeenCalled());
    expect(savedBody(api.PUT).steps.map((step: { property: string }) => step.property)).toEqual(['brightness', 'state']);
  });

  it('adds a wait step and saves its duration in seconds', async () => {
    serve(automation());
    api.PUT.mockResolvedValue({ data: automation(), response: new Response() });
    await renderEditor('/automations/3');

    fireEvent.click(await screen.findByRole('button', { name: '+ Wait' }));
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Duration' }), { target: { value: '4' } });
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Unit' }));
    fireEvent.click(screen.getByRole('option', { name: 'hours' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(api.PUT).toHaveBeenCalled());
    expect(savedBody(api.PUT).steps[2]).toEqual({ type: 'wait', seconds: 14_400 });
  });

  it('shows a saved wait in its largest whole unit', async () => {
    serve(automation({ steps: [{ type: 'wait', seconds: 7_200 }] }));
    await renderEditor('/automations/3');

    expect(await screen.findByRole('spinbutton', { name: 'Duration' })).toHaveValue(2);
    expect(screen.getByRole('combobox', { name: 'Unit' })).toHaveTextContent('hours');
  });

  it('turns a trigger into an interval, describes it in plain words and saves it in seconds', async () => {
    serve(automation());
    api.PUT.mockResolvedValue({ data: automation(), response: new Response() });
    await renderEditor('/automations/3');

    fireEvent.mouseDown(await screen.findByRole('combobox', { name: 'Trigger' }));
    fireEvent.click(screen.getByRole('option', { name: 'Every…' }));
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Interval' }), { target: { value: '2' } });
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Unit' }));
    fireEvent.click(screen.getByRole('option', { name: 'hours' }));

    expect(screen.getByText(/^Every 2 h, set hallway-lamp state to ON/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(api.PUT).toHaveBeenCalled());
    expect(savedBody(api.PUT).triggers).toEqual([{ type: 'interval', seconds: 7_200 }]);
  });

  it('creates a new automation and opens it', async () => {
    api.POST.mockResolvedValue({ data: automation({ id: 9 }), response: new Response() });
    serve(automation({ id: 9 }));
    await renderEditor('/automations/new');

    expect(screen.getByRole('switch', { name: 'Enabled' })).toBeChecked();
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'Evening lights' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(screen.getByRole('status', { name: 'location' })).toHaveTextContent('/automations/9'));
    expect(savedBody(api.POST)).toMatchObject({ name: 'Evening lights', enabled: true, triggers: [{ type: 'schedule', at: '19:00' }] });
  });

  it('deletes only after the confirmation', async () => {
    serve(automation());
    api.DELETE.mockResolvedValue({ data: { message: 'Automation deleted' }, response: new Response() });
    await renderEditor('/automations/3');

    fireEvent.click(await screen.findByRole('button', { name: 'Delete' }));
    expect(api.DELETE).not.toHaveBeenCalled();
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(screen.getByRole('status', { name: 'location' })).toHaveTextContent(/^\/automations$/));
    expect(api.DELETE).toHaveBeenCalledWith('/automations/{id}', { params: { path: { id: 3 } } });
  });

  it('gives viewers every control read-only, with no Enabled switch, Save, Run now, Cancel run or Delete', async () => {
    serve(automation({ status: 'running' }), [run(2, { status: 'waiting', current_step: 3, resume_at: '2026-10-06T22:00:00Z' })]);
    await renderEditor('/automations/3', ['view_automations']);

    expect(await screen.findByRole('slider', { name: 'Value' })).toBeDisabled();
    await waitFor(() => expect(document.querySelectorAll('[data-ui=automation-run]')).toHaveLength(1));
    expect(screen.queryByRole('button', { name: 'Run now' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Cancel run' })).toBeNull();
    expect(screen.getByRole('combobox', { name: /If a trigger fires while already running/ })).toHaveAttribute('aria-disabled', 'true');
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
    const failure = 'step 1 (set hallway-lamp state to ON) failed: hallway-lamp did not acknowledge the command within 10s';
    serve(automation(), [
      run(3, 1, {}),
      run(2, 2, { status: 'failed', current_step: 1, error: failure }),
      run(1, 31, {}),
    ]);
    await renderEditor('/automations/3');

    await waitFor(() => expect(document.querySelectorAll('[data-ui=automation-run]')).toHaveLength(2));
    const [succeeded, failed] = document.querySelectorAll<HTMLElement>('[data-ui=automation-run]');
    expect(succeeded.querySelector('[data-ui=status-pill]')).toHaveTextContent(/^succeeded$/);
    expect(succeeded.lastElementChild).toHaveTextContent(/\b2\b/);
    expect(failed.querySelector('[data-ui=status-pill]')).toHaveTextContent(/^failed$/);
    expect(failed.lastElementChild).toHaveTextContent(failure);
  });

  it('shows where a waiting run is and how late a missed one was', async () => {
    serve(automation(), [
      run(2, { status: 'waiting', current_step: 3, resume_at: '2026-10-06T22:00:00Z' }),
      run(1, { status: 'missed', due_at: '2026-10-02T18:00:00Z', past_grace_seconds: 7_200 }),
    ]);
    await renderEditor('/automations/3');

    await waitFor(() => expect(document.querySelectorAll('[data-ui=automation-run]')).toHaveLength(2));
    const [waiting, missed] = document.querySelectorAll<HTMLElement>('[data-ui=automation-run]');
    expect(waiting.querySelector('[data-ui=status-pill]')).toHaveTextContent(/^waiting$/);
    expect(waiting.lastElementChild).toHaveTextContent('waiting · step 3 of 4 · resumes Tue 6 Oct, 23:00');
    expect(missed.querySelector('[data-ui=status-pill]')).toHaveTextContent(/^missed$/);
    expect(missed).toHaveTextContent('Fri 2 Oct, 19:00');
    expect(missed.lastElementChild).toHaveTextContent('hub was down - 2 h past the grace window');
  });

  it('saves the choice of what a trigger does while the automation is already running', async () => {
    serve(automation());
    api.PUT.mockResolvedValue({ data: automation({ mode: 'restart' }), response: new Response() });
    await renderEditor('/automations/3');

    const mode = await screen.findByRole('combobox', { name: /If a trigger fires while already running/ });
    expect(mode).toHaveTextContent('Ignore it (single)');
    fireEvent.mouseDown(mode);
    fireEvent.click(screen.getByRole('option', { name: 'Start over (restart)' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(api.PUT).toHaveBeenCalled());
    expect(savedBody(api.PUT).mode).toBe('restart');
  });

  it('runs the saved automation now', async () => {
    serve(automation());
    api.POST.mockResolvedValue({ data: run(4, { trigger_kind: 'manual', status: 'running' }), response: new Response(null, { status: 202 }) });
    await renderEditor('/automations/3');

    fireEvent.click(await screen.findByRole('button', { name: 'Run now' }));

    await waitFor(() => expect(api.POST).toHaveBeenCalledWith('/automations/{id}/run', { params: { path: { id: 3 } } }));
  });

  it('keeps Run now off until unsaved changes are saved', async () => {
    serve(automation());
    api.PUT.mockResolvedValue({ data: automation({ name: 'Evening lamp' }), response: new Response() });
    await renderEditor('/automations/3');

    fireEvent.change(await screen.findByRole('textbox', { name: 'Name' }), { target: { value: 'Evening lamp' } });
    const runNow = screen.getByRole('button', { name: 'Run now' });
    expect(runNow).toBeDisabled();
    fireEvent.mouseOver(runNow.parentElement!);
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Save first');

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Run now' })).toBeEnabled());
  });

  it('cancels an active run from Recent runs, and offers it only on active runs', async () => {
    serve(automation({ status: 'running' }), [
      run(2, { status: 'waiting', current_step: 3, resume_at: '2026-10-06T22:00:00Z' }),
      run(1, { status: 'succeeded', current_step: 4 }),
    ]);
    api.POST.mockResolvedValue({ data: run(2, { status: 'cancelled', current_step: 3 }), response: new Response() });
    await renderEditor('/automations/3');

    await waitFor(() => expect(document.querySelectorAll('[data-ui=automation-run]')).toHaveLength(2));
    const [waiting, succeeded] = document.querySelectorAll<HTMLElement>('[data-ui=automation-run]');
    expect(within(succeeded).queryByRole('button', { name: 'Cancel run' })).toBeNull();
    fireEvent.click(within(waiting).getByRole('button', { name: 'Cancel run' }));

    await waitFor(() =>
      expect(api.POST).toHaveBeenCalledWith('/automations/{id}/runs/{runId}/cancel', { params: { path: { id: 3, runId: 2 } } }),
    );
  });

  it('shows skipped and cancelled runs with what happened to them', async () => {
    serve(automation(), [run(2, { status: 'skipped' }), run(1, { status: 'cancelled', current_step: 3 })]);
    await renderEditor('/automations/3');

    await waitFor(() => expect(document.querySelectorAll('[data-ui=automation-run]')).toHaveLength(2));
    const [skipped, cancelled] = document.querySelectorAll<HTMLElement>('[data-ui=automation-run]');
    expect(skipped.querySelector('[data-ui=status-pill]')).toHaveTextContent(/^skipped$/);
    expect(skipped.lastElementChild).toHaveTextContent('already running');
    expect(cancelled.querySelector('[data-ui=status-pill]')).toHaveTextContent(/^cancelled$/);
    expect(cancelled.lastElementChild).toHaveTextContent('cancelled on step 3 of 4');
  });

  it('stacks the summary, When, Then and Recent runs on phones with Run now and Save in the bottom bar', async () => {
    serve(automation());
    await renderEditor('/automations/3', editorPermissions, 390);

    expect((await screen.findByRole('button', { name: 'Save' })).closest('[data-ui=sticky-footer]')).not.toBeNull();
    expect(screen.getByRole('button', { name: 'Run now' }).closest('[data-ui=sticky-footer]')).not.toBeNull();
    const titles = Array.from(document.querySelectorAll('[data-ui=card-header] h2'), (heading) => heading.textContent);
    expect(titles).toEqual(['In plain words', 'When any of these happens', 'Then in this order', 'Recent runs']);
  });
});
