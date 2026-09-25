import { ThemeProvider } from '@mui/material';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';
import { AuthContext } from '../providers/AuthContext';
import { theme } from '../ui/theme';
import TopAppBar from './TopAppBar';

const admin = { id: 1, username: 'tom', roles: ['admin'], permissions: ['view_notifications'] };

function renderBar() {
  return render(
    <ThemeProvider theme={theme}>
      <AuthContext.Provider value={{ user: admin, refresh: async () => {} }}>
        <MemoryRouter>
          <TopAppBar pageTitle="Alerts & Notifications" />
        </MemoryRouter>
      </AuthContext.Provider>
    </ThemeProvider>,
  );
}

function press(control: HTMLElement) {
  control.focus();
  fireEvent.click(control);
}

const barButtons = () =>
  Array.from(screen.getByRole('banner').querySelectorAll('button, a')).map((control) => control.getAttribute('aria-label'));

const accountMenuEntries = () => {
  press(screen.getByRole('button', { name: 'account' }));
  return within(screen.getByRole('menu')).getAllByRole('menuitem').map((item) => item.textContent);
};

describe('TopAppBar', () => {
  it('shows only the menu, title, bell and avatar', () => {
    renderBar();

    expect(barButtons()).toEqual(['menu', 'notifications', 'account']);
    expect(screen.getByRole('heading', { name: 'Alerts & Notifications' })).toBeInTheDocument();
    expect(screen.queryByText('Sensor Hub')).not.toBeInTheDocument();
  });

  it('keeps Theme and Documentation in the avatar menu', () => {
    renderBar();

    expect(accountMenuEntries()).toEqual(['Theme', 'Documentation', 'My sessions', 'Change password', 'Logout']);
    expect(screen.getByRole('menuitem', { name: 'Documentation' })).toHaveAttribute('href', '/docs/');
  });

  it('opens the theme choices from the avatar menu', () => {
    renderBar();

    press(screen.getByRole('button', { name: 'account' }));
    press(screen.getByRole('menuitem', { name: 'Theme' }));

    const choices = within(screen.getByRole('menu')).getAllByRole('menuitem').map((item) => item.textContent);
    expect(choices).toEqual(['Light', 'Dark', 'System']);
  });
});
