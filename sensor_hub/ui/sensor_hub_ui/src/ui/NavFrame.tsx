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
  Menu,
  MenuItem,
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
const menuWidth = 260;
const menuMargin = 16;

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

interface NavAccountMenuProps {
  id: string;
  label: string;
  anchorEl: HTMLElement | null;
  onClose: () => void;
  heading: ReactNode;
  children?: ReactNode;
}

export function NavAccountMenu({ id, label, anchorEl, onClose, heading, children }: NavAccountMenuProps) {
  return (
    <Menu
      id={id}
      data-ui="nav-account-menu"
      variant="menu"
      anchorEl={anchorEl}
      open={anchorEl !== null}
      onClose={onClose}
      marginThreshold={menuMargin}
      anchorOrigin={{ vertical: 'top', horizontal: 'left' }}
      transformOrigin={{ vertical: 'bottom', horizontal: 'left' }}
      slotProps={{
        list: { 'aria-label': label },
        paper: { sx: { width: `min(${menuWidth}px, calc(100vw - ${menuMargin * 2}px))` } },
      }}
    >
      <Typography
        data-ui="nav-account-menu-heading"
        component="div"
        variant="caption"
        color="text.secondary"
        noWrap
        sx={{ paddingX: 2, paddingY: 1 }}
      >
        {heading}
      </Typography>
      {children}
    </Menu>
  );
}

interface MenuChoice<T extends string> {
  value: T;
  label: string;
  icon: ReactNode;
}

interface MenuSegmentsProps<T extends string> {
  label: string;
  choices: readonly MenuChoice<T>[];
  value: T | undefined;
  onChange: (value: T) => void;
}

export function MenuSegments<T extends string>({ label, choices, value, onChange }: MenuSegmentsProps<T>) {
  return (
    <Box
      role="group"
      aria-label={label}
      data-ui="menu-segments"
      sx={{ display: 'flex', marginX: 1, marginBottom: 1, border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden' }}
    >
      {choices.map((choice) => (
        <MenuItem
          key={choice.value}
          role="menuitemradio"
          aria-checked={choice.value === value}
          selected={choice.value === value}
          onClick={() => onChange(choice.value)}
          sx={{
            flex: '1 1 0',
            flexDirection: 'column',
            gap: 0.25,
            minHeight: 0,
            paddingX: 0.5,
            paddingY: 0.75,
            typography: 'caption',
            color: 'text.secondary',
            '& + &': { borderLeft: 1, borderColor: 'divider' },
            '&.Mui-selected': { color: 'primary.main' },
          }}
        >
          {choice.icon}
          {choice.label}
        </MenuItem>
      ))}
    </Box>
  );
}
