import { ThemeProvider } from '@mui/material';
import { fireEvent, render, screen, waitForElementToBeRemoved, within } from '@testing-library/react';
import { useState } from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { describe, expect, it } from 'vitest';
import { AuthContext, type AuthUser } from '../providers/AuthContext';
import { SidebarContext } from '../providers/SidebarContextType';
import { theme } from '../ui/theme';
import AppNav from './AppNav';

const admin = { id: 1, username: 'testadmin', roles: ['admin'], permissions: [] };
const user = {
  id: 2,
  username: 'testuser',
  roles: ['user'],
  permissions: [
    'view_dashboards',
    'manage_dashboards',
    'view_api_docs',
    'manage_api_keys',
    'view_mqtt',
    'view_drivers',
    'view_measurement_types',
    'control_sensors',
  ],
};
const viewer = {
  id: 3,
  username: 'testviewer',
  roles: ['viewer'],
  permissions: ['view_dashboards', 'view_api_docs', 'manage_api_keys', 'view_mqtt', 'view_drivers', 'view_measurement_types'],
};

const adminItems = ['Dashboards', 'Sensors', 'Data Retention', 'Properties', 'MQTT', 'Alerts & Notifications', 'User Management'];
const accountItems = ['Sessions', 'Developer', 'Documentation', 'Logout'];

function Location() {
  return <output aria-label="location">{useLocation().pathname}</output>;
}

function Shell() {
  const [open, setOpen] = useState(false);
  return (
    <SidebarContext.Provider value={{ open, setOpen }}>
      <button onClick={() => setOpen(true)}>menu</button>
      <AppNav />
    </SidebarContext.Provider>
  );
}

function press(control: HTMLElement) {
  control.focus();
  fireEvent.click(control);
}

function renderNav({ as = admin as AuthUser | 'loading', at = '/dashboard' } = {}) {
  render(
    <ThemeProvider theme={theme}>
      <AuthContext.Provider value={{ user: as === 'loading' ? undefined : as, refresh: async () => {} }}>
        <MemoryRouter initialEntries={[at]}>
          <Shell />
          <Routes>
            <Route path="*" element={<Location />} />
          </Routes>
        </MemoryRouter>
      </AuthContext.Provider>
    </ThemeProvider>,
  );
  press(screen.getByRole('button', { name: 'menu' }));
}

const nav = () => screen.getByRole('navigation', { name: 'Main' });

const lists = () => Array.from(nav().querySelectorAll('[data-ui=nav-list]'));

const labels = (list: Element) => Array.from(list.querySelectorAll('[data-ui=nav-item]')).map((item) => item.textContent);

const current = () => within(nav()).queryAllByRole('button', { current: 'page' }).map((item) => item.textContent);

describe('AppNav', () => {
  it('is a navigation landmark labelled Main', () => {
    renderNav();

    expect(nav().tagName).toBe('NAV');
  });

  it('shows the logo, the name and a close button in the brand row', async () => {
    renderNav();

    const brand = nav().querySelector<HTMLElement>('[data-ui=nav-brand]')!;
    expect(brand.querySelector('img')).toHaveAttribute('src', '/sensor_hub.svg');
    expect(brand).toHaveTextContent('Sensor Hub');
    press(within(brand).getByRole('button', { name: 'close navigation' }));
    await waitForElementToBeRemoved(nav);
  });

  it('lists the seven main items for an admin, then the account items below a divider', () => {
    renderNav({ as: admin });

    const [main, account] = lists();
    expect(labels(main)).toEqual(adminItems);
    expect(main.nextElementSibling?.tagName).toBe('HR');
    expect(labels(account)).toEqual(accountItems);
    expect(within(account as HTMLElement).getByRole('link', { name: 'Documentation' })).toHaveAttribute('href', '/docs/');
  });

  it.each([
    ['user', user],
    ['viewer', viewer],
  ])('shows a %s only the items their role grants', (_, as) => {
    renderNav({ as });

    const [main, account] = lists();
    expect(labels(main)).toEqual(['Dashboards', 'MQTT']);
    expect(labels(account)).toEqual(accountItems);
  });

  it.each([
    ['view_dashboards', ['Dashboards']],
    ['view_sensors', ['Sensors', 'Data Retention']],
    ['view_properties', ['Properties']],
    ['view_mqtt', ['MQTT']],
    ['view_alerts', ['Alerts & Notifications']],
    ['view_notifications', ['Alerts & Notifications']],
    ['manage_notifications', ['Alerts & Notifications']],
    ['manage_oauth', ['Alerts & Notifications']],
    ['view_users', ['User Management']],
    ['view_roles', ['User Management']],
  ])('gates items on %s', (permission, items) => {
    renderNav({ as: { id: 4, username: 'someone', roles: [], permissions: [permission] } });

    const [main, account] = lists();
    expect(labels(main)).toEqual(items);
    expect(labels(account)).toEqual(['Sessions', 'Documentation', 'Logout']);
  });

  it.each([
    ['/dashboard', 'Dashboards'],
    ['/sensors-overview', 'Sensors'],
    ['/sensor/7', 'Sensors'],
    ['/data-retention', 'Data Retention'],
    ['/properties-overview', 'Properties'],
    ['/mqtt', 'MQTT'],
    ['/notifications', 'Alerts & Notifications'],
    ['/admin', 'User Management'],
  ])('marks only %s as the current page with %s', (at, item) => {
    renderNav({ at });

    expect(current()).toEqual([item]);
    expect(Array.from(nav().querySelectorAll('.Mui-selected')).map((selected) => selected.textContent)).toEqual([item]);
  });

  it.each(['/account/sessions', '/account/change-password', '/account/developer'])('marks nothing as current on %s', (at) => {
    renderNav({ at });

    expect(current()).toEqual([]);
    expect(nav().querySelectorAll('.Mui-selected')).toHaveLength(0);
  });

  it('shows skeleton rows instead of items while auth loads', () => {
    renderNav({ as: 'loading' });

    expect(nav().querySelectorAll('[data-ui=nav-skeleton] .MuiSkeleton-root')).toHaveLength(adminItems.length);
    expect(nav().querySelectorAll('[data-ui=nav-item]')).toHaveLength(0);
    expect(within(nav()).queryByText('Loading...')).not.toBeInTheDocument();
  });

  it.each([
    ['loading', 'loading' as const],
    ['signed out', null],
    ['signed in', admin],
  ])('has no Login item while %s', (_, as) => {
    renderNav({ as });

    expect(within(nav()).queryByText('Login')).not.toBeInTheDocument();
  });

  it('closes the drawer and navigates when an item is picked', async () => {
    renderNav({ at: '/dashboard' });

    press(within(nav()).getByRole('button', { name: 'Sensors' }));

    await waitForElementToBeRemoved(nav);
    expect(screen.getByRole('status', { name: 'location', hidden: true })).toHaveTextContent('/sensors-overview');
  });
});
