import type { ReactNode } from 'react';
import { Box, CircularProgress, Divider, Typography } from '@mui/material';
import AnchoredMenu, { type MenuPlacement } from './AnchoredMenu';

interface MenuPanelProps {
  anchorEl: HTMLElement | null;
  onClose: () => void;
  placement?: MenuPlacement;
  title: string;
  meta?: ReactNode;
  loading?: boolean;
  footer?: ReactNode;
  children?: ReactNode;
}

const panelMaxHeight = 450;

export default function MenuPanel({ anchorEl, onClose, placement, title, meta, loading = false, footer, children }: MenuPanelProps) {
  return (
    <AnchoredMenu
      data-ui="menu-panel"
      anchorEl={anchorEl}
      onClose={onClose}
      placement={placement}
      width="md"
      maxHeight={panelMaxHeight}
      spacedItems
    >
      <Box
        data-ui="menu-panel-header"
        sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1, paddingX: 2, paddingY: 1 }}
      >
        <Typography variant="subtitle1" noWrap sx={{ fontWeight: 'fontWeightBold' }}>
          {title}
        </Typography>
        {meta && (
          <Typography variant="caption" color="text.secondary" noWrap>
            {meta}
          </Typography>
        )}
      </Box>
      <Divider />
      {loading ? (
        <Box role="status" aria-label="Loading" sx={{ display: 'flex', justifyContent: 'center', paddingY: 3 }}>
          <CircularProgress size={24} />
        </Box>
      ) : (
        children
      )}
      {footer && <Divider />}
      {footer && (
        <Box data-ui="menu-panel-footer" sx={{ padding: 1, textAlign: 'center' }}>
          {footer}
        </Box>
      )}
    </AnchoredMenu>
  );
}
