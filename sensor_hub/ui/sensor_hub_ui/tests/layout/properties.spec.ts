import { expect, test, type Page } from './test';
import { viewports } from './checks';
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

test.describe('Properties Overview at 1440x900', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('pins the header to the top and keeps the rail the same distance below it when a later group is opened from the rail', async ({ page }) => {
    const { rail } = await openProperties(page);
    const header = page.locator('[data-ui=sticky-bar]');
    const headerBox = (await header.boundingBox())!;
    const railGap = (await rail.boundingBox())!.y - headerBox.y;

    const last = rail.getByRole('link').last();
    await last.click();
    await expect(last).toHaveAttribute('aria-current', 'true');

    expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(0);
    expect((await header.boundingBox())!.y).toBeCloseTo(0, 0);
    expect((await rail.boundingBox())!.y).toBeCloseTo(railGap, 0);
  });
});
