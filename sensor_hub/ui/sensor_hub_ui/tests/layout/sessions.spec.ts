import { expect, test, type Page } from './test';
import { viewports } from './checks';
import { signIn } from './users';

function sessionsCard(page: Page) {
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'Active Sessions', exact: true }) });
}

async function openSessions(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/account/sessions');
  await page.waitForLoadState('networkidle');
}

test.describe('Sessions at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('each row has a ⋮ with Revoke, which revokes the session', async ({ page }) => {
    await openSessions(page);
    const sessions = sessionsCard(page);
    await expect(sessions.locator('.MuiDataGrid-root')).toHaveCount(0);

    const rows = sessions.locator('[data-ui=data-table-row]');
    await expect(rows).toHaveCount(10);
    await sessions.getByRole('textbox', { name: 'Search' }).fill('192.168.1.21');
    await expect(rows).toHaveCount(1);
    await rows.getByRole('button', { name: /actions/i }).click();
    await expect(page.getByRole('menu').getByRole('menuitem')).toHaveText(['Revoke']);
    await page.getByRole('menuitem', { name: 'Revoke' }).click();
    await expect(rows).toHaveCount(0);
  });

  test('the current session cannot be revoked', async ({ page }) => {
    await openSessions(page);
    const current = sessionsCard(page).locator('[data-ui=data-table-row]', { hasText: '(this session)' });
    await current.getByRole('button', { name: /actions/i }).click();
    await expect(page.getByRole('menuitem', { name: 'Revoke' })).toBeDisabled();
  });
});

for (const viewport of viewports.filter((viewport) => viewport.tier === 'wide')) {
  test.describe(`Sessions at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test('is a DataGrid with a Revoke button per row', async ({ page }) => {
      await openSessions(page);
      const sessions = sessionsCard(page);
      await expect(sessions.locator('.MuiDataGrid-root')).toHaveCount(1);
      await sessions.getByRole('button', { name: 'Search' }).click();
      await sessions.getByRole('searchbox').fill('192.168.1.22');
      const rows = sessions.locator('.MuiDataGrid-row');
      await expect(rows).toHaveCount(1);
      await rows.getByRole('button', { name: 'Revoke' }).click();
      await expect(rows).toHaveCount(0);
    });
  });
}
