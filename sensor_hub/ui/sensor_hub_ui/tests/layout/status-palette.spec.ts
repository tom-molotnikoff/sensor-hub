import { expect, test, type Page } from '@playwright/test';
import { signIn } from './users';

async function healthPieGoodColour(page: Page) {
  await page.goto('/sensors-overview');
  await page.waitForLoadState('networkidle');
  const card = page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'Sensor Health', exact: true }) });
  const goodSlice = card.locator('.recharts-sector').first();
  let fill = '';
  await expect
    .poll(async () => (fill = await goodSlice.evaluate((slice) => getComputedStyle(slice).fill)))
    .toMatch(/^rgb/);
  return fill;
}

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`status colours in ${colorScheme}`, () => {
    test.use({ viewport: { width: 1440, height: 900 }, colorScheme });

    test('a good sensor is the same colour on the health pie, the sensor page chip and the uptime bar', async ({ page }) => {
      await signIn(page, 'admin');
      const good = await healthPieGoodColour(page);

      await page.goto('/sensor/1');
      const healthChip = page.getByText('Health', { exact: true }).locator('xpath=..').locator('.MuiChip-root');
      await expect(healthChip).toHaveText('good');
      await expect(healthChip).toHaveCSS('color', good);

      await page.goto('/dashboard');
      await expect(page.locator('.MuiLinearProgress-bar')).toHaveCSS('background-color', good);
    });
  });
}

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`Sensor Summary status pill in ${colorScheme}`, () => {
    test.use({ viewport: { width: 390, height: 844 }, colorScheme });

    test('a good sensor has the same colour as the health pie slice', async ({ page }) => {
      await signIn(page, 'admin');
      const good = await healthPieGoodColour(page);
      const pill = page.locator('[data-ui=data-table-row]', { hasText: 'attic-bulb' }).locator('[data-ui=status-pill]');
      await expect(pill).toHaveText('good');
      await expect(pill).toHaveCSS('color', good);
    });
  });
}
