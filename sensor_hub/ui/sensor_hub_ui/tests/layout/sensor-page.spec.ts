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

function historyTable(page: Page) {
  return page.locator('[data-ui=card]', { has: page.getByRole('button', { name: 'Refresh' }) });
}

test.describe('sensor health history table', () => {
  test('is a list with status pills behind "Show more" at 390', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await openSensor(page);
    const history = historyTable(page);
    const rows = history.locator('[data-ui=data-table-row]');

    await expect(history.locator('.MuiDataGrid-root')).toHaveCount(0);
    await expect(rows).toHaveCount(10);
    await history.getByRole('button', { name: /^Show more \(\d+\)$/ }).click();
    await expect(rows).not.toHaveCount(10);
    await expect(history.getByRole('button', { name: /^Show more/ })).toHaveCount(0);
    await expect(rows.locator('[data-ui=status-pill][data-status=bad]')).toHaveText(['bad']);
  });

  test('is a DataGrid at 1440', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await openSensor(page);

    await expect(historyTable(page).locator('.MuiDataGrid-root [role=gridcell]').first()).toBeVisible();
  });
});
