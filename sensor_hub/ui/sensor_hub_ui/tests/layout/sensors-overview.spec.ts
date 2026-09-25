import { expect, test, type Locator, type Page } from './test';
import { chartAreaHeight } from '../../src/ui/theme/tokens';
import { checks, narrowWide, viewports, type Tier } from './checks';
import { signIn, type SignedInUser } from './users';

const cardPadding: Record<Tier, number> = { compact: 12, wide: 20 };
const gap: Record<Tier, number> = { compact: 12, wide: 16 };
const cardTitleSize: Record<Tier, string> = { compact: '18px', wide: '20px' };
const adminSpans = [4, 4, 4, 8, 4, 12];

async function openOverview(page: Page) {
  await signIn(page, 'admin');
  await page.goto('/sensors-overview');
  await page.waitForLoadState('networkidle');
  const card = page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'Add Sensor' }) });
  await expect(card.getByRole('button', { name: 'Create' })).toBeVisible();
  return card;
}

async function spaceUnderButtons(card: Locator) {
  return card.evaluate((element) => {
    const rows = element.querySelectorAll('[data-ui=inline]');
    const lastRow = rows[rows.length - 1].getBoundingClientRect();
    const style = getComputedStyle(element);
    const innerBottom = element.getBoundingClientRect().bottom - parseFloat(style.borderBottomWidth);
    return { space: innerBottom - lastRow.bottom, padding: parseFloat(style.paddingBottom) };
  });
}

async function themeColour(page: Page, variable: string) {
  return page.evaluate((name) => {
    const probe = document.createElement('div');
    probe.style.color = `var(${name})`;
    document.body.append(probe);
    const colour = getComputedStyle(probe).color;
    probe.remove();
    return colour;
  }, variable);
}

async function gridItems(page: Page) {
  return page.locator('[data-ui=page-grid] > [data-ui=page-grid-item]').evaluateAll((items) =>
    items.map((item) => {
      const box = item.getBoundingClientRect();
      const content = item.firstElementChild!.getBoundingClientRect();
      return { left: box.left, top: box.top, width: box.width, height: box.height, contentHeight: content.height };
    }),
  );
}

async function pieChartHeights(page: Page, user: SignedInUser) {
  await signIn(page, user);
  await page.goto('/sensors-overview');
  await page.waitForLoadState('networkidle');
  const heights = [];
  for (const title of ['Sensor Health', 'Sensor Types']) {
    const card = page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: title, exact: true }) });
    const area = card.locator('[data-ui=chart-area]');
    await expect(area.locator('.recharts-wrapper > svg')).toBeVisible();
    heights.push((await area.boundingBox())!.height);
  }
  return heights;
}

for (const viewport of viewports) {
  test.describe(`Sensors Overview at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    test('Add Sensor ends one card padding below its buttons, with and without a driver', async ({ page }) => {
      const card = await openOverview(page);
      expect(await spaceUnderButtons(card)).toEqual({ space: cardPadding[viewport.tier], padding: cardPadding[viewport.tier] });

      await card.getByRole('combobox', { name: 'Sensor Driver' }).click();
      await page.getByRole('option').first().click();
      await expect(card.getByRole('textbox')).not.toHaveCount(1);
      expect(await spaceUnderButtons(card)).toEqual({ space: cardPadding[viewport.tier], padding: cardPadding[viewport.tier] });
    });

    test('Card uses the density padding, gap and card title size', async ({ page }) => {
      const card = await openOverview(page);
      const style = await card.evaluate((element) => {
        const computed = getComputedStyle(element);
        return { padding: computed.padding, gap: computed.rowGap };
      });
      expect(style).toEqual({ padding: `${cardPadding[viewport.tier]}px`, gap: `${gap[viewport.tier]}px` });

      const title = card.getByRole('heading', { name: 'Add Sensor' });
      await expect(title).toHaveCSS('font-size', cardTitleSize[viewport.tier]);
      await expect(title).toHaveCSS('font-weight', '600');
    });

    test('PageGrid lays items out by tier', async ({ page }) => {
      await openOverview(page);
      const grid = await page.locator('[data-ui=page-grid]').evaluate((element) => {
        const style = getComputedStyle(element);
        const box = element.getBoundingClientRect();
        return { left: box.left, width: box.width, gap: parseFloat(style.columnGap) };
      });
      const items = await gridItems(page);
      expect(items).toHaveLength(adminSpans.length);

      if (viewport.tier === 'compact') {
        for (const [index, item] of items.entries()) {
          expect(item.left).toBeCloseTo(grid.left, 0);
          expect(item.width).toBeCloseTo(grid.width, 0);
          expect(item.height).toBeCloseTo(item.contentHeight, 0);
          if (index > 0) expect(item.top).toBeGreaterThan(items[index - 1].top);
        }
      } else {
        const column = (grid.width - 11 * grid.gap) / 12;
        for (const [index, item] of items.entries()) {
          const span = adminSpans[index];
          expect(item.width).toBeCloseTo(span * column + (span - 1) * grid.gap, 0);
          expect(item.height).toBeCloseTo(item.contentHeight, 0);
        }
      }
    });

    for (const user of ['admin', 'viewer'] as const) {
      test(`Sensor Health and Sensor Types charts are their token height as ${user}`, async ({ page }) => {
        expect(await pieChartHeights(page, user)).toEqual([
          chartAreaHeight.md[viewport.tier],
          chartAreaHeight.md[viewport.tier],
        ]);
      });
    }

    test('an Inline with more items than fit wraps inside its width', async ({ page }) => {
      const card = await openOverview(page);
      const row = card.locator('[data-ui=inline]').last();
      const before = await row.boundingBox();
      await row.evaluate((element) => {
        const button = element.firstElementChild!;
        for (let copy = 0; copy < 12; copy++) element.append(button.cloneNode(true));
      });
      const after = await row.boundingBox();
      expect(after!.width).toBeCloseTo(before!.width, 0);
      expect(after!.height).toBeGreaterThan(before!.height);
      const overflowing = await row.evaluate((element) => {
        const right = element.getBoundingClientRect().right;
        return Array.from(element.children).filter((child) => child.getBoundingClientRect().right > right + 0.5).length;
      });
      expect(overflowing).toBe(0);
      await checks.noSidewaysScroll(page, viewport.tier, 'admin');
    });
  });
}

for (const colorScheme of ['light', 'dark'] as const) {
  test.describe(`Card surface in ${colorScheme}`, () => {
    test.use({ viewport: { width: 1440, height: 900 }, colorScheme });

    test('is flat, with a 1px divider border on the paper background', async ({ page }) => {
      const card = await openOverview(page);
      await expect(card).toHaveCSS('box-shadow', 'none');
      await expect(card).toHaveCSS('border-top-width', '1px');
      await expect(card).toHaveCSS('border-top-style', 'solid');
      await expect(card).toHaveCSS('border-top-color', await themeColour(page, '--mui-palette-divider'));
      await expect(card).toHaveCSS('background-color', await themeColour(page, '--mui-palette-background-paper'));
    });
  });
}

test.describe(`Sensors Overview at ${narrowWide.width}x${narrowWide.height}`, () => {
  test.use({ viewport: { width: narrowWide.width, height: narrowWide.height } });

  test('wraps card header actions that do not fit onto their own row inside the card', async ({ page }) => {
    await openOverview(page);
    const card = page.locator('[data-ui=card]', { has: page.getByRole('heading', { name: 'Total Readings For Each Sensor' }) });
    await expect(card.getByText(/^Sampled /)).toBeVisible();

    const header = await card.locator('[data-ui=card-header]').evaluate((element) => {
      const [title, actions] = [element.firstElementChild!, element.lastElementChild!].map((part) => part.getBoundingClientRect());
      const box = element.getBoundingClientRect();
      const content = element.lastElementChild!;
      return {
        actionsBelowTitle: actions.top >= title.bottom,
        actionsInside: actions.left >= box.left && actions.right <= box.right,
        actionsSqueezed: content.scrollWidth > content.clientWidth,
      };
    });
    expect(header).toEqual({ actionsBelowTitle: true, actionsInside: true, actionsSqueezed: false });
    await checks.noSidewaysScroll(page, 'wide', 'admin');
  });
});
