import { expect, test, type Page } from './test';
import { recordDashboardWrites } from './dashboards';
import { signIn } from './users';

async function openDashboard(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/dashboard');
  await page.waitForLoadState('networkidle');
}

test.describe('Dashboard at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('keeps the Dashboards bar title and the picker, lock, New Dashboard and delete in the page body', async ({ page }) => {
    await openDashboard(page);
    await expect(page.locator('[data-ui=app-bar-title]')).toHaveText('Dashboards');
    await expect(page.locator('[data-ui=page-header]')).toHaveCount(0);

    const body = page.locator('[data-ui=page] [data-ui=action-bar]');
    await expect(body.getByRole('combobox')).toHaveText('Layout ★');
    for (const name of ['Edit dashboard', 'New dashboard', 'Delete dashboard']) {
      await expect(body.getByRole('button', { name })).toBeVisible();
    }
  });

  test('stacks every widget full width in desktop reading order, with no grid library', async ({ page }) => {
    await openDashboard(page);
    await expect(page.locator('.react-grid-layout, .react-grid-item')).toHaveCount(0);

    const stack = page.locator('[data-ui=stack]:has(> [data-ui=dashboard-slot])');
    const items = stack.locator('[data-ui=dashboard-slot]');
    await expect(items).toHaveCount(20);

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
      'Comparison Chart',
      'Live Readings Table',
      'Weather Forecast',
      'Notifications',
      'Alert Summary',
      'Markdown Note',
      'Heatmap',
      'Sensor Toggle',
      'Sensor Detail',
    ]);

    const stackWidth = await stack.evaluate((element) => element.getBoundingClientRect().width);
    for (const width of await items.evaluateAll((elements) => elements.map((element) => element.getBoundingClientRect().width))) {
      expect(width).toBe(stackWidth);
    }
  });

  test('frames are their compact height, or their content height', async ({ page }) => {
    await openDashboard(page);
    const items = page.locator('[data-ui=dashboard-slot]');
    await expect(items).toHaveCount(20);

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
    expect(frames.map((frame) => frame.token)).toEqual([
      '280', '140', '220', '220', '220', null, null, '140', null, '200', '160',
      '280', null, null, null, null, null, '260', '120', null,
    ]);
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
    const { height, frameHeight, overflowing } = await unknown.evaluate((element) => {
      const frame = element.firstElementChild as HTMLElement;
      return {
        height: element.getBoundingClientRect().height,
        frameHeight: frame.getBoundingClientRect().height,
        overflowing: frame.scrollHeight > frame.clientHeight + 1,
      };
    });
    expect(height).toBe(frameHeight);
    expect(overflowing).toBe(false);
  });

  test('every table in a widget frame is inset the same distance from the frame', async ({ page }) => {
    await openDashboard(page);
    for (const frame of await page.locator('[data-widget-state]').all()) {
      await frame.scrollIntoViewIfNeeded();
    }
    const tables = page.locator('[data-ui=frame-body] [data-ui=data-table]');
    await expect(tables).toHaveCount(2);
    const insets = await tables.evaluateAll((elements) =>
      elements.map((table) => {
        const body = table.closest('[data-ui=frame-body]')!.getBoundingClientRect();
        const box = table.getBoundingClientRect();
        return { left: Math.round(box.left - body.left), right: Math.round(body.right - box.right) };
      }),
    );
    expect(insets).toEqual([insets[0], insets[0]]);
    expect(insets[0].left).toBe(insets[0].right);
    expect(insets[0].left).toBeGreaterThan(0);
  });

  test('sends no write request for a minute outside edit mode', async ({ page }) => {
    await page.clock.install();
    const writes = recordDashboardWrites(page);
    await openDashboard(page);
    await page.clock.runFor(60_000);
    await page.waitForLoadState('networkidle');
    expect(writes).toEqual([]);
  });
});
