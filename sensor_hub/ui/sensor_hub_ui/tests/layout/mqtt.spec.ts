import { expect, test, type Page } from '@playwright/test';
import { viewports } from './checks';
import { signIn } from './users';

const lists = [
  { title: 'MQTT Brokers', row: 'Garage Mosquitto' },
  { title: 'MQTT Subscriptions', row: 'zigbee2mqtt/attic/+' },
];

async function openMqtt(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/mqtt');
  await page.waitForLoadState('networkidle');
}

function card(page: Page, title: string) {
  return page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: title, exact: true }) });
}

for (const viewport of viewports) {
  test.describe(`MQTT at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    for (const { title, row } of lists) {
      test(`${title} is a ${viewport.tier === 'compact' ? 'list' : 'DataGrid'} whose rows open the row menu`, async ({ page }) => {
        await openMqtt(page);
        const list = card(page, title);

        if (viewport.tier === 'compact') {
          await expect(list.locator('.MuiDataGrid-root')).toHaveCount(0);
          await list.getByRole('button', { name: new RegExp(row.replace(/[+/]/g, '\\$&')) }).click();
        } else {
          await expect(list.locator('.MuiDataGrid-root')).toHaveCount(1);
          await list.getByRole('gridcell', { name: row, exact: true }).click();
        }
        await expect(page.getByRole('menu').getByRole('menuitem')).toHaveText(['Disable', 'Delete']);
      });
    }
  });
}

test.describe('MQTT subscriptions at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('shows ten subscriptions before "Show more"', async ({ page }) => {
    await openMqtt(page);
    const subscriptions = card(page, 'MQTT Subscriptions');

    await expect(subscriptions.locator('[data-ui=data-table-row]')).toHaveCount(10);
    await subscriptions.getByRole('button', { name: 'Show more (2)' }).click();
    await expect(subscriptions.locator('[data-ui=data-table-row]')).toHaveCount(12);
    await expect(subscriptions.locator('[data-ui=status-pill][data-status=unknown]')).toHaveCount(3);
  });
});
