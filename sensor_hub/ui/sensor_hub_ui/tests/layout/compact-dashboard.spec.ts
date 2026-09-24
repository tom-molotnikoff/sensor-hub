import { expect, test, type Page } from '@playwright/test';
import { signIn } from './users';

async function openDashboard(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/dashboard');
  await page.waitForLoadState('networkidle');
}

test.describe('Dashboard at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('stacks every widget full width in desktop reading order, with no grid library', async ({ page }) => {
    await openDashboard(page);
    await expect(page.locator('.react-grid-layout, .react-grid-item')).toHaveCount(0);

    const stack = page.locator('[data-ui=stack]', { has: page.locator('[data-ui=dashboard-slot]') });
    const items = stack.locator('[data-ui=dashboard-slot]');
    await expect(items).toHaveCount(11);

    const titles = await items.evaluateAll((elements) =>
      elements.map((element) => (element.querySelector('.MuiTypography-caption, .MuiTypography-root')?.textContent ?? '').split(':')[0]),
    );
    expect(titles).toEqual([
      'Readings Chart',
      'Sensor Uptime',
      'Sensor Health',
      'Sensor Types',
      'Health Timeline',
      'Reading Statistics',
      'Unknown widget',
      'Current Reading',
      'Group Summary',
      'Gauge',
      'Min / Max / Avg',
    ]);

    const stackWidth = await stack.evaluate((element) => element.getBoundingClientRect().width);
    for (const width of await items.evaluateAll((elements) => elements.map((element) => element.getBoundingClientRect().width))) {
      expect(width).toBe(stackWidth);
    }
  });

  test('frames are their compact height, or their content height', async ({ page }) => {
    await openDashboard(page);
    const items = page.locator('[data-ui=dashboard-slot]');
    await expect(items).toHaveCount(11);

    const frames = await items.evaluateAll((elements) =>
      elements.map((element) => {
        const frame = element.firstElementChild as HTMLElement;
        return {
          token: element.getAttribute('data-ui-height'),
          height: element.getBoundingClientRect().height,
          frameHeight: frame.getBoundingClientRect().height,
          overflowing: frame.scrollHeight > frame.clientHeight + 1,
        };
      }),
    );
    expect(frames.map((frame) => frame.token)).toEqual(['280', '140', '220', '220', '220', null, null, '140', null, '200', '160']);
    for (const frame of frames) {
      if (frame.token) {
        expect(frame.height).toBe(Number(frame.token));
      } else {
        expect(frame.height).toBeGreaterThan(0);
        expect(frame.frameHeight).toBe(frame.height);
        expect(frame.overflowing).toBe(false);
      }
    }
  });

  test('an unknown widget type shows the Unknown widget frame at its content height', async ({ page }) => {
    await openDashboard(page);
    const unknown = page.locator('[data-ui=dashboard-slot]', { hasText: 'Unknown widget' });
    await expect(unknown).toContainText('Unknown widget: retired-widget');
    await expect(unknown).not.toHaveAttribute('data-ui-height');
    const { height, contentHeight } = await unknown.evaluate((element) => ({
      height: element.getBoundingClientRect().height,
      contentHeight: (element.firstElementChild as HTMLElement).scrollHeight,
    }));
    expect(height).toBe(contentHeight);
  });

  test('sends no write request for a minute outside edit mode', async ({ page }) => {
    await page.clock.install();
    const writes: string[] = [];
    page.on('request', (request) => {
      if (request.method() !== 'GET' && request.url().includes('/api/dashboards')) writes.push(`${request.method()} ${request.url()}`);
    });
    await openDashboard(page);
    await page.clock.runFor(60_000);
    await page.waitForLoadState('networkidle');
    expect(writes).toEqual([]);
  });
});
