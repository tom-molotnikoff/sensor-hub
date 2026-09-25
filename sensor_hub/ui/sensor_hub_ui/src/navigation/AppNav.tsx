import type { ReactNode } from 'react';
import { useContext } from 'react';
import { useLocation, useNavigate } from 'react-router';
import SensorsIcon from '@mui/icons-material/Sensors';
import SettingsIcon from '@mui/icons-material/Settings';
import HistoryIcon from '@mui/icons-material/History';
import PeopleIcon from '@mui/icons-material/People';
import NotificationsActiveIcon from '@mui/icons-material/NotificationsActive';
import DashboardIcon from '@mui/icons-material/Dashboard';
import IntegrationInstructionsIcon from '@mui/icons-material/IntegrationInstructions';
import MenuBookIcon from '@mui/icons-material/MenuBook';
import LogoutIcon from '@mui/icons-material/Logout';
import CellTowerIcon from '@mui/icons-material/CellTower';
import StorageIcon from '@mui/icons-material/Storage';
import { SidebarContext } from '../providers/SidebarContextType';
import { useAuth, type AuthUser } from '../providers/AuthContext';
import { apiClient } from '../gen/client';
import { setCsrfToken } from '../api/Csrf';
import { hasPerm } from '../tools/Utils';
import NavFrame, { NavDivider, NavItem, NavList, NavSkeleton } from '../ui/NavFrame';

interface NavEntry {
  label: string;
  path: string;
  sections: string[];
  icon: ReactNode;
  permissions: string[];
}

const mainEntries: NavEntry[] = [
  { label: 'Dashboards', path: '/dashboard', sections: ['/dashboard'], icon: <DashboardIcon />, permissions: ['view_dashboards'] },
  { label: 'Sensors', path: '/sensors-overview', sections: ['/sensors-overview', '/sensor'], icon: <SensorsIcon />, permissions: ['view_sensors'] },
  { label: 'Data Retention', path: '/data-retention', sections: ['/data-retention'], icon: <StorageIcon />, permissions: ['view_sensors'] },
  { label: 'Properties', path: '/properties-overview', sections: ['/properties-overview'], icon: <SettingsIcon />, permissions: ['view_properties'] },
  { label: 'MQTT', path: '/mqtt', sections: ['/mqtt'], icon: <CellTowerIcon />, permissions: ['view_mqtt'] },
  {
    label: 'Alerts & Notifications',
    path: '/notifications',
    sections: ['/notifications'],
    icon: <NotificationsActiveIcon />,
    permissions: ['view_alerts', 'view_notifications', 'manage_notifications', 'manage_oauth'],
  },
  { label: 'User Management', path: '/admin', sections: ['/admin'], icon: <PeopleIcon />, permissions: ['view_users', 'view_roles'] },
];

const allowed = (user: AuthUser | undefined, permissions: string[]) => permissions.some((permission) => hasPerm(user, permission));

const inSection = (pathname: string, section: string) => pathname === section || pathname.startsWith(`${section}/`);

function AppNav() {
  const { open, setOpen } = useContext(SidebarContext);
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { user, refresh } = useAuth();

  const close = () => setOpen(false);
  const handleNavigate = (path: string) => {
    close();
    navigate(path);
  };

  const doLogout = async () => {
    await apiClient.POST('/auth/logout').catch(() => undefined);
    setCsrfToken(null);
    await refresh();
    close();
    navigate('/login');
  };

  return (
    <NavFrame open={open} onClose={close} logo="/sensor_hub.svg" name="Sensor Hub">
      {user === undefined ? (
        <NavSkeleton rows={mainEntries.length} />
      ) : (
        <NavList>
          {mainEntries
            .filter((entry) => allowed(user, entry.permissions))
            .map((entry) => (
              <NavItem
                key={entry.path}
                icon={entry.icon}
                label={entry.label}
                active={entry.sections.some((section) => inSection(pathname, section))}
                onClick={() => handleNavigate(entry.path)}
              />
            ))}
        </NavList>
      )}
      {user && (
        <>
          <NavDivider />
          <NavList>
            <NavItem icon={<HistoryIcon />} label="Sessions" onClick={() => handleNavigate('/account/sessions')} />
            {allowed(user, ['manage_api_keys', 'view_api_docs']) && (
              <NavItem icon={<IntegrationInstructionsIcon />} label="Developer" onClick={() => handleNavigate('/account/developer')} />
            )}
            <NavItem icon={<MenuBookIcon />} label="Documentation" href="/docs/" />
            <NavItem icon={<LogoutIcon />} label="Logout" onClick={doLogout} />
          </NavList>
        </>
      )}
    </NavFrame>
  );
}

export default AppNav;
