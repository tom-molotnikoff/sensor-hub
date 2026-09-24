import { expect, test } from '@playwright/test';
import { signIn } from './users';

test.describe('ChartArea in a widget frame', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  for (const label of ['Sensor Health', 'Sensor Types']) {
    test(`fills the ${label} widget body instead of taking its token height`, async ({ page }) => {
      await signIn(page, 'admin');
      await page.goto('/dashboard');
      await page.waitForLoadState('networkidle');

      const frame = page.locator('[data-widget-state]', { has: page.getByText(label, { exact: true }) });
      const area = frame.locator('[data-ui=chart-area]');
      await expect(area.locator('.recharts-wrapper > svg')).toBeVisible();
      await expect(area).not.toHaveAttribute('data-ui-min-height');

      const boxes = await frame.evaluate((element) => {
        const body = element.lastElementChild!.getBoundingClientRect();
        const chart = element.querySelector('[data-ui=chart-area]')!.getBoundingClientRect();
        return { body: [body.left, body.top, body.width, body.height], chart: [chart.left, chart.top, chart.width, chart.height] };
      });
      boxes.chart.forEach((value, index) => expect(value).toBeCloseTo(boxes.body[index], 0));
    });
  }
});

test.describe('ChartArea while sensors load', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  for (const path of ['/sensors-overview', '/dashboard']) {
    test(`shows a visible loader on ${path}`, async ({ page }) => {
      await page.routeWebSocket('**/api/sensors/ws', () => {});
      await signIn(page, 'admin');
      await page.goto(path);

      const loaders = page.locator('[data-ui=chart-area] [data-testid=widget-loader] svg');
      await expect(loaders).toHaveCount(2);
      for (const box of await loaders.evaluateAll((svgs) => svgs.map((svg) => svg.getBoundingClientRect().height))) {
        expect(box).toBeGreaterThan(0);
      }
    });
  }
});
