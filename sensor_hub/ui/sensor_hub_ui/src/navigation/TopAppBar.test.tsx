import { ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
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

const barButtons = () =>
  Array.from(screen.getByRole('banner').querySelectorAll('button, a')).map((control) => control.getAttribute('aria-label'));

describe('TopAppBar', () => {
  it('shows only the menu, title and bell, with no avatar', () => {
    renderBar();

    expect(barButtons()).toEqual(['menu', 'notifications']);
    expect(screen.getByRole('heading', { name: 'Alerts & Notifications' })).toBeInTheDocument();
    expect(screen.queryByText('Sensor Hub')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'account' })).not.toBeInTheDocument();
  });
});
