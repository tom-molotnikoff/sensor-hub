import { useEffect, useState } from 'react';

const SEPARATOR = ',';

export function useScrollSpy(ids: string[], landingOffset: number): string | undefined {
  const joined = ids.join(SEPARATOR);
  const [inBandId, setInBandId] = useState<string>();

  useEffect(() => {
    const sectionIds = joined === '' ? [] : joined.split(SEPARATOR);
    if (sectionIds.length === 0 || typeof IntersectionObserver === 'undefined') return;

    const inBand = new Set<string>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) inBand.add(entry.target.id);
          else inBand.delete(entry.target.id);
        }
        const first = sectionIds.find((id) => inBand.has(id));
        if (first) setInBandId(first);
      },
      { rootMargin: `-${landingOffset}px 0px 0px 0px`, threshold: 0 },
    );

    for (const id of sectionIds) {
      const element = document.getElementById(id);
      if (element) observer.observe(element);
    }
    return () => observer.disconnect();
  }, [joined, landingOffset]);

  return inBandId !== undefined && ids.includes(inBandId) ? inBandId : ids[0];
}
