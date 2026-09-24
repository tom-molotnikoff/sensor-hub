import { Drawer, ListItem, ListItemButton, ListItemText, List, Toolbar, Divider, IconButton, Typography, ListItemIcon } from '@mui/material';
import {useContext} from "react";
import CloseIcon from '@mui/icons-material/Close';
import SensorsIcon from '@mui/icons-material/Sensors';
import SettingsIcon from '@mui/icons-material/Settings';
import HistoryIcon from '@mui/icons-material/History';
import PeopleIcon from '@mui/icons-material/People';
import NotificationsActiveIcon from '@mui/icons-material/NotificationsActive';
import DashboardIcon from '@mui/icons-material/Dashboard';
import IntegrationInstructionsIcon from '@mui/icons-material/IntegrationInstructions';
import MenuBookIcon from '@mui/icons-material/MenuBook';
import LogoutIcon from '@mui/icons-material/Logout';
import LoginIcon from '@mui/icons-material/Login';
import CellTowerIcon from '@mui/icons-material/CellTower';
import StorageIcon from '@mui/icons-material/Storage';
import {SidebarContext} from "../providers/SidebarContextType.tsx";
import {useNavigate} from "react-router";
import { useAuth } from '../providers/AuthContext.tsx';
import { apiClient } from '../gen/client';
import { setCsrfToken } from '../api/Csrf';
import {hasPerm} from "../tools/Utils.ts";
import Inline from '../ui/Inline';

function SidebarHeader({ onClose }: { onClose: () => void }) {
  return (
    <Toolbar variant="regular">
      <Inline>
        <IconButton edge="start" color="inherit" aria-label="menu" onClick={onClose}>
          <CloseIcon />
        </IconButton>
        <Typography variant="h6" component="div" color="inherit">
          Sensor Hub
        </Typography>
      </Inline>
    </Toolbar>
  );
}

function NavigationSidebar() {
  const {open, setOpen} = useContext(SidebarContext);

  const navigate = useNavigate();
  const { user, refresh } = useAuth();

  const handleNavigate = (path: string) => { setOpen(false); navigate(path); };

  const doLogout = async () => {
    await apiClient.POST('/auth/logout').catch(() => undefined);
    setCsrfToken(null);
    await refresh();
    setOpen(false);
    navigate('/login');
  }

  if (user === undefined) return (
    <Drawer
      variant="temporary"
      ModalProps={{
        keepMounted: false,
      }}
      open={open}
      onClose={() => setOpen(false)}
    >
      <SidebarHeader onClose={() => setOpen(!open)} />
      <Divider />
      <List>
        <ListItem>
          <ListItemText primary="Loading..." />
        </ListItem>
      </List>
      <Divider />
    </Drawer>
  );


  return (
    <Drawer
      variant="temporary"
      ModalProps={{
        keepMounted: false,
      }}
      open={open}
      onClose={() => setOpen(false)}
    >
      <SidebarHeader onClose={() => setOpen(!open)} />
      <Divider />
      <List>
        { (hasPerm(user, 'view_dashboards') && (
          <ListItem disablePadding>
            <ListItemButton onClick={() => handleNavigate('/dashboard')}>
              <ListItemIcon><DashboardIcon /></ListItemIcon>
              <ListItemText primary="Dashboards" />
            </ListItemButton>
          </ListItem>
        ))}
        { (hasPerm(user, 'view_sensors') && (
          <ListItem disablePadding>
            <ListItemButton onClick={() => handleNavigate('/sensors-overview')}>
              <ListItemIcon><SensorsIcon /></ListItemIcon>
              <ListItemText primary="Sensors" />
            </ListItemButton>
          </ListItem>
        ))}
        { (hasPerm(user, 'view_sensors') && (
          <ListItem disablePadding>
            <ListItemButton onClick={() => handleNavigate('/data-retention')}>
              <ListItemIcon><StorageIcon /></ListItemIcon>
              <ListItemText primary="Data Retention" />
            </ListItemButton>
          </ListItem>
        ))}
        { (hasPerm(user, 'view_properties') && (
          <ListItem disablePadding>
            <ListItemButton onClick={() => handleNavigate('/properties-overview')}>
              <ListItemIcon><SettingsIcon /></ListItemIcon>
              <ListItemText primary="Properties" />
            </ListItemButton>
          </ListItem>
        ))}
        { (hasPerm(user, 'view_mqtt') && (
          <ListItem disablePadding>
            <ListItemButton onClick={() => handleNavigate('/mqtt')}>
              <ListItemIcon><CellTowerIcon /></ListItemIcon>
              <ListItemText primary="MQTT" />
            </ListItemButton>
          </ListItem>
        ))}
        { ((hasPerm(user, 'view_alerts') || hasPerm(user, 'view_notifications') || hasPerm(user, 'manage_notifications') || hasPerm(user, 'manage_oauth')) && (
          <ListItem disablePadding>
            <ListItemButton onClick={() => handleNavigate('/notifications')}>
              <ListItemIcon><NotificationsActiveIcon /></ListItemIcon>
              <ListItemText primary="Alerts & Notifications" />
            </ListItemButton>
          </ListItem>
        ))}
        { user && (
          <>
            <ListItem disablePadding>
              <ListItemButton onClick={() => handleNavigate('/account/sessions')}>
                <ListItemIcon><HistoryIcon /></ListItemIcon>
                <ListItemText primary="Sessions" />
              </ListItemButton>
            </ListItem>
            { (hasPerm(user, 'manage_api_keys') || hasPerm(user, 'view_api_docs')) && (
              <ListItem disablePadding>
                <ListItemButton onClick={() => handleNavigate('/account/developer')}>
                  <ListItemIcon><IntegrationInstructionsIcon /></ListItemIcon>
                  <ListItemText primary="Developer" />
                </ListItemButton>
              </ListItem>
            )}
            <ListItem disablePadding>
              <ListItemButton component="a" href="/docs/">
                <ListItemIcon><MenuBookIcon /></ListItemIcon>
                <ListItemText primary="Documentation" />
              </ListItemButton>
            </ListItem>
            { (hasPerm(user,'view_users') || hasPerm(user,'view_roles')) && (
              <ListItem disablePadding>
                <ListItemButton onClick={() => handleNavigate('/admin')}>
                  <ListItemIcon><PeopleIcon /></ListItemIcon>
                  <ListItemText primary="User Management" />
                </ListItemButton>
              </ListItem>
            )}
            <ListItem disablePadding>
              <ListItemButton onClick={doLogout}>
                <ListItemIcon><LogoutIcon /></ListItemIcon>
                <ListItemText primary="Logout" />
              </ListItemButton>
            </ListItem>
          </>
        )}
        { !user && (
          <ListItem disablePadding>
            <ListItemButton onClick={() => handleNavigate('/login')}>
              <ListItemIcon><LoginIcon /></ListItemIcon>
              <ListItemText primary="Login" />
            </ListItemButton>
          </ListItem>
        )}
      </List>
      <Divider />
    </Drawer>
  );
}

export default NavigationSidebar