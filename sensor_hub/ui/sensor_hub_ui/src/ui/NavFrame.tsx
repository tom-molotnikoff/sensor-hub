import type { ReactNode } from 'react';
import {
  Box,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Skeleton,
  Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { navDrawer } from './theme/tokens';

interface NavFrameProps {
  open: boolean;
  onClose: () => void;
  logo: string;
  name: string;
  children?: ReactNode;
}

const logoSize = 32;
const indicatorWidth = 3;

export default function NavFrame({ open, onClose, logo, name, children }: NavFrameProps) {
  return (
    <Drawer
      variant="temporary"
      open={open}
      onClose={onClose}
      ModalProps={{ keepMounted: false }}
      slotProps={{
        paper: {
          sx: { width: `min(${navDrawer.width}px, calc(100vw - ${navDrawer.pageVisible}px))` },
        },
      }}
    >
      <Box
        component="nav"
        aria-label="Main"
        sx={{ display: 'flex', flexDirection: 'column', flex: '1 0 auto', bgcolor: 'nav.bg', color: 'nav.text' }}
      >
        <Box
          data-ui="nav-brand"
          sx={{ display: 'flex', alignItems: 'center', gap: 1.5, paddingLeft: 2, paddingRight: 1, paddingY: 1.5 }}
        >
          <Box component="img" src={logo} alt="" sx={{ width: logoSize, height: logoSize, flexShrink: 0 }} />
          <Typography variant="pageTitle" component="div" noWrap sx={{ flex: '1 1 auto', minWidth: 0 }}>
            {name}
          </Typography>
          <IconButton color="inherit" aria-label="close navigation" onClick={onClose} sx={{ '&:hover': { bgcolor: 'nav.hover' } }}>
            <CloseIcon />
          </IconButton>
        </Box>
        {children}
      </Box>
    </Drawer>
  );
}

export function NavList({ children }: { children?: ReactNode }) {
  return <List data-ui="nav-list">{children}</List>;
}

export function NavDivider() {
  return <Divider sx={{ borderColor: 'nav.hover' }} />;
}

interface NavItemProps {
  icon: ReactNode;
  label: string;
  active?: boolean;
  onClick?: () => void;
  href?: string;
}

const itemSx = {
  position: 'relative',
  color: 'nav.text',
  '&:hover, &.Mui-focusVisible': { bgcolor: 'nav.hover' },
  '&.Mui-selected, &.Mui-selected:hover, &.Mui-selected.Mui-focusVisible': { bgcolor: 'nav.activeBg', color: 'nav.activeText' },
  '&.Mui-selected::before': {
    content: '""',
    position: 'absolute',
    left: 0,
    top: 0,
    bottom: 0,
    width: indicatorWidth,
    bgcolor: 'nav.indicator',
  },
} as const;

export function NavItem({ icon, label, active = false, onClick, href }: NavItemProps) {
  const content = (
    <>
      <ListItemIcon sx={{ color: 'inherit' }}>{icon}</ListItemIcon>
      <ListItemText primary={label} />
    </>
  );
  return (
    <ListItem disablePadding data-ui="nav-item">
      {href ? (
        <ListItemButton component="a" href={href} sx={itemSx}>
          {content}
        </ListItemButton>
      ) : (
        <ListItemButton selected={active} aria-current={active ? 'page' : undefined} onClick={onClick} sx={itemSx}>
          {content}
        </ListItemButton>
      )}
    </ListItem>
  );
}

export function NavSkeleton({ rows }: { rows: number }) {
  return (
    <List data-ui="nav-skeleton" aria-busy="true">
      {Array.from({ length: rows }, (_, row) => (
        <ListItem key={row}>
          <Skeleton variant="rounded" width="100%" height={logoSize} sx={{ bgcolor: 'nav.hover' }} />
        </ListItem>
      ))}
    </List>
  );
}
