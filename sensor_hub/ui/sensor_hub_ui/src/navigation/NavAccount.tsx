import { useId, useState, type MouseEvent } from 'react';
import { ListItemIcon, ListItemText, MenuItem, useColorScheme } from '@mui/material';
import WbSunnyIcon from '@mui/icons-material/WbSunny';
import DarkModeIcon from '@mui/icons-material/DarkMode';
import LaptopIcon from '@mui/icons-material/Laptop';
import HistoryIcon from '@mui/icons-material/History';
import AccountCircleIcon from '@mui/icons-material/AccountCircle';
import IntegrationInstructionsIcon from '@mui/icons-material/IntegrationInstructions';
import MenuBookIcon from '@mui/icons-material/MenuBook';
import LogoutIcon from '@mui/icons-material/Logout';
import { useAuth, type AuthUser } from '../providers/AuthContext';
import { apiClient } from '../gen/client';
import { setCsrfToken } from '../api/Csrf';
import { hasAnyPerm } from '../tools/Utils';
import { NavAccountBlock } from '../ui/NavFrame';
import AnchoredMenu, { MenuDivider, MenuHeading, MenuNewTabLink, MenuSegments } from '../ui/AnchoredMenu';

const modes = [
  { value: 'light', label: 'Light', icon: <WbSunnyIcon fontSize="small" /> },
  { value: 'dark', label: 'Dark', icon: <DarkModeIcon fontSize="small" /> },
  { value: 'system', label: 'System', icon: <LaptopIcon fontSize="small" /> },
] as const;

const docsHref = '/docs/';

interface NavAccountProps {
  user: NonNullable<AuthUser>;
  onNavigate: (path: string) => void;
}

function NavAccount({ user, onNavigate }: NavAccountProps) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const menuId = useId();
  const headingId = useId();
  const { mode, setMode } = useColorScheme();
  const { refresh } = useAuth();

  const close = () => setAnchor(null);
  const go = (path: string) => {
    close();
    onNavigate(path);
  };

  const doLogout = async () => {
    close();
    await apiClient.POST('/auth/logout').catch(() => undefined);
    setCsrfToken(null);
    await refresh();
    onNavigate('/login');
  };

  return (
    <>
      <NavAccountBlock
        initial={user.username.charAt(0).toUpperCase()}
        name={user.username}
        detail={user.roles.join(', ')}
        menuId={menuId}
        menuOpen={anchor !== null}
        onClick={(event: MouseEvent<HTMLElement>) => setAnchor(event.currentTarget)}
      />
      <AnchoredMenu
        id={menuId}
        data-ui="nav-account-menu"
        variant="menu"
        anchorEl={anchor}
        onClose={close}
        width="sm"
        placement="above-start"
        labelledBy={headingId}
      >
        <MenuHeading id={headingId}>
          Signed in as <strong>{user.username}</strong>
        </MenuHeading>
        <MenuSegments label="Theme" choices={modes} value={mode} onChange={setMode} />
        <MenuDivider />
        <MenuItem onClick={() => go('/account/sessions')}>
          <ListItemIcon>
            <HistoryIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>My sessions</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => go('/account/change-password')}>
          <ListItemIcon>
            <AccountCircleIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Change password</ListItemText>
        </MenuItem>
        {hasAnyPerm(user, ['manage_api_keys', 'view_api_docs']) && (
          <MenuItem onClick={() => go('/account/developer')}>
            <ListItemIcon>
              <IntegrationInstructionsIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>Developer</ListItemText>
          </MenuItem>
        )}
        <MenuNewTabLink href={docsHref} onClick={close}>
          <ListItemIcon>
            <MenuBookIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Documentation</ListItemText>
        </MenuNewTabLink>
        <MenuDivider />
        <MenuItem onClick={doLogout}>
          <ListItemIcon>
            <LogoutIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Logout</ListItemText>
        </MenuItem>
      </AnchoredMenu>
    </>
  );
}

export default NavAccount;
