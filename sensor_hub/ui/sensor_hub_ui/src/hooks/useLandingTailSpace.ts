import { useEffect, useState } from 'react';

interface LandingTailSpace {
  sectionsRef: (node: HTMLElement | null) => void;
  tailSpace: number;
}

export function useLandingTailSpace(landingOffset: number): LandingTailSpace {
  const [sections, setSections] = useState<HTMLElement | null>(null);
  const [tailSpace, setTailSpace] = useState(0);

  useEffect(() => {
    if (sections === null) return;

    const measure = () => {
      const room = window.innerHeight - landingOffset;
      const last = sections.lastElementChild;
      const lastHeight = last === null ? 0 : last.getBoundingClientRect().height;
      const scrolls = sections.getBoundingClientRect().height > room;
      setTailSpace(scrolls ? Math.max(0, room - lastHeight) : 0);
    };

    measure();
    window.addEventListener('resize', measure);
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
    observer?.observe(sections);

    return () => {
      window.removeEventListener('resize', measure);
      observer?.disconnect();
    };
  }, [sections, landingOffset]);

  return { sectionsRef: setSections, tailSpace };
}
