import type { ReactNode } from 'react';
import { useContext, useState } from 'react';
import { useLocation, useNavigate } from 'react-router';
import SensorsIcon from '@mui/icons-material/Sensors';
import SettingsIcon from '@mui/icons-material/Settings';
import PeopleIcon from '@mui/icons-material/People';
import NotificationsActiveIcon from '@mui/icons-material/NotificationsActive';
import DashboardIcon from '@mui/icons-material/Dashboard';
import CellTowerIcon from '@mui/icons-material/CellTower';
import StorageIcon from '@mui/icons-material/Storage';
import { SidebarContext } from '../providers/SidebarContextType';
import { useAuth } from '../providers/AuthContext';
import { hasAnyPerm, hasPerm } from '../tools/Utils';
import NavFrame, { NavItem, NavList, NavSkeleton } from '../ui/NavFrame';
import NotificationBell from '../components/NotificationBell';
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

const inSection = (pathname: string, section: string) => pathname === section || pathname.startsWith(`${section}/`);

const isCurrent = (pathname: string, entry: NavEntry) =>
  [entry.path, ...(entry.alsoMarks ?? [])].some((section) => inSection(pathname, section));

interface AppNavProps {
  permanent?: boolean;
}

function AppNav({ permanent = false }: AppNavProps) {
  const { open, setOpen, collapsed, toggleCollapsed } = useContext(SidebarContext);
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { user } = useAuth();
  const [navElement, setNavElement] = useState<HTMLElement | null>(null);

  const close = () => setOpen(false);
  const handleNavigate = (path: string) => {
    close();
    navigate(path);
  };
  const rail = permanent && collapsed;
  const frame = permanent
    ? ({ variant: 'permanent', rail, onToggleRail: toggleCollapsed } as const)
    : ({ variant: 'temporary', open, onClose: close } as const);

  return (
    <NavFrame
      {...frame}
      logo="/sensor_hub.svg"
      name="Sensor Hub"
      navRef={setNavElement}
      brandAction={permanent && hasPerm(user, 'view_notifications') && <NotificationBell panelBeside={navElement} />}
      foot={user && <NavAccount user={user} onNavigate={handleNavigate} menuBeside={rail ? navElement : null} />}
    >
      {user === undefined ? (
        <NavSkeleton rows={mainEntries.length} />
      ) : (
        <NavList>
          {mainEntries
            .filter((entry) => hasAnyPerm(user, entry.permissions))
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
