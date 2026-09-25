import { expect, type Locator, type Page } from '@playwright/test';
import type { LayoutCheck } from './routes';
import type { LayoutUser } from './users';

export type Tier = 'compact' | 'wide';

export const viewports = [
  { tier: 'compact', width: 390, height: 844 },
  { tier: 'wide', width: 1440, height: 900 },
] as const;

export const narrowWide = { tier: 'wide', width: 900, height: 900 } as const;

export const wideViewports = [narrowWide, viewports[1]] as const;

export const contractViewports = [...viewports, narrowWide] as const;

export const navBackground = { light: 'rgb(33, 30, 27)', dark: 'rgb(18, 18, 18)' } as const;

export const navCollapsedKey = 'sensor-hub.nav.collapsed';

export type NavState = 'expanded' | 'rail';

export async function saveNav(page: Page, state: NavState) {
  await page.addInitScript(([key, value]) => localStorage.setItem(key, value), [navCollapsedKey, String(state === 'rail')]);
}

export interface RailSample {
  moment: 'run' | 'frame' | 'end';
  uncovered: string[];
  covers: number;
  editControls: number;
}

export interface RailTransition {
  durations: number[];
  samples: RailSample[];
  ended: boolean;
}

type WatchedWindow = Window & { railTransition?: RailTransition };

export async function watchRailTransition(page: Page) {
  await page.evaluate(() => {
    const drawer = document.querySelector<HTMLElement>('[data-ui=nav-drawer]')!;
    const watch: RailTransition = { durations: [], samples: [], ended: false };
    (window as WatchedWindow).railTransition = watch;
    let frame = 0;

    const sample = (moment: RailSample['moment']) => {
      const frames = [...document.querySelectorAll<HTMLElement>('[data-widget-id]')];
      watch.samples.push({
        moment,
        uncovered: frames.filter((item) => !item.querySelector('[data-ui=frame-placeholder]')).map((item) => item.dataset.widgetId!),
        covers: document.querySelectorAll('[data-ui=frame-cover]').length,
        editControls: [
          ...document.querySelectorAll('.drag-handle, .react-resizable-handle, [aria-label="Configure widget"], [aria-label="Remove widget"]'),
        ].filter((control) => control.checkVisibility()).length,
      });
    };
    const everyFrame = () => {
      sample('frame');
      frame = requestAnimationFrame(everyFrame);
    };
    const ownWidth = (event: TransitionEvent) => event.target === drawer && event.propertyName === 'width';

    drawer.addEventListener('transitionrun', (event) => {
      if (!ownWidth(event)) return;
      watch.durations = drawer
        .getAnimations()
        .filter((animation) => animation instanceof CSSTransition && animation.transitionProperty === 'width')
        .map((animation) => Number(animation.effect!.getTiming().duration));
      sample('run');
      frame = requestAnimationFrame(everyFrame);
    });
    drawer.addEventListener('transitionend', (event) => {
      if (!ownWidth(event)) return;
      cancelAnimationFrame(frame);
      sample('end');
      watch.ended = true;
    });
  });
}

export async function railTransition(page: Page): Promise<RailTransition> {
  await page.waitForFunction(() => (window as WatchedWindow).railTransition?.ended);
  return page.evaluate(() => (window as WatchedWindow).railTransition!);
}

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

  const nav = page.getByRole('navigation', { name: 'Main' });
  if (tier === 'wide') {
    await expect(nav, 'nav without any click').toBeVisible();
    await expect(page.locator('[data-ui=app-bar]'), 'app bar').toHaveCount(0);
    await expect(page.locator('h1'), 'h1').toHaveCount(1);
    const firstRow = root.locator(':scope > :first-child');
    await expect(firstRow, 'page header as the first row').toHaveAttribute('data-ui', 'page-header');
    await expect(firstRow.locator(':scope > h1'), 'h1 in the page header').toHaveCount(1);
  } else {
    await expect(page.locator('[data-ui=app-bar]'), 'app bar').toHaveCount(1);
    await expect(nav, 'closed nav').toHaveCount(0);
    await expect(page.locator('[data-ui=page-header]'), 'page header').toHaveCount(0);
  }
}

const pageTitleType: Record<Tier, { fontSize: string; fontWeight: string }> = {
  compact: { fontSize: '18px', fontWeight: '500' },
  wide: { fontSize: '24px', fontWeight: '600' },
};

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

export function titleLocator(page: Page, tier: Tier) {
  return page.locator(tier === 'wide' ? '[data-ui=page-header] > h1' : '[data-ui=app-bar-title]');
}

export async function pageTitle(page: Page, tier: Tier) {
  return titleLocator(page, tier).evaluate((title) => {
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

export async function clippedGlyphs(title: Locator) {
  return title.evaluate((root) => {
    const clipped: string[] = [];
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      if (!node.textContent?.trim()) continue;
      const range = document.createRange();
      range.selectNodeContents(node);
      const glyphs = range.getBoundingClientRect();
      for (let box = node.parentElement; box; box = box === root ? null : box.parentElement) {
        const style = getComputedStyle(box);
        if (style.overflowX === 'visible' && style.overflowY === 'visible') continue;
        const bounds = box.getBoundingClientRect();
        if (glyphs.top < bounds.top || glyphs.bottom > bounds.bottom || box.scrollHeight > box.clientHeight) {
          clipped.push(
            `"${node.textContent}" glyphs ${glyphs.top}-${glyphs.bottom} in ${box.tagName.toLowerCase()} ${bounds.top}-${bounds.bottom} scrollHeight ${box.scrollHeight} clientHeight ${box.clientHeight}`,
          );
        }
      }
    }
    return clipped;
  });
}

async function compactAppBar(page: Page) {
  const bar = page.locator('[data-ui=app-bar]');
  const controls = await appBarControls(page);
  expect(controls.map((control) => control.label), 'app bar controls').toEqual(['menu', 'notifications']);
  expect(controls.filter((control) => !control.inViewport), 'app bar controls outside the viewport').toEqual([]);
  await expect(bar.getByText('Sensor Hub', { exact: true }), 'app bar brand').toHaveCount(0);

  const scheme = await page.locator('html').evaluate((html) => (html.classList.contains('dark') ? 'dark' : 'light'));
  await expect(bar, 'app bar background').toHaveCSS('background-color', navBackground[scheme]);
}

async function appBar(page: Page, tier: Tier) {
  if (tier === 'compact') await compactAppBar(page);
  else await expect(page.locator('[data-ui=app-bar]'), 'app bar').toHaveCount(0);

  expect(await pageTitle(page, tier), 'page title').toMatchObject({
    ...pageTitleType[tier],
    singleLine: true,
    ellipsis: true,
  });
  expect(await clippedGlyphs(titleLocator(page, tier)), 'page title glyphs clipped').toEqual([]);
}

export const checks: Record<LayoutCheck, (page: Page, tier: Tier, user: LayoutUser) => Promise<void>> = {
  noSidewaysScroll,
  noCollapsedContent,
  shell,
  appBar,
};
