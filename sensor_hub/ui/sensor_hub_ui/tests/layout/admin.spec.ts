import { expect, test, type Page } from './test';
import { viewports } from './checks';
import { signIn } from './users';

function usersCard(page: Page) {
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'Manage Users', exact: true }) });
}

async function openAdmin(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/admin');
  await page.waitForLoadState('networkidle');
}

for (const viewport of viewports) {
  test.describe(`Users at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test(`is a ${viewport.tier === 'compact' ? 'list' : 'DataGrid'} whose rows open the user menu`, async ({ page }) => {
      await openAdmin(page);
      const users = usersCard(page);

      if (viewport.tier === 'compact') {
        await expect(users.locator('.MuiDataGrid-root')).toHaveCount(0);
        await users.getByRole('button', { name: /^bea\b/ }).click();
      } else {
        await expect(users.locator('.MuiDataGrid-root')).toHaveCount(1);
        await users.getByRole('gridcell', { name: 'bea', exact: true }).click();
      }
      await expect(page.getByRole('menu').getByRole('menuitem')).toHaveText(['Edit', 'Delete', 'Force change password']);

      await page.getByRole('menuitem', { name: 'Edit' }).click();
      await expect(page.getByRole('dialog').getByLabel('Username')).toHaveValue('bea');
    });

    test('lists roles and shows the permissions of the selected one', async ({ page }) => {
      await openAdmin(page);
      await page.getByRole('button', { name: 'viewer', exact: true }).click();
      await expect(page.getByText('Editing: viewer')).toBeVisible();
      await expect(page.getByRole('switch', { name: 'view_sensors', exact: true })).toBeChecked();
    });
  });
}

test.describe('Users at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('shows ten users before "Show more"', async ({ page }) => {
    await openAdmin(page);
    const users = usersCard(page);

    await expect(users.locator('[data-ui=data-table-row]')).toHaveCount(10);
    await users.getByRole('button', { name: /^Show more/ }).click();
    await expect(users.locator('[data-ui=data-table-row]')).toHaveCount(13);
  });
});
