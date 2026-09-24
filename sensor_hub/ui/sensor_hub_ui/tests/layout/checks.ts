import { expect, type Page } from '@playwright/test';
import type { LayoutCheck } from './routes';
import type { LayoutUser } from './users';

export type Tier = 'compact' | 'wide';

export const viewports = [
  { tier: 'compact', width: 390, height: 844 },
  { tier: 'wide', width: 1440, height: 900 },
] as const;

const pagePadding: Record<Tier, number> = { compact: 12, wide: 24 };

async function noSidewaysScroll(page: Page) {
  const { scrollWidth, innerWidth } = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    innerWidth: window.innerWidth,
  }));
  expect(scrollWidth, 'document scroll width').toBeLessThanOrEqual(innerWidth);
}

async function noCollapsedContent(page: Page) {
  const problems = await page.evaluate(() => {
    const describe = (element: Element) =>
      `${element.getAttribute('data-ui')} "${(element.textContent ?? '').trim().slice(0, 40)}"`;
    const found: string[] = [];
    for (const card of document.querySelectorAll('[data-ui=card]')) {
      if (!card.querySelector('[data-ui=card-body]')) found.push(`${describe(card)} has no card-body`);
    }
    for (const element of document.querySelectorAll('[data-ui=card-body], [data-ui=chart-area]')) {
      const height = element.getBoundingClientRect().height;
      const minHeight = Number(element.getAttribute('data-ui-min-height') ?? 0);
      if (height <= 0 || height + 0.5 < minHeight) {
        found.push(`${describe(element)} is ${height}px tall, needs more than 0 and at least ${minHeight}px`);
      }
    }
    return found;
  });
  expect(problems, 'collapsed cards and chart areas').toEqual([]);
}

async function shell(page: Page, tier: Tier) {
  const root = page.locator('[data-ui=page]');
  await expect(root, 'page root').toHaveCount(1);

  const padding = await root.evaluate((element) => {
    const style = getComputedStyle(element);
    return [style.paddingTop, style.paddingRight, style.paddingBottom, style.paddingLeft];
  });
  expect(padding, 'page padding').toEqual(Array(4).fill(`${pagePadding[tier]}px`));

  const overflowX = await root.evaluate((element) =>
    [document.documentElement, document.body, element].map((node) => getComputedStyle(node).overflowX),
  );
  expect(overflowX.filter((value) => value === 'hidden' || value === 'clip'), 'hidden overflow on html, body or page').toEqual([]);
}

const pageTitleSize: Record<Tier, string> = { compact: '18px', wide: '20px' };

export async function appBarControls(page: Page) {
  return page
    .locator('[data-ui=app-bar]')
    .locator('button, a')
    .evaluateAll((controls) =>
      controls.map((control) => {
        const { left, right } = control.getBoundingClientRect();
        return { label: control.getAttribute('aria-label'), inViewport: left >= 0 && right <= window.innerWidth };
      }),
    );
}

export async function appBarTitle(page: Page) {
  return page.locator('[data-ui=app-bar-title]').evaluate((title) => {
    const style = getComputedStyle(title);
    return {
      fontSize: style.fontSize,
      fontWeight: style.fontWeight,
      singleLine: title.getBoundingClientRect().height < 2 * parseFloat(style.lineHeight),
      ellipsis: style.whiteSpace === 'nowrap' && style.overflowX === 'hidden' && style.textOverflow === 'ellipsis',
      truncated: title.scrollWidth > title.clientWidth,
    };
  });
}

async function appBar(page: Page, tier: Tier, user: LayoutUser) {
  const bell = user === 'admin' ? ['notifications'] : [];
  const expected =
    tier === 'compact'
      ? ['menu', ...bell, 'account']
      : ['menu', ...bell, 'theme switcher', 'documentation', 'account'];
  const controls = await appBarControls(page);
  expect(controls.map((control) => control.label), 'app bar controls').toEqual(expected);
  expect(controls.filter((control) => !control.inViewport), 'app bar controls outside the viewport').toEqual([]);
  await expect(page.locator('[data-ui=app-bar]').getByText('Sensor Hub', { exact: true }), 'app bar brand').toHaveCount(
    tier === 'wide' ? 1 : 0,
  );

  const title = await appBarTitle(page);
  expect(title, 'app bar title').toMatchObject({
    fontSize: pageTitleSize[tier],
    fontWeight: '500',
    singleLine: true,
    ellipsis: true,
  });

  await page.getByRole('button', { name: 'account' }).click();
  const menu = page.getByRole('menu');
  const entries = await menu.getByRole('menuitem').allTextContents();
  const moved = entries.filter((entry) => entry === 'Theme' || entry === 'Documentation');
  expect(moved, 'avatar menu entries').toEqual(tier === 'compact' ? ['Theme', 'Documentation'] : []);
  await page.keyboard.press('Escape');
  await expect(menu).toHaveCount(0);
}

export const checks: Record<LayoutCheck, (page: Page, tier: Tier, user: LayoutUser) => Promise<void>> = {
  noSidewaysScroll,
  noCollapsedContent,
  shell,
  appBar,
};
