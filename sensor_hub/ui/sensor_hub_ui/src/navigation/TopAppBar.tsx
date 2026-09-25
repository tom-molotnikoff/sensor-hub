import { useContext } from 'react';
import { SidebarContext } from '../providers/SidebarContextType';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import NotificationBell from '../components/NotificationBell';
import AppBar from '../ui/AppBar';

interface TopAppBarProps {
  pageTitle: string;
}

function TopAppBar({ pageTitle }: TopAppBarProps) {
  const { open, setOpen } = useContext(SidebarContext);
  const { user } = useAuth();

  return (
    <AppBar title={pageTitle} onMenuClick={() => setOpen(!open)}>
      {hasPerm(user, 'view_notifications') && <NotificationBell />}
    </AppBar>
  );
}

export default TopAppBar;
