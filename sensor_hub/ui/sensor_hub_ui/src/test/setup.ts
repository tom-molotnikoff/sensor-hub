import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

afterEach(() => {
  cleanup();
});

if (!window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

if (!globalThis.ResizeObserver) {
  class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }

  globalThis.ResizeObserver = ResizeObserver;
}

// jsdom runs no CSS transitions, so an element has no animations.
if (!Element.prototype.getAnimations) {
  Element.prototype.getAnimations = () => [];
}
