import { ThemeProvider } from '@mui/material';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthContext } from '../providers/AuthContext';
import { theme } from '../ui/theme';
import TopAppBar from './TopAppBar';

const admin = { id: 1, username: 'tom', roles: ['admin'], permissions: ['view_notifications'] };

function atWidth(width: number) {
  vi.stubGlobal('matchMedia', (query: string) => {
    const minWidth = /\(min-width:\s*(\d+)px\)/.exec(query)?.[1];
    const maxWidth = /\(max-width:\s*(\d+)px\)/.exec(query)?.[1];
    return {
      matches: (minWidth === undefined || width >= Number(minWidth)) && (maxWidth === undefined || width <= Number(maxWidth)),
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    };
  });
}

function renderBar(width: number) {
  atWidth(width);
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

const accountMenuEntries = () => {
  fireEvent.click(screen.getByRole('button', { name: 'account' }));
  return within(screen.getByRole('menu')).getAllByRole('menuitem').map((item) => item.textContent);
};

describe('TopAppBar', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('shows only the menu, title, bell and avatar at 390px', () => {
    renderBar(390);

    expect(barButtons()).toEqual(['menu', 'notifications', 'account']);
    expect(screen.getByRole('heading', { name: 'Alerts & Notifications' })).toBeInTheDocument();
    expect(screen.queryByText('Sensor Hub')).not.toBeInTheDocument();
  });

  it('moves Theme and Documentation into the avatar menu at 390px', () => {
    renderBar(390);

    expect(accountMenuEntries()).toEqual(['Theme', 'Documentation', 'My sessions', 'Change password', 'Logout']);
    expect(screen.getByRole('menuitem', { name: 'Documentation' })).toHaveAttribute('href', '/docs/');
  });

  it('opens the theme choices from the avatar menu at 390px', () => {
    renderBar(390);

    fireEvent.click(screen.getByRole('button', { name: 'account' }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Theme' }));

    const choices = within(screen.getByRole('menu')).getAllByRole('menuitem').map((item) => item.textContent);
    expect(choices).toEqual(['Light', 'Dark', 'System']);
  });

  it('keeps the brand, theme switcher and documentation icons at 1440px', () => {
    renderBar(1440);

    expect(barButtons()).toEqual(['menu', 'notifications', 'theme switcher', 'documentation', 'account']);
    expect(screen.getByText('Sensor Hub')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'documentation' })).toHaveAttribute('href', '/docs/');
  });

  it('has no Theme or Documentation in the avatar menu at 1440px', () => {
    renderBar(1440);

    expect(accountMenuEntries()).toEqual(['My sessions', 'Change password', 'Logout']);
  });
});
