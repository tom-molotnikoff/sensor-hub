import { expect, test, type Page } from '@playwright/test';
import { viewports } from './checks';
import { signIn } from './users';

const metricSizes = ['24px', '36px', '56px'];

async function openDashboard(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/dashboard');
  await page.waitForLoadState('networkidle');
  for (const frame of await page.locator('[data-widget-state]').all()) {
    await frame.scrollIntoViewIfNeeded();
  }
  await expect(page.locator('[data-ui=metric-value]')).toHaveCount(6);
  await expect(page.locator('[data-ui=metric-value]', { hasText: '21.3' })).toHaveCount(2);
}

interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

function inside(inner: Box, outer: Box) {
  return (
    inner.x >= outer.x - 0.5 &&
    inner.x + inner.width <= outer.x + outer.width + 0.5 &&
    inner.y >= outer.y - 0.5 &&
    inner.y + inner.height <= outer.y + outer.height + 0.5
  );
}

for (const viewport of viewports) {
  test.describe(`Metric widgets at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test('every metric value is on the metric type scale and fits its widget', async ({ page }) => {
      await openDashboard(page);
      const values = await page.locator('[data-ui=metric-value]').evaluateAll((elements) =>
        elements.map((element) => {
          const body = element.closest('[data-widget-state]')!.lastElementChild!;
          const box = element.getBoundingClientRect();
          const frame = body.getBoundingClientRect();
          return {
            text: element.textContent,
            fontSize: getComputedStyle(element).fontSize,
            clipped: element.scrollWidth > element.clientWidth,
            inFrame: box.left >= frame.left - 0.5 && box.right <= frame.right + 0.5 && box.top >= frame.top - 0.5 && box.bottom <= frame.bottom + 0.5,
          };
        }),
      );
      for (const value of values) {
        expect(metricSizes, `${value.text} font size`).toContain(value.fontSize);
        expect(value, `${value.text} fits`).toMatchObject({ clipped: false, inFrame: true });
      }
    });
  });
}

test.describe('Gauge at 1440x900', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('the dial grows past 140px in a 6x5 frame', async ({ page }) => {
    await openDashboard(page);
    const dial = page.locator('[data-widget-id=gauge] [data-ui=metric-dial]');
    const { width, height } = (await dial.boundingBox())!;
    expect(width).toBeGreaterThan(140);
    expect(height).toBeCloseTo(width, 0);
  });
});

test.describe('Gauge at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('the dial, value and label fit the 200px frame', async ({ page }) => {
    await openDashboard(page);
    const slot = page.locator('[data-ui=dashboard-slot]', { hasText: 'Gauge' });
    await expect(slot).toHaveAttribute('data-ui-height', '200');
    await slot.scrollIntoViewIfNeeded();
    const slotBox = (await slot.boundingBox())!;
    const body = (await slot.locator('[data-widget-state] > :last-child').boundingBox())!;
    for (const part of ['[data-ui=metric-dial]', '[data-ui=metric-value]', '[data-ui=metric] > .MuiTypography-root']) {
      const box = (await slot.locator(part).first().boundingBox())!;
      expect(inside(box, body), `${part} inside the frame body`).toBe(true);
      expect(inside(box, slotBox), `${part} inside the slot`).toBe(true);
    }
  });
});
