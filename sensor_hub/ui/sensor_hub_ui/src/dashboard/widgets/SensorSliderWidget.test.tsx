import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CommandStatusMessage, Reading, Sensor, SensorCommandAccepted } from '../../gen/aliases';
import SensorSliderWidget from './SensorSliderWidget';

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
  useSensorContext: () => ({ sensors, loaded: true }),
}));

vi.mock('../../hooks/useCurrentReadings', () => ({
  useCurrentReadings: (options?: { onCommandStatus?: (message: CommandStatusMessage) => void }) => {
    commandStatusHandler = options?.onCommandStatus;
    return currentReadings;
  },
  useCurrentReadingsReady: () => true,
}));

vi.mock('../../providers/AuthContext', () => ({
  useAuth: () => ({ user: authUser }),
}));

vi.mock('../WidgetUpdateContext', () => ({
  useReportWidgetUpdate: () => reportUpdateMock,
}));

const operator = { id: 1, username: 'operator', roles: [], permissions: ['control_sensors'] };

function makeBulb(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 7,
    name: 'attic-bulb',
    external_id: 'attic-bulb',
    sensor_driver: 'mqtt-zigbee2mqtt',
    config: {},
    metadata: {
      exposes: [{ type: 'light', features: [
        { type: 'binary', property: 'state', access: 7, value_on: 'ON', value_off: 'OFF' },
        { type: 'numeric', property: 'brightness', access: 7, value_min: 0, value_max: 254, value_step: 2 },
      ] }],
    },
    capabilities: [
      { property: 'state', type: 'binary', value_on: 'ON', value_off: 'OFF' },
      { property: 'brightness', type: 'numeric', min: 0, max: 254 },
    ],
    health_status: 'good',
    health_reason: 'ok',
    enabled: true,
    status: 'active',
    retention_hours: null,
    ...overrides,
  };
}

function brightnessReading(value: number): Reading {
  return {
    id: 99,
    sensor_name: 'attic-bulb',
    measurement_type: 'brightness',
    numeric_value: value,
    text_state: null,
    unit: '',
    time: '2026-10-09T08:00:00Z',
  };
}

function status(status: CommandStatusMessage['status']): CommandStatusMessage {
  return {
    type: 'command_status',
    id: 42,
    sensor_id: 7,
    property: 'brightness',
    value: '64',
    status,
    acknowledged_at: null,
    acknowledged_value: null,
  };
}

function renderSlider(config: Record<string, unknown> = { sensorId: 7, property: 'brightness' }) {
  return render(<SensorSliderWidget id="widget-1" config={config} isEditing={false} />);
}

function slider() {
  return screen.getByRole('slider', { name: 'Set attic-bulb brightness' });
}

async function slideTo(value: number) {
  fireEvent.change(slider(), { target: { value } });
  await act(() => Promise.resolve());
}

describe('SensorSliderWidget', () => {
  beforeEach(() => {
    sensors.splice(0, sensors.length, makeBulb());
    Object.keys(currentReadings).forEach((key) => delete currentReadings[key]);
    currentReadings['attic-bulb'] = { brightness: brightnessReading(200) };
    authUser = operator;
    commandStatusHandler = undefined;
    postMock.mockReset();
    postMock.mockResolvedValue({
      data: { id: 42, status: 'sent', property: 'brightness', value: '64' } satisfies SensorCommandAccepted,
    });
    reportUpdateMock.mockReset();
  });

  it('shows the reported value on a slider ranged and stepped by the device exposes', () => {
    const { container } = renderSlider();

    expect(container.querySelector('[data-ui=metric-value]')).toHaveTextContent('200');
    expect(slider()).toHaveAttribute('aria-valuenow', '200');
    expect(slider()).toHaveAttribute('min', '0');
    expect(slider()).toHaveAttribute('max', '254');
    expect(slider()).toHaveAttribute('step', '2');
    expect(slider()).not.toBeDisabled();
  });

  it('sends the chosen value as a command and shows it as acknowledged once the device confirms it', async () => {
    renderSlider();

    await slideTo(64);

    expect(postMock).toHaveBeenCalledWith('/sensors/{id}/command', {
      params: { path: { id: 7 } },
      body: { property: 'brightness', value: '64' },
    });
    expect(screen.getByText('Setting to 64…')).toBeInTheDocument();
    expect(slider()).toHaveAttribute('aria-valuenow', '64');

    currentReadings['attic-bulb'] = { brightness: brightnessReading(64) };
    act(() => commandStatusHandler?.(status('acknowledged')));

    expect(await screen.findByText('Acknowledged')).toBeInTheDocument();
    expect(slider()).toHaveAttribute('aria-valuenow', '64');
  });

  it('goes back to the reported value and says so when the command times out', async () => {
    renderSlider();

    await slideTo(64);
    act(() => commandStatusHandler?.(status('timed_out')));

    await waitFor(() => expect(slider()).toHaveAttribute('aria-valuenow', '200'));
    expect(screen.getByRole('alert')).toHaveTextContent('Command timed out');
    expect(screen.queryByText('Setting to 64…')).not.toBeInTheDocument();
  });

  it('shows the command as acknowledged when the acknowledgement beats the reply to the send', async () => {
    let resolvePost: ((value: { data: SensorCommandAccepted }) => void) | undefined;
    postMock.mockImplementation(() => new Promise((resolve) => { resolvePost = resolve; }));
    renderSlider();

    await slideTo(64);
    act(() => commandStatusHandler?.(status('acknowledged')));
    expect(screen.getByText('Setting to 64…')).toBeInTheDocument();

    await act(async () => resolvePost?.({ data: { id: 42, status: 'sent', property: 'brightness', value: '64' } }));

    expect(screen.getByText('Acknowledged')).toBeInTheDocument();
  });

  it('stays inert until the first reading and for a user without control permission', () => {
    delete currentReadings['attic-bulb'];
    const { rerender } = renderSlider();
    expect(slider()).toBeDisabled();
    expect(screen.getByText('Waiting for a reading')).toBeInTheDocument();

    currentReadings['attic-bulb'] = { brightness: brightnessReading(200) };
    authUser = { id: 2, username: 'viewer', roles: [], permissions: [] };
    rerender(<SensorSliderWidget id="widget-1" config={{ sensorId: 7, property: 'brightness' }} isEditing={false} />);
    expect(slider()).toBeDisabled();
  });

  it('asks to be configured when the property is not one of the sensor\'s numeric capabilities', () => {
    renderSlider({ sensorId: 7, property: 'effect' });

    expect(screen.getByText('Select a controllable sensor and numeric property')).toBeInTheDocument();
    expect(screen.queryByRole('slider')).not.toBeInTheDocument();
  });
});
