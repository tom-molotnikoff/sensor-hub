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

  test('titles the page with the dashboard name, the lock right after it and the dashboard actions at the end of the row', async ({ page }) => {
    await openDashboard(page);
    const header = page.locator('[data-ui=page-header]');
    const heading = header.getByRole('heading', { level: 1 });
    const title = heading.getByRole('button', { name: 'Layout', exact: true });
    await expect(title).toHaveText('Layout');
    await expect(title.locator('.MuiButton-endIcon > svg')).toBeVisible();
    await expect(page.locator('h1 [role=combobox], [role=combobox]:has(h1)')).toHaveCount(0);
    await expect(page.locator('[data-ui=action-bar]'), 'separate toolbar row').toHaveCount(0);

    const type = await Promise.all(
      [heading, title].map((element) =>
        element.evaluate((node) => {
          const style = getComputedStyle(node);
          return { fontSize: style.fontSize, fontWeight: style.fontWeight, textTransform: style.textTransform };
        }),
      ),
    );
    expect(type[1], 'title button type').toEqual({ ...type[0], textTransform: 'none' });

    const [row, name, lock, create, remove] = await Promise.all(
      [
        header,
        title,
        header.getByRole('button', { name: 'Edit dashboard' }),
        header.getByRole('button', { name: 'New dashboard' }),
        header.getByRole('button', { name: 'Delete dashboard' }),
      ].map(async (element) => (await element.boundingBox())!),
    );
    const middle = (box: typeof row) => box.y + box.height / 2;
    for (const control of [lock, create, remove]) expect(Math.abs(middle(control) - middle(name))).toBeLessThan(2);
    expect(lock.x - (name.x + name.width), 'gap between the title and the lock').toBeGreaterThanOrEqual(0);
    expect(lock.x - (name.x + name.width), 'gap between the title and the lock').toBeLessThanOrEqual(24);
    expect(create.x).toBeGreaterThan(lock.x + lock.width + 100);
    expect(remove.x).toBeGreaterThan(create.x + create.width);
    expect(Math.abs(row.x + row.width - (remove.x + remove.width)), 'delete at the end of the row').toBeLessThan(1);
  });

  test('switches dashboard from the title menu, with the star only on the default entry', async ({ page }) => {
    const copy = await copyLayoutDashboard(page, () => false);
    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');
    const title = page.locator('h1').getByRole('button');
    await expect(title).toHaveText(copy.name);

    await title.click();
    const menu = page.getByRole('menu', { name: copy.name });
    const entries = await menu.getByRole('menuitem').allTextContents();
    expect(entries).toContain(copy.name);
    expect(entries.filter((entry) => entry.includes('★'))).toEqual(['Layout ★']);

    await menu.getByRole('menuitem', { name: 'Layout ★' }).click();
    await expect(menu).toHaveCount(0);
    await expect(title).toHaveText('Layout');
    await expect(page.locator('[data-widget-id]').first()).toBeVisible();
    await copy.remove();
  });
});
