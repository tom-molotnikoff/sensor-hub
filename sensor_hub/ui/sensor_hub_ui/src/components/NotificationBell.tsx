import { useState } from 'react';
import {
  IconButton,
  Badge,
  Menu,
  MenuItem,
  ListItemText,
  ListItemIcon,
  Typography,
  Divider,
  Box,
  Button,
  CircularProgress,
} from '@mui/material';
import NotificationsIcon from '@mui/icons-material/Notifications';
import InfoIcon from '@mui/icons-material/Info';
import WarningIcon from '@mui/icons-material/Warning';
import ErrorIcon from '@mui/icons-material/Error';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import { useNotifications } from '../providers/NotificationContext';
import { useNavigate } from 'react-router';
import type { NotificationSeverity } from '../gen/aliases';
import { useIsMobile } from '../hooks/useMobile';
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
  const open = Boolean(anchorEl);
  const isMobile = useIsMobile();

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
      <Menu
        anchorEl={anchorEl}
        open={open}
        onClose={handleClose}
        anchorOrigin={{ 
          vertical: 'bottom', 
          horizontal: isMobile ? 'center' : 'right' 
        }}
        transformOrigin={{ 
          vertical: 'top', 
          horizontal: isMobile ? 'center' : 'right' 
        }}
        slotProps={{
          paper: {
            sx: { 
              width: isMobile ? '90vw' : 360, 
              maxWidth: 360,
              maxHeight: 450 
            },
          },
          root: {
            slotProps: {
              backdrop: {
                sx: { position: 'fixed' }
              }
            }
          }
        }}
      >
        <Box sx={{ px: 2, py: 1, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Typography variant="subtitle1" sx={{
            fontWeight: "bold"
          }}>
            Notifications
          </Typography>
          {unreadCount > 0 && (
            <Typography variant="caption" sx={{
              color: "text.secondary"
            }}>
              {unreadCount} unread
            </Typography>
          )}
        </Box>
        <Divider />
        
        {loading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
            <CircularProgress size={24} />
          </Box>
        ) : recentNotifications.length === 0 ? (
          <Box sx={{ py: 3, textAlign: 'center' }}>
            <CheckCircleIcon color="disabled" sx={{ fontSize: 40, mb: 1 }} />
            <Typography variant="body2" sx={{
              color: "text.secondary"
            }}>
              No notifications
            </Typography>
          </Box>
        ) : (
          recentNotifications.map((notif) => (
            <MenuItem
              key={notif.notification_id}
              onClick={() => handleNotificationClick(notif.notification_id!, notif.is_read ?? false)}
              sx={{
                backgroundColor: notif.is_read ? 'transparent' : 'action.hover',
                py: 1.5,
              }}
            >
              <ListItemIcon sx={{ minWidth: 36 }}>
                {getSeverityIcon(notif.notification!.severity!)}
              </ListItemIcon>
              <ListItemText
                primary={
                  <Typography
                    variant="body2"
                    noWrap
                    sx={{
                      fontWeight: notif.is_read ? 'normal' : 'bold'
                    }}
                  >
                    {notif.notification!.title}
                  </Typography>
                }
                secondary={
                  <>
                    <Typography
                      variant="caption"
                      noWrap
                      component="span"
                      sx={{
                        color: "text.secondary",
                        display: "block"
                      }}>
                      {notif.notification!.message}
                    </Typography>
                    <Typography variant="caption" component="span" sx={{
                      color: "text.disabled"
                    }}>
                      {formatTimeAgo(notif.notification!.created_at!)}
                    </Typography>
                  </>
                }
                slotProps={{ secondary: { component: 'div' } }}
              />
            </MenuItem>
          ))
        )}
        
        {notifications && notifications.length > 0 && [
          <Divider key="divider" />,
          <Box key="view-all" sx={{ p: 1, textAlign: 'center' }}>
            <Button size="small" onClick={handleViewAll}>
              View all notifications
            </Button>
          </Box>
        ]}
      </Menu>
    </>
  );
}
