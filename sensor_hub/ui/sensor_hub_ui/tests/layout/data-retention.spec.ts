import { expect, test, type Page } from './test';
import { viewports } from './checks';
import { signIn } from './users';

async function openRetention(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/data-retention');
  await page.waitForLoadState('networkidle');
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'Sensor Retention Overview' }) });
}

test.describe('Data Retention at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("lists every sensor's effective retention", async ({ page }) => {
    const card = await openRetention(page);
    const rows = card.locator('[data-ui=data-table-row]');

    await expect(card.locator('.MuiDataGrid-root')).toHaveCount(0);
    await card.getByRole('button', { name: /^Show more/ }).click();
    await expect(card.getByRole('button', { name: /^Show more/ })).toHaveCount(0);
    await expect(rows).toHaveCount(15);
    for (const meta of await rows.locator('[data-ui=data-table-meta]').allTextContents()) {
      expect(meta).toMatch(/^(Custom|Global default) · \d+ (hours?|days?|weeks?)$/);
    }
    await expect(rows.filter({ hasText: 'attic-bulb' }).locator('[data-ui=data-table-meta]')).toHaveText('Custom · 5 days');
    await expect(rows.filter({ hasText: 'garage-temp' }).locator('[data-ui=data-table-meta]')).toHaveText('Global default · 90 days');
  });
});

for (const viewport of viewports) {
  test.describe(`Data Retention at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test('opens the retention editor from a row', async ({ page }) => {
      const card = await openRetention(page);
      if (viewport.tier === 'compact') {
        await card.getByRole('button', { name: /kitchen-plug/ }).click();
      } else {
        await card.getByRole('gridcell', { name: 'kitchen-plug' }).click();
      }
      await page.getByRole('menuitem', { name: 'Edit Retention' }).click();

      const dialog = page.getByRole('dialog', { name: 'Edit Data Retention' });
      await expect(dialog.getByLabel('Sensor')).toHaveValue('kitchen-plug');
      await expect(dialog.getByRole('spinbutton', { name: 'Retention' })).toHaveValue('2');
    });
  });
}
