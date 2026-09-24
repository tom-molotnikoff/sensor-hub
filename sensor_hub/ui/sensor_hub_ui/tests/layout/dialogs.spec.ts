import { expect, test, type Page } from '@playwright/test';
import { signIn } from './users';

const viewports = [
  { tier: 'compact', width: 390, height: 844 },
  { tier: 'wide', width: 1440, height: 900 },
] as const;

const dialogs: { name: string; path: string; open: (page: Page) => Promise<void> }[] = [
  {
    name: 'Create Alert',
    path: '/notifications',
    open: (page) => page.getByRole('button', { name: 'Create Alert Rule' }).first().click(),
  },
  {
    name: 'Widget Config',
    path: '/dashboard',
    open: async (page) => {
      await page.getByRole('button', { name: 'Edit dashboard' }).click();
      await page.locator('[data-widget-state]').getByRole('button').first().click();
    },
  },
  {
    name: 'Delete sensor confirmation',
    path: '/sensor/1',
    open: (page) => page.getByRole('button', { name: 'Delete', exact: true }).click(),
  },
];

async function paperBox(page: Page) {
  const paper = page.getByRole('dialog');
  await expect(paper).toBeVisible();
  await expect(paper).toHaveCSS('opacity', '1');
  return paper.evaluate((element) => {
    const { left, top, right, bottom } = element.getBoundingClientRect();
    return { left, top, right, bottom, viewportWidth: window.innerWidth, viewportHeight: window.innerHeight };
  });
}

for (const viewport of viewports) {
  test.describe(`dialogs at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    for (const dialog of dialogs) {
      test(`${dialog.name} ${viewport.tier === 'compact' ? 'fills the viewport' : 'is a centred box'}`, async ({ page }) => {
        await signIn(page, 'admin');
        await page.goto(dialog.path);
        await page.waitForLoadState('networkidle');
        await dialog.open(page);

        const box = await paperBox(page);
        if (viewport.tier === 'compact') {
          expect(box).toEqual({
            left: 0,
            top: 0,
            right: box.viewportWidth,
            bottom: box.viewportHeight,
            viewportWidth: viewport.width,
            viewportHeight: viewport.height,
          });
        } else {
          expect(box.left).toBeGreaterThan(0);
          expect(box.top).toBeGreaterThan(0);
          expect(Math.abs(box.left - (box.viewportWidth - box.right))).toBeLessThanOrEqual(1);
          expect(Math.abs(box.top - (box.viewportHeight - box.bottom))).toBeLessThanOrEqual(1);
        }
      });
    }
  });
}
