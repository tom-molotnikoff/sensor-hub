import { renderHook, act } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useLoaderVisibility } from './useLoaderVisibility';

describe('useLoaderVisibility', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  it('is not visible when never loading', () => {
    const { result } = renderHook(() => useLoaderVisibility(false));
    expect(result.current).toBe(false);
  });

  it('shows immediately when loading starts', () => {
    const { result } = renderHook(({ l }) => useLoaderVisibility(l), { initialProps: { l: true } });
    expect(result.current).toBe(true);
  });

  it('stays visible for at least minVisibleMs after loading ends (anti-flash)', () => {
    const { result, rerender } = renderHook(({ l }) => useLoaderVisibility(l, { minVisibleMs: 350 }), {
      initialProps: { l: true },
    });
    expect(result.current).toBe(true);

    // Data arrives almost immediately.
    rerender({ l: false });
    expect(result.current).toBe(true);

    act(() => {
      vi.advanceTimersByTime(349);
    });
    expect(result.current).toBe(true);

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(false);
  });

  it('hides promptly once the min window has already elapsed', () => {
    const { result, rerender } = renderHook(({ l }) => useLoaderVisibility(l, { minVisibleMs: 200 }), {
      initialProps: { l: true },
    });

    act(() => {
      vi.advanceTimersByTime(500);
    });
    rerender({ l: false });
    expect(result.current).toBe(false);
  });
});
