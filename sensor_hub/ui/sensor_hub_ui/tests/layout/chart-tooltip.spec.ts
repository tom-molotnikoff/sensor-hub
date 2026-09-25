import { hexToRgb, type Palette } from '@mui/material/styles';
import { theme } from '../../src/ui/theme';
import { expect, test } from './test';
import { signIn } from './users';

const isoTimestamp = /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/;
const localeTime = /^(Mon|Tue|Wed|Thu|Fri|Sat|Sun) \d{1,2} (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sept?|Oct|Nov|Dec), \d{2}:\d{2}$/;

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`readings chart tooltip in ${colorScheme}`, () => {
    test.use({ viewport: { width: 1440, height: 900 }, colorScheme, locale: 'en-GB' });

    test('sits on the paper surface with a divider border, text-coloured names and a locale time header', async ({ page }) => {
      const { palette } = (theme as unknown as { colorSchemes: Record<string, { palette: Palette }> }).colorSchemes[colorScheme];
      await signIn(page, 'admin');
      await page.goto('/dashboard');

      const frame = page.locator('[data-widget-state]', { has: page.getByText(/^Readings Chart(:|$)/) });
      const chart = frame.locator('[data-ui=chart-area] .recharts-wrapper');
      await expect(chart.locator('.recharts-line').first()).toBeVisible();
      await chart.hover();

      const tooltip = frame.locator('[data-ui=chart-tooltip]');
      await expect(tooltip).toBeVisible();
      await expect(tooltip).toHaveCSS('background-color', hexToRgb(palette.background.paper));
      for (const side of ['top', 'right', 'bottom', 'left']) {
        await expect(tooltip).toHaveCSS(`border-${side}-color`, hexToRgb(palette.divider));
      }
      await expect(tooltip.locator('[data-ui=chart-tooltip-name]').first()).toHaveCSS('color', hexToRgb(palette.text.primary));

      const time = tooltip.locator('[data-ui=chart-tooltip-time]');
      await expect(time).toHaveText(localeTime);
      await expect(time).not.toHaveText(isoTimestamp);
    });
  });
}
