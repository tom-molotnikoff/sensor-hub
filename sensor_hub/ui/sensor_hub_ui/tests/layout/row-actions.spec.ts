import { expect, test, type Locator, type Page } from './test';
import { signIn } from './users';

const pendingName = '0x54ef441000a1b2c3';
const dismissedName = '0x842e14fffe9d8a7b';

function card(page: Page, title: string) {
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: title }) });
}

async function openOverview(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/sensors-overview');
  await page.waitForLoadState('networkidle');
  const pending = card(page, 'Pending Sensors');
  await expect(pending.getByText(pendingName)).toBeVisible();
  return pending;
}

async function captureActions(page: Page) {
  const calls: string[] = [];
  await page.route(/\/api\/sensors\/(approve|dismiss)\/\d+$/, async (route) => {
    calls.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`);
    await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
  });
  return calls;
}

async function showDismissed(pending: Locator) {
  await pending.getByRole('button', { name: /Show dismissed sensors/ }).click();
  await expect(pending.getByText(dismissedName)).toBeVisible();
}

async function openActionMenu(page: Page, pending: Locator, name: string) {
  await pending.getByRole('button', { name: `Actions for ${name}` }).click();
  await expect(page.getByRole('menu')).toBeVisible();
}

async function compactAction(page: Page, pending: Locator, name: string, action: string) {
  await openActionMenu(page, pending, name);
  await page.getByRole('menuitem', { name: action }).click();
}

async function wideAction(pending: Locator, name: string, action: string) {
  await pending.getByRole('row', { name: new RegExp(name) }).getByRole('button', { name: action }).click();
}

const scenarios = [
  { action: 'Approve', name: pendingName, dismissed: false },
  { action: 'Dismiss', name: pendingName, dismissed: false },
  { action: 'Restore', name: dismissedName, dismissed: true },
];

test.describe('Pending Sensors row actions', () => {
  for (const { action, name, dismissed } of scenarios) {
    test(`${action} calls the same endpoint from the ⋮ menu at 390 as from the row button at 1440`, async ({ page }) => {
      const calls = await captureActions(page);

      await page.setViewportSize({ width: 1440, height: 900 });
      let pending = await openOverview(page);
      if (dismissed) await showDismissed(pending);
      await wideAction(pending, name, action);
      await expect.poll(() => calls.length).toBe(1);

      await page.setViewportSize({ width: 390, height: 844 });
      pending = await openOverview(page);
      if (dismissed) await showDismissed(pending);
      await compactAction(page, pending, name, action);
      await expect.poll(() => calls.length).toBe(2);

      expect(calls[1]).toBe(calls[0]);
      expect(calls[0]).toMatch(action === 'Dismiss' ? /^POST \/api\/sensors\/dismiss\/\d+$/ : /^POST \/api\/sensors\/approve\/\d+$/);
    });
  }

  test('each compact row has a ⋮ menu with the actions the wide row has', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const pending = await openOverview(page);

    const rows = pending.locator('[data-ui=data-table-row]');
    await expect(rows).toHaveCount(3);
    for (const row of await rows.all()) {
      await expect(row.getByRole('button', { name: /^Actions for / })).toHaveCount(1);
    }
    await openActionMenu(page, pending, pendingName);
    await expect(page.getByRole('menuitem')).toHaveText(['Approve', 'Dismiss']);
  });
});

test.describe('Total Readings', () => {
  test('is a list without row actions at 390', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await signIn(page, 'admin');
    await page.goto('/sensors-overview');
    await page.waitForLoadState('networkidle');
    const totals = card(page, 'Total Readings For Each Sensor');

    await expect(totals.locator('[data-ui=data-table-row]').first()).toBeVisible();
    await expect(totals.locator('.MuiDataGrid-root')).toHaveCount(0);
    await expect(totals.getByRole('button', { name: /^Actions for / })).toHaveCount(0);
  });

  test('is a DataGrid at 1440', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await signIn(page, 'admin');
    await page.goto('/sensors-overview');
    await page.waitForLoadState('networkidle');

    await expect(card(page, 'Total Readings For Each Sensor').locator('.MuiDataGrid-root [role=gridcell]').first()).toBeVisible();
  });

  test('still renders its table in the reading statistics widget', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await signIn(page, 'admin');
    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');

    const frame = page.locator('[data-widget-state]', { has: page.getByText(/^Reading Statistics/) });
    await expect(frame.locator('.MuiDataGrid-root [role=gridcell]').first()).toBeVisible();
  });
});
