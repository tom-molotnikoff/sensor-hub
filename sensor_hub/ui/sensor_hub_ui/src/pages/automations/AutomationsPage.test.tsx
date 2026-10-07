import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Automation, Sensor } from '../../gen/aliases';
import { installFakeWebSocket } from '../../test/fakeWebSocket';
import { editorPermissions, renderAutomationPages, serveGets } from './automationPageHarness';
import AutomationsPage from './AutomationsPage';

const api = vi.hoisted(() => ({ GET: vi.fn(), POST: vi.fn(), PUT: vi.fn() }));

vi.mock('../../gen/client', () => ({ apiClient: api }));

const socket: Sensor = {
  id: 6,
  name: 'study-socket',
  sensor_driver: 'zigbee2mqtt',
  config: {},
  health_status: 'good',
  health_reason: '',
  enabled: true,
  status: 'active',
  capabilities: [{ property: 'state', type: 'binary', value_on: 'ON', value_off: 'OFF' }],
};

function automation(id: number, name: string, overrides: Partial<Automation>): Automation {
  return {
    id,
    name,
    enabled: true,
    mode: 'single',
    triggers: [{ id, type: 'schedule', at: '06:15', days: ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] }],
    steps: [{ type: 'set', sensor_id: socket.id, property: 'state', value: 'OFF' }],
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
  automation(6, 'Hallway colour', {
    status: 'broken',
    status_reason: 'step 2: hallway-lamp no longer has color_temp_preset',
    next_fire_at: null,
  }),
  automation(5, 'Lounge heat on', {
    triggers: [{ id: 5, type: 'reading', sensor_id: socket.id, measurement_type: 'temperature', operator: 'falls_below', threshold: 16, rearm_margin: 0.2, hold_seconds: 0 }],
    next_fire_at: null,
  }),
];

function renderList(width: number, permissions = editorPermissions) {
  serveGets(api.GET, permissions, { '/automations': automations });
  return renderAutomationPages({ routes: { '/automations': <AutomationsPage /> }, at: '/automations', width, sensors: [socket] });
}

const rowOf = async (name: string) => (await screen.findByText(name)).closest<HTMLElement>('[role=row]')!;

describe('AutomationsPage', () => {
  let restoreWebSocket: () => void;

  beforeEach(() => {
    restoreWebSocket = installFakeWebSocket();
    Object.values(api).forEach((mock) => mock.mockReset());
  });
  afterEach(() => {
    restoreWebSocket();
    vi.unstubAllGlobals();
  });

  it('shows the enabled switch, then the automation, Status and Next columns in that order', async () => {
    await renderList(1280);

    const row = await rowOf('Office plug off at night');
    expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual(['', 'Automation', 'Status', 'Next']);
    expect(row).toHaveTextContent(socket.name);
    expect(within(row).getByRole('switch', { name: 'Office plug off at night enabled' })).toBeChecked();
    expect(row).toHaveTextContent('Armed');
    expect(within(row).getAllByRole('gridcell').at(-1)).toHaveTextContent('23:30');
    expect(screen.getByRole('button', { name: 'New automation' })).toBeInTheDocument();
  });

  it('flags a failed last run beside Armed, and shows Off with a dash for Next', async () => {
    await renderList(1280);

    expect(await rowOf('Pump cycle')).toHaveTextContent('Armedlast run failed');
    expect(await rowOf('Office plug off at night')).not.toHaveTextContent('last run failed');
    const off = await rowOf('Christmas lights');
    expect(off).toHaveTextContent('Off');
    expect(off).not.toHaveTextContent('last run failed');
    expect(within(off).getAllByRole('gridcell').at(-1)).toHaveTextContent(/^-$/);
    expect(await rowOf('Evening lights')).toHaveTextContent('Running');
  });

  it('shows Broken with its reason, and a dash for Next', async () => {
    await renderList(1280);

    const broken = await rowOf('Hallway colour');
    expect(broken).toHaveTextContent('Brokenstep 2: hallway-lamp no longer has color_temp_preset');
    expect(within(broken).getAllByRole('gridcell').at(-1)).toHaveTextContent(/^-$/);
  });

  it('shows "on reading" for Next when an automation only has reading triggers', async () => {
    await renderList(1280);

    expect(within(await rowOf('Lounge heat on')).getAllByRole('gridcell').at(-1)).toHaveTextContent('on reading');
  });

  it('switches an automation on and off from its row', async () => {
    api.PUT.mockResolvedValue({ data: automation(1, 'Office plug off at night', { enabled: false, status: 'off' }), response: new Response() });
    await renderList(1280);

    fireEvent.click(within(await rowOf('Office plug off at night')).getByRole('switch'));

    await waitFor(() =>
      expect(api.PUT).toHaveBeenCalledWith('/automations/{id}/enabled', { params: { path: { id: 1 } }, body: { enabled: false } }),
    );
  });

  it('gives viewers the list without enabled switches or New automation', async () => {
    await renderList(1280, ['view_automations']);

    await screen.findByText('Office plug off at night');
    expect(screen.queryByRole('switch')).toBeNull();
    expect(screen.queryByRole('button', { name: 'New automation' })).toBeNull();
  });

  it('shows each automation on phones as its name, summary and status', async () => {
    await renderList(390);

    const item = (await screen.findByText('Pump cycle')).closest<HTMLElement>('[data-ui=data-table-row]')!;
    expect(item.querySelector('[data-ui=data-table-title]')).toHaveTextContent('Pump cycle');
    const meta = item.querySelector('[data-ui=data-table-meta]');
    expect(meta).toHaveTextContent(socket.name);
    expect(meta).toHaveTextContent('last run failed');
    expect(item.querySelector('[data-ui=status-pill]')).toHaveTextContent('Armed');

    const broken = (await screen.findByText('Hallway colour')).closest<HTMLElement>('[data-ui=data-table-row]')!;
    expect(broken.querySelector('[data-ui=data-table-meta]')).toHaveTextContent('step 2: hallway-lamp no longer has color_temp_preset');
    expect(broken.querySelector('[data-ui=status-pill]')).toHaveTextContent('Broken');
  });
});
