import { createContext, useContext, type MouseEvent, type ReactElement, type ReactNode, type Ref } from 'react';
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
  Tooltip,
  Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import KeyboardDoubleArrowLeftIcon from '@mui/icons-material/KeyboardDoubleArrowLeft';
import KeyboardDoubleArrowRightIcon from '@mui/icons-material/KeyboardDoubleArrowRight';
import UnfoldMoreIcon from '@mui/icons-material/UnfoldMore';
import { navDrawer, navPermanent } from './theme/tokens';
import { charcoalSurface } from './charcoalSurface';

type NavFrameVariant =
  | { variant: 'permanent'; rail: boolean; onToggleRail: () => void }
  | { variant: 'temporary'; open: boolean; onClose: () => void };

type NavFrameProps = NavFrameVariant & {
  logo: string;
  name: string;
  brandAction?: ReactNode;
  navRef?: Ref<HTMLElement>;
  foot?: ReactNode;
  children?: ReactNode;
};

const logoSize = 32;
const avatarSize = 32;
const skeletonRowHeight = 32;
const indicatorWidth = 3;

const paperSx = { ...charcoalSurface.paint, borderRight: 0 } as const;

const NavRail = createContext(false);

function RailTooltip({ label, children }: { label: string; children: ReactElement }) {
  const rail = useContext(NavRail);
  return (
    <Tooltip title={rail ? label : ''} placement="right">
      {children}
    </Tooltip>
  );
}

const railRowSx = { justifyContent: 'center', '& .MuiListItemIcon-root': { minWidth: 0 } } as const;

export default function NavFrame({ logo, name, brandAction, navRef, foot, children, ...frame }: NavFrameProps) {
  const temporary = frame.variant === 'temporary';
  const rail = frame.variant === 'permanent' && frame.rail;
  const width = rail ? navPermanent.rail : navPermanent.expanded;
  const drawerProps = temporary
    ? {
        variant: 'temporary' as const,
        open: frame.open,
        onClose: frame.onClose,
        ModalProps: { keepMounted: false },
        slotProps: {
          paper: { sx: { ...paperSx, width: `min(${navDrawer.width}px, calc(100vw - ${navDrawer.pageVisible}px))` } },
        },
      }
    : {
        variant: 'permanent' as const,
        sx: { width, flexShrink: 0 },
        slotProps: { paper: { sx: { ...paperSx, width } } },
      };

  return (
    <Drawer {...drawerProps}>
      <NavRail.Provider value={rail}>
        <Box
          ref={navRef}
          component="nav"
          aria-label="Main"
          className={charcoalSurface.content.className}
          sx={[charcoalSurface.content.sx, { display: 'flex', flexDirection: 'column', flex: '1 0 auto' }]}
        >
          <Box
            data-ui="nav-brand"
            sx={{
              display: 'flex',
              flexDirection: rail ? 'column' : 'row',
              alignItems: 'center',
              gap: rail ? 1 : 1.5,
              paddingLeft: rail ? 0 : 2,
              paddingRight: rail ? 0 : 1,
              paddingY: 1.5,
            }}
          >
            <Box component="img" src={logo} alt="" sx={{ width: logoSize, height: logoSize, flexShrink: 0 }} />
            {!rail && (
              <Typography variant="cardTitle" component="div" noWrap sx={{ flex: '1 1 auto', minWidth: 0 }}>
                {name}
              </Typography>
            )}
            {brandAction}
            {temporary && (
              <IconButton color="inherit" aria-label="close navigation" onClick={frame.onClose}>
                <CloseIcon />
              </IconButton>
            )}
          </Box>
          {children}
          {(foot || !temporary) && (
            <Box data-ui="nav-foot" sx={{ marginTop: 'auto' }}>
              <Divider />
              <Box sx={{ padding: 1 }}>
                {frame.variant === 'permanent' && <NavCollapseToggle rail={rail} onToggle={frame.onToggleRail} />}
                {foot}
              </Box>
            </Box>
          )}
        </Box>
      </NavRail.Provider>
    </Drawer>
  );
}

const footRowSx = {
  borderRadius: 1,
  '&:hover, &.Mui-focusVisible': { bgcolor: 'nav.hover' },
} as const;

function NavCollapseToggle({ rail, onToggle }: { rail: boolean; onToggle: () => void }) {
  const label = rail ? 'Expand' : 'Collapse';
  return (
    <RailTooltip label={label}>
      <ListItemButton
        data-ui="nav-collapse"
        aria-label={label}
        aria-expanded={!rail}
        onClick={onToggle}
        sx={{ ...footRowSx, color: 'nav.muted', ...(rail && railRowSx) }}
      >
        <ListItemIcon sx={{ color: 'inherit' }}>{rail ? <KeyboardDoubleArrowRightIcon /> : <KeyboardDoubleArrowLeftIcon />}</ListItemIcon>
        {!rail && <ListItemText primary={label} />}
      </ListItemButton>
    </RailTooltip>
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
  const rail = useContext(NavRail);
  return (
    <ListItem disablePadding data-ui="nav-item">
      <RailTooltip label={label}>
        <ListItemButton
          selected={active}
          aria-current={active ? 'page' : undefined}
          aria-label={rail ? label : undefined}
          onClick={onClick}
          sx={{ ...itemSx, ...(rail && railRowSx) }}
        >
          <ListItemIcon sx={{ color: 'inherit' }}>{icon}</ListItemIcon>
          {!rail && <ListItemText primary={label} />}
        </ListItemButton>
      </RailTooltip>
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
  const rail = useContext(NavRail);
  return (
    <ListItemButton
      data-ui="nav-account"
      aria-haspopup="menu"
      aria-controls={menuOpen ? menuId : undefined}
      aria-expanded={menuOpen}
      aria-label={rail ? name : undefined}
      onClick={onClick}
      sx={{
        ...footRowSx,
        gap: 1.5,
        paddingX: 1,
        justifyContent: rail ? 'center' : undefined,
        color: 'nav.text',
        bgcolor: menuOpen ? 'nav.hover' : undefined,
      }}
    >
      <Avatar aria-hidden sx={{ width: avatarSize, height: avatarSize }}>
        {initial}
      </Avatar>
      {!rail && (
        <ListItemText
          primary={name}
          secondary={detail}
          slotProps={{ primary: { noWrap: true }, secondary: { noWrap: true, sx: { color: 'nav.muted' } } }}
          sx={{ minWidth: 0 }}
        />
      )}
      {!rail && <UnfoldMoreIcon fontSize="small" sx={{ color: 'nav.muted' }} />}
    </ListItemButton>
  );
}
