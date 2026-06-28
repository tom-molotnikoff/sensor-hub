import { useEffect, useRef, useState } from 'react';

export const DEFAULT_MIN_VISIBLE_MS = 350;

interface Options {
  /** Minimum time the loader stays visible once shown, to avoid flicker on fast loads. */
  minVisibleMs?: number;
}

/**
 * Anti-flash gate for widget loaders. Once a loader becomes visible it stays
 * visible for at least `minVisibleMs`, so quick/warm-cache loads don't blink.
 *
 * Returns whether the loader should currently be shown.
 */
export function useLoaderVisibility(isLoading: boolean, options: Options = {}): boolean {
  const minVisibleMs = options.minVisibleMs ?? DEFAULT_MIN_VISIBLE_MS;
  const [showLoader, setShowLoader] = useState(isLoading);
  const shownAtRef = useRef<number | null>(isLoading ? Date.now() : null);

  useEffect(() => {
    if (isLoading) {
      if (!showLoader) {
        shownAtRef.current = Date.now();
        setShowLoader(true);
      }
      return;
    }

    // Loading finished — keep the loader up until the min window has elapsed.
    if (!showLoader) return;
    const shownAt = shownAtRef.current ?? Date.now();
    const remaining = minVisibleMs - (Date.now() - shownAt);
    if (remaining <= 0) {
      setShowLoader(false);
      return;
    }
    const timer = setTimeout(() => setShowLoader(false), remaining);
    return () => clearTimeout(timer);
  }, [isLoading, showLoader, minVisibleMs]);

  return showLoader;
}
