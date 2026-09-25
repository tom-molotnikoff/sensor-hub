import {Menu, MenuItem, useColorScheme, ListItemIcon, ListItemText} from '@mui/material';
import WbSunnyIcon from '@mui/icons-material/WbSunny';
import DarkModeIcon from '@mui/icons-material/DarkMode';
import LaptopIcon from '@mui/icons-material/Laptop';
import HistoryIcon from '@mui/icons-material/History';
import CheckIcon from '@mui/icons-material/Check';
import AccountCircle from '@mui/icons-material/AccountCircle';
import ExitToAppIcon from '@mui/icons-material/ExitToApp';
import {SidebarContext} from "../providers/SidebarContextType.tsx";
import {useContext, useState} from "react";
import { useNavigate } from 'react-router';
import { useAuth } from '../providers/AuthContext.tsx';
import { apiClient } from '../gen/client';
import { setCsrfToken } from '../api/Csrf';
import {hasPerm} from "../tools/Utils.ts";
import HelpIcon from '@mui/icons-material/Help';
import NotificationBell from "../components/NotificationBell";
import AppBar from '../ui/AppBar';

interface TopAppBarProps {
  pageTitle: string;
}

const modes = [
  { mode: 'light', label: 'Light' },
  { mode: 'dark', label: 'Dark' },
  { mode: 'system', label: 'System' },
] as const;

const docsHref = '/docs/';

function TopAppBar({ pageTitle }: TopAppBarProps) {
  const {open, setOpen} = useContext(SidebarContext);
  const {mode, setMode} = useColorScheme();
  const [themeAnchor, setThemeAnchor] = useState<null | HTMLElement>(null);
  const [accountAnchor, setAccountAnchor] = useState<null | HTMLElement>(null);
  const navigate = useNavigate();
  const { user, refresh } = useAuth();

  const handleThemeClose = () => {
    setThemeAnchor(null);
  };

  const handleAccountOpen = (event: React.MouseEvent<HTMLElement>) => {
    setAccountAnchor(event.currentTarget);
  };
  const handleAccountClose = () => setAccountAnchor(null);

  const handleThemeFromAccount = () => {
    setThemeAnchor(accountAnchor);
    handleAccountClose();
  };

  const handleModeChange = (newMode: 'light' | 'dark' | 'system') => {
    setMode(newMode);
    handleThemeClose();
  };

  const doLogout = async () => {
    try {
      await apiClient.POST('/auth/logout');
    } catch {
      // ignore
    }
    setCsrfToken(null);
    await refresh();
    handleAccountClose();
    navigate('/login');
  };

  let ModeIcon = WbSunnyIcon;
  if (mode === 'dark') ModeIcon = DarkModeIcon;
  else if (mode === 'system') ModeIcon = LaptopIcon;

  const accountMenuItems: React.ReactNode[] = [
    <MenuItem key="theme" onClick={handleThemeFromAccount}>
      <ListItemIcon><ModeIcon fontSize="small" /></ListItemIcon>
      Theme
    </MenuItem>,
    <MenuItem key="docs" component="a" href={docsHref} onClick={handleAccountClose}>
      <ListItemIcon><HelpIcon fontSize="small" /></ListItemIcon>
      Documentation
    </MenuItem>,
  ];
  if (user) {
    accountMenuItems.push(
      <MenuItem key="mysessions" onClick={() => { handleAccountClose(); navigate('/account/sessions'); }}>
        <ListItemIcon><HistoryIcon fontSize="small" /></ListItemIcon>
        My sessions
      </MenuItem>
    );
    accountMenuItems.push(
      <MenuItem key="changepw" onClick={() => { handleAccountClose(); navigate('/account/change-password'); }}>
        <ListItemIcon><AccountCircle fontSize="small" /></ListItemIcon>
        Change password
      </MenuItem>
    );
    accountMenuItems.push(
      <MenuItem key="logout" onClick={doLogout}>
        <ListItemIcon><ExitToAppIcon fontSize="small" /></ListItemIcon>
        Logout
      </MenuItem>
    );
  } else {
    accountMenuItems.push(
      <MenuItem key="login" onClick={() => { handleAccountClose(); navigate('/login'); }}>
        <ListItemIcon><AccountCircle fontSize="small" /></ListItemIcon>
        Login
      </MenuItem>
    );
  }

  return (
    <>
      <AppBar
        title={pageTitle}
        onMenuClick={() => setOpen(!open)}
        account={{ initial: user?.username?.charAt(0).toUpperCase() ?? 'S', onClick: handleAccountOpen }}
      >
        {user && hasPerm(user, 'view_notifications') && <NotificationBell />}
      </AppBar>
      <Menu
        anchorEl={themeAnchor}
        open={Boolean(themeAnchor)}
        onClose={handleThemeClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        {modes.map((option) => (
          <MenuItem key={option.mode} selected={mode === option.mode} onClick={() => handleModeChange(option.mode)}>
            <ListItemText>{option.label}</ListItemText>
            {mode === option.mode && <CheckIcon fontSize="small" />}
          </MenuItem>
        ))}
      </Menu>
      <Menu anchorEl={accountAnchor} open={Boolean(accountAnchor)} onClose={handleAccountClose} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }} transformOrigin={{ vertical: 'top', horizontal: 'right' }}>
        {accountMenuItems}
      </Menu>
    </>
  );
}

export default TopAppBar;
