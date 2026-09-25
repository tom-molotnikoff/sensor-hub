import { useId, useState, type MouseEvent } from 'react';
import { Divider, ListItemIcon, ListItemText, MenuItem, useColorScheme } from '@mui/material';
import WbSunnyIcon from '@mui/icons-material/WbSunny';
import DarkModeIcon from '@mui/icons-material/DarkMode';
import LaptopIcon from '@mui/icons-material/Laptop';
import HistoryIcon from '@mui/icons-material/History';
import AccountCircleIcon from '@mui/icons-material/AccountCircle';
import IntegrationInstructionsIcon from '@mui/icons-material/IntegrationInstructions';
import MenuBookIcon from '@mui/icons-material/MenuBook';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';
import LogoutIcon from '@mui/icons-material/Logout';
import { useAuth, type AuthUser } from '../providers/AuthContext';
import { apiClient } from '../gen/client';
import { setCsrfToken } from '../api/Csrf';
import { hasPerm } from '../tools/Utils';
import { MenuSegments, NavAccountBlock, NavAccountMenu } from '../ui/NavFrame';

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
      <NavAccountMenu
        id={menuId}
        label="Account"
        anchorEl={anchor}
        onClose={close}
        heading={
          <>
            Signed in as <strong>{user.username}</strong>
          </>
        }
      >
        <MenuSegments label="Theme" choices={modes} value={mode} onChange={setMode} />
        <Divider />
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
        {(hasPerm(user, 'manage_api_keys') || hasPerm(user, 'view_api_docs')) && (
          <MenuItem onClick={() => go('/account/developer')}>
            <ListItemIcon>
              <IntegrationInstructionsIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>Developer</ListItemText>
          </MenuItem>
        )}
        <MenuItem component="a" href={docsHref} target="_blank" rel="noopener noreferrer" onClick={close}>
          <ListItemIcon>
            <MenuBookIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Documentation</ListItemText>
          <OpenInNewIcon fontSize="small" color="action" titleAccess="opens in a new tab" />
        </MenuItem>
        <Divider />
        <MenuItem onClick={doLogout}>
          <ListItemIcon>
            <LogoutIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Logout</ListItemText>
        </MenuItem>
      </NavAccountMenu>
    </>
  );
}

export default NavAccount;
