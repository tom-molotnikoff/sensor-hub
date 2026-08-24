import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PropertyDefinitionsResponse } from '../gen/aliases';
import { FakeWebSocket, installFakeWebSocket } from '../test/fakeWebSocket';

const { getMock, patchMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  patchMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: {
    GET: getMock,
    PATCH: patchMock,
  },
}));

const definitionsResponse: PropertyDefinitionsResponse = {
  definitions: [
    {
      key: 'sensor.discovery.skip',
      label: 'Skip sensor discovery',
      description: "Don't try to auto-discover sensors at startup.",
      type: 'bool',
      default: 'false',
      group: 'sensors',
      apply: 'live',
      readOnly: false,
    },
    {
      key: 'sensor.collection.interval',
      label: 'Collection interval',
      description: 'How often every enabled sensor is polled.',
      type: 'int',
      default: '300',
      group: 'sensors',
      unit: 'seconds',
      apply: 'next-cycle',
      readOnly: false,
    },
    {
      key: 'database.path',
      label: 'Database file',
      description: 'SQLite database file.',
      type: 'string',
      default: 'data/sensor_hub.db',
      group: 'advanced',
      apply: 'readonly',
      readOnly: true,
    },
  ],
  groups: [
    { id: 'sensors', label: 'Sensors & collection', description: 'How often sensors are polled.', order: 1 },
    { id: 'advanced', label: 'Advanced', description: 'Rarely-changed settings.', order: 2 },
  ],
};

const serverValues: Record<string, string> = {
  'sensor.discovery.skip': 'true',
  'sensor.collection.interval': '300',
  'database.path': '/var/lib/sensor-hub/sensor_hub.db',
};

let restoreWebSocket: () => void;

async function renderPage(permissions: string[]) {
  // Fresh imports per test so the session cache in usePropertyDefinitions is empty,
  // and so the AuthContext instance matches the one the page imports.
  const { default: PropertiesPage } = await import('./PropertiesPage');
  const { AuthContext } = await import('../providers/AuthContext');

  render(
    <AuthContext.Provider value={{ user: { id: 1, username: 'owner', roles: [], permissions }, refresh: async () => {} }}>
      <PropertiesPage />
    </AuthContext.Provider>,
  );

  await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
  act(() => {
    FakeWebSocket.instances[0].serverSends(JSON.stringify(serverValues));
  });
  await waitFor(() => expect(screen.getByText('Skip sensor discovery')).toBeInTheDocument());
}

describe('PropertiesPage', () => {
  beforeEach(() => {
    getMock.mockReset();
    patchMock.mockReset();
    vi.resetModules();
    restoreWebSocket = installFakeWebSocket();
    getMock.mockResolvedValue({ data: definitionsResponse });
    patchMock.mockResolvedValue({ data: { message: 'ok' } });
  });

  afterEach(() => {
    restoreWebSocket();
  });

  it('renders a field for each definition carrying the current value from the value feed', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).toBeChecked();
    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(300);
    expect(screen.getByText('/var/lib/sensor-hub/sensor_hub.db')).toBeInTheDocument();
  });

  it('saves edits through PATCH /properties with database.path excluded from the payload', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(patchMock).toHaveBeenCalledTimes(1));
    expect(patchMock).toHaveBeenCalledWith('/properties', {
      body: {
        'sensor.discovery.skip': 'true',
        'sensor.collection.interval': '120',
      },
    });
  });

  it('undoes one modified field back to the saved value, leaving other edits untouched', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });
    fireEvent.click(screen.getByRole('switch', { name: 'Skip sensor discovery' }));

    fireEvent.click(screen.getByRole('button', { name: 'Undo changes to Collection interval' }));

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(300);
    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).not.toBeChecked();
    expect(screen.queryByRole('button', { name: 'Undo changes to Collection interval' })).not.toBeInTheDocument();
  });

  it('disables every control and shows no save control for a user without manage_properties', async () => {
    await renderPage(['view_properties']);

    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).toBeDisabled();
    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: /save/i })).not.toBeInTheDocument();
  });
});
