import { Box, Button } from '@mui/material';
import type { RowAction } from './columns';

export default function RowActionButtons({ actions }: { actions: RowAction[] }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, height: '100%' }}>
      {actions.map((action) => (
        <Button
          key={action.label}
          size="small"
          variant={action.variant ?? 'outlined'}
          color={action.color}
          startIcon={action.icon}
          disabled={action.disabled}
          onClick={(event) => {
            event.stopPropagation();
            action.onClick();
          }}
        >
          {action.label}
        </Button>
      ))}
    </Box>
  );
}
