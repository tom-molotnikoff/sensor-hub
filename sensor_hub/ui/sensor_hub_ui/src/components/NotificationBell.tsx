import { useState } from 'react';
import {
  IconButton,
  Badge,
  MenuItem,
  ListItemText,
  ListItemIcon,
  Typography,
  Button,
} from '@mui/material';
import NotificationsIcon from '@mui/icons-material/Notifications';
import InfoIcon from '@mui/icons-material/Info';
import WarningIcon from '@mui/icons-material/Warning';
import ErrorIcon from '@mui/icons-material/Error';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import { useNotifications } from '../providers/NotificationContext';
import { useNavigate } from 'react-router';
import type { NotificationSeverity } from '../gen/aliases';
import MenuPanel from '../ui/MenuPanel';
import EmptyState from '../ui/EmptyState';
import { logger } from '../tools/logger';

function getSeverityIcon(severity: NotificationSeverity) {
  switch (severity) {
    case 'info':
      return <InfoIcon color="info" fontSize="small" />;
    case 'warning':
      return <WarningIcon color="warning" fontSize="small" />;
    case 'error':
      return <ErrorIcon color="error" fontSize="small" />;
    default:
      return <InfoIcon fontSize="small" />;
  }
}

function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  
  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 7) return `${diffDays}d ago`;
  
  return date.toLocaleDateString();
}

export default function NotificationBell() {
  const navigate = useNavigate();
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const { notifications, unreadCount, loading, markAsRead } = useNotifications();

  const handleOpen = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleNotificationClick = async (notificationId: number, isRead: boolean) => {
    if (!isRead) {
      try {
        await markAsRead(notificationId);
      } catch (err) {
        logger.error('Failed to mark as read:', err);
      }
    }
  };

  const handleViewAll = () => {
    handleClose();
    navigate('/notifications');
  };

  const recentNotifications = notifications?.slice(0, 5) ?? [];

  return (
    <>
      <IconButton color="inherit" aria-label="notifications" onClick={handleOpen}>
        <Badge badgeContent={unreadCount} color="error" max={99}>
          <NotificationsIcon />
        </Badge>
      </IconButton>
      <MenuPanel
        anchorEl={anchorEl}
        onClose={handleClose}
        title="Notifications"
        meta={unreadCount > 0 ? `${unreadCount} unread` : undefined}
        loading={loading}
        footer={notifications && notifications.length > 0 && (
          <Button size="small" onClick={handleViewAll}>
            View all notifications
          </Button>
        )}
      >
        {recentNotifications.length === 0 ? (
          <EmptyState
            size="sm"
            icon={<CheckCircleIcon color="disabled" fontSize="large" />}
            title="No notifications"
          />
        ) : (
          recentNotifications.map((notif) => (
            <MenuItem
              key={notif.notification_id}
              onClick={() => handleNotificationClick(notif.notification_id!, notif.is_read ?? false)}
              sx={{ backgroundColor: notif.is_read ? 'transparent' : 'action.hover' }}
            >
              <ListItemIcon>
                {getSeverityIcon(notif.notification!.severity!)}
              </ListItemIcon>
              <ListItemText
                primary={
                  <Typography
                    variant="body2"
                    noWrap
                    sx={{ fontWeight: notif.is_read ? 'fontWeightRegular' : 'fontWeightBold' }}
                  >
                    {notif.notification!.title}
                  </Typography>
                }
                secondary={
                  <>
                    <Typography variant="caption" noWrap component="div" color="text.secondary">
                      {notif.notification!.message}
                    </Typography>
                    <Typography variant="caption" component="span" color="text.disabled">
                      {formatTimeAgo(notif.notification!.created_at!)}
                    </Typography>
                  </>
                }
                slotProps={{ secondary: { component: 'div' } }}
              />
            </MenuItem>
          ))
        )}
      </MenuPanel>
    </>
  );
}
