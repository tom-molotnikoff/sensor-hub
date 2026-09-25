import { expect, test, type Locator, type Page } from './test';
import {
  contractViewports,
  narrowWide,
  navBackground,
  navCollapsedKey,
  saveNav,
  viewports,
  wideViewports,
  type NavState,
} from './checks';
import { signIn } from './users';

const activeBackground = 'rgba(237, 81, 37, 0.18)';
const indicator = 'rgb(237, 81, 37)';
const darkDivider = 'rgb(51, 51, 51)';
const navWidth: Record<NavState, number> = { expanded: 256, rail: 72 };
const navStates = ['expanded', 'rail'] as const;

async function paintedBackground(nav: Locator) {
  return nav.evaluate((element) => {
    for (let node: Element | null = element; node; node = node.parentElement) {
      const style = getComputedStyle(node);
      if (style.backgroundColor !== 'rgba(0, 0, 0, 0)') return { color: style.backgroundColor, image: style.backgroundImage };
    }
    return null;
  });
}

async function openNav(page: Page, path: string, state?: NavState) {
  await signIn(page, 'admin');
  if (state) await saveNav(page, state);
  await page.goto(path);
  await page.waitForLoadState('networkidle');
  if (page.viewportSize()!.width < narrowWide.width) await page.getByRole('button', { name: 'menu' }).click();
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

      test('is charcoal and leaves part of the page visible', async ({ page }) => {
        const nav = await openNav(page, '/dashboard');

        expect(await paintedBackground(nav), 'nav background').toEqual({ color: navBackground[colorScheme], image: 'none' });
        await expect(nav.locator('hr').first(), 'nav parts resolve the dark scheme').toHaveCSS('border-bottom-color', darkDivider);
        const box = await nav.boundingBox();
        expect(box!.x + box!.width, 'drawer right edge').toBeLessThan(viewport.width);
      });
    });
  }
}

for (const viewport of wideViewports) {
  test.describe(`permanent nav at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height }, colorScheme: 'light' });

    for (const state of navStates) {
      test(`sits beside the page ${state} without any click`, async ({ page }) => {
        const nav = await openNav(page, '/dashboard', state);

        const [navBox, pageBox] = [await nav.boundingBox(), await page.locator('[data-ui=page]').boundingBox()];
        expect(navBox, 'nav box').toMatchObject({ x: 0, y: 0, width: navWidth[state], height: viewport.height });
        expect(pageBox!.x, 'page left edge').toBeGreaterThanOrEqual(navBox!.width);
        await expect(page.locator('[data-ui=app-bar]')).toHaveCount(0);
      });

      test(`opens the bell panel to the right of the ${state} nav and inside the viewport`, async ({ page }) => {
        const nav = await openNav(page, '/dashboard', state);
        const navBox = await nav.boundingBox();

        await nav.getByRole('button', { name: 'notifications', exact: true }).click();

        const paper = page.locator('[data-ui=menu-panel] .MuiPaper-root');
        await expect(paper).toHaveCSS('opacity', '1');
        const panelBox = await paper.boundingBox();
        expect(panelBox!.x, 'panel left edge right of the nav').toBeGreaterThanOrEqual(navBox!.x + navBox!.width);
        expect(panelBox!.y, 'panel top edge').toBeGreaterThanOrEqual(0);
        expect(panelBox!.x + panelBox!.width, 'panel right edge').toBeLessThanOrEqual(viewport.width);
        expect(panelBox!.y + panelBox!.height, 'panel bottom edge').toBeLessThanOrEqual(viewport.height);
        await expect(page.getByRole('menu').getByText('Notifications', { exact: true })).toBeVisible();
      });
    }

    test('shows the logo, the name and the bell with the unread count, left to right', async ({ page }) => {
      const nav = await openNav(page, '/dashboard', 'expanded');
      const brand = nav.locator('[data-ui=nav-brand]');

      const logo = brand.locator('img');
      await expect(logo).toHaveAttribute('src', '/sensor_hub.svg');
      expect(await logo.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0), 'logo loaded').toBe(true);
      const name = brand.getByText('Sensor Hub', { exact: true });
      const bell = brand.getByRole('button', { name: 'notifications', exact: true });
      const [logoBox, nameBox, bellBox] = [await logo.boundingBox(), await name.boundingBox(), await bell.boundingBox()];
      expect(logoBox!.x + logoBox!.width, 'logo before the name').toBeLessThanOrEqual(nameBox!.x);
      expect(nameBox!.x + nameBox!.width, 'name before the bell').toBeLessThanOrEqual(bellBox!.x);
      await expect(bell.locator('.MuiBadge-badge'), 'unread count').toHaveText(/^[1-9]\d*$/);
      await expect(brand.getByRole('button', { name: 'close navigation' })).toHaveCount(0);
    });

    test('stacks the logo over the bell on the rail, hides the name and shows only the avatar', async ({ page }) => {
      const nav = await openNav(page, '/dashboard', 'rail');
      const brand = nav.locator('[data-ui=nav-brand]');

      const logo = brand.locator('img');
      const bell = brand.getByRole('button', { name: 'notifications', exact: true });
      const [logoBox, bellBox] = [await logo.boundingBox(), await bell.boundingBox()];
      expect(logoBox!.y + logoBox!.height, 'logo above the bell').toBeLessThanOrEqual(bellBox!.y);
      await expect(brand.getByText('Sensor Hub', { exact: true })).toHaveCount(0);
      await expect(bell.locator('.MuiBadge-badge'), 'unread count').toHaveText(/^[1-9]\d*$/);
      const block = nav.locator('[data-ui=nav-account]');
      await expect(block.locator('.MuiAvatar-root')).toHaveText('T');
      await expect(block).toHaveText('T');
    });

    test('opens the account menu to the right of the rail and inside the viewport', async ({ page }) => {
      const nav = await openNav(page, '/dashboard', 'rail');
      const navBox = await nav.boundingBox();

      await nav.locator('[data-ui=nav-account]').click();

      const menu = page.getByRole('menu', { name: 'Signed in as testadmin' });
      const paper = page.locator('[data-ui=nav-account-menu] .MuiPaper-root');
      await expect(paper).toHaveCSS('opacity', '1');
      const menuBox = await paper.boundingBox();
      expect(menuBox!.x, 'menu left edge right of the rail').toBeGreaterThanOrEqual(navBox!.x + navBox!.width);
      expect(menuBox!.y, 'menu top edge').toBeGreaterThanOrEqual(0);
      expect(menuBox!.x + menuBox!.width, 'menu right edge').toBeLessThanOrEqual(viewport.width);
      expect(menuBox!.y + menuBox!.height, 'menu bottom edge').toBeLessThanOrEqual(viewport.height);
      await expect(menu.getByRole('menuitem', { name: 'Logout' })).toBeInViewport({ ratio: 1 });
    });
  });
}

const navBoxWidth = async (nav: Locator) => (await nav.boundingBox())!.width;

const savedNav = (page: Page) => page.evaluate((key) => localStorage.getItem(key), navCollapsedKey);

for (const [width, state] of [
  [900, 'rail'],
  [1199, 'rail'],
  [1200, 'expanded'],
  [1440, 'expanded'],
] as const) {
  test.describe(`nav with nothing saved at ${width}x900`, () => {
    test.use({ viewport: { width, height: 900 }, colorScheme: 'light' });

    test(`loads ${state}`, async ({ page }) => {
      const nav = await openNav(page, '/dashboard');

      expect(await navBoxWidth(nav), 'nav width').toBe(navWidth[state]);
      await expect(nav.locator('[data-ui=nav-collapse]')).toHaveAttribute('aria-expanded', String(state === 'expanded'));
      expect(await savedNav(page), 'saved choice').toBeNull();
    });
  });
}

test.describe('nav rail at 1440x900', () => {
  test.use({ viewport: { width: 1440, height: 900 }, colorScheme: 'light' });

  test('collapses to a rail and expands again with the toggle, saving each choice', async ({ page }) => {
    const nav = await openNav(page, '/dashboard');
    const toggle = nav.locator('[data-ui=nav-collapse]');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');

    await toggle.click();

    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(await navBoxWidth(nav), 'collapsed width').toBe(navWidth.rail);
    expect(await savedNav(page), 'saved choice').toBe('true');

    await toggle.click();

    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(await navBoxWidth(nav), 'expanded width').toBe(navWidth.expanded);
    expect(await savedNav(page), 'saved choice').toBe('false');
  });

  test('stays collapsed after a reload', async ({ page }) => {
    const nav = await openNav(page, '/dashboard');
    await nav.locator('[data-ui=nav-collapse]').click();
    await expect(nav.locator('[data-ui=nav-collapse]')).toHaveAttribute('aria-expanded', 'false');

    await page.reload();
    await page.waitForLoadState('networkidle');

    await expect(nav.locator('[data-ui=nav-collapse]')).toHaveAttribute('aria-expanded', 'false');
    expect(await navBoxWidth(nav), 'width after reload').toBe(navWidth.rail);
  });

  test('follows the window across 1200px while nothing is saved', async ({ page }) => {
    const nav = await openNav(page, '/dashboard');

    await page.setViewportSize({ width: 1000, height: 900 });
    await expect.poll(() => navBoxWidth(nav), { message: 'width at 1000px' }).toBe(navWidth.rail);

    await page.setViewportSize({ width: 1440, height: 900 });
    await expect.poll(() => navBoxWidth(nav), { message: 'width at 1440px' }).toBe(navWidth.expanded);
    expect(await savedNav(page), 'saved choice').toBeNull();
  });

  for (const state of navStates) {
    test(`stays ${state} across 1200px once saved`, async ({ page }) => {
      const nav = await openNav(page, '/dashboard', state);
      expect(await navBoxWidth(nav), 'width at 1440px').toBe(navWidth[state]);

      await page.setViewportSize({ width: 1000, height: 900 });
      await page.waitForFunction(() => window.innerWidth === 1000);
      expect(await navBoxWidth(nav), 'width at 1000px').toBe(navWidth[state]);

      await page.setViewportSize({ width: 1440, height: 900 });
      await page.waitForFunction(() => window.innerWidth === 1440);
      expect(await navBoxWidth(nav), 'width back at 1440px').toBe(navWidth[state]);
    });
  }

  test('shows an item label in a tooltip to the right on hover', async ({ page }) => {
    const nav = await openNav(page, '/dashboard', 'rail');
    const item = nav.getByRole('button', { name: 'Sensors', exact: true });

    await item.hover();

    const tooltip = page.getByRole('tooltip');
    await expect(tooltip).toHaveText('Sensors');
    const [itemBox, tooltipBox] = [await item.boundingBox(), await tooltip.boundingBox()];
    expect(tooltipBox!.x, 'tooltip left edge right of the item').toBeGreaterThanOrEqual(itemBox!.x + itemBox!.width);
  });

  test('shows an item label in a tooltip to the right on keyboard focus', async ({ page }) => {
    const nav = await openNav(page, '/dashboard', 'rail');
    const item = nav.getByRole('button', { name: 'MQTT', exact: true });

    await nav.getByRole('button', { name: 'Properties', exact: true }).focus();
    await page.keyboard.press('Tab');

    await expect(item).toBeFocused();
    const tooltip = page.getByRole('tooltip');
    await expect(tooltip).toHaveText('MQTT');
    const [itemBox, tooltipBox] = [await item.boundingBox(), await tooltip.boundingBox()];
    expect(tooltipBox!.x, 'tooltip left edge right of the item').toBeGreaterThanOrEqual(itemBox!.x + itemBox!.width);
  });

  test('keeps the rail when navigating', async ({ page }) => {
    const nav = await openNav(page, '/dashboard');
    await nav.locator('[data-ui=nav-collapse]').click();

    await nav.getByRole('button', { name: 'Sensors', exact: true }).click();

    await expect(page).toHaveURL(/\/sensors-overview$/);
    await expect(nav.locator('[data-ui=nav-collapse]')).toHaveAttribute('aria-expanded', 'false');
    expect(await navBoxWidth(nav), 'width after navigating').toBe(navWidth.rail);
  });
});

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

  test('has only the account block in the foot and no collapse toggle', async ({ page }) => {
    const nav = await openNav(page, '/dashboard');

    const buttons = nav.locator('[data-ui=nav-foot]').getByRole('button');
    await expect(buttons).toHaveCount(1);
    await expect(buttons).toHaveAttribute('data-ui', 'nav-account');
    await expect(nav.locator('[data-ui=nav-collapse]')).toHaveCount(0);
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
  await openNav(page, '/dashboard', 'expanded');
  const block = page.locator('[data-ui=nav-account]');
  await block.click();
  const menu = page.getByRole('menu', { name: 'Signed in as testadmin' });
  await expect(menu).toBeVisible();
  await expect(block).toHaveAttribute('aria-expanded', 'true');
  return { block, menu };
}

for (const viewport of contractViewports) {
  test.describe(`nav account menu at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height }, colorScheme: 'light' });

    test('shows the account block at the foot of the nav', async ({ page }) => {
      const nav = await openNav(page, '/dashboard', 'expanded');

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
