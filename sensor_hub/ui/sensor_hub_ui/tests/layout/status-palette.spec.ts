import { expect, test, type Page } from '@playwright/test';
import { signIn } from './users';

async function healthPieGoodColour(page: Page) {
  await page.goto('/sensors-overview');
  await page.waitForLoadState('networkidle');
  const card = page.getByRole('heading', { name: 'Sensor Health', exact: true }).locator('xpath=..');
  const goodSlice = card.locator('.recharts-sector').first();
  const fill = () => goodSlice.evaluate((slice) => getComputedStyle(slice).fill);
  await expect.poll(fill).toMatch(/^rgb/);
  return fill();
}

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`status colours in ${colorScheme}`, () => {
    test.use({ viewport: { width: 1440, height: 900 }, colorScheme });

    test('a good sensor is the same colour on the health pie, the sensor page chip and the uptime bar', async ({ page }) => {
      await signIn(page, 'admin');
      const good = await healthPieGoodColour(page);

      await page.goto('/sensor/1');
      await expect(page.getByText('good', { exact: true })).toHaveCSS('color', good);

      await page.goto('/dashboard');
      await expect(page.locator('.MuiLinearProgress-bar')).toHaveCSS('background-color', good);
    });
  });
}
