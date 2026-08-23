import React, { useEffect, useState, useCallback } from 'react';
import { useAuth } from './AuthContext';
import { NotificationContext, type NotificationContextValue } from './NotificationContext';
import type { UserNotification, ChannelPreference, Notification } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { WEBSOCKET_BASE } from '../environment/Environment';
import { logger } from '../tools/logger';

export default function NotificationProvider({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();
  const [notifications, setNotifications] = useState<UserNotification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [preferences, setPreferences] = useState<ChannelPreference[]>([]);
  const [fetched, setFetched] = useState(false);

  const hasPermission = user?.permissions?.includes('view_notifications');
  // Loading until the user is known; a user who cannot view notifications has nothing to load.
  const loading = user === undefined || (!!user && !!hasPermission && !fetched);

  const refresh = useCallback((): Promise<void> => {
    if (!user || !hasPermission) return Promise.resolve();
    return Promise.all([
      apiClient.GET('/notifications', { params: { query: { limit: 50, offset: 0, unread_only: false } } }),
      apiClient.GET('/notifications/unread-count'),
      apiClient.GET('/notifications/preferences'),
    ])
      .then(([notifsRes, countRes, prefsRes]) => {
        setNotifications(notifsRes.data ?? []);
        setUnreadCount(countRes.data?.count ?? 0);
        setPreferences(prefsRes.data ?? []);
      })
      .catch((err) => logger.error('Failed to load notifications:', err))
      .finally(() => setFetched(true));
  }, [user, hasPermission]);

  const markAsRead = useCallback(async (notificationId: number) => {
    await apiClient.POST('/notifications/{id}/read', { params: { path: { id: notificationId } } });
    setNotifications(prev =>
      prev.map(n =>
        n.notification_id === notificationId ? { ...n, is_read: true } : n
      )
    );
    setUnreadCount(prev => Math.max(0, prev - 1));
  }, []);

  const dismiss = useCallback(async (notificationId: number) => {
    await apiClient.POST('/notifications/{id}/dismiss', { params: { path: { id: notificationId } } });
    setNotifications(prev => prev.filter(n => n.notification_id !== notificationId));
    setUnreadCount(prev => Math.max(0, prev - 1));
  }, []);

  const markAllAsRead = useCallback(async () => {
    await apiClient.POST('/notifications/bulk/read', {});
    setNotifications(prev => prev.map(n => ({ ...n, is_read: true })));
    setUnreadCount(0);
  }, []);

  const dismissAll = useCallback(async () => {
    await apiClient.POST('/notifications/bulk/dismiss', {});
    setNotifications([]);
    setUnreadCount(0);
  }, []);

  const updatePreference = useCallback(async (pref: ChannelPreference) => {
    await apiClient.POST('/notifications/preferences', { body: pref as never });
    setPreferences(prev => {
      const idx = prev.findIndex(p => p.category === pref.category);
      if (idx >= 0) {
        const updated = [...prev];
        updated[idx] = pref;
        return updated;
      }
      return [...prev, pref];
    });
  }, []);

  // Initial load
  useEffect(() => {
    if (!user || !hasPermission) return;
    void refresh();
  }, [user, hasPermission, refresh]);

  // WebSocket subscription for real-time updates
  useEffect(() => {
    if (!user || !hasPermission) return;

    const ws = new WebSocket(`${WEBSOCKET_BASE}/notifications/ws`);
    
    ws.onmessage = (event) => {
      if (!event.data || event.data === 'null') return;
      try {
        const newNotif = JSON.parse(event.data) as Notification;
        const userNotif: UserNotification = {
          id: 0,
          user_id: user.id,
          notification_id: newNotif.id,
          is_read: false,
          is_dismissed: false,
          notification: newNotif,
        };
        setNotifications(prev => [userNotif, ...prev]);
        setUnreadCount(prev => prev + 1);
      } catch (err) {
        logger.error('Failed to parse notification WebSocket message:', err);
      }
    };

    ws.onerror = (err) => {
      logger.error('Notifications WebSocket error:', err);
    };

    ws.onclose = (event) => {
      logger.debug('Notifications WebSocket closed', event);
    };

    return () => ws.close();
  }, [user, hasPermission]);

  const value: NotificationContextValue = {
    notifications,
    unreadCount,
    preferences,
    loading,
    refresh,
    markAsRead,
    dismiss,
    markAllAsRead,
    dismissAll,
    updatePreference,
  };

  return (
    <NotificationContext.Provider value={value}>
      {children}
    </NotificationContext.Provider>
  );
}
