import { ThemeProvider } from '@mui/material';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Dashboard, DashboardConfig } from '../gen/aliases';
import { theme } from '../ui/theme';
import DashboardPage from './DashboardPage';

function dashboard(id: number, name: string, isDefault: boolean): Dashboard {
  return {
    id,
    user_id: 1,
    name,
    config: '{"widgets":[]}',
    shared: false,
    is_default: isDefault,
    created_at: '2026-09-25T00:00:00Z',
    updated_at: '2026-09-25T00:00:00Z',
  };
}

const layout = dashboard(1, 'Layout', true);
const kitchen = dashboard(2, 'Kitchen', false);

const dashboardState = {
  dashboards: [] as Dashboard[],
  activeDashboard: null as Dashboard | null,
  config: { widgets: [] } as DashboardConfig,
  isEditing: false,
  loading: true,
  setIsEditing: vi.fn(),
  setActiveDashboard: vi.fn(),
  updateWidgets: vi.fn(),
  addWidget: vi.fn(),
  removeWidget: vi.fn(),
  updateWidgetConfig: vi.fn(),
  saveDashboard: vi.fn(),
  createDashboard: vi.fn(),
  deleteDashboard: vi.fn(),
  refreshDashboards: vi.fn(),
};

vi.mock('./DashboardContext', () => ({ useDashboard: () => dashboardState }));
vi.mock('./DashboardProvider', () => ({
  DashboardProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
const authState = { user: { id: 1, username: 'tom', roles: [] as string[], permissions: [] as string[] } };

vi.mock('../providers/AuthContext', () => ({ useAuth: () => authState }));
vi.mock('../navigation/AppNav', () => ({ default: () => <nav>sidebar</nav> }));
vi.mock('../navigation/TopAppBar', () => ({
  default: ({ pageTitle }: { pageTitle: string }) => <header>{pageTitle}</header>,
}));

function atWidth(width: number) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: Number(/\(min-width:\s*(\d+)px\)/.exec(query)?.[1] ?? 0) <= width,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }));
}

function renderLoadedPage(width: number) {
  atWidth(width);
  Object.assign(dashboardState, { loading: false, dashboards: [layout, kitchen], activeDashboard: layout });
  authState.user.permissions = ['manage_dashboards'];
  return render(
    <ThemeProvider theme={theme}>
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

describe('DashboardPage', () => {
  beforeEach(() => {
    Object.assign(dashboardState, { loading: true, dashboards: [], activeDashboard: null });
    authState.user.permissions = [];
    dashboardState.setActiveDashboard.mockClear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('paints the page shell and a grid skeleton while the dashboards request is in flight', () => {
    render(<DashboardPage />);

    expect(screen.getByText('Dashboards')).toBeInTheDocument();
    expect(screen.getByText('sidebar')).toBeInTheDocument();
    expect(screen.getByTestId('dashboard-skeleton')).toBeInTheDocument();
  });

  it('drops the skeleton once the dashboards have loaded', () => {
    dashboardState.loading = false;
    render(<MemoryRouter><DashboardPage /></MemoryRouter>);

    expect(screen.queryByTestId('dashboard-skeleton')).not.toBeInTheDocument();
  });

  it('titles the wide page with a button naming the dashboard, followed by the lock, with New Dashboard and delete at the end', () => {
    const { container } = renderLoadedPage(1440);

    const header = container.querySelector<HTMLElement>('[data-ui=page-header]')!;
    const heading = within(header).getByRole('heading', { level: 1 });
    const title = within(heading).getByRole('button', { name: 'Layout' });
    expect(title).toHaveTextContent(/^Layout$/);
    expect(title.querySelector('[data-testid=ExpandMoreIcon]')).not.toBeNull();
    expect(heading.nextElementSibling).toContainElement(within(header).getByRole('button', { name: 'Edit dashboard' }));
    const actions = header.querySelector<HTMLElement>('[data-ui=page-actions]')!;
    expect(header.lastElementChild).toBe(actions);
    expect(within(actions).getAllByRole('button').map((button) => button.getAttribute('aria-label') ?? button.textContent)).toEqual([
      'New dashboard',
      'Delete dashboard',
    ]);
    expect(container.querySelector('[data-ui=action-bar]')).toBeNull();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
  });

  it('lists every dashboard with a star on the default from the wide title and switches to the chosen one', () => {
    renderLoadedPage(1440);

    fireEvent.click(screen.getByRole('button', { name: 'Layout' }));
    const menu = screen.getByRole('menu', { name: 'Layout' });
    expect(within(menu).getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['Layout ★', 'Kitchen']);

    fireEvent.click(within(menu).getByRole('menuitem', { name: 'Kitchen' }));
    expect(dashboardState.setActiveDashboard).toHaveBeenCalledWith(kitchen);
  });

  it('keeps the Dashboards bar title and the picker, lock, New Dashboard and delete in the compact page body', () => {
    const { container } = renderLoadedPage(390);

    expect(within(screen.getByRole('banner')).getByText('Dashboards')).toBeInTheDocument();
    expect(container.querySelector('[data-ui=page-header]')).toBeNull();
    const body = container.querySelector<HTMLElement>('[data-ui=action-bar]')!;
    expect(within(body).getByRole('combobox')).toHaveTextContent('Layout ★');
    expect(within(body).getByRole('button', { name: 'Edit dashboard' })).toBeInTheDocument();
    expect(within(body).getByRole('button', { name: 'New dashboard' })).toBeInTheDocument();
    expect(within(body).getByRole('button', { name: 'Delete dashboard' })).toBeInTheDocument();
  });
});
