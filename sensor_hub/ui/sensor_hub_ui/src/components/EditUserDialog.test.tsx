import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { User } from '../gen/aliases';
import EditUserDialog from './EditUserDialog';

const { getMock, postMock, putMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
  putMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: { GET: getMock, POST: postMock, PUT: putMock },
}));

vi.mock('../providers/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, username: 'admin', roles: ['admin'], permissions: ['manage_users'] } }),
}));

vi.mock('../tools/logger', () => ({
  logger: { error: vi.fn() },
}));

const viewer: User = {
  id: 7,
  username: 'viewer',
  email: 'viewer@example.com',
  disabled: false,
  must_change_password: false,
  roles: ['viewer'],
  permissions: [],
  created_at: '2026-10-08T12:00:00Z',
  updated_at: '2026-10-08T12:00:00Z',
};

function ok(data?: unknown) {
  return { data, response: new Response(null, { status: 200 }) };
}

describe('EditUserDialog', () => {
  beforeEach(() => {
    getMock.mockReset().mockResolvedValue({ data: [{ id: 1, name: 'admin' }, { id: 3, name: 'viewer' }] });
    postMock.mockReset().mockResolvedValue(ok());
    putMock.mockReset().mockResolvedValue(ok({ message: 'user disabled' }));
  });

  it('disables the user through the disabled endpoint when the switch is turned on', async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn().mockResolvedValue(undefined);
    render(<EditUserDialog open onClose={onClose} onSaved={onSaved} selectedUser={viewer} />);

    const toggle = screen.getByRole('switch', { name: 'Disabled' });
    expect(toggle).not.toBeChecked();
    fireEvent.click(toggle);
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(putMock).toHaveBeenCalledWith('/users/{id}/disabled', { params: { path: { id: 7 } }, body: { disabled: true } });
    expect(onClose).toHaveBeenCalled();
  });

  it('leaves the disabled flag alone when the switch is not changed', async () => {
    const onSaved = vi.fn().mockResolvedValue(undefined);
    render(<EditUserDialog open onClose={vi.fn()} onSaved={onSaved} selectedUser={viewer} />);

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(putMock).not.toHaveBeenCalled();
  });

  it('shows the API message and stays open when the change is refused', async () => {
    putMock.mockResolvedValue({
      error: { message: 'disabling this user would leave no enabled admin' },
      response: new Response(null, { status: 409, statusText: 'Conflict' }),
    });
    const onClose = vi.fn();
    render(<EditUserDialog open onClose={onClose} onSaved={vi.fn()} selectedUser={viewer} />);

    fireEvent.click(screen.getByRole('switch', { name: 'Disabled' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('disabling this user would leave no enabled admin')).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(postMock).not.toHaveBeenCalled();
  });

  it('does not let users disable themselves', () => {
    render(<EditUserDialog open onClose={vi.fn()} onSaved={vi.fn()} selectedUser={{ ...viewer, id: 1, username: 'admin' }} />);

    expect(screen.getByRole('switch', { name: 'Disabled (you cannot disable yourself)' })).toBeDisabled();
  });
});
