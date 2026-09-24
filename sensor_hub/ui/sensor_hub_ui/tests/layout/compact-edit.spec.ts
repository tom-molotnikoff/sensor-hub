import { expect, test, type Page } from '@playwright/test';
import { checks } from './checks';
import { copyLayoutDashboard, storedWidgets, type StoredWidget } from './dashboards';

async function openDashboard(page: Page) {
  await page.goto('/dashboard');
  await page.waitForLoadState('networkidle');
  await expect(page.locator('[data-ui=dashboard-slot]').first()).toBeVisible();
}

async function startEditing(page: Page) {
  await page.getByRole('button', { name: 'Edit dashboard' }).click();
}

async function saveDashboard(page: Page) {
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Edit dashboard' })).toBeVisible();
  await page.waitForLoadState('networkidle');
}

function slot(page: Page, title: string) {
  return page.locator('[data-ui=dashboard-slot]', { hasText: title });
}

function layoutsById(widgets: StoredWidget[]) {
  return Object.fromEntries(widgets.map((widget) => [widget.id, JSON.stringify(widget.layout)]));
}

test.describe('Editing a dashboard at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('every frame can be removed, configurable ones configured, with Add widget last and no handles', async ({ page }) => {
    const copy = await copyLayoutDashboard(page);
    await openDashboard(page);
    await startEditing(page);

    await expect(page.getByText('Arrange the layout on a wider screen')).toBeVisible();
    const slots = page.locator('[data-ui=dashboard-slot]');
    await expect(slots).toHaveCount(copy.widgets.length);
    for (const frame of await slots.all()) {
      await expect(frame.getByRole('button', { name: 'Remove widget' })).toHaveCount(1);
    }
    for (const title of ['Readings Chart', 'Sensor Uptime', 'Health Timeline']) {
      await expect(slot(page, title).getByRole('button', { name: 'Configure widget' })).toHaveCount(1);
    }
    await expect(page.locator('.drag-handle, .react-resizable-handle, .react-grid-layout')).toHaveCount(0);
    await expect(page.locator('[data-testid=DragIndicatorIcon]')).toHaveCount(0);

    const add = page.getByRole('button', { name: 'Add widget', exact: true });
    const lastSlot = (await slots.last().boundingBox())!;
    expect((await add.boundingBox())!.y).toBeGreaterThanOrEqual(lastSlot.y + lastSlot.height);

    await copy.remove();
  });

  test('adding a widget leaves every other layout untouched and puts the new one at the bottom', async ({ page, browser }) => {
    const copy = await copyLayoutDashboard(page);
    const before = await storedWidgets(page, copy.id);
    await openDashboard(page);
    await startEditing(page);

    await page.getByRole('button', { name: 'Add widget', exact: true }).click();
    await page.getByRole('dialog', { name: 'Add Widget' }).getByRole('button', { name: /^Gauge/ }).click();
    await saveDashboard(page);

    const after = await storedWidgets(page, copy.id);
    const added = after.filter((widget) => !before.some((existing) => existing.id === widget.id));
    expect(added).toHaveLength(1);
    expect(added[0].type).toBe('gauge');
    const bottom = Math.max(...before.map((widget) => widget.layout.y + widget.layout.h));
    expect(added[0].layout).toEqual({ x: 0, y: bottom, w: 3, h: 3 });
    const { [added[0].id]: _added, ...kept } = layoutsById(after);
    expect(kept).toEqual(layoutsById(before));

    const wide = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    await wide.context().addCookies(await page.context().cookies());
    await wide.addInitScript((value) => localStorage.setItem('sensor-hub-active-dashboard-id', value), String(copy.id));
    await wide.goto('/dashboard');
    await wide.waitForLoadState('networkidle');
    await expect(wide.locator(`[data-widget-id="${added[0].id}"]`)).toBeVisible();
    await checks.noSidewaysScroll(wide, 'wide', 'admin');
    await checks.noCollapsedContent(wide, 'wide', 'admin');
    const boxes = await wide.locator('[data-widget-id]').evaluateAll((items) =>
      items.map((item) => ({ id: (item as HTMLElement).dataset.widgetId, box: item.getBoundingClientRect().toJSON() as DOMRect })),
    );
    const addedBox = boxes.find((item) => item.id === added[0].id)!.box;
    for (const { id, box } of boxes) {
      if (id !== added[0].id) expect(addedBox.top, `${added[0].id} below ${id}`).toBeGreaterThanOrEqual(box.bottom);
    }
    await wide.close();

    await copy.remove();
  });

  test('removing a widget leaves the remaining layouts untouched', async ({ page }) => {
    const copy = await copyLayoutDashboard(page);
    const before = await storedWidgets(page, copy.id);
    await openDashboard(page);
    await startEditing(page);

    await slot(page, 'Sensor Types').getByRole('button', { name: 'Remove widget' }).click();
    await saveDashboard(page);

    const after = await storedWidgets(page, copy.id);
    const { 'sensor-type-pie': removed, ...kept } = layoutsById(before);
    expect(removed).toBeDefined();
    expect(layoutsById(after)).toEqual(kept);

    await copy.remove();
  });

  test('reconfiguring a widget changes only its config', async ({ page }) => {
    const copy = await copyLayoutDashboard(page);
    const before = await storedWidgets(page, copy.id);
    await openDashboard(page);
    await startEditing(page);

    await slot(page, 'Sensor Uptime').getByRole('button', { name: 'Configure widget' }).click();
    const dialog = page.getByRole('dialog', { name: 'Configure Sensor Uptime' });
    await dialog.getByRole('combobox').first().click();
    await page.getByRole('option').nth(1).click();
    await dialog.getByRole('button', { name: 'Save' }).click();
    await expect(dialog).toHaveCount(0);
    await saveDashboard(page);

    const after = await storedWidgets(page, copy.id);
    expect(layoutsById(after)).toEqual(layoutsById(before));
    const changed = after.filter(
      (widget) => JSON.stringify(widget.config) !== JSON.stringify(before.find((existing) => existing.id === widget.id)!.config),
    );
    expect(changed.map((widget) => widget.id)).toEqual(['uptime']);
    expect(after.map(({ id, type }) => ({ id, type }))).toEqual(before.map(({ id, type }) => ({ id, type })));

    await copy.remove();
  });
});
