import { renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useTier } from './tiers';

function atWidth(width: number) {
  vi.stubGlobal('matchMedia', (query: string) => {
    const minWidth = Number(/\(min-width:\s*(\d+)px\)/.exec(query)?.[1]);
    return {
      matches: width >= minWidth,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    };
  });
}

describe('useTier', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each([320, 390, 899])('is compact at %ipx', (width) => {
    atWidth(width);
    expect(renderHook(() => useTier()).result.current).toBe('compact');
  });

  it.each([900, 1440, 2560])('is wide at %ipx', (width) => {
    atWidth(width);
    expect(renderHook(() => useTier()).result.current).toBe('wide');
  });
});
