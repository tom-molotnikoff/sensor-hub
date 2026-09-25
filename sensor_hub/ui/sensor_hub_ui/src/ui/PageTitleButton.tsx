import type { MouseEvent } from 'react';
import { Box, Button } from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';

interface PageTitleButtonProps {
  id: string;
  label: string;
  menuId: string;
  open: boolean;
  onClick: (event: MouseEvent<HTMLElement>) => void;
}

export default function PageTitleButton({ id, label, menuId, open, onClick }: PageTitleButtonProps) {
  return (
    <Button
      id={id}
      data-ui="page-title-button"
      color="inherit"
      endIcon={<ExpandMoreIcon />}
      aria-haspopup="menu"
      aria-controls={open ? menuId : undefined}
      aria-expanded={open}
      onClick={onClick}
      sx={{ font: 'inherit', letterSpacing: 'inherit', textTransform: 'none', paddingX: 1, marginLeft: -1 }}
    >
      <Box component="span" data-ui="page-title-label" sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {label}
      </Box>
    </Button>
  );
}
