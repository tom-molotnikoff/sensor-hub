import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { MQTTClientCreated } from '../gen/aliases';
import MqttClientsCard from './MqttClientsCard';

const { getMock, postMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: { GET: getMock, POST: postMock, PUT: vi.fn(), DELETE: vi.fn() },
}));

vi.mock('../providers/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, username: 'admin', roles: [], permissions: ['view_mqtt', 'manage_mqtt'] },
  }),
}));

vi.mock('../tools/logger', () => ({
  logger: { error: vi.fn() },
}));

const created: MQTTClientCreated = {
  id: 7,
  name: 'zigbee2mqtt',
  topic_prefix: 'zigbee2mqtt/',
  enabled: true,
  connected: false,
  last_connected_at: null,
  created_at: '2026-10-08T12:00:00Z',
  updated_at: '2026-10-08T12:00:00Z',
  password: 'a'.repeat(64),
};

describe('MqttClientsCard', () => {
  beforeEach(() => {
    getMock.mockReset().mockResolvedValue({ data: [] });
    postMock.mockReset().mockResolvedValue({ data: created, response: new Response() });
  });

  it('shows a created client password once, dismissed only by Close', async () => {
    render(<MqttClientsCard />);

    fireEvent.click(screen.getByRole('button', { name: 'Add Client' }));
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'zigbee2mqtt' } });
    fireEvent.change(screen.getByRole('textbox', { name: 'Topic prefix' }), { target: { value: 'zigbee2mqtt/' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    const dialog = await screen.findByRole('dialog', { name: 'Password for zigbee2mqtt' });
    expect(postMock).toHaveBeenCalledWith('/mqtt/clients', { body: { name: 'zigbee2mqtt', topic_prefix: 'zigbee2mqtt/' } });
    expect(dialog).toHaveTextContent("This password is shown once. Put it in your device's MQTT config now.");
    expect(screen.getByRole('textbox', { name: 'Password' })).toHaveValue(created.password);
    expect(screen.getByRole('button', { name: 'Copy password' })).toBeInTheDocument();

    fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(screen.getByRole('dialog', { name: 'Password for zigbee2mqtt' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Password for zigbee2mqtt' })).not.toBeInTheDocument());
  });
});
