import { act, renderHook } from '@testing-library/react';
import { useContext, type ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SidebarContextProvider } from './SidebarContextProvider';
import { SidebarContext } from './SidebarContextType';

const key = 'sensor-hub.nav.collapsed';

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

function renderSidebar() {
  return renderHook(() => useContext(SidebarContext), {
    wrapper: ({ children }: { children: ReactNode }) => <SidebarContextProvider>{children}</SidebarContextProvider>,
  });
}

describe('SidebarContextProvider', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it.each([
    [900, true],
    [1199, true],
    [1200, false],
    [1440, false],
  ])('with nothing saved at %ipx, collapsed is %s and nothing is stored', (width, collapsed) => {
    atWidth(width);

    expect(renderSidebar().result.current.collapsed).toBe(collapsed);
    expect(localStorage.getItem(key)).toBeNull();
  });

  it.each([
    ['true', 1440, true],
    ['false', 900, false],
  ])('keeps a saved %s at %ipx', (saved, width, collapsed) => {
    localStorage.setItem(key, saved);
    atWidth(width);

    expect(renderSidebar().result.current.collapsed).toBe(collapsed);
  });

  it('saves the choice each time it is toggled', () => {
    atWidth(1440);
    const { result } = renderSidebar();

    act(() => result.current.toggleCollapsed());
    expect(result.current.collapsed).toBe(true);
    expect(localStorage.getItem(key)).toBe('true');

    act(() => result.current.toggleCollapsed());
    expect(result.current.collapsed).toBe(false);
    expect(localStorage.getItem(key)).toBe('false');
  });
});
