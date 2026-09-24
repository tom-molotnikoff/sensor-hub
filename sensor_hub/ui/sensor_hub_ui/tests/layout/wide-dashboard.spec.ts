import { expect, test, type Page } from '@playwright/test';
import { signIn } from './users';

const columns = 12;
const margin = 16;
const rowHeight = 80;

interface GridLayout {
  x: number;
  y: number;
  w: number;
  h: number;
}

interface StoredDashboard {
  id: number;
  name: string;
  config: string;
}

async function storedLayouts(page: Page, id: number): Promise<Record<string, GridLayout>> {
  const response = await page.request.get(`/api/dashboards/${id}`);
  expect(response.status()).toBe(200);
  const dashboard = (await response.json()) as StoredDashboard;
  const widgets = (JSON.parse(dashboard.config) as { widgets: { id: string; layout: GridLayout }[] }).widgets;
  return Object.fromEntries(widgets.map((widget) => [widget.id, widget.layout]));
}

async function dashboardId(page: Page, name: string) {
  const response = await page.request.get('/api/dashboards');
  const dashboards = (await response.json()) as StoredDashboard[];
  return dashboards.find((dashboard) => dashboard.name === name)!.id;
}

async function renderedGrid(page: Page) {
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
    { columns, margin, rowHeight },
  );
}

async function renderedLayouts(page: Page) {
  const { items } = await renderedGrid(page);
  return Object.fromEntries(items.map((item) => [item.id, item.layout]));
}

async function openDashboard(page: Page, id?: number) {
  await signIn(page, 'admin');
  if (id !== undefined) {
    await page.addInitScript((value) => localStorage.setItem('sensor-hub-active-dashboard-id', value), String(id));
  }
  await page.goto('/dashboard');
  await page.waitForLoadState('networkidle');
  await expect(page.locator('[data-widget-id]').first()).toBeVisible();
}

test.describe('Wide dashboard', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('a widget stored at w=3 takes 3/12 of the grid at 1000px and 1440px', async ({ page }) => {
    await openDashboard(page);
    for (const width of [1440, 1000]) {
      await page.setViewportSize({ width, height: 900 });
      await expect
        .poll(async () => {
          const { columnWidth, items } = await renderedGrid(page);
          const uptime = items.find((item) => item.id === 'uptime')!;
          return Math.abs(uptime.width - (3 * columnWidth + 2 * margin));
        }, { message: `uptime width at ${width}px` })
        .toBeLessThan(1);
    }
  });

  test('resizing the window outside edit mode keeps the stored layout and writes nothing', async ({ page }) => {
    const writes: string[] = [];
    page.on('request', (request) => {
      if (request.method() !== 'GET' && request.url().includes('/api/dashboards')) writes.push(`${request.method()} ${request.url()}`);
    });
    await openDashboard(page);
    const stored = await storedLayouts(page, await dashboardId(page, 'Layout'));

    for (const width of [1440, 1200, 1000, 900, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      await expect.poll(() => renderedLayouts(page), { message: `layout at ${width}px` }).toEqual(stored);
    }
    await page.waitForLoadState('networkidle');
    expect(writes).toEqual([]);
  });

  test('resizing a widget in edit mode saves the 12-column layout shown on screen', async ({ page }) => {
    const csrf = { 'X-CSRF-Token': (await signIn(page, 'admin'))! };
    const fixture = await storedLayouts(page, await dashboardId(page, 'Layout'));
    const created = await page.request.post('/api/dashboards', {
      headers: csrf,
      data: {
        name: `Resize ${test.info().workerIndex}-${Date.now()}`,
        config: {
          widgets: [
            { id: 'uptime', type: 'uptime', config: { sensorId: 1 }, layout: fixture['uptime'] },
            { id: 'sensor-health-pie', type: 'sensor-health-pie', config: {}, layout: fixture['sensor-health-pie'] },
          ],
        },
      },
    });
    expect(created.status()).toBe(201);
    const { id } = (await created.json()) as { id: number };

    await openDashboard(page, id);
    await page.getByRole('button', { name: 'Edit dashboard' }).click();

    const handle = page.locator('[data-widget-id=uptime] .react-resizable-handle').first();
    const box = (await handle.boundingBox())!;
    const { columnWidth } = await renderedGrid(page);
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width / 2 + 2 * (columnWidth + margin), box.y + box.height / 2, { steps: 10 });
    await page.mouse.up();

    await expect.poll(async () => (await renderedLayouts(page))['uptime'].w).toBe(5);
    const shown = await renderedLayouts(page);
    await page.getByRole('button', { name: 'Save' }).click();
    await page.waitForLoadState('networkidle');

    await expect.poll(() => storedLayouts(page, id)).toEqual(shown);
    await page.request.delete(`/api/dashboards/${id}`, { headers: csrf });
  });
});
