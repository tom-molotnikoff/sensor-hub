import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import { useBounded } from './useBounded';
import { useInsetApplied } from './inset';
import { responsivePixels } from './tiers';
import { theme } from './theme';
import { density } from './theme/tokens';

interface ProseProps {
  children?: ReactNode;
}

const block = { marginTop: 0, marginBottom: 1 };

const typography = {
  ...theme.typography.body,
  minWidth: 0,
  '& > :first-child': { marginTop: 0 },
  '& > :last-child': { marginBottom: 0 },
  '& h1': { ...theme.typography.cardTitle, ...block },
  '& h2, & h3, & h4, & h5, & h6': { ...theme.typography.sectionTitle, marginTop: 1, marginBottom: 0.5 },
  '& p': block,
  '& ul, & ol': { ...block, paddingLeft: 3 },
  '& a': { color: 'primary.main' },
  '& code': {
    fontFamily: 'monospace',
    fontSize: theme.typography.bodySmall.fontSize,
    bgcolor: 'action.hover',
    paddingX: 0.5,
    borderRadius: 0.5,
  },
  '& pre': {
    ...block,
    bgcolor: 'action.hover',
    padding: 1.5,
    borderRadius: 1,
    overflow: 'auto',
    '& code': { bgcolor: 'transparent', paddingX: 0 },
  },
  '& blockquote': {
    ...block,
    marginX: 0,
    paddingLeft: 2,
    borderLeft: 3,
    borderColor: 'primary.main',
    color: 'text.secondary',
  },
  '& hr': { border: 'none', borderTop: 1, borderColor: 'divider', marginY: 1 },
  '& table': { ...block, borderCollapse: 'collapse', width: '100%' },
  '& th, & td': { border: 1, borderColor: 'divider', paddingX: 1, paddingY: 0.5 },
} as const;

export default function Prose({ children }: ProseProps) {
  const bounded = useBounded();
  const insetApplied = useInsetApplied();

  return (
    <Box
      data-ui="prose"
      sx={{
        ...typography,
        ...(bounded && {
          height: '100%',
          minHeight: 0,
          overflow: 'auto',
          padding: insetApplied ? 0 : responsivePixels(density.card),
          boxSizing: 'border-box',
        }),
      }}
    >
      {children}
    </Box>
  );
}
