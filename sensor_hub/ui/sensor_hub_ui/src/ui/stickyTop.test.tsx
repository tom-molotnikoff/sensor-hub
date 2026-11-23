import { ThemeProvider } from '@mui/material';
import { renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PageHeaderHeightContext, pageHeaderHeightVar, stickyTop, useStickyTop } from './stickyTop';
import { theme } from './theme';
import { wideMediaQuery } from './tiers';

function atWidth(width: number) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches:
      Number(/\(min-width:\s*(\d+)px\)/.exec(query)?.[1] ?? 0) <= width && !query.includes('orientation: landscape'),
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }));
}

function stickyTopAt(width: number, headerHeight?: number) {
  atWidth(width);
  return renderHook(() => useStickyTop(), {
    wrapper: ({ children }) => (
      <ThemeProvider theme={theme}>
        {headerHeight === undefined ? (
          children
        ) : (
          <PageHeaderHeightContext.Provider value={headerHeight}>{children}</PageHeaderHeightContext.Provider>
        )}
      </ThemeProvider>
    ),
  }).result.current;
}

describe('useStickyTop', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each([
    [390, 56],
    [600, 64],
    [900, 0],
    [1440, 0],
  ])('puts sticky content at a %ipx wide viewport %ipx from the top', (width, top) => {
    expect(stickyTopAt(width)).toBe(top);
  });

  it.each([
    [390, 56],
    [600, 64],
    [900, 71],
    [1440, 71],
  ])('puts sticky content under a 71px page header at a %ipx wide viewport %ipx from the top', (width, top) => {
    expect(stickyTopAt(width, 71)).toBe(top);
  });
});

describe('stickyTop', () => {
  it('sits its offset below the 56px app bar on compact and grows with the taller sm app bar', () => {
    expect(stickyTop(theme, 40)).toMatchObject({ top: 56 + 40, '@media (min-width:600px)': { top: 64 + 40 } });
  });

  it('sits its offset below the page header on wide', () => {
    expect(stickyTop(theme, 40)[wideMediaQuery]).toEqual({ top: `calc(var(${pageHeaderHeightVar}, 0px) + 40px)` });
  });
});
