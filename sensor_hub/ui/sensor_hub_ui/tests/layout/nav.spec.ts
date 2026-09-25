import { expect, test, type Locator, type Page } from './test';
import { viewports } from './checks';
import { signIn } from './users';

const navBackground = { light: 'rgb(33, 30, 27)', dark: 'rgb(18, 18, 18)' } as const;
const activeBackground = 'rgba(237, 81, 37, 0.18)';
const indicator = 'rgb(237, 81, 37)';

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

        await expect(nav).toHaveCSS('background-color', navBackground[colorScheme]);
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
