import type { ReactNode } from 'react';
import { Box, Typography } from '@mui/material';
import { responsivePixels } from './tiers';
import { density } from './theme/tokens';

interface CardProps {
  title?: string;
  actions?: ReactNode;
  variant?: 'default' | 'inset';
  id?: string;
  children?: ReactNode;
}

export default function Card({ title, actions, variant = 'default', id, children }: CardProps) {
  return (
    <Box
      id={id}
      data-ui="card"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: responsivePixels(density.gap),
        minWidth: 0,
        padding: responsivePixels(density.card),
        border: 1,
        borderColor: 'divider',
        borderRadius: 2,
        boxShadow: 'none',
        bgcolor: variant === 'inset' ? 'background.default' : 'background.paper',
      }}
    >
      {(title || actions) && (
        <Box data-ui="card-header" sx={{ display: 'flex', alignItems: 'center', gap: responsivePixels(density.gap) }}>
          {title && (
            <Typography variant="cardTitle" noWrap>
              {title}
            </Typography>
          )}
          {actions && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0, marginLeft: 'auto' }}>{actions}</Box>
          )}
        </Box>
      )}
      <Box data-ui="card-body" sx={{ flex: '1 1 auto', minWidth: 0 }}>
        {children}
      </Box>
    </Box>
  );
}
