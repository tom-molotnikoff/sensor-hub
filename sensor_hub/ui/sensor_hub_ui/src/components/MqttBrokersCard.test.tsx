import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { MQTTBroker } from '../gen/aliases';
import MqttBrokersCard from './MqttBrokersCard';

const { getMock, postMock, putMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
  putMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: { GET: getMock, POST: postMock, PUT: putMock, DELETE: vi.fn() },
}));

vi.mock('../providers/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, username: 'admin', roles: [], permissions: ['view_mqtt', 'manage_mqtt'] },
  }),
}));

vi.mock('../tools/logger', () => ({
  logger: { error: vi.fn() },
}));

const broker: MQTTBroker = {
  id: 3,
  name: 'home',
  type: 'external',
  host: 'mqtt.home.lan',
  port: 1883,
  username: 'hub',
  client_id: 'sensor-hub-home',
  password_status: 'set',
  enabled: true,
  created_at: '2026-10-08T12:00:00Z',
  updated_at: '2026-10-08T12:00:00Z',
};

async function openRowMenu() {
  render(<MqttBrokersCard />);
  fireEvent.click(await screen.findByText('home'));
}

async function openEditDialog() {
  await openRowMenu();
  fireEvent.click(screen.getByRole('menuitem', { name: 'Edit' }));
  return screen.findByRole('dialog', { name: 'Edit MQTT Broker' });
}

function sentBody() {
  expect(putMock).toHaveBeenCalledTimes(1);
  return putMock.mock.calls[0][1].body as Record<string, unknown>;
}

describe('MqttBrokersCard', () => {
  beforeEach(() => {
    getMock.mockReset().mockResolvedValue({ data: [broker] });
    postMock.mockReset().mockResolvedValue({ data: { id: 3 }, response: new Response() });
    putMock.mockReset().mockResolvedValue({ data: { message: 'ok' }, response: new Response() });
  });

  it('toggles a broker without dropping its username or sending a password', async () => {
    await openRowMenu();
    fireEvent.click(screen.getByRole('menuitem', { name: 'Disable' }));

    await waitFor(() => expect(putMock).toHaveBeenCalled());
    const body = sentBody();
    expect(body).toMatchObject({ name: 'home', username: 'hub', client_id: 'sensor-hub-home', enabled: false });
    expect(body).not.toHaveProperty('password');
    expect(body).not.toHaveProperty('password_status');
  });

  it('shows a stored password as an empty field reading "unchanged" and keeps it when left empty', async () => {
    const dialog = await openEditDialog();
    const password = within(dialog).getByLabelText('Password');
    expect(password).toHaveValue('');
    expect(password).toHaveAttribute('placeholder', 'unchanged');

    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(putMock).toHaveBeenCalled());
    expect(sentBody()).not.toHaveProperty('password');
  });

  it('replaces the stored password with one typed in', async () => {
    const dialog = await openEditDialog();
    fireEvent.change(within(dialog).getByLabelText('Password'), { target: { value: 'new-secret' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(putMock).toHaveBeenCalled());
    expect(sentBody()).toMatchObject({ password: 'new-secret' });
  });

  it('clears the stored password by sending an empty one', async () => {
    const dialog = await openEditDialog();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Clear' }));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(putMock).toHaveBeenCalled());
    expect(sentBody()).toMatchObject({ password: '' });
  });

  it('offers no Clear for a broker without a password', async () => {
    getMock.mockResolvedValue({ data: [{ ...broker, password_status: 'unset' }] });
    const dialog = await openEditDialog();

    expect(within(dialog).queryByRole('button', { name: 'Clear' })).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText('Password')).not.toHaveAttribute('placeholder', 'unchanged');
  });
});
