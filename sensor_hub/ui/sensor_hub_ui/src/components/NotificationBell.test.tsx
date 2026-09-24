import { ThemeProvider } from '@mui/material';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';
import type { UserNotification } from '../gen/aliases';
import { NotificationContext, type NotificationContextValue } from '../providers/NotificationContext';
import { theme } from '../ui/theme';
import NotificationBell from './NotificationBell';

function makeNotification(id: number, isRead: boolean): UserNotification {
  return {
    notification_id: id,
    is_read: isRead,
    notification: {
      title: `Title ${id}`,
      message: `Message ${id}`,
      severity: 'warning',
      created_at: new Date().toISOString(),
    },
  } as UserNotification;
}

function renderBell(overrides: Partial<NotificationContextValue>) {
  const value: NotificationContextValue = {
    notifications: [],
    unreadCount: 0,
    preferences: [],
    loading: false,
    refresh: async () => {},
    markAsRead: vi.fn(async () => {}),
    dismiss: async () => {},
    markAllAsRead: async () => {},
    dismissAll: async () => {},
    updatePreference: async () => {},
    ...overrides,
  };
  render(
    <ThemeProvider theme={theme}>
      <MemoryRouter>
        <NotificationContext.Provider value={value}>
          <NotificationBell />
        </NotificationContext.Provider>
      </MemoryRouter>
    </ThemeProvider>,
  );
  const bell = screen.getByRole('button', { name: 'notifications' });
  bell.focus();
  fireEvent.click(bell);
  return value;
}

describe('NotificationBell', () => {
  it('lists recent notifications with the unread count and a link to all of them', () => {
    const value = renderBell({ notifications: [makeNotification(1, false), makeNotification(2, true)], unreadCount: 1 });

    expect(screen.getByText('1 unread')).toBeInTheDocument();
    expect(screen.getByText('Title 1')).toBeInTheDocument();
    expect(screen.getByText('Message 2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View all notifications' })).toBeInTheDocument();

    fireEvent.click(screen.getByText('Title 1'));
    expect(value.markAsRead).toHaveBeenCalledWith(1);
  });

  it('says there are no notifications when the list is empty', () => {
    renderBell({});

    expect(screen.getByText('No notifications')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'View all notifications' })).toBeNull();
  });

  it('shows a spinner while notifications load', () => {
    renderBell({ loading: true });

    expect(screen.getByRole('status', { name: 'Loading' })).toBeInTheDocument();
    expect(screen.queryByText('No notifications')).toBeNull();
  });
});
