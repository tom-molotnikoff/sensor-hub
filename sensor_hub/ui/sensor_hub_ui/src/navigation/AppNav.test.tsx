import { ThemeProvider } from '@mui/material';
import { fireEvent, render, screen, waitFor, waitForElementToBeRemoved, within } from '@testing-library/react';
import { useState } from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthContext, type AuthUser } from '../providers/AuthContext';
import { NotificationContext, type NotificationContextValue } from '../providers/NotificationContext';
import { SidebarContext } from '../providers/SidebarContextType';
import { theme } from '../ui/theme';
import AppNav from './AppNav';

const { postMock } = vi.hoisted(() => ({ postMock: vi.fn() }));

vi.mock('../gen/client', () => ({ apiClient: { POST: postMock } }));

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
const accountDestinations = ['Sessions', 'My sessions', 'Change password', 'Developer', 'Documentation', 'Logout'];

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

function renderNav({ as = admin as AuthUser | 'loading', at = '/dashboard', refresh = async () => {} } = {}) {
  render(
    <ThemeProvider theme={theme}>
      <AuthContext.Provider value={{ user: as === 'loading' ? undefined : as, refresh }}>
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

const accountBlock = () => nav().querySelector<HTMLElement>('[data-ui=nav-account]');

const location = () => screen.getByRole('status', { name: 'location', hidden: true });

function openAccountMenu() {
  press(accountBlock()!);
  return screen.getByRole('menu', { name: /^Signed in as / });
}

function outline(menu: HTMLElement) {
  return Array.from(menu.children).map((part) => {
    if (part.getAttribute('role') === 'separator') return 'divider';
    const group = part.querySelector<HTMLElement>('[role=group]');
    if (group) {
      const choices = within(group).getAllByRole('menuitemradio').map((choice) => choice.textContent);
      return `${group.getAttribute('aria-label')}: ${choices.join(' / ')}`;
    }
    return part.querySelector('.MuiListItemText-root')?.textContent ?? part.textContent;
  });
}

describe('AppNav', () => {
  afterEach(() => {
    postMock.mockReset();
    localStorage.clear();
    document.documentElement.className = '';
  });

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

  it('lists only the seven main items for an admin', () => {
    renderNav({ as: admin });

    expect(lists()).toHaveLength(1);
    expect(labels(lists()[0])).toEqual(adminItems);
    for (const destination of accountDestinations) {
      expect(within(nav()).queryByRole('button', { name: destination })).not.toBeInTheDocument();
      expect(within(nav()).queryByRole('link', { name: destination })).not.toBeInTheDocument();
    }
  });

  it.each([
    ['user', user],
    ['viewer', viewer],
  ])('shows a %s only the items their role grants', (_, as) => {
    renderNav({ as });

    expect(lists()).toHaveLength(1);
    expect(labels(lists()[0])).toEqual(['Dashboards', 'MQTT']);
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

    expect(lists()).toHaveLength(1);
    expect(labels(lists()[0])).toEqual(items);
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
    expect(location()).toHaveTextContent('/sensors-overview');
  });

  it.each([
    ['loading', 'loading' as const],
    ['signed out', null],
  ])('has no account block while %s', (_, as) => {
    renderNav({ as });

    expect(accountBlock()).toBeNull();
  });

  it.each([
    [['admin'], 'admin'],
    [['admin', 'viewer'], 'admin, viewer'],
  ])('shows the avatar initial, username, roles %j and an unfold icon at the foot', (roles, joined) => {
    renderNav({ as: { ...admin, roles } });

    const foot = nav().querySelector<HTMLElement>('[data-ui=nav-foot]')!;
    expect(foot).toBe(nav().lastElementChild);
    const block = accountBlock()!;
    expect(foot).toContainElement(block);
    expect(block.querySelector('.MuiAvatar-root')).toHaveTextContent(/^T$/);
    expect(within(block).getByText('testadmin')).toBeInTheDocument();
    expect(within(block).getByText(joined)).toHaveClass('MuiTypography-noWrap');
    expect(within(block).getByTestId('UnfoldMoreIcon')).toBeInTheDocument();
    expect(block).toHaveAttribute('aria-haspopup', 'menu');
    expect(block).toHaveAttribute('aria-expanded', 'false');
  });

  it('opens the account menu with everything about the admin, in order', () => {
    renderNav({ as: admin });
    const block = accountBlock();

    const menu = openAccountMenu();

    expect(block).toHaveAttribute('aria-expanded', 'true');
    expect(document.getElementById(block!.getAttribute('aria-controls')!)).toContainElement(menu);
    expect(outline(menu)).toEqual([
      'Signed in as testadmin',
      'Theme: Light / Dark / System',
      'divider',
      'My sessions',
      'Change password',
      'Developer',
      'Documentation',
      'divider',
      'Logout',
    ]);
  });

  it.each([
    ['manage_api_keys', true],
    ['view_api_docs', true],
    ['view_dashboards', false],
  ])('gates Developer on %s', (permission, shown) => {
    renderNav({ as: { id: 4, username: 'someone', roles: ['user'], permissions: [permission] } });

    const items = outline(openAccountMenu());

    expect(items.includes('Developer')).toBe(shown);
    expect(items.filter((item) => item !== 'Developer')).toEqual([
      'Signed in as someone',
      'Theme: Light / Dark / System',
      'divider',
      'My sessions',
      'Change password',
      'Documentation',
      'divider',
      'Logout',
    ]);
  });

  it('names the menu by its heading and keeps only list items in it', () => {
    renderNav();

    const menu = openAccountMenu();

    expect(menu).toHaveAccessibleName('Signed in as testadmin');
    expect(menu.tagName).toBe('UL');
    expect(Array.from(menu.children).map((part) => part.tagName)).toEqual(Array(menu.children.length).fill('LI'));
    expect(within(menu).getByRole('group', { name: 'Theme' }).parentElement).toHaveAttribute('role', 'none');
    expect(document.getElementById(menu.getAttribute('aria-labelledby')!)!.parentElement).toHaveAttribute('role', 'none');
  });

  it('switches to dark mode and shows Dark as selected', async () => {
    renderNav();

    press(within(openAccountMenu()).getByRole('menuitemradio', { name: 'Dark' }));

    await waitFor(() => expect(document.documentElement).toHaveClass('dark'));
    const menu = screen.getByRole('menu', { name: 'Signed in as testadmin' });
    expect(within(menu).getByRole('menuitemradio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true');
    expect(within(menu).getByRole('menuitemradio', { name: 'Light' })).toHaveAttribute('aria-checked', 'false');
    expect(within(menu).getByRole('menuitemradio', { name: 'System' })).toHaveAttribute('aria-checked', 'false');
  });

  it('opens the documentation in a new tab and says so', () => {
    renderNav();

    const docs = within(openAccountMenu()).getByRole('menuitem', { name: 'Documentation opens in a new tab' });

    expect(docs).toHaveAttribute('href', '/docs/');
    expect(docs).toHaveAttribute('target', '_blank');
    expect(docs).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it.each([
    ['My sessions', '/account/sessions'],
    ['Change password', '/account/change-password'],
    ['Developer', '/account/developer'],
  ])('closes the menu and the drawer and goes to %s', async (item, path) => {
    renderNav();

    press(within(openAccountMenu()).getByRole('menuitem', { name: item }));

    await waitForElementToBeRemoved(nav);
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    expect(location()).toHaveTextContent(path);
  });

  it('logs out, refreshes auth and lands on /login', async () => {
    postMock.mockResolvedValue({});
    const refresh = vi.fn(async () => {});
    renderNav({ refresh });

    press(within(openAccountMenu()).getByRole('menuitem', { name: 'Logout' }));

    await waitFor(() => expect(location()).toHaveTextContent('/login'));
    expect(postMock).toHaveBeenCalledWith('/auth/logout');
    expect(refresh).toHaveBeenCalledOnce();
  });
});

const notifications: NotificationContextValue = {
  notifications: [],
  unreadCount: 6,
  preferences: [],
  loading: false,
  refresh: async () => {},
  markAsRead: async () => {},
  dismiss: async () => {},
  markAllAsRead: async () => {},
  dismissAll: async () => {},
  updatePreference: async () => {},
};

function renderPermanentNav(as: AuthUser = admin) {
  render(
    <ThemeProvider theme={theme}>
      <AuthContext.Provider value={{ user: as, refresh: async () => {} }}>
        <NotificationContext.Provider value={notifications}>
          <MemoryRouter initialEntries={['/dashboard']}>
            <AppNav permanent />
          </MemoryRouter>
        </NotificationContext.Provider>
      </AuthContext.Provider>
    </ThemeProvider>,
  );
}

const brandParts = () =>
  Array.from(nav().querySelector('[data-ui=nav-brand]')!.children).map(
    (part) => part.getAttribute('aria-label') ?? part.getAttribute('src') ?? part.textContent,
  );

describe('AppNav permanent', () => {
  it('is on screen without opening it and has no close button', () => {
    renderPermanentNav();

    expect(nav().tagName).toBe('NAV');
    expect(labels(lists()[0])).toEqual(adminItems);
    expect(within(nav()).queryByRole('button', { name: 'close navigation' })).not.toBeInTheDocument();
  });

  it('shows the logo, the name and the bell with the unread count, left to right', () => {
    renderPermanentNav();

    expect(brandParts()).toEqual(['/sensor_hub.svg', 'Sensor Hub', 'notifications']);
    expect(within(nav()).getByRole('button', { name: 'notifications' })).toHaveTextContent('6');
  });

  it('shows only the logo and the name to a user without view_notifications', () => {
    renderPermanentNav({ id: 4, username: 'someone', roles: ['user'], permissions: ['view_dashboards'] });

    expect(brandParts()).toEqual(['/sensor_hub.svg', 'Sensor Hub']);
  });
});
