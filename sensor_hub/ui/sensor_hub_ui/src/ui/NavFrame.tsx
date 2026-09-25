import type { MouseEvent, ReactNode } from 'react';
import {
  Avatar,
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
import UnfoldMoreIcon from '@mui/icons-material/UnfoldMore';
import { navDrawer } from './theme/tokens';

interface NavFrameProps {
  open: boolean;
  onClose: () => void;
  logo: string;
  name: string;
  foot?: ReactNode;
  children?: ReactNode;
}

const logoSize = 32;
const avatarSize = 32;
const skeletonRowHeight = 32;
const indicatorWidth = 3;

export default function NavFrame({ open, onClose, logo, name, foot, children }: NavFrameProps) {
  return (
    <Drawer
      variant="temporary"
      open={open}
      onClose={onClose}
      ModalProps={{ keepMounted: false }}
      slotProps={{
        paper: {
          sx: {
            width: `min(${navDrawer.width}px, calc(100vw - ${navDrawer.pageVisible}px))`,
            bgcolor: 'nav.bg',
            backgroundImage: 'none',
          },
        },
      }}
    >
      <Box
        component="nav"
        aria-label="Main"
        className="dark"
        sx={{ display: 'flex', flexDirection: 'column', flex: '1 0 auto', color: 'nav.text' }}
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
        {foot && (
          <Box data-ui="nav-foot" sx={{ marginTop: 'auto' }}>
            <Divider />
            <Box sx={{ padding: 1 }}>{foot}</Box>
          </Box>
        )}
      </Box>
    </Drawer>
  );
}

export function NavList({ children }: { children?: ReactNode }) {
  return <List data-ui="nav-list">{children}</List>;
}

interface NavItemProps {
  icon: ReactNode;
  label: string;
  active?: boolean;
  onClick?: () => void;
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

export function NavItem({ icon, label, active = false, onClick }: NavItemProps) {
  return (
    <ListItem disablePadding data-ui="nav-item">
      <ListItemButton selected={active} aria-current={active ? 'page' : undefined} onClick={onClick} sx={itemSx}>
        <ListItemIcon sx={{ color: 'inherit' }}>{icon}</ListItemIcon>
        <ListItemText primary={label} />
      </ListItemButton>
    </ListItem>
  );
}

export function NavSkeleton({ rows }: { rows: number }) {
  return (
    <List data-ui="nav-skeleton" aria-busy="true">
      {Array.from({ length: rows }, (_, row) => (
        <ListItem key={row}>
          <Skeleton variant="rounded" width="100%" height={skeletonRowHeight} />
        </ListItem>
      ))}
    </List>
  );
}

interface NavAccountBlockProps {
  initial: string;
  name: string;
  detail: string;
  menuId: string;
  menuOpen: boolean;
  onClick: (event: MouseEvent<HTMLElement>) => void;
}

export function NavAccountBlock({ initial, name, detail, menuId, menuOpen, onClick }: NavAccountBlockProps) {
  return (
    <ListItemButton
      data-ui="nav-account"
      aria-haspopup="menu"
      aria-controls={menuOpen ? menuId : undefined}
      aria-expanded={menuOpen}
      onClick={onClick}
      sx={{
        gap: 1.5,
        paddingX: 1,
        borderRadius: 1,
        color: 'nav.text',
        bgcolor: menuOpen ? 'nav.hover' : undefined,
        '&:hover, &.Mui-focusVisible': { bgcolor: 'nav.hover' },
      }}
    >
      <Avatar aria-hidden sx={{ width: avatarSize, height: avatarSize }}>
        {initial}
      </Avatar>
      <ListItemText
        primary={name}
        secondary={detail}
        slotProps={{ primary: { noWrap: true }, secondary: { noWrap: true, sx: { color: 'nav.muted' } } }}
        sx={{ minWidth: 0 }}
      />
      <UnfoldMoreIcon fontSize="small" sx={{ color: 'nav.muted' }} />
    </ListItemButton>
  );
}
