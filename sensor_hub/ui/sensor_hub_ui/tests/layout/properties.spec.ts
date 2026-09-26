import { expect, test, type Locator, type Page } from './test';
import { contractViewports, viewports, type Tier } from './checks';
import { signIn } from './users';

async function openProperties(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/properties-overview');
  await page.waitForLoadState('networkidle');
  const rail = page.getByRole('navigation', { name: 'Property groups' });
  const sections = page.locator('[data-ui=anchor-stack-sections]');
  await expect(sections.locator('[data-ui=card]').first()).toBeVisible();
  return { rail, sections };
}

for (const viewport of viewports) {
  test.describe(`Properties Overview at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test(viewport.tier === 'wide' ? 'sits the rail beside the sections' : 'stacks the rail above the sections', async ({ page }) => {
      const { rail, sections } = await openProperties(page);
      const railBox = (await rail.boundingBox())!;
      const sectionsBox = (await sections.boundingBox())!;

      if (viewport.tier === 'wide') {
        expect(railBox.x + railBox.width).toBeLessThanOrEqual(sectionsBox.x);
        expect(Math.abs(railBox.y - sectionsBox.y)).toBeLessThan(1);
      } else {
        expect(railBox.y + railBox.height).toBeLessThanOrEqual(sectionsBox.y);
        expect(Math.abs(railBox.width - sectionsBox.width)).toBeLessThan(1);
      }
    });
  });
}

function pageTitle(page: Page, tier: Tier) {
  return page.locator(tier === 'wide' ? '[data-ui=page-header]' : '[data-ui=app-bar]');
}

async function top(locator: Locator) {
  return (await locator.boundingBox())!.y;
}

function innerScrollers(page: Page) {
  return page.evaluate(() =>
    [...document.querySelectorAll<HTMLElement>('body *')]
      .filter((element) => ['auto', 'scroll'].includes(getComputedStyle(element).overflowY))
      .filter((element) => element.scrollHeight > element.clientHeight)
      .map((element) => `${element.tagName.toLowerCase()} ${element.getAttribute('data-ui')} ${element.scrollHeight}/${element.clientHeight}`),
  );
}

for (const viewport of contractViewports) {
  test.describe(`Properties Overview scrolling at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test('scrolls the groups with the document alone, with no inner scroller', async ({ page }) => {
      await openProperties(page);
      const scrollable = await page.evaluate(() => document.documentElement.scrollHeight - window.innerHeight);

      expect(scrollable).toBeGreaterThan(0);
      expect(await innerScrollers(page)).toEqual([]);
    });

    test('keeps the page title and the properties bar still while a wheel over the groups scrolls the page', async ({ page }) => {
      const { rail, sections } = await openProperties(page);
      const title = pageTitle(page, viewport.tier);
      const bar = page.locator('[data-ui=sticky-bar]');
      const before = { title: await top(title), bar: await top(bar), rail: await top(rail), sections: await top(sections) };

      const box = (await sections.boundingBox())!;
      await page.mouse.move(box.x + box.width / 2, Math.min(viewport.height - 1, box.y + box.height / 2));
      await page.mouse.wheel(0, 200);
      await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(200);

      expect(await top(title)).toBeCloseTo(before.title, 0);
      expect(await top(title)).toBeCloseTo(0, 0);
      expect(await top(bar)).toBeCloseTo(before.bar, 0);
      expect(await top(sections)).toBeCloseTo(before.sections - 200, 0);
      if (viewport.tier === 'wide') expect(await top(rail)).toBeCloseTo(before.rail, 0);
      expect(await innerScrollers(page)).toEqual([]);
    });
  });
}

test.describe('Properties Overview at 1440x900', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('keeps the header and rail exactly where they start when a later group is opened from the rail', async ({ page }) => {
    const { rail } = await openProperties(page);
    const header = page.locator('[data-ui=sticky-bar]');
    const headerTop = await top(header);
    const railTop = await top(rail);

    const last = rail.getByRole('link').last();
    await last.click();
    await expect(last).toHaveAttribute('aria-current', 'true');

    expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(0);
    expect(await top(header)).toBeCloseTo(headerTop, 0);
    expect(await top(rail)).toBeCloseTo(railTop, 0);
  });
});
