import type { ReactNode } from 'react';
import { Box, Typography } from '@mui/material';
import { useBounded } from './useBounded';
import { responsivePixels } from './tiers';
import { density } from './theme/tokens';

interface CardProps {
  title?: string;
  actions?: ReactNode;
  variant?: 'default' | 'inset';
  id?: string;
  children?: ReactNode;
}

const surface = {
  padding: responsivePixels(density.card),
  border: 1,
  borderColor: 'divider',
  borderRadius: 2,
  boxShadow: 'none',
};

export default function Card({ title, actions, variant = 'default', id, children }: CardProps) {
  const bounded = useBounded();
  const heading = bounded ? undefined : title;

  return (
    <Box
      id={id}
      data-ui="card"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: responsivePixels(density.gap),
        minWidth: 0,
        ...(bounded
          ? { height: '100%', minHeight: 0 }
          : { ...surface, bgcolor: variant === 'inset' ? 'background.default' : 'background.paper' }),
      }}
    >
      {(heading || actions) && (
        <Box
          data-ui="card-header"
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: responsivePixels(density.gap),
            ...(bounded && { paddingX: responsivePixels(density.card), paddingTop: 1 }),
          }}
        >
          {heading && (
            <Typography variant="cardTitle" noWrap>
              {heading}
            </Typography>
          )}
          {actions && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0, marginLeft: 'auto' }}>{actions}</Box>
          )}
        </Box>
      )}
      <Box
        data-ui="card-body"
        sx={{
          flex: '1 1 auto',
          minWidth: 0,
          ...(bounded && { display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'auto' }),
        }}
      >
        {children}
      </Box>
    </Box>
  );
}
