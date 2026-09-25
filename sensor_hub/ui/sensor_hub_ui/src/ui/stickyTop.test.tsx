import { ThemeProvider } from '@mui/material';
import { renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useStickyTop } from './stickyTop';
import { theme } from './theme';

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

function stickyTopAt(width: number) {
  atWidth(width);
  return renderHook(() => useStickyTop(), { wrapper: ({ children }) => <ThemeProvider theme={theme}>{children}</ThemeProvider> })
    .result.current;
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
});
