import { useLayoutEffect, useState } from 'react';

interface MeasuredHeight {
  measuredRef: (node: HTMLElement | null) => void;
  height: number;
}

export function useMeasuredHeight(): MeasuredHeight {
  const [element, setElement] = useState<HTMLElement | null>(null);
  const [height, setHeight] = useState(0);

  useLayoutEffect(() => {
    if (element === null) return;

    const measure = () => setHeight(element.getBoundingClientRect().height);

    measure();
    window.addEventListener('resize', measure);
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
    observer?.observe(element);

    return () => {
      window.removeEventListener('resize', measure);
      observer?.disconnect();
    };
  }, [element]);

  return { measuredRef: setElement, height };
}
