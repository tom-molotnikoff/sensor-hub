import { expect, test, type Locator, type Page } from './test';
import { clippedGlyphs, saveNav, viewports, type Tier } from './checks';
import { signIn } from './users';

const descenders = 'Typography jpgqy';

interface TitleCase {
  name: string;
  path: string;
  tiers: readonly Tier[];
  title: (page: Page) => Locator;
}

const titles: readonly TitleCase[] = [
  { name: 'page title', path: '/sensor/1', tiers: ['wide'], title: (page) => page.locator('[data-ui=page-header] > h1') },
  { name: 'dashboard title', path: '/dashboard', tiers: ['wide'], title: (page) => page.locator('[data-ui=page-header] > h1') },
  { name: 'app bar title', path: '/sensor/1', tiers: ['compact'], title: (page) => page.locator('[data-ui=app-bar-title]') },
  { name: 'card title', path: '/sensor/1', tiers: ['compact', 'wide'], title: (page) => page.locator('[data-ui=card-header] > h2').first() },
  { name: 'sticky bar title', path: '/properties-overview', tiers: ['compact', 'wide'], title: (page) => page.locator('[data-ui=sticky-bar] > h2') },
  { name: 'widget frame title', path: '/dashboard', tiers: ['compact', 'wide'], title: (page) => page.locator('[data-ui=frame-title]').first() },
  { name: 'nav brand', path: '/sensor/1', tiers: ['wide'], title: (page) => page.locator('[data-ui=nav-brand]') },
];

for (const viewport of viewports) {
  test.describe(`titles at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    for (const { name, path, title } of titles.filter((entry) => entry.tiers.includes(viewport.tier))) {
      test(`keeps the descenders of the ${name} inside its box`, async ({ page }) => {
        await signIn(page, 'admin');
        await saveNav(page, 'expanded');
        await page.goto(path);
        await page.waitForLoadState('networkidle');
        const element = title(page);
        await expect(element).toBeVisible();

        await element.evaluate((root, text) => {
          const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
          for (let node = walker.nextNode(); node; node = walker.nextNode()) {
            if (node.textContent?.trim()) {
              node.textContent = text;
              return;
            }
          }
        }, descenders);

        await expect(element).toContainText(descenders);
        expect(await clippedGlyphs(element)).toEqual([]);
      });
    }
  });
}
