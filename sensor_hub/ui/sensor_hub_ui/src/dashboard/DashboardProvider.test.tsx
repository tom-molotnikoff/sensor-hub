import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useDashboard } from './DashboardContext';
import { DashboardProvider } from './DashboardProvider';

const api = vi.hoisted(() => ({ GET: vi.fn(), PUT: vi.fn() }));

vi.mock('../gen/client', () => ({ apiClient: api }));

const storedConfig = {
  widgets: [{ id: 'note', type: 'markdown-note', config: {}, layout: { x: 0, y: 0, w: 3, h: 2 } }],
  breakpoints: { lg: 12, md: 10, sm: 6 },
};

const stored = { id: 7, name: 'Home', is_default: true, config: JSON.stringify(storedConfig) };

function renderDashboard() {
  return renderHook(() => useDashboard(), { wrapper: DashboardProvider }).result;
}

function bodyOf(mock: ReturnType<typeof vi.fn>) {
  return mock.mock.calls[0][mock.mock.calls[0].length - 1].body as { config: Record<string, unknown> };
}

describe('DashboardProvider', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    api.GET.mockResolvedValue({ data: [stored] });
    api.PUT.mockResolvedValue({});
  });

  it('saves a dashboard loaded with breakpoints without sending them', async () => {
    const dashboard = renderDashboard();
    await waitFor(() => expect(dashboard.current.loading).toBe(false));

    await act(() => dashboard.current.saveDashboard());

    const { config } = bodyOf(api.PUT);
    expect(config).not.toHaveProperty('breakpoints');
    expect(config.widgets).toEqual(storedConfig.widgets);
  });

});
