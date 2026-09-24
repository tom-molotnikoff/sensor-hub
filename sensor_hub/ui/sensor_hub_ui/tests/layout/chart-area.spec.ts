import { expect, test } from '@playwright/test';
import { signIn } from './users';

test.describe('ChartArea in a widget frame', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  for (const label of ['Sensor Health', 'Sensor Types', 'Readings Chart', 'Health Timeline']) {
    test(`fills the ${label} widget body instead of taking its token height`, async ({ page }) => {
      await signIn(page, 'admin');
      await page.goto('/dashboard');
      await page.waitForLoadState('networkidle');

      const frame = page.locator('[data-widget-state]', { has: page.getByText(new RegExp(`^${label}(:|$)`)) });
      const area = frame.locator('[data-ui=chart-area]');
      await expect(area.locator('.recharts-wrapper > svg')).toBeVisible();
      await expect(area).not.toHaveAttribute('data-ui-min-height');

      const { body, chart } = await frame.evaluate((element) => {
        const box = (target: Element) => {
          const { left, top, right, bottom } = target.getBoundingClientRect();
          return { left, top, right, bottom };
        };
        return { body: box(element.lastElementChild!), chart: box(element.querySelector('[data-ui=chart-area]')!) };
      });
      expect(chart.left).toBeCloseTo(body.left, 0);
      expect(chart.right).toBeCloseTo(body.right, 0);
      expect(chart.bottom).toBeCloseTo(body.bottom, 0);
      expect(chart.top).toBeGreaterThanOrEqual(body.top - 0.5);
      expect(chart.bottom - chart.top).toBeGreaterThan((body.bottom - body.top) / 2);
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
