import { expect, test } from '@playwright/test';
import { checks } from './checks';
import { layoutChecks, routes } from './routes';
import { signIn } from './users';

const viewports = [
  { tier: 'compact', width: 390, height: 844 },
  { tier: 'wide', width: 1440, height: 900 },
] as const;

const colorSchemes = ['light', 'dark'] as const;

for (const route of routes) {
  for (const user of route.users) {
    for (const viewport of viewports) {
      for (const colorScheme of colorSchemes) {
        test.describe(`${route.path} as ${user} at ${viewport.width}x${viewport.height} in ${colorScheme}`, () => {
          test.use({ viewport: { width: viewport.width, height: viewport.height }, colorScheme });

          test('holds the layout contract', async ({ page }) => {
            await signIn(page, user);
            await page.goto(route.path);
            await page.waitForLoadState('networkidle');
            await expect(page).toHaveURL(new RegExp(`${route.path}$`));
            await expect(page.locator('html')).toHaveClass(new RegExp(`\\b${colorScheme}\\b`));
            for (const check of route.checks ?? layoutChecks) {
              await checks[check](page, viewport.tier, user);
            }
          });
        });
      }
    }
  }
}
