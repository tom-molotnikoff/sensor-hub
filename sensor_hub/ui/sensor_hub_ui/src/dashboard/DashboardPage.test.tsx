import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DashboardConfig } from '../gen/aliases';
import DashboardPage from './DashboardPage';

const dashboardState = {
  dashboards: [],
  activeDashboard: null,
  config: { widgets: [], breakpoints: {} } as unknown as DashboardConfig,
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
vi.mock('../providers/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, username: 'tom', roles: [], permissions: [] } }),
}));
vi.mock('../navigation/NavigationSidebar', () => ({ default: () => <nav>sidebar</nav> }));
vi.mock('../navigation/TopAppBar', () => ({
  default: ({ pageTitle }: { pageTitle: string }) => <header>{pageTitle}</header>,
}));

describe('DashboardPage', () => {
  beforeEach(() => {
    dashboardState.loading = true;
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
});
