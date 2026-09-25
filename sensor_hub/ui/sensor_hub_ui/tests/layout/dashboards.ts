import { expect, type Page } from '@playwright/test';
import { signIn } from './users';

export const gridColumns = 12;
export const gridMargin = 16;
export const gridRowHeight = 80;

export interface GridLayout {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface StoredWidget {
  id: string;
  type: string;
  config: Record<string, unknown>;
  layout: GridLayout;
}

interface StoredDashboard {
  id: number;
  name: string;
  config: string;
}

export async function storedWidgets(page: Page, id: number): Promise<StoredWidget[]> {
  const response = await page.request.get(`/api/dashboards/${id}`);
  expect(response.status()).toBe(200);
  const dashboard = (await response.json()) as StoredDashboard;
  return (JSON.parse(dashboard.config) as { widgets: StoredWidget[] }).widgets;
}

export async function storedLayouts(page: Page, id: number): Promise<Record<string, GridLayout>> {
  return Object.fromEntries((await storedWidgets(page, id)).map((widget) => [widget.id, widget.layout]));
}

export async function dashboardId(page: Page, name: string) {
  const response = await page.request.get('/api/dashboards');
  const dashboards = (await response.json()) as StoredDashboard[];
  return dashboards.find((dashboard) => dashboard.name === name)!.id;
}

export async function renderedGrid(page: Page) {
  return page.locator('.react-grid-layout').evaluate(
    (grid, { columns, margin, rowHeight }) => {
      const container = grid.getBoundingClientRect();
      const columnWidth = (container.width - margin * (columns - 1) - margin * 2) / columns;
      const items = [...grid.querySelectorAll<HTMLElement>('[data-widget-id]')].map((item) => {
        const box = item.getBoundingClientRect();
        return {
          id: item.dataset.widgetId!,
          width: box.width,
          layout: {
            x: Math.round((box.left - container.left - margin) / (columnWidth + margin)),
            y: Math.round((box.top - container.top - margin) / (rowHeight + margin)),
            w: Math.round((box.width + margin) / (columnWidth + margin)),
            h: Math.round((box.height + margin) / (rowHeight + margin)),
          },
        };
      });
      return { columnWidth, items };
    },
    { columns: gridColumns, margin: gridMargin, rowHeight: gridRowHeight },
  );
}

export async function renderedLayouts(page: Page) {
  const { items } = await renderedGrid(page);
  return Object.fromEntries(items.map((item) => [item.id, item.layout]));
}

export function recordDashboardWrites(page: Page) {
  const writes: string[] = [];
  page.on('request', (request) => {
    if (request.method() !== 'GET' && request.url().includes('/api/dashboards')) writes.push(`${request.method()} ${request.url()}`);
  });
  return writes;
}

export function recordApiRequests(page: Page) {
  const requests: string[] = [];
  page.on('request', (request) => {
    if (new URL(request.url()).pathname.startsWith('/api/')) requests.push(`${request.method()} ${request.url()}`);
  });
  return requests;
}

export async function pausePolling(page: Page) {
  await page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' });
    document.dispatchEvent(new Event('visibilitychange'));
  });
}

export async function copyLayoutDashboard(page: Page, pick?: (widget: StoredWidget) => boolean) {
  const csrf = { 'X-CSRF-Token': (await signIn(page, 'admin'))! };
  const widgets = (await storedWidgets(page, await dashboardId(page, 'Layout'))).filter(pick ?? (() => true));
  const name = `Copy ${Date.now()}-${Math.random().toString(36).slice(2)}`;
  const created = await page.request.post('/api/dashboards', { headers: csrf, data: { name, config: { widgets } } });
  expect(created.status()).toBe(201);
  const { id } = (await created.json()) as { id: number };
  await page.addInitScript((value) => localStorage.setItem('sensor-hub-active-dashboard-id', value), String(id));
  return {
    id,
    name,
    widgets,
    remove: async () => {
      const response = await page.request.delete(`/api/dashboards/${id}`, { headers: csrf });
      expect(response.ok(), `delete dashboard ${id}`).toBe(true);
    },
  };
}
