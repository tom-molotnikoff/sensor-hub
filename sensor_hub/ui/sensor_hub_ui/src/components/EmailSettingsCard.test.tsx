import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { EmailSettings } from '../gen/aliases';
import EmailSettingsCard from './EmailSettingsCard';

const { getMock, postMock, putMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
  putMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: { GET: getMock, POST: postMock, PUT: putMock, DELETE: vi.fn() },
}));

vi.mock('../tools/logger', () => ({
  logger: { error: vi.fn() },
}));

const saved: EmailSettings = {
  host: 'smtp.example.com',
  port: 587,
  security: 'starttls',
  username: 'hub',
  from_address: 'alerts@example.com',
  password_status: 'set',
  last_error: null,
  last_sent_at: null,
};

function ok<T>(data: T) {
  return Promise.resolve({ data, response: new Response() });
}

function failed(status: number, message: string) {
  return Promise.resolve({ error: { message }, response: new Response(null, { status }) });
}

async function renderCard() {
  render(<EmailSettingsCard />);
  return screen.findByLabelText('Password');
}

function sentBody() {
  return putMock.mock.calls.at(-1)![1].body as Record<string, unknown>;
}

describe('EmailSettingsCard', () => {
  beforeEach(() => {
    getMock.mockReset().mockImplementation(() => ok(saved));
    putMock.mockReset().mockImplementation(() => ok(saved));
    postMock.mockReset().mockImplementation(() => ok({ message: 'Test email sent to admin@example.com' }));
  });

  it('shows the saved settings and that email is ready to send', async () => {
    await renderCard();

    expect(screen.getByLabelText(/Host/)).toHaveValue('smtp.example.com');
    expect(screen.getByLabelText('Port')).toHaveValue(587);
    expect(screen.getByLabelText('Username')).toHaveValue('hub');
    expect(screen.getByLabelText('From address')).toHaveValue('alerts@example.com');
    expect(screen.getByText('Ready to send')).toBeInTheDocument();
    expect(screen.getByText('Last sent: Never')).toBeInTheDocument();
  });

  it('is not ready to send without a stored password', async () => {
    getMock.mockImplementation(() => ok({ ...saved, password_status: 'unset' }));
    await renderCard();

    expect(screen.getByText('Not ready to send')).toBeInTheDocument();
    expect(screen.queryByText('Ready to send')).not.toBeInTheDocument();
  });

  it('shows a stored password as an empty field reading "unchanged" and keeps it when left empty', async () => {
    const password = await renderCard();
    expect(password).toHaveValue('');
    expect(password).toHaveAttribute('placeholder', 'unchanged');

    fireEvent.change(screen.getByLabelText(/Host/), { target: { value: 'mail.example.com' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(putMock).toHaveBeenCalled());
    const body = sentBody();
    expect(body).toMatchObject({
      host: 'mail.example.com', port: 587, security: 'starttls', username: 'hub', from_address: 'alerts@example.com',
    });
    expect(body).not.toHaveProperty('password');
    expect(body).not.toHaveProperty('password_status');
    expect(await screen.findByText('Email settings saved.')).toBeInTheDocument();
  });

  it('replaces the stored password with one typed in', async () => {
    const password = await renderCard();
    fireEvent.change(password, { target: { value: 'new-secret' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(putMock).toHaveBeenCalled());
    expect(sentBody()).toMatchObject({ password: 'new-secret' });
  });

  it('clears the stored password by sending an empty one', async () => {
    await renderCard();
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }));
    expect(screen.getByText('The stored password will be removed.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(putMock).toHaveBeenCalled());
    expect(sentBody()).toMatchObject({ password: '' });
  });

  it('offers no Clear when no password is stored', async () => {
    getMock.mockImplementation(() => ok({ ...saved, password_status: 'unset' }));
    const password = await renderCard();

    expect(screen.queryByRole('button', { name: 'Clear' })).not.toBeInTheDocument();
    expect(password).not.toHaveAttribute('placeholder', 'unchanged');
  });

  it('marks a password that needs re-entry, and says what to do', async () => {
    getMock.mockImplementation(() => ok({ ...saved, password_status: 'needs_reentry' }));
    await renderCard();

    expect(screen.getByText('Needs re-entry')).toBeInTheDocument();
    expect(screen.getByText('This secret could not be decrypted. Enter it again.')).toBeInTheDocument();
    expect(screen.getByText('Not ready to send')).toBeInTheDocument();
  });

  it('warns that nothing is encrypted when security is none', async () => {
    await renderCard();
    expect(screen.queryByText(/cross the network unencrypted/)).not.toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Security' }));
    fireEvent.click(within(screen.getByRole('listbox')).getByRole('option', { name: 'None (unencrypted)' }));

    expect(screen.getByText(/the password and every message cross the network unencrypted/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(putMock).toHaveBeenCalled());
    expect(sentBody()).toMatchObject({ security: 'none' });
  });

  it('shows the warning for saved settings with no security', async () => {
    getMock.mockImplementation(() => ok({ ...saved, security: 'none' }));
    await renderCard();

    expect(screen.getByText(/the password and every message cross the network unencrypted/)).toBeInTheDocument();
  });

  it('shows the last send time and the last error', async () => {
    getMock.mockImplementation(() => ok({
      ...saved, last_sent_at: '2026-10-08T12:00:00Z', last_error: '535 5.7.8 Authentication failed',
    }));
    await renderCard();

    expect(screen.getByText(`Last sent: ${new Date('2026-10-08T12:00:00Z').toLocaleString()}`)).toBeInTheDocument();
    expect(screen.getByText('Last error: 535 5.7.8 Authentication failed')).toBeInTheDocument();
  });

  it('shows the server message when saving is rejected', async () => {
    putMock.mockImplementation(() => failed(400, 'from_address must be an email address'));
    await renderCard();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('from_address must be an email address')).toBeInTheDocument();
  });

  it('sends a test email, shows the result and the new last sent time', async () => {
    await renderCard();
    const sentAt = '2026-10-09T09:15:00Z';
    getMock.mockImplementation(() => ok({ ...saved, last_sent_at: sentAt }));
    fireEvent.click(screen.getByRole('button', { name: 'Send test email' }));

    expect(await screen.findByText('Test email sent to admin@example.com')).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith('/email/smtp/test');
    expect(await screen.findByText(`Last sent: ${new Date(sentAt).toLocaleString()}`)).toBeInTheDocument();
  });

  it("shows the SMTP server's error when the test email fails, and the updated last error", async () => {
    await renderCard();
    postMock.mockImplementation(() => failed(502, '535 5.7.8 Authentication failed'));
    getMock.mockImplementation(() => ok({ ...saved, last_error: '535 5.7.8 Authentication failed' }));
    fireEvent.click(screen.getByRole('button', { name: 'Send test email' }));

    expect(await screen.findByText('535 5.7.8 Authentication failed')).toBeInTheDocument();
    expect(await screen.findByText('Last error: 535 5.7.8 Authentication failed')).toBeInTheDocument();
  });
});
