import { expect, test, type Page } from './test';
import { signIn } from './users';

async function openSummary(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/sensors-overview');
  await page.waitForLoadState('networkidle');
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'Sensor Summary' }) });
}

test.describe('Sensor Summary at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('is a list of sensor rows with no grid', async ({ page }) => {
    const card = await openSummary(page);
    const row = card.locator('[data-ui=data-table-row]', { hasText: 'hallway-motion' });

    await expect(card.locator('.MuiDataGrid-root')).toHaveCount(0);
    await expect(row.locator('[data-ui=data-table-title]')).toHaveText('hallway-motion');
    await expect(row.locator('[data-ui=data-table-meta]')).toHaveText('mqtt-zigbee2mqtt · disabled');
    await expect(row.locator('[data-ui=status-pill]')).toHaveText('good');
    await expect(row.locator('[data-ui=data-table-title]')).toHaveCSS('font-size', '15px');
    await expect(row.locator('[data-ui=data-table-meta]')).toHaveCSS('font-size', '13px');
  });

  test('opens the row menu when a row is tapped', async ({ page }) => {
    const card = await openSummary(page);
    await card.getByRole('textbox', { name: 'Search' }).fill('porch-light');
    await card.getByRole('button', { name: /porch-light/ }).click();

    await expect(page.getByRole('menuitem', { name: 'Trigger Reading' })).toBeVisible();
    await page.getByRole('menuitem', { name: 'View Details' }).click();
    await expect(page).toHaveURL(/\/sensor\/\d+$/);
  });

  test('narrows rows by any displayed value and offers no grid controls', async ({ page }) => {
    const card = await openSummary(page);
    const titles = card.locator('[data-ui=data-table-title]');
    const searchBox = card.getByRole('textbox', { name: 'Search' });

    await searchBox.fill('DISABLED');
    await expect(titles).toHaveText(['hallway-motion', 'loft-hygrometer']);
    await searchBox.fill('Http-Temp');
    await expect(titles).toHaveCount(9);
    await searchBox.fill('fixture');
    await expect(titles).toHaveCount(7);

    await expect(card.getByRole('columnheader')).toHaveCount(0);
    for (const control of ['Columns', 'Filters', 'Export', 'Sort']) {
      await expect(card.getByRole('button', { name: control })).toHaveCount(0);
    }
  });

  test('shows ten rows at a time, and ten again after the search changes', async ({ page }) => {
    const card = await openSummary(page);
    const titles = card.locator('[data-ui=data-table-title]');

    await expect(titles).toHaveCount(10);
    await card.getByRole('button', { name: 'Show more (5)' }).click();
    await expect(titles).toHaveCount(15);
    await expect(card.getByRole('button', { name: /Show more/ })).toHaveCount(0);

    await card.getByRole('textbox', { name: 'Search' }).fill('e');
    await expect(titles).toHaveCount(10);
  });
});

test.describe('Sensor Summary at 1440x900', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('is a DataGrid with sort, filter, export and the column picker', async ({ page }) => {
    const card = await openSummary(page);
    const grid = card.locator('.MuiDataGrid-root');
    await expect(grid).toBeVisible();
    await expect(card.getByRole('textbox', { name: 'Search' })).toHaveCount(0);

    const names = grid.locator('[data-field=name][role=gridcell]');
    await grid.getByRole('columnheader', { name: 'Sensor Name' }).click();
    await grid.getByRole('columnheader', { name: 'Sensor Name' }).click();
    await expect(names.first()).toHaveText('Study');

    await card.getByRole('button', { name: 'Columns' }).click();
    await expect(page.getByRole('checkbox', { name: 'Health Reason' })).toBeVisible();
    await page.keyboard.press('Escape');

    await card.getByRole('button', { name: 'Filters' }).click();
    await expect(page.getByRole('combobox', { name: 'Column' })).toHaveText('Sensor Name');
    await page.getByPlaceholder('Filter value').fill('porch');
    await expect(names).toHaveText(['porch-light']);
    await page.keyboard.press('Escape');

    await card.getByRole('button', { name: 'Export' }).click();
    await expect(page.getByRole('menuitem', { name: 'Download as CSV' })).toBeVisible();
  });
});
