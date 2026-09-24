import { expect, test, type Page } from '@playwright/test';
import { viewports } from './checks';
import { signIn } from './users';

function apiKeys(page: Page) {
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'API Keys', exact: true }) });
}

async function openDeveloper(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/account/developer');
  await page.waitForLoadState('networkidle');
}

for (const viewport of viewports) {
  test.describe(`API keys at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test(`is a ${viewport.tier === 'compact' ? 'list' : 'DataGrid'} whose rows open the key menu`, async ({ page }) => {
      await openDeveloper(page);
      const keys = apiKeys(page);

      if (viewport.tier === 'compact') {
        await expect(keys.locator('.MuiDataGrid-root')).toHaveCount(0);
        await keys.getByRole('button', { name: /^Loft Pi/ }).click();
      } else {
        await expect(keys.locator('.MuiDataGrid-root')).toHaveCount(1);
        await keys.getByRole('gridcell', { name: 'Loft Pi', exact: true }).click();
      }
      await expect(page.getByRole('menu').getByRole('menuitem')).toHaveText(['Revoke', 'Delete']);
    });
  });
}

test.describe('Developer page at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('API keys show a status pill per key and ten before "Show more"', async ({ page }) => {
    await openDeveloper(page);
    const keys = apiKeys(page);

    await expect(keys.locator('[data-ui=data-table-row]')).toHaveCount(10);
    await keys.getByRole('button', { name: 'Show more (2)' }).click();
    await expect(keys.locator('[data-ui=data-table-row]')).toHaveCount(12);
    await expect(keys.locator('[data-ui=status-pill][data-status=bad]')).toHaveCount(2);
    await expect(keys.locator('[data-ui=status-pill][data-status=warn]')).toHaveCount(1);
  });

  test('Swagger UI scrolls sideways inside its own region', async ({ page }) => {
    await openDeveloper(page);
    const frame = page.locator('[data-ui=swagger-frame]');
    await expect(frame.locator('.opblock').first()).toBeVisible();

    const region = await frame.evaluate((element) => ({
      overflowX: getComputedStyle(element).overflowX,
      right: element.getBoundingClientRect().right,
    }));
    expect(region.overflowX).toBe('auto');
    expect(region.right).toBeLessThanOrEqual(390);
    const { scrollWidth, innerWidth } = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      innerWidth: window.innerWidth,
    }));
    expect(scrollWidth).toBeLessThanOrEqual(innerWidth);
  });
});
