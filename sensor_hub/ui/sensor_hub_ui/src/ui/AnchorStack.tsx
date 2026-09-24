import { useEffect, useState, type ReactNode } from 'react';
import { Box } from '@mui/material';
import { responsivePixels } from './tiers';
import { density } from './theme/tokens';

function useTailRoom(sections: HTMLElement | null, landingOffset: number) {
  const [room, setRoom] = useState(0);

  useEffect(() => {
    if (sections === null) return;

    const measure = () => {
      const visible = window.innerHeight - landingOffset;
      const last = sections.lastElementChild;
      const lastHeight = last === null ? 0 : last.getBoundingClientRect().height;
      const scrolls = sections.getBoundingClientRect().height > visible;
      setRoom(scrolls ? Math.max(0, visible - lastHeight) : 0);
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

  return room;
}

interface AnchorStackProps {
  landingOffset: number;
  children?: ReactNode;
}

export default function AnchorStack({ landingOffset, children }: AnchorStackProps) {
  const [sections, setSections] = useState<HTMLElement | null>(null);
  const room = useTailRoom(sections, landingOffset);

  return (
    <Box data-ui="anchor-stack" sx={{ minWidth: 0 }}>
      <Box
        ref={setSections}
        data-ui="anchor-stack-sections"
        sx={{
          display: 'flex',
          flexDirection: 'column',
          gap: responsivePixels(density.gap),
          minWidth: 0,
          '& > *': { scrollMarginTop: landingOffset },
        }}
      >
        {children}
      </Box>
      <Box aria-hidden data-ui="anchor-stack-room" sx={{ height: room }} />
    </Box>
  );
}
