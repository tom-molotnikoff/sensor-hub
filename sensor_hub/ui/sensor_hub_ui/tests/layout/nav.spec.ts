import { expect, test, type Locator, type Page } from './test';
import { viewports } from './checks';
import { signIn } from './users';

const navBackground = { light: 'rgb(33, 30, 27)', dark: 'rgb(18, 18, 18)' } as const;
const activeBackground = 'rgba(237, 81, 37, 0.18)';
const indicator = 'rgb(237, 81, 37)';
const darkDivider = 'rgb(51, 51, 51)';

async function paintedBackground(nav: Locator) {
  return nav.evaluate((element) => {
    for (let node: Element | null = element; node; node = node.parentElement) {
      const style = getComputedStyle(node);
      if (style.backgroundColor !== 'rgba(0, 0, 0, 0)') return { color: style.backgroundColor, image: style.backgroundImage };
    }
    return null;
  });
}

async function openNav(page: Page, path: string) {
  await signIn(page, 'admin');
  await page.goto(path);
  await page.waitForLoadState('networkidle');
  await page.getByRole('button', { name: 'menu' }).click();
  const nav = page.getByRole('navigation', { name: 'Main' });
  await expect(nav).toBeVisible();
  return nav;
}

async function itemStates(nav: Locator) {
  return nav.locator('[data-ui=nav-item] > :first-child').evaluateAll((items) =>
    items.map((item) => {
      const bar = getComputedStyle(item, '::before');
      return {
        label: item.textContent,
        current: item.getAttribute('aria-current'),
        background: getComputedStyle(item).backgroundColor,
        bar:
          bar.content === 'none'
            ? null
            : { background: bar.backgroundColor, width: bar.width, left: bar.left, position: bar.position },
      };
    }),
  );
}

for (const viewport of viewports) {
  for (const colorScheme of ['light', 'dark'] as const) {
    test.describe(`nav at ${viewport.width}x${viewport.height} in ${colorScheme}`, () => {
      test.use({ viewport: { width: viewport.width, height: viewport.height }, colorScheme });

      test('is a charcoal drawer that leaves part of the page visible', async ({ page }) => {
        const nav = await openNav(page, '/dashboard');

        expect(await paintedBackground(nav), 'nav background').toEqual({ color: navBackground[colorScheme], image: 'none' });
        await expect(nav.locator('hr').first(), 'nav parts resolve the dark scheme').toHaveCSS('border-bottom-color', darkDivider);
        const box = await nav.boundingBox();
        expect(box!.x + box!.width, 'drawer right edge').toBeLessThan(viewport.width);
      });
    });
  }
}

test.describe('nav at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 }, colorScheme: 'light' });

  test('shows the logo, the name and a close button in the brand row', async ({ page }) => {
    const nav = await openNav(page, '/dashboard');
    const brand = nav.locator('[data-ui=nav-brand]');

    const logo = brand.locator('img');
    await expect(logo).toHaveAttribute('src', '/sensor_hub.svg');
    expect(await logo.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0), 'logo loaded').toBe(true);
    await expect(brand).toContainText('Sensor Hub');
    await brand.getByRole('button', { name: 'close navigation' }).click();
    await expect(nav).toHaveCount(0);
  });

  test('closes and navigates when an item is picked', async ({ page }) => {
    const nav = await openNav(page, '/dashboard');

    await nav.getByRole('button', { name: 'Sensors' }).click();

    await expect(page).toHaveURL(/\/sensors-overview$/);
    await expect(nav).toHaveCount(0);
  });
});

test.describe('nav current page at 1440x900', () => {
  test.use({ viewport: { width: 1440, height: 900 }, colorScheme: 'light' });

  const routes = [
    ['/sensor/7', 'Sensors'],
    ['/sensors-overview', 'Sensors'],
    ['/dashboard', 'Dashboards'],
    ['/data-retention', 'Data Retention'],
    ['/properties-overview', 'Properties'],
    ['/mqtt', 'MQTT'],
    ['/notifications', 'Alerts & Notifications'],
    ['/admin', 'User Management'],
  ] as const;

  for (const [path, label] of routes) {
    test(`marks ${label} on ${path} with the selected background and the indicator bar`, async ({ page }) => {
      const states = await itemStates(await openNav(page, path));

      const marked = states.filter((state) => state.current === 'page');
      expect(marked.map((state) => state.label)).toEqual([label]);
      expect(marked[0]).toMatchObject({
        background: activeBackground,
        bar: { background: indicator, width: '3px', left: '0px', position: 'absolute' },
      });
      const others = states.filter((state) => state.label !== label);
      expect(others.filter((state) => state.current !== null || state.bar !== null || state.background === activeBackground)).toEqual([]);
    });
  }

  test('marks nothing on an account page', async ({ page }) => {
    const states = await itemStates(await openNav(page, '/account/sessions'));

    expect(states.filter((state) => state.current !== null || state.bar !== null || state.background === activeBackground)).toEqual([]);
  });
});

async function openAccountMenu(page: Page) {
  await openNav(page, '/dashboard');
  const block = page.locator('[data-ui=nav-account]');
  await block.click();
  const menu = page.getByRole('menu', { name: 'Signed in as testadmin' });
  await expect(menu).toBeVisible();
  await expect(block).toHaveAttribute('aria-expanded', 'true');
  return { block, menu };
}

for (const viewport of viewports) {
  test.describe(`nav account menu at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height }, colorScheme: 'light' });

    test('shows the account block at the foot of the nav', async ({ page }) => {
      const nav = await openNav(page, '/dashboard');

      const foot = nav.locator('[data-ui=nav-foot]');
      const block = foot.locator('[data-ui=nav-account]');
      await expect(block).toContainText('testadmin');
      await expect(block).toContainText('admin');
      const [footBox, navBox] = [await foot.boundingBox(), await nav.boundingBox()];
      expect(footBox!.y + footBox!.height, 'foot bottom edge').toBeCloseTo(navBox!.y + navBox!.height, 0);
    });

    test('opens above the block and lies entirely inside the viewport', async ({ page }) => {
      const { block, menu } = await openAccountMenu(page);

      const paper = page.locator('[data-ui=nav-account-menu] .MuiPaper-root');
      await expect(paper).toHaveCSS('opacity', '1');
      const [menuBox, blockBox] = [await paper.boundingBox(), await block.boundingBox()];
      expect(menuBox!.y + menuBox!.height, 'menu bottom edge above the block').toBeLessThanOrEqual(blockBox!.y + 1);
      expect(menuBox!.x, 'menu left edge').toBeGreaterThanOrEqual(0);
      expect(menuBox!.y, 'menu top edge').toBeGreaterThanOrEqual(0);
      expect(menuBox!.x + menuBox!.width, 'menu right edge').toBeLessThanOrEqual(viewport.width);
      expect(menuBox!.y + menuBox!.height, 'menu bottom edge').toBeLessThanOrEqual(viewport.height);
      await expect(menu.getByRole('menuitem', { name: 'Logout' })).toBeInViewport({ ratio: 1 });
    });
  });
}

test.describe('nav account menu at 1440x900 in light', () => {
  test.use({ viewport: { width: 1440, height: 900 }, colorScheme: 'light' });

  test('switches to dark and shows Dark as selected', async ({ page }) => {
    const { menu } = await openAccountMenu(page);

    await menu.getByRole('menuitemradio', { name: 'Dark' }).click();

    await expect(page.locator('html')).toHaveClass(/\bdark\b/);
    await expect(menu.getByRole('menuitemradio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true');
    await expect(menu.getByRole('menuitemradio', { name: 'Light' })).toHaveAttribute('aria-checked', 'false');
  });

  test('opens the documentation in a new tab', async ({ page }) => {
    const { menu } = await openAccountMenu(page);

    const docs = menu.getByRole('menuitem', { name: 'Documentation opens in a new tab' });
    const [popup] = await Promise.all([page.waitForEvent('popup'), docs.click()]);

    await expect(popup).toHaveURL(/\/docs\/$/);
    await expect(page).toHaveURL(/\/dashboard$/);
  });

  test('logs out and lands on /login', async ({ page }) => {
    const { menu } = await openAccountMenu(page);

    await menu.getByRole('menuitem', { name: 'Logout' }).click();

    await expect(page).toHaveURL(/\/login$/);
    expect((await page.request.get('/api/auth/me')).status(), 'session after logout').toBe(401);
  });
});
