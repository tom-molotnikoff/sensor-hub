import { expect, test, type Page } from './test';
import { viewports } from './checks';
import { signIn } from './users';

function alertRules(page: Page) {
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'Alert Rules', exact: true }) });
}

async function openNotifications(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/notifications');
  await page.waitForLoadState('networkidle');
}

for (const viewport of viewports) {
  test.describe(`Alert rules at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test(`is a ${viewport.tier === 'compact' ? 'list' : 'DataGrid'} whose rows open the rule menu`, async ({ page }) => {
      await openNotifications(page);
      const rules = alertRules(page);

      if (viewport.tier === 'compact') {
        await expect(rules.locator('.MuiDataGrid-root')).toHaveCount(0);
        await rules.getByRole('button', { name: /seed-sensor-01/ }).first().click();
      } else {
        await expect(rules.locator('.MuiDataGrid-root')).toHaveCount(1);
        await rules.getByRole('gridcell', { name: 'seed-sensor-01', exact: true }).first().click();
      }
      await expect(page.getByRole('menu').getByRole('menuitem')).toHaveText(['Edit', 'Delete', 'View History']);

      await page.getByRole('menuitem', { name: 'View History' }).click();
      await expect(page.getByRole('dialog').getByRole('listitem')).toHaveCount(12);
    });
  });
}

test.describe('Alert rules at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('shows ten rules before "Show more"', async ({ page }) => {
    await openNotifications(page);
    const rules = alertRules(page);

    await expect(rules.locator('[data-ui=data-table-row]')).toHaveCount(10);
    await rules.getByRole('button', { name: 'Show more (2)' }).click();
    await expect(rules.locator('[data-ui=data-table-row]')).toHaveCount(12);
    await expect(rules.locator('[data-ui=status-pill][data-status=unknown]')).toHaveCount(3);
  });
});
