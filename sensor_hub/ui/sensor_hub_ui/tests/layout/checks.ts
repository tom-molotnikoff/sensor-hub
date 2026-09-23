import { expect, type Page } from '@playwright/test';
import type { LayoutCheck } from './routes';

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

export const checks: Record<LayoutCheck, (page: Page) => Promise<void>> = {
  noSidewaysScroll,
  noCollapsedContent,
};
