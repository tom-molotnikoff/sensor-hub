import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Capability, CommandStatusMessage, Reading, Sensor, SensorCommandAccepted } from '../../gen/aliases';
import SensorToggleWidget from './SensorToggleWidget';

const { postMock, reportUpdateMock } = vi.hoisted(() => ({
  postMock: vi.fn(),
  reportUpdateMock: vi.fn(),
}));

const currentReadings: Record<string, Record<string, Reading>> = {};
const sensors: Sensor[] = [];
let authUser: { id: number; username: string; roles: string[]; permissions?: string[] } | null = null;
let commandStatusHandler: ((message: CommandStatusMessage) => void) | undefined;

vi.mock('../../gen/client', () => ({
  apiClient: {
    POST: postMock,
  },
}));

vi.mock('../../hooks/useSensorContext', () => ({
  useSensorContext: () => ({
    sensors,
    loaded: true,
  }),
}));

vi.mock('../../hooks/useCurrentReadings', () => ({
  useCurrentReadings: (options?: { onCommandStatus?: (message: CommandStatusMessage) => void }) => {
    commandStatusHandler = options?.onCommandStatus;
    return currentReadings;
  },
  useCurrentReadingsReady: () => true,
}));

vi.mock('../../providers/AuthContext', () => ({
  useAuth: () => ({
    user: authUser,
  }),
}));

vi.mock('../WidgetUpdateContext', () => ({
  useReportWidgetUpdate: () => reportUpdateMock,
}));

function makeCapability(overrides: Partial<Capability> = {}): Capability {
  return {
    property: 'state',
    type: 'binary',
    value_on: 'ENABLED',
    value_off: 'DISABLED',
    ...overrides,
  };
}

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 7,
    name: 'office-plug',
    external_id: 'office-plug',
    sensor_driver: 'zigbee2mqtt',
    config: {},
    metadata: {},
    capabilities: [makeCapability()],
    health_status: 'good',
    health_reason: 'ok',
    enabled: true,
    status: 'active',
    retention_hours: null,
    ...overrides,
  };
}

function makeReading(overrides: Partial<Reading> = {}): Reading {
  return {
    id: 99,
    sensor_name: 'office-plug',
    measurement_type: 'state',
    numeric_value: null,
    text_state: 'ENABLED',
    unit: '',
    time: '2026-05-09T18:00:00Z',
    ...overrides,
  };
}

function extractTranslateXPixels(transform: string): number {
  const match = transform.match(/translateX\(([-\d.]+)px\)/);
  if (!match) {
    throw new Error(`Expected translateX(...) in transform string, received: ${transform}`);
  }
  return Number(match[1]);
}

describe('SensorToggleWidget', () => {
  beforeEach(() => {
    sensors.splice(0, sensors.length);
    Object.keys(currentReadings).forEach((key) => delete currentReadings[key]);
    authUser = null;
    commandStatusHandler = undefined;
    postMock.mockReset();
    reportUpdateMock.mockReset();
  });

  it('stays indeterminate until the first reading arrives instead of defaulting to off', () => {
    sensors.splice(0, sensors.length, makeSensor());
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    const { rerender } = render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const initialToggle = screen.getByRole('checkbox', { name: /toggle office-plug state/i });
    expect(initialToggle).toHaveAttribute('aria-checked', 'mixed');
    expect(initialToggle).toHaveAttribute('aria-disabled', 'true');

    fireEvent.click(initialToggle);
    expect(postMock).not.toHaveBeenCalled();

    currentReadings['office-plug'] = { state: makeReading() };
    rerender(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const resolvedToggle = screen.getByRole('checkbox', { name: /toggle office-plug state/i });
    expect(resolvedToggle).toBeChecked();
    expect(resolvedToggle).toHaveAttribute('aria-disabled', 'false');
  });

  it('treats canonical true readings as on when capability values are ON and OFF', () => {
    sensors.splice(0, sensors.length, makeSensor({
      capabilities: [makeCapability({ value_on: 'ON', value_off: 'OFF' })],
    }));
    currentReadings['office-plug'] = { state: makeReading({ text_state: 'true' }) };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    expect(screen.getByRole('checkbox', { name: /toggle office-plug state/i })).toBeChecked();
  });

  it('renders the current binary state and flips optimistically when sending a command', async () => {
    sensors.splice(0, sensors.length, makeSensor());
    currentReadings['office-plug'] = { state: makeReading() };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    let resolvePost: ((value: { data: SensorCommandAccepted }) => void) | undefined;
    postMock.mockImplementation(
      () =>
        new Promise<{ data: SensorCommandAccepted }>((resolve) => {
          resolvePost = resolve;
        }),
    );

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const toggle = screen.getByRole('checkbox', { name: /toggle office-plug state/i });
    expect(toggle).toBeChecked();

    fireEvent.click(toggle);

    expect(postMock).toHaveBeenCalledWith('/sensors/{id}/command', {
      params: { path: { id: 7 } },
      body: { property: 'state', value: 'DISABLED' },
    });
    expect(toggle).not.toBeChecked();
    expect(reportUpdateMock).toHaveBeenCalledTimes(1);

    if (!resolvePost) {
      throw new Error('Expected command request to be pending');
    }

    resolvePost({
      data: {
        id: 42,
        status: 'sent',
        property: 'state',
        value: 'DISABLED',
      },
    });
  });

  it('reverts the optimistic state and shows an error snackbar when the command fails', async () => {
    sensors.splice(0, sensors.length, makeSensor());
    currentReadings['office-plug'] = { state: makeReading() };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    postMock.mockResolvedValue({
      data: {
        id: 42,
        status: 'sent',
        property: 'state',
        value: 'DISABLED',
      } satisfies SensorCommandAccepted,
    });

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const toggle = screen.getByRole('checkbox', { name: /toggle office-plug state/i });
    fireEvent.click(toggle);

    expect(toggle).not.toBeChecked();
    await Promise.resolve();

    commandStatusHandler?.({
      type: 'command_status',
      id: 42,
      sensor_id: 7,
      property: 'state',
      value: 'DISABLED',
      status: 'failed',
      acknowledged_at: null,
      acknowledged_value: null,
    });

    await waitFor(() =>
      expect(screen.getByRole('checkbox', { name: /toggle office-plug state/i })).toBeChecked(),
    );
    expect(await screen.findByText('Command failed')).toBeInTheDocument();
    expect(reportUpdateMock).toHaveBeenCalledTimes(2);
  });

  it('clears optimistic state when the live reading matches semantically', async () => {
    sensors.splice(0, sensors.length, makeSensor({
      capabilities: [makeCapability({ value_on: 'ON', value_off: 'OFF' })],
    }));
    currentReadings['office-plug'] = { state: makeReading({ text_state: 'false' }) };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    postMock.mockResolvedValue({
      data: {
        id: 44,
        status: 'sent',
        property: 'state',
        value: 'ON',
      } satisfies SensorCommandAccepted,
    });

    const { rerender } = render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const toggle = screen.getByRole('checkbox', { name: /toggle office-plug state/i });
    expect(toggle).not.toBeChecked();

    fireEvent.click(toggle);
    expect(toggle).toBeChecked();

    currentReadings['office-plug'] = { state: makeReading({ text_state: 'true' }) };
    rerender(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    currentReadings['office-plug'] = { state: makeReading({ text_state: 'false' }) };
    rerender(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    await waitFor(() =>
      expect(screen.getByRole('checkbox', { name: /toggle office-plug state/i })).not.toBeChecked(),
    );
  });

  it('renders as a read-only switch when the user lacks control permission', () => {
    sensors.splice(0, sensors.length, makeSensor());
    currentReadings['office-plug'] = { state: makeReading() };
    authUser = {
      id: 2,
      username: 'viewer',
      roles: [],
      permissions: ['view_readings'],
    };

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const toggle = screen.getByRole('checkbox', { name: /toggle office-plug state/i });
    expect(toggle).toBeChecked();
    expect(toggle).toHaveAttribute('aria-disabled', 'true');

    fireEvent.click(toggle);

    expect(postMock).not.toHaveBeenCalled();
  });

  it('snaps on when dragged past the late latch', () => {
    sensors.splice(0, sensors.length, makeSensor());
    currentReadings['office-plug'] = { state: makeReading({ text_state: 'DISABLED' }) };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    postMock.mockResolvedValue({
      data: {
        id: 43,
        status: 'sent',
        property: 'state',
        value: 'ENABLED',
      } satisfies SensorCommandAccepted,
    });

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const toggle = screen.getByRole('checkbox', { name: /toggle office-plug state/i });
    const control = screen.getByTestId('sensor-toggle-control');

    expect(toggle).not.toBeChecked();

    fireEvent.pointerDown(control, { clientX: 100, pointerId: 1 });
    fireEvent.pointerMove(control, { clientX: 195, pointerId: 1 });
    fireEvent.pointerUp(control, { clientX: 195, pointerId: 1 });

    expect(postMock).toHaveBeenCalledWith('/sensors/{id}/command', {
      params: { path: { id: 7 } },
      body: { property: 'state', value: 'ENABLED' },
    });
    expect(toggle).toBeChecked();
    expect(reportUpdateMock).toHaveBeenCalledTimes(1);
  });

  it('returns to the original side when dragged short of the midpoint', () => {
    sensors.splice(0, sensors.length, makeSensor());
    currentReadings['office-plug'] = { state: makeReading({ text_state: 'DISABLED' }) };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const toggle = screen.getByRole('checkbox', { name: /toggle office-plug state/i });
    const control = screen.getByTestId('sensor-toggle-control');

    fireEvent.pointerDown(control, { clientX: 100, pointerId: 1 });
    fireEvent.pointerMove(control, { clientX: 135, pointerId: 1 });
    fireEvent.pointerUp(control, { clientX: 135, pointerId: 1 });

    expect(toggle).not.toBeChecked();
    expect(postMock).not.toHaveBeenCalled();
    expect(reportUpdateMock).not.toHaveBeenCalled();
  });

  it('renders the tactile control wider within the existing widget body', () => {
    sensors.splice(0, sensors.length, makeSensor());
    currentReadings['office-plug'] = { state: makeReading() };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    expect(screen.getByTestId('sensor-toggle-control')).toHaveStyle({
      maxWidth: '220px',
      height: '72px',
    });
  });

  it('uses a stronger centered glow around the thumb when switched on', () => {
    sensors.splice(0, sensors.length, makeSensor());
    currentReadings['office-plug'] = { state: makeReading({ text_state: 'ENABLED' }) };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const boxShadow = getComputedStyle(screen.getByTestId('sensor-toggle-thumb')).boxShadow;

    expect(boxShadow).toContain('0 0 18px');
    expect(boxShadow).not.toContain('0 10px 24px');
  });

  it('holds the thumb on its starting side until a late latch is crossed', () => {
    sensors.splice(0, sensors.length, makeSensor());
    currentReadings['office-plug'] = { state: makeReading({ text_state: 'DISABLED' }) };
    authUser = {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['control_sensors'],
    };

    render(
      <SensorToggleWidget
        id="widget-1"
        config={{ sensorId: 7, property: 'state' }}
        isEditing={false}
      />,
    );

    const control = screen.getByTestId('sensor-toggle-control');
    const thumb = screen.getByTestId('sensor-toggle-thumb');

    fireEvent.pointerDown(control, { clientX: 100, pointerId: 1 });
    fireEvent.pointerMove(control, { clientX: 175, pointerId: 1 });

    expect(extractTranslateXPixels(getComputedStyle(thumb).transform)).toBeLessThan(60);

    fireEvent.pointerMove(control, { clientX: 195, pointerId: 1 });

    expect(extractTranslateXPixels(getComputedStyle(thumb).transform)).toBeGreaterThan(84);
  });
});
