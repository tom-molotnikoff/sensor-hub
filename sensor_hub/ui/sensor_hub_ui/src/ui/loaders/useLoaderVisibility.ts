import { useEffect, useRef, useState } from 'react';

export const DEFAULT_MIN_VISIBLE_MS = 350;
export const DEFAULT_SHOW_AFTER_MS = 100;

interface Options {
  /** Minimum time the loader stays visible once shown, to avoid flicker on fast loads. */
  minVisibleMs?: number;
  /** How long loading must last before a loader is shown at all. */
  showAfterMs?: number;
}

/**
 * Anti-flash gate for widget loaders. A load that finishes within `showAfterMs` never shows a
 * loader; one that does not shows it and keeps it for at least `minVisibleMs`.
 */
export function useLoaderVisibility(isLoading: boolean, options: Options = {}): boolean {
  const minVisibleMs = options.minVisibleMs ?? DEFAULT_MIN_VISIBLE_MS;
  const showAfterMs = options.showAfterMs ?? DEFAULT_SHOW_AFTER_MS;
  const [showLoader, setShowLoader] = useState(false);
  const shownAtRef = useRef<number | null>(null);

  useEffect(() => {
    if (isLoading) {
      if (showLoader) return;
      const timer = setTimeout(() => {
        shownAtRef.current = Date.now();
        setShowLoader(true);
      }, showAfterMs);
      return () => clearTimeout(timer);
    }

    if (!showLoader) return;
    const shownAt = shownAtRef.current ?? Date.now();
    const remaining = minVisibleMs - (Date.now() - shownAt);
    if (remaining <= 0) {
      setShowLoader(false);
      return;
    }
    const timer = setTimeout(() => setShowLoader(false), remaining);
    return () => clearTimeout(timer);
  }, [isLoading, showLoader, minVisibleMs, showAfterMs]);

  return showLoader;
}
