import type { ReactNode } from 'react';
import { useContext } from 'react';
import { useLocation, useNavigate } from 'react-router';
import SensorsIcon from '@mui/icons-material/Sensors';
import SettingsIcon from '@mui/icons-material/Settings';
import PeopleIcon from '@mui/icons-material/People';
import NotificationsActiveIcon from '@mui/icons-material/NotificationsActive';
import DashboardIcon from '@mui/icons-material/Dashboard';
import CellTowerIcon from '@mui/icons-material/CellTower';
import StorageIcon from '@mui/icons-material/Storage';
import { SidebarContext } from '../providers/SidebarContextType';
import { useAuth, type AuthUser } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import NavFrame, { NavItem, NavList, NavSkeleton } from '../ui/NavFrame';
import NavAccount from './NavAccount';

interface NavEntry {
  label: string;
  path: string;
  alsoMarks?: string[];
  icon: ReactNode;
  permissions: string[];
}

const mainEntries: NavEntry[] = [
  { label: 'Dashboards', path: '/dashboard', icon: <DashboardIcon />, permissions: ['view_dashboards'] },
  { label: 'Sensors', path: '/sensors-overview', alsoMarks: ['/sensor'], icon: <SensorsIcon />, permissions: ['view_sensors'] },
  { label: 'Data Retention', path: '/data-retention', icon: <StorageIcon />, permissions: ['view_sensors'] },
  { label: 'Properties', path: '/properties-overview', icon: <SettingsIcon />, permissions: ['view_properties'] },
  { label: 'MQTT', path: '/mqtt', icon: <CellTowerIcon />, permissions: ['view_mqtt'] },
  {
    label: 'Alerts & Notifications',
    path: '/notifications',
    icon: <NotificationsActiveIcon />,
    permissions: ['view_alerts', 'view_notifications', 'manage_notifications', 'manage_oauth'],
  },
  { label: 'User Management', path: '/admin', icon: <PeopleIcon />, permissions: ['view_users', 'view_roles'] },
];

const allowed = (user: AuthUser | undefined, permissions: string[]) => permissions.some((permission) => hasPerm(user, permission));

const inSection = (pathname: string, section: string) => pathname === section || pathname.startsWith(`${section}/`);

const isCurrent = (pathname: string, entry: NavEntry) =>
  [entry.path, ...(entry.alsoMarks ?? [])].some((section) => inSection(pathname, section));

function AppNav() {
  const { open, setOpen } = useContext(SidebarContext);
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { user } = useAuth();

  const close = () => setOpen(false);
  const handleNavigate = (path: string) => {
    close();
    navigate(path);
  };

  return (
    <NavFrame
      open={open}
      onClose={close}
      logo="/sensor_hub.svg"
      name="Sensor Hub"
      foot={user && <NavAccount user={user} onNavigate={handleNavigate} />}
    >
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
                active={isCurrent(pathname, entry)}
                onClick={() => handleNavigate(entry.path)}
              />
            ))}
        </NavList>
      )}
    </NavFrame>
  );
}

export default AppNav;
