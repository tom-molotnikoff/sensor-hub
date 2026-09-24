import { expect, test, type Locator, type Page } from '@playwright/test';
import { chartAreaHeight } from '../../src/ui/theme/tokens';
import { viewports } from './checks';
import { signIn } from './users';

function card(page: Page, title: string) {
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: title, exact: true }) });
}

async function spaceUnderButtons(target: Locator) {
  return target.evaluate((element) => {
    const rows = element.querySelectorAll('[data-ui=inline]');
    const lastRow = rows[rows.length - 1].getBoundingClientRect();
    const style = getComputedStyle(element);
    const innerBottom = element.getBoundingClientRect().bottom - parseFloat(style.borderBottomWidth);
    return { space: innerBottom - lastRow.bottom, padding: parseFloat(style.paddingBottom) };
  });
}

async function openSensor(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/sensor/1');
  await page.waitForLoadState('networkidle');
  const edit = card(page, 'Edit Sensor Details');
  await expect(edit.getByRole('button').last()).toBeVisible();
  return edit;
}

for (const viewport of viewports) {
  test.describe(`sensor page at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test('Edit Sensor Details ends one card padding below its buttons, even beside a taller Sensor Info', async ({ page }) => {
      const edit = await openSensor(page);
      const { space, padding } = await spaceUnderButtons(edit);
      expect(space).toBeCloseTo(padding, 0);

      const info = card(page, 'seed-sensor-01');
      await info.locator('[data-ui=card-body]').evaluate((body) => {
        const filler = document.createElement('div');
        filler.style.height = '1200px';
        body.append(filler);
      });
      const [infoHeight, editHeight] = await Promise.all([info, edit].map(async (target) => (await target.boundingBox())!.height));
      expect(infoHeight).toBeGreaterThan(editHeight);
      expect((await spaceUnderButtons(edit)).space).toBeCloseTo(padding, 0);
    });

    test('the health history and readings charts are their token height with a drawn chart', async ({ page }) => {
      await openSensor(page);
      for (const title of ['Sensor Health History', 'Indoor Temperature Data']) {
        const area = card(page, title).locator('[data-ui=chart-area]');
        const surface = area.locator('.recharts-wrapper > svg');
        await expect(surface).toBeVisible();
        expect((await area.boundingBox())!.height).toBeCloseTo(chartAreaHeight.lg[viewport.tier], 0);
        expect((await surface.boundingBox())!.height).toBeGreaterThan(0);
      }
    });
  });
}
