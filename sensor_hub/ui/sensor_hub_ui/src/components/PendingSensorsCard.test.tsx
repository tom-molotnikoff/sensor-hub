import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Sensor } from '../gen/aliases';
import PendingSensorsCard from './PendingSensorsCard';

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: {
    GET: getMock,
    POST: vi.fn(),
  },
}));

vi.mock('../providers/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 1,
      username: 'operator',
      roles: [],
      permissions: ['manage_sensors'],
    },
  }),
}));

vi.mock('../tools/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 1,
    name: 'front-door',
    external_id: 'front-door',
    sensor_driver: 'zigbee2mqtt',
    config: {},
    metadata: {},
    health_status: 'good',
    health_reason: 'ok',
    enabled: true,
    status: 'pending',
    retention_hours: null,
    ...overrides,
  };
}

describe('PendingSensorsCard', () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it('shows manufacturer and model alongside the pending device name when available', async () => {
    getMock.mockImplementation(async (_path, options: { params: { path: { status: string } } }) => ({
      data: options.params.path.status === 'pending'
        ? [makeSensor({
            metadata: {
              manufacturer: 'Aqara',
              model: 'MCCGQ11LM',
              description: 'Door/window contact sensor',
            },
          })]
        : [],
    }));

    render(<PendingSensorsCard />);

    expect(await screen.findByText('front-door')).toBeInTheDocument();
    expect(screen.getByText('Aqara MCCGQ11LM · zigbee2mqtt')).toBeInTheDocument();
    expect(screen.queryByText('Door/window contact sensor')).not.toBeInTheDocument();
  });

  it('falls back to the device name when manufacturer and model are absent', async () => {
    getMock.mockImplementation(async (_path, options: { params: { path: { status: string } } }) => ({
      data: options.params.path.status === 'pending'
        ? [makeSensor({
            name: 'garage-motion',
            metadata: {
              description: 'Motion sensor',
            },
          })]
        : [],
    }));

    render(<PendingSensorsCard />);

    expect(await screen.findByText('garage-motion')).toBeInTheDocument();
    expect(screen.queryByText('Motion sensor')).not.toBeInTheDocument();
    expect(screen.queryByText(/Aqara MCCGQ11LM/)).not.toBeInTheDocument();
  });
});
