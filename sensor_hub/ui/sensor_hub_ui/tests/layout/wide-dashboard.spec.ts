import { isDeepStrictEqual } from 'node:util';
import { expect, test, type Page } from './test';
import {
  copyLayoutDashboard,
  dashboardId,
  gridMargin as margin,
  recordDashboardWrites,
  renderedGrid,
  renderedLayouts,
  storedLayouts,
} from './dashboards';
import { signIn } from './users';

async function showDashboard(page: Page) {
  await page.goto('/dashboard');
  await page.waitForLoadState('networkidle');
  await expect(page.locator('[data-widget-id]').first()).toBeVisible();
}

async function openDashboard(page: Page) {
  await signIn(page, 'admin');
  await showDashboard(page);
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
    const writes = recordDashboardWrites(page);
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
    const copy = await copyLayoutDashboard(page, (widget) => ['uptime', 'sensor-health-pie'].includes(widget.id));
    await showDashboard(page);
    await page.getByRole('button', { name: 'Edit dashboard' }).click();

    const handle = page.locator('[data-widget-id=uptime] .react-resizable-handle').first();
    const box = (await handle.boundingBox())!;
    const { columnWidth } = await renderedGrid(page);
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width / 2 + 2 * (columnWidth + margin), box.y + box.height / 2, { steps: 10 });
    await page.mouse.up();

    await expect.poll(async () => (await renderedLayouts(page))['uptime'].w).toBe(5);
    await page.getByRole('button', { name: 'Save' }).click();
    await page.waitForLoadState('networkidle');

    await expect
      .poll(async () => {
        const [stored, shown] = [await storedLayouts(page, copy.id), await renderedLayouts(page)];
        return { stored, matchesScreen: isDeepStrictEqual(stored, shown) };
      })
      .toMatchObject({ stored: { uptime: { w: 5 } }, matchesScreen: true });
    await copy.remove();
  });
});
