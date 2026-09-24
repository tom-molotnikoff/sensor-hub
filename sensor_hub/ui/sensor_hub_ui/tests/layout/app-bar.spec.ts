import { expect, test, type Page } from '@playwright/test';
import { appBarControls, appBarTitle, checks } from './checks';
import { signIn } from './users';

async function openNotifications(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/notifications');
  await page.waitForLoadState('networkidle');
}

async function chooseDark(page: Page) {
  await page.getByRole('menuitem', { name: 'Dark' }).click();
  await expect(page.locator('html')).toHaveClass(/\bdark\b/);
}

test.describe('app bar at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 }, colorScheme: 'light' });

  test('truncates a long title to one line and keeps the bell and avatar on screen', async ({ page }) => {
    await openNotifications(page);
    await page
      .locator('[data-ui=app-bar-title]')
      .evaluate((title) => (title.textContent = 'Alerts, Notifications, Preferences and Delivery History'));

    expect(await appBarTitle(page)).toMatchObject({ singleLine: true, ellipsis: true, truncated: true });
    const controls = await appBarControls(page);
    expect(controls.map((control) => control.label)).toEqual(['menu', 'notifications', 'account']);
    expect(controls.every((control) => control.inViewport)).toBe(true);
    await checks.noSidewaysScroll(page, 'compact', 'admin');
  });

  test('switches theme from the avatar menu like the wide theme icon', async ({ page }) => {
    await openNotifications(page);
    await page.getByRole('button', { name: 'account' }).click();
    await page.getByRole('menuitem', { name: 'Theme' }).click();
    await chooseDark(page);
  });

  test('links to the documentation from the avatar menu like the wide documentation icon', async ({ page }) => {
    await openNotifications(page);
    await page.getByRole('button', { name: 'account' }).click();
    await expect(page.getByRole('menuitem', { name: 'Documentation' })).toHaveAttribute('href', '/docs/');
  });
});

test.describe('app bar at 1440x900', () => {
  test.use({ viewport: { width: 1440, height: 900 }, colorScheme: 'light' });

  test('switches theme from the theme icon', async ({ page }) => {
    await openNotifications(page);
    await page.getByRole('button', { name: 'theme switcher' }).click();
    await chooseDark(page);
  });

  test('links to the documentation from the documentation icon', async ({ page }) => {
    await openNotifications(page);
    await expect(page.getByRole('link', { name: 'documentation' })).toHaveAttribute('href', '/docs/');
  });
});
