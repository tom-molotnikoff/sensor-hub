import type { ReactNode, Ref } from 'react';
import { Box, Typography } from '@mui/material';
import { responsive, responsivePixels } from './tiers';
import { stickyTop } from './stickyTop';
import { density } from './theme/tokens';

interface StickyProps {
  offset?: number;
  children?: ReactNode;
}

export default function Sticky({ offset = 0, children }: StickyProps) {
  return (
    <Box data-ui="sticky" sx={(theme) => ({ position: 'sticky', ...stickyTop(theme, offset), minWidth: 0 })}>
      {children}
    </Box>
  );
}

const barPadding = 12;

interface StickyBarProps {
  title: string;
  actions?: ReactNode;
  ref?: Ref<HTMLDivElement>;
}

export function StickyBar({ title, actions, ref }: StickyBarProps) {
  return (
    <Box
      ref={ref}
      data-ui="sticky-bar"
      sx={(theme) => ({
        position: 'sticky',
        ...stickyTop(theme, 0),
        zIndex: 1,
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        gap: responsivePixels(density.gap),
        marginTop: responsive({ compact: `-${density.page.compact}px`, wide: '0px' }),
        paddingTop: responsive({ compact: `${density.page.compact + barPadding}px`, wide: `${barPadding}px` }),
        paddingBottom: `${barPadding}px`,
        minWidth: 0,
        bgcolor: 'background.default',
        borderBottom: 1,
        borderColor: 'divider',
      })}
    >
      <Typography variant="cardTitle" noWrap>
        {title}
      </Typography>
      {actions && <Box sx={{ marginLeft: 'auto', minWidth: 0 }}>{actions}</Box>}
    </Box>
  );
}

interface StickyFooterProps {
  children?: ReactNode;
}

export function StickyFooter({ children }: StickyFooterProps) {
  return (
    <Box
      data-ui="sticky-footer"
      sx={{
        position: 'sticky',
        bottom: 0,
        zIndex: 1,
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        justifyContent: 'flex-end',
        gap: 1,
        marginBottom: responsive({ compact: `-${density.page.compact}px`, wide: `-${density.page.wide}px` }),
        paddingY: `${barPadding}px`,
        minWidth: 0,
        bgcolor: 'background.default',
        borderTop: 1,
        borderColor: 'divider',
      }}
    >
      {children}
    </Box>
  );
}
