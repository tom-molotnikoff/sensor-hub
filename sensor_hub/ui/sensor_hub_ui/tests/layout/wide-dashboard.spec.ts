import { isDeepStrictEqual } from 'node:util';
import { expect, test, type Page } from './test';
import { railTransition, watchRailTransition } from './checks';
import {
  copyLayoutDashboard,
  dashboardId,
  gridMargin as margin,
  pausePolling,
  recordApiRequests,
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

const mediumName = 'Upstairs Bedrooms, Landing and Loft Conversion';
const longName = `${mediumName} Environmental Monitoring with Every Sensor in the House`;

async function headerActions(page: Page) {
  return page
    .locator('[data-ui=page-actions] button')
    .evaluateAll((buttons) => buttons.map((button) => button.getAttribute('aria-label') ?? button.textContent));
}

async function titleRoom(page: Page, editing: boolean) {
  await page.goto('/dashboard');
  await page.waitForLoadState('networkidle');
  if (editing) await page.getByRole('button', { name: 'Edit dashboard' }).click();
  await expect(page.getByRole('button', { name: editing ? 'Lock dashboard' : 'Edit dashboard' })).toBeVisible();
  return page.locator('[data-ui=page-header]').evaluate((header) => {
    const label = header.querySelector('[data-ui=page-title-label]')!;
    const heading = header.querySelector('h1')!.getBoundingClientRect();
    const actions = header.querySelector('[data-ui=page-actions]')!.getBoundingClientRect();
    return {
      text: label.textContent,
      truncated: label.scrollWidth > label.clientWidth,
      ellipsis: getComputedStyle(label).textOverflow === 'ellipsis',
      room: actions.left - parseFloat(getComputedStyle(header).columnGap) - heading.right,
    };
  });
}

async function widenUptimeByTwoColumns(page: Page) {
  const handle = page.locator('[data-widget-id=uptime] .react-resizable-handle').first();
  const box = (await handle.boundingBox())!;
  const { columnWidth } = await renderedGrid(page);
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width / 2 + 2 * (columnWidth + margin), box.y + box.height / 2, { steps: 10 });
  await page.mouse.up();
}

type KeptWindow = Window & { keptContent?: WeakSet<Element> };

const widgetContent = '[data-widget-id] [data-ui=frame-content] > *, .recharts-wrapper';

async function markWidgetContent(page: Page) {
  return page.evaluate((selector) => {
    const nodes = [...document.querySelectorAll(selector)];
    (window as KeptWindow).keptContent = new WeakSet(nodes);
    return nodes.length;
  }, widgetContent);
}

async function keptWidgetContent(page: Page) {
  return page.evaluate(
    (selector) => [...document.querySelectorAll(selector)].filter((node) => (window as KeptWindow).keptContent?.has(node)).length,
    widgetContent,
  );
}

async function widgetStates(page: Page) {
  return page
    .locator('[data-widget-id]')
    .evaluateAll((items) => items.map((item) => [item.getAttribute('data-widget-id'), item.querySelector('[data-widget-state]')?.getAttribute('data-widget-state')]));
}

async function chartWidths(page: Page) {
  return page.locator('.recharts-responsive-container').evaluateAll((charts) =>
    charts.map((chart) => ({
      container: Math.round(chart.getBoundingClientRect().width),
      surface: Math.round(chart.querySelector('.recharts-wrapper > svg.recharts-surface')!.getBoundingClientRect().width),
    })),
  );
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

    await widenUptimeByTwoColumns(page);

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

  test('titles the page with the lock at the content edge, the dashboard name after it and the dashboard actions at the end of the row', async ({ page }) => {
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

    const lock = header.getByRole('button', { name: 'Edit dashboard' });
    await expect(header.locator('[data-ui=page-before-title]').getByRole('button')).toHaveCount(1);
    await expect(header.locator('[data-ui=page-before-title]').getByRole('button', { name: 'Edit dashboard' })).toBeVisible();
    const edges = await page.locator('[data-ui=page]').evaluate((root) => ({
      glyph: root.querySelector('[data-ui=page-before-title] svg path')!.getBoundingClientRect().left,
      content: root.getBoundingClientRect().left + parseFloat(getComputedStyle(root).paddingLeft),
    }));
    expect(Math.abs(edges.glyph - edges.content), 'lock glyph left edge against the page content').toBeLessThan(1);
    expect(await headerActions(page)).toEqual(['New dashboard', 'Delete dashboard']);

    const [row, headingBox, name, lockBox, create, remove] = await Promise.all(
      [
        header,
        heading,
        title,
        lock,
        header.getByRole('button', { name: 'New dashboard' }),
        header.getByRole('button', { name: 'Delete dashboard' }),
      ].map(async (element) => (await element.boundingBox())!),
    );
    expect(name, 'title button inside the heading box').toEqual(headingBox);
    const middle = (box: typeof row) => box.y + box.height / 2;
    for (const control of [lockBox, create, remove]) expect(Math.abs(middle(control) - middle(name))).toBeLessThan(2);
    expect(name.x - (lockBox.x + lockBox.width), 'gap between the lock and the title').toBeGreaterThanOrEqual(0);
    expect(name.x - (lockBox.x + lockBox.width), 'gap between the lock and the title').toBeLessThanOrEqual(16);
    expect(create.x).toBeGreaterThan(name.x + name.width + 100);
    expect(remove.x).toBeGreaterThan(create.x + create.width);
    expect(Math.abs(row.x + row.width - (remove.x + remove.width)), 'delete at the end of the row').toBeLessThan(1);
  });

  test('keeps the lock in place and puts the edit controls with the dashboard actions while editing', async ({ page }) => {
    await openDashboard(page);
    const header = page.locator('[data-ui=page-header]');
    const before = (await header.getByRole('button', { name: 'Edit dashboard' }).boundingBox())!;

    await header.getByRole('button', { name: 'Edit dashboard' }).click();

    const after = (await header.getByRole('button', { name: 'Lock dashboard' }).boundingBox())!;
    expect(after).toEqual(before);
    await expect(header.locator('[data-ui=page-before-title]').getByRole('button')).toHaveCount(1);
    expect(await headerActions(page)).toEqual(['Save', 'Add Widget', 'New dashboard', 'Delete dashboard']);
  });

  test('shows the whole dashboard name while there is room and ellipsises it only against the actions', async ({ page }) => {
    for (const editing of [false, true]) {
      const fits = await copyLayoutDashboard(page, () => false, mediumName);
      const fitting = await titleRoom(page, editing);
      expect(fitting, `${mediumName} while ${editing ? 'editing' : 'viewing'}`).toMatchObject({ text: mediumName, truncated: false });
      expect(fitting.room, 'free room after the title').toBeGreaterThan(16);
      await fits.remove();

      const long = await copyLayoutDashboard(page, () => false, longName);
      const cut = await titleRoom(page, editing);
      expect(cut, `${longName} while ${editing ? 'editing' : 'viewing'}`).toMatchObject({ text: longName, truncated: true, ellipsis: true });
      expect(Math.abs(cut.room), 'title reaching the actions').toBeLessThan(1);
      await long.remove();
    }
  });

  test('keeps the lock at the same place for a short and a long dashboard name', async ({ page }) => {
    const places: Record<string, { x: number; y: number }> = {};
    for (const name of ['Hall', longName]) {
      const copy = await copyLayoutDashboard(page, () => false, name);
      await page.goto('/dashboard');
      await page.waitForLoadState('networkidle');
      await expect(page.locator('[data-ui=page-title-label]')).toHaveText(name);
      const { x, y } = (await page.locator('[data-ui=page-header]').getByRole('button', { name: 'Edit dashboard' }).boundingBox())!;
      places[name] = { x, y };
      await copy.remove();
    }
    expect(places[longName]).toEqual(places['Hall']);
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

  test('covers every widget with its placeholder while the nav animates, then shows it again at the new width without refetching or saving', async ({ page }) => {
    await openDashboard(page);
    await expect(page.locator('.recharts-surface').first()).toBeVisible();
    const stored = await storedLayouts(page, await dashboardId(page, 'Layout'));
    const [states, widthsBefore, marked] = [await widgetStates(page), await chartWidths(page), await markWidgetContent(page)];
    await pausePolling(page);
    const requests = recordApiRequests(page);
    await watchRailTransition(page);

    await page.locator('[data-ui=nav-collapse]').click();

    const { durations, samples } = await railTransition(page);
    expect(durations, 'nav width transition').toEqual([180]);
    expect(samples[0].moment).toBe('run');
    expect(samples.at(-1)!.moment).toBe('end');
    for (const sample of samples) {
      expect(sample, `widgets at ${sample.moment}`).toMatchObject({ uncovered: ['retired'], editControls: 0 });
    }

    await expect(page.locator('[data-ui=frame-placeholder]')).toHaveCount(0);
    await expect.poll(() => renderedLayouts(page), { message: 'layout at the new width' }).toEqual(stored);
    await expect
      .poll(async () => (await chartWidths(page)).every(({ container, surface }, i) => surface === container && container > widthsBefore[i].container))
      .toBe(true);
    expect(requests, 'requests sent because of the toggle').toEqual([]);
    expect(await keptWidgetContent(page), 'widget content kept mounted').toBe(marked);
    expect(await widgetStates(page)).toEqual(states);
  });

  test('animates the nav while editing, and editing carries on afterwards', async ({ page }) => {
    const copy = await copyLayoutDashboard(page, (widget) => ['uptime', 'sensor-health-pie'].includes(widget.id));
    await showDashboard(page);
    await page.getByRole('button', { name: 'Edit dashboard' }).click();
    const writes = recordDashboardWrites(page);
    await watchRailTransition(page);

    await page.locator('[data-ui=nav-collapse]').click();

    const { durations, samples } = await railTransition(page);
    expect(durations, 'nav width transition').toEqual([180]);
    for (const sample of samples) {
      expect(sample, `widgets at ${sample.moment}`).toMatchObject({ uncovered: [], covers: 0 });
      expect(sample.editControls, `edit controls at ${sample.moment}`).toBeGreaterThan(0);
    }
    expect(writes, 'writes during the toggle').toEqual([]);

    await expect(page.locator('[data-widget-id=uptime]').getByRole('button', { name: 'Configure widget' })).toBeVisible();
    await expect
      .poll(async () => {
        const { columnWidth, items } = await renderedGrid(page);
        const uptime = items.find((item) => item.id === 'uptime')!;
        return Math.abs(uptime.width - (3 * columnWidth + 2 * margin));
      }, { message: 'uptime settled at the new width' })
      .toBeLessThan(1);
    await widenUptimeByTwoColumns(page);
    await expect.poll(async () => (await renderedLayouts(page))['uptime'].w).toBe(5);
    await page.getByRole('button', { name: 'Save' }).click();
    await page.waitForLoadState('networkidle');

    await expect.poll(async () => (await storedLayouts(page, copy.id))['uptime'].w).toBe(5);
    await copy.remove();
  });
});
