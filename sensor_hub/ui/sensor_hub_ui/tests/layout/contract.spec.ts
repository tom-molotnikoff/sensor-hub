import { expect, test } from './test';
import { checks, contractViewports, saveNav } from './checks';
import { defaultChecks, routes } from './routes';
import { signIn } from './users';

const colorSchemes = ['light', 'dark'] as const;

for (const route of routes) {
  for (const user of route.users) {
    for (const viewport of contractViewports) {
      for (const colorScheme of colorSchemes) {
        test.describe(`${route.path} as ${user} at ${viewport.width}x${viewport.height} in ${colorScheme}`, () => {
          test.use({ viewport: { width: viewport.width, height: viewport.height }, colorScheme });

          test('holds the layout contract', async ({ page }) => {
            await signIn(page, user);
            await saveNav(page, 'expanded');
            await page.goto(route.path);
            await page.waitForLoadState('networkidle');
            await expect(page).toHaveURL(new RegExp(`${route.path}$`));
            await expect(page.locator('html')).toHaveClass(new RegExp(`\\b${colorScheme}\\b`));
            for (const check of route.checks ?? defaultChecks) {
              await checks[check](page, viewport.tier, user);
            }
          });
        });
      }
    }
  }
}
