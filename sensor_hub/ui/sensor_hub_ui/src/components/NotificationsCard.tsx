import { useState } from 'react';
import {
  Button,
  Chip,
  IconButton,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tab,
  Tabs,
  Typography,
} from '@mui/material';
import { CascadeRowsLoader } from '../ui/loaders';
import MoreVertIcon from '@mui/icons-material/MoreVert';
import InfoIcon from '@mui/icons-material/Info';
import WarningIcon from '@mui/icons-material/Warning';
import ErrorIcon from '@mui/icons-material/Error';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import { useNotifications } from '../providers/NotificationContext';
import type { NotificationSeverity, NotificationCategory } from '../gen/aliases';
import Card from '../ui/Card';
import EmptyState from '../ui/EmptyState';
import Inline from '../ui/Inline';
import Stack from '../ui/Stack';

function getSeverityIcon(severity: NotificationSeverity) {
  switch (severity) {
    case 'info': return <InfoIcon color="info" />;
    case 'warning': return <WarningIcon color="warning" />;
    case 'error': return <ErrorIcon color="error" />;
    default: return <InfoIcon />;
  }
}

function getSeverityColor(severity: NotificationSeverity): 'info' | 'warning' | 'error' | 'default' {
  switch (severity) {
    case 'info': return 'info';
    case 'warning': return 'warning';
    case 'error': return 'error';
    default: return 'default';
  }
}

function getCategoryLabel(category: NotificationCategory): string {
  switch (category) {
    case 'threshold_alert': return 'Alert';
    case 'user_management': return 'User';
    case 'config_change': return 'Config';
    default: return category;
  }
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

export default function NotificationsCard() {
  const { notifications, loading, markAsRead, dismiss, markAllAsRead, dismissAll, refresh } = useNotifications();
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedNotifId, setSelectedNotifId] = useState<number | null>(null);
  const [tabValue, setTabValue] = useState(0);

  const handleMenuOpen = (event: React.MouseEvent<HTMLElement>, notifId: number) => {
    event.stopPropagation();
    setAnchorEl(event.currentTarget);
    setSelectedNotifId(notifId);
  };

  const handleMenuClose = () => { setAnchorEl(null); setSelectedNotifId(null); };

  const handleMarkAsRead = async () => {
    if (selectedNotifId) await markAsRead(selectedNotifId);
    handleMenuClose();
  };

  const handleDismiss = async () => {
    if (selectedNotifId) await dismiss(selectedNotifId);
    handleMenuClose();
  };

  const filteredNotifications = tabValue === 0
    ? notifications.filter(n => !n.is_read)
    : notifications;

  return (
    <Card title="Notifications">
      <Stack>
        <Inline>
          <Button variant="outlined" size="small" onClick={() => refresh()}>Refresh</Button>
          <Button variant="outlined" size="small" onClick={markAllAsRead}>Mark All Read</Button>
          <Button variant="outlined" size="small" color="warning" onClick={dismissAll}>Dismiss All</Button>
        </Inline>
        <Tabs value={tabValue} onChange={(_, v) => setTabValue(v)}>
          <Tab label={`Unread (${notifications.filter(n => !n.is_read).length})`} />
          <Tab label={`All (${notifications.length})`} />
        </Tabs>
        {loading ? (
          <CascadeRowsLoader />
        ) : filteredNotifications.length === 0 ? (
          <EmptyState
            icon={<CheckCircleIcon fontSize="large" />}
            title={tabValue === 0 ? 'No unread notifications' : 'No notifications'}
            size="sm"
          />
        ) : (
          <List disablePadding data-ui="notification-list">
            {filteredNotifications.map((notif, index) => (
              <ListItem
                key={notif.notification_id}
                alignItems="flex-start"
                disableGutters
                divider={index < filteredNotifications.length - 1}
                secondaryAction={
                  <IconButton
                    edge="end"
                    size="small"
                    aria-label={`Actions for ${notif.notification!.title}`}
                    onClick={(e) => handleMenuOpen(e, notif.notification_id!)}
                  >
                    <MoreVertIcon />
                  </IconButton>
                }
              >
                <ListItemIcon>{getSeverityIcon(notif.notification!.severity!)}</ListItemIcon>
                <ListItemText
                  disableTypography
                  primary={
                    <Inline>
                      <Typography variant={notif.is_read ? 'body' : 'sectionTitle'} component="p">{notif.notification!.title}</Typography>
                      <Chip label={getCategoryLabel(notif.notification!.category!)} size="small" color={getSeverityColor(notif.notification!.severity!)} variant="outlined" />
                      {!notif.is_read && <Chip label="New" size="small" color="primary" />}
                    </Inline>
                  }
                  secondary={
                    <>
                      <Typography variant="bodySmall" component="p" sx={{ color: 'text.secondary' }}>{notif.notification!.message}</Typography>
                      <Typography variant="caption" sx={{ color: 'text.disabled' }}>{formatDate(notif.notification!.created_at!)}</Typography>
                    </>
                  }
                />
              </ListItem>
            ))}
          </List>
        )}
      </Stack>
      <Menu anchorEl={anchorEl} open={Boolean(anchorEl)} onClose={handleMenuClose}>
        {[
          <MenuItem key="mark-read" onClick={handleMarkAsRead}>Mark as Read</MenuItem>,
          <MenuItem key="dismiss" onClick={handleDismiss}>Dismiss</MenuItem>
        ]}
      </Menu>
    </Card>
  );
}
