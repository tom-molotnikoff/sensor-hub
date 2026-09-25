import { expect, test, type Page } from './test';
import { appBarControls, checks, narrowWide, pageTitle, titleLocator, viewports } from './checks';
import { signIn } from './users';

async function openNotifications(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/notifications');
  await page.waitForLoadState('networkidle');
}

const longTitle = 'Alerts, Notifications, Preferences and Delivery History';

async function chooseDark(page: Page) {
  await page.getByRole('menuitem', { name: 'Dark' }).click();
  await expect(page.locator('html')).toHaveClass(/\bdark\b/);
}

async function openAccountMenu(page: Page) {
  await page.locator('[data-ui=nav-account]').click();
  const menu = page.getByRole('menu', { name: 'Signed in as testadmin' });
  await expect(menu).toBeVisible();
  return menu;
}

test.describe('app bar at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 }, colorScheme: 'light' });

  test('truncates a long title to one line and keeps the bell and avatar on screen', async ({ page }) => {
    await openNotifications(page);
    await titleLocator(page, 'compact').evaluate((title, text) => (title.textContent = text), longTitle);

    expect(await pageTitle(page, 'compact')).toMatchObject({ singleLine: true, ellipsis: true, truncated: true });
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

test.describe('wide shell at 1440x900', () => {
  test.use({ viewport: { width: viewports[1].width, height: viewports[1].height }, colorScheme: 'light' });

  test('has no theme icon and switches theme from the account menu', async ({ page }) => {
    await openNotifications(page);
    await expect(page.getByRole('button', { name: 'theme switcher' })).toHaveCount(0);

    const menu = await openAccountMenu(page);
    await menu.getByRole('menuitemradio', { name: 'Dark' }).click();

    await expect(page.locator('html')).toHaveClass(/\bdark\b/);
  });

  test('has no documentation icon and links to the documentation from the account menu', async ({ page }) => {
    await openNotifications(page);
    await expect(page.getByRole('link', { name: 'documentation', exact: true })).toHaveCount(0);

    const menu = await openAccountMenu(page);

    await expect(menu.getByRole('menuitem', { name: 'Documentation opens in a new tab' })).toHaveAttribute('href', '/docs/');
  });

  test('has no avatar button and shows the account block in the nav', async ({ page }) => {
    await openNotifications(page);
    await expect(page.getByRole('button', { name: 'account', exact: true })).toHaveCount(0);

    await expect(page.getByRole('navigation', { name: 'Main' }).locator('[data-ui=nav-account]')).toContainText('testadmin');
  });
});

test.describe(`page header at ${narrowWide.width}x${narrowWide.height} with the nav expanded`, () => {
  test.use({ viewport: { width: narrowWide.width, height: narrowWide.height }, colorScheme: 'light' });

  test('truncates a long title to one line and keeps the actions on one row', async ({ page }) => {
    await openNotifications(page);
    await titleLocator(page, 'wide').evaluate((title, text) => (title.textContent = text), `${longTitle} ${longTitle}`);
    await page.locator('[data-ui=page-actions]').evaluate((actions) => {
      for (const label of ['Export', 'Mark all read', 'Preferences']) {
        const action = document.createElement('button');
        action.textContent = label;
        actions.append(action);
      }
    });

    expect(await pageTitle(page, 'wide')).toMatchObject({ singleLine: true, ellipsis: true, truncated: true });
    const actions = await page
      .locator('[data-ui=page-actions] > button')
      .evaluateAll((buttons) => buttons.map((button) => button.getBoundingClientRect()).map(({ top, right }) => ({ top, right })));
    expect(new Set(actions.map((action) => action.top)).size, 'action rows').toBe(1);
    expect(actions.filter((action) => action.right > narrowWide.width), 'actions outside the viewport').toEqual([]);
    await checks.noSidewaysScroll(page, 'wide', 'admin');
  });
});
