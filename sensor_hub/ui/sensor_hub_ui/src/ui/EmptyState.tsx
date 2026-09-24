import { Box, Typography, Button } from '@mui/material';
import { Link as RouterLink } from 'react-router';
import { theme } from './theme';
import Inline from './Inline';
import { useBounded } from './useBounded';
import { emptyStateMinHeight, type EmptyStateSize } from './theme/tokens';

interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  actionLabel?: string;
  actionHref?: string;
  onAction?: () => void;
  size?: EmptyStateSize;
  actions?: React.ReactNode;
}

export default function EmptyState({
  icon,
  title,
  description,
  actionLabel,
  actionHref,
  onAction,
  size = 'md',
  actions,
}: EmptyStateProps) {
  const bounded = useBounded();

  return (
    <Box
      data-ui="empty-state"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 1.5,
        px: 2,
        ...(bounded ? { height: '100%', py: 1 } : { minHeight: emptyStateMinHeight[size], py: 4 }),
        textAlign: 'center',
      }}
    >
      <Box sx={{ color: 'text.disabled' }}>{icon}</Box>
      <Typography variant="body1" sx={{ fontWeight: theme.typography.sectionTitle.fontWeight }}>
        {title}
      </Typography>
      {description && (
        <Typography
          variant="body2"
          sx={{
            color: "text.secondary",
            maxWidth: 360
          }}>
          {description}
        </Typography>
      )}
      {actionLabel && (onAction || actionHref) && (
        <Button
          variant="outlined"
          size="small"
          sx={{ mt: 1 }}
          {...(onAction ? { onClick: onAction } : { component: RouterLink, to: actionHref })}
        >
          {actionLabel}
        </Button>
      )}
      {actions && <Inline>{actions}</Inline>}
    </Box>
  );
}
