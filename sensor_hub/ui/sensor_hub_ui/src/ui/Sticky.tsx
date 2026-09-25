import type { ReactNode, Ref } from 'react';
import { Box, Typography, type Theme } from '@mui/material';
import type { CSSObject } from '@mui/system';
import { responsive, responsivePixels, wideMediaQuery } from './tiers';
import { density } from './theme/tokens';

function belowAppBar(theme: Theme, offset: number): CSSObject {
  const rules: CSSObject = {};
  for (const [key, value] of Object.entries(theme.mixins.toolbar)) {
    if (key === 'minHeight') rules.top = (value as number) + offset;
    else if (typeof value === 'object' && value !== null && 'minHeight' in value) {
      rules[key] = { top: (value as { minHeight: number }).minHeight + offset };
    }
  }
  return { ...rules, [wideMediaQuery]: { top: offset } };
}

interface StickyProps {
  offset?: number;
  children?: ReactNode;
}

export default function Sticky({ offset = 0, children }: StickyProps) {
  return (
    <Box data-ui="sticky" sx={(theme) => ({ position: 'sticky', ...belowAppBar(theme, offset), minWidth: 0 })}>
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
        ...belowAppBar(theme, 0),
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
