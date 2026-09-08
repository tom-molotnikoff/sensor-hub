import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { useScrollSpy } from './useScrollSpy';

class FakeIntersectionObserver {
  static instances: FakeIntersectionObserver[] = [];
  readonly options: IntersectionObserverInit;
  private readonly targets: Element[] = [];
  private readonly callback: IntersectionObserverCallback;

  constructor(callback: IntersectionObserverCallback, options: IntersectionObserverInit) {
    this.callback = callback;
    this.options = options;
    FakeIntersectionObserver.instances.push(this);
  }

  observe(target: Element) {
    this.targets.push(target);
  }

  unobserve() {}

  disconnect() {}

  crossInto(ids: string[]) {
    const entries = this.targets.map((target) => ({
      target,
      isIntersecting: ids.includes(target.id),
    }));
    this.callback(entries as unknown as IntersectionObserverEntry[], this as never);
  }
}

const nativeIntersectionObserver = globalThis.IntersectionObserver;

function renderSections(ids: string[], landingOffset: number) {
  for (const id of ids) {
    const section = document.createElement('section');
    section.id = id;
    document.body.append(section);
  }
  return renderHook(() => useScrollSpy(ids, landingOffset));
}

describe('useScrollSpy', () => {
  beforeEach(() => {
    FakeIntersectionObserver.instances = [];
    globalThis.IntersectionObserver = FakeIntersectionObserver as never;
  });

  afterEach(() => {
    globalThis.IntersectionObserver = nativeIntersectionObserver;
    document.body.replaceChildren();
  });

  it('watches only the part of the page below the landing line', () => {
    renderSections(['sensors', 'mqtt', 'advanced'], 88);

    expect(FakeIntersectionObserver.instances[0].options.rootMargin).toBe('-88px 0px 0px 0px');
  });

  it('reports the first section still reaching the landing line', () => {
    const { result } = renderSections(['sensors', 'mqtt', 'advanced'], 88);

    act(() => {
      FakeIntersectionObserver.instances[0].crossInto(['mqtt', 'advanced']);
    });

    expect(result.current).toBe('mqtt');
  });

  it('reports the first section before anything has been observed', () => {
    const { result } = renderSections(['sensors', 'mqtt', 'advanced'], 88);

    expect(result.current).toBe('sensors');
  });

  it('falls back to the first section when the current one is filtered away', () => {
    const ids = ['sensors', 'mqtt', 'advanced'];
    const { result, rerender } = renderSections(ids, 88);

    act(() => {
      FakeIntersectionObserver.instances[0].crossInto(['advanced']);
    });
    expect(result.current).toBe('advanced');

    ids.splice(0, ids.length, 'sensors', 'mqtt');
    rerender();

    expect(result.current).toBe('sensors');
  });
});
