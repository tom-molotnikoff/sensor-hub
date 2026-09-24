import { Box, Typography, Button } from '@mui/material';
import { useNavigate } from 'react-router';
import { emptyStateMinHeight, type EmptyStateSize } from './theme/tokens';

interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  actionLabel?: string;
  actionHref?: string;
  onAction?: () => void;
  size?: EmptyStateSize;
}

export default function EmptyState({
  icon,
  title,
  description,
  actionLabel,
  actionHref,
  onAction,
  size = 'md',
}: EmptyStateProps) {
  const navigate = useNavigate();

  const handleClick = () => {
    if (onAction) {
      onAction();
    } else if (actionHref) {
      navigate(actionHref);
    }
  };

  return (
    <Box
      data-ui="empty-state"
      data-ui-min-height={emptyStateMinHeight[size]}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: emptyStateMinHeight[size],
        gap: 1.5,
        py: 4,
        px: 2,
        textAlign: 'center',
      }}
    >
      <Box sx={{ color: 'text.disabled' }}>{icon}</Box>
      <Typography variant="sectionTitle">
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
        <Button variant="outlined" size="small" onClick={handleClick} sx={{ mt: 1 }}>
          {actionLabel}
        </Button>
      )}
    </Box>
  );
}
