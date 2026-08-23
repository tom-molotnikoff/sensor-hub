import { useState, useEffect } from 'react';
import { Typography, Switch, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, Alert } from '@mui/material';
import LayoutCard from '../tools/LayoutCard';
import { useNotifications } from '../providers/NotificationContext';
import type { NotificationCategory, ChannelPreference } from '../gen/aliases';
import {TypographyH2} from "../tools/Typography.tsx";

interface CategoryConfig {
  category: NotificationCategory;
  label: string;
  description: string;
}

const CATEGORIES: CategoryConfig[] = [
  { category: 'threshold_alert', label: 'Threshold Alerts', description: 'Notifications when sensor readings exceed configured thresholds' },
  { category: 'user_management', label: 'User Management', description: 'Notifications about user creation, deletion, and role changes' },
  { category: 'config_change', label: 'Configuration Changes', description: 'Notifications when sensors are added, updated, or removed' },
];

export default function NotificationPreferencesCard() {
  const { preferences, updatePreference } = useNotifications();
  const [localPrefs, setLocalPrefs] = useState<Record<NotificationCategory, ChannelPreference>>({} as Record<NotificationCategory, ChannelPreference>);
  const [saving, setSaving] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const prefMap: Record<NotificationCategory, ChannelPreference> = {} as Record<NotificationCategory, ChannelPreference>;
    CATEGORIES.forEach(({ category }) => {
      const existing = preferences.find(p => p.category === category);
      prefMap[category] = existing || { category, email_enabled: true, inapp_enabled: true };
    });
    void Promise.resolve().then(() => setLocalPrefs(prefMap));
  }, [preferences]);

  const handleToggle = async (category: NotificationCategory, channel: 'email' | 'inapp', value: boolean) => {
    const currentPref = localPrefs[category];
    const newPref: ChannelPreference = {
      ...currentPref,
      [channel === 'email' ? 'email_enabled' : 'inapp_enabled']: value,
    };
    setLocalPrefs(prev => ({ ...prev, [category]: newPref }));
    setSaving(`${category}-${channel}`);
    setError(null);
    try {
      await updatePreference(newPref);
    } catch (err) {
      setError(`Failed to save preference: ${String(err)}`);
      setLocalPrefs(prev => ({ ...prev, [category]: currentPref }));
    } finally {
      setSaving(null);
    }
  };

  return (
    <LayoutCard variant="secondary" changes={{ alignItems: 'stretch', height: '100%', width: '100%' }}>
      <TypographyH2>Notification Preferences</TypographyH2>
      <Typography
        variant="body2"
        sx={{
          color: "text.secondary",
          mb: 3
        }}>
        Configure which notifications you receive via email and in-app notifications.
      </Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>{error}</Alert>}
      <TableContainer component={Paper} variant="outlined">
        <Table>
          <TableHead>
            <TableRow>
              <TableCell><strong>Category</strong></TableCell>
              <TableCell align="center"><strong>Email</strong></TableCell>
              <TableCell align="center"><strong>In-App</strong></TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {CATEGORIES.map(({ category, label, description }) => {
              const pref = localPrefs[category];
              if (!pref) return null;
              return (
                <TableRow key={category}>
                  <TableCell>
                    <Typography variant="subtitle2">{label}</Typography>
                    <Typography variant="caption" sx={{
                      color: "text.secondary"
                    }}>{description}</Typography>
                  </TableCell>
                  <TableCell align="center">
                    <Switch checked={pref.email_enabled} onChange={(e) => handleToggle(category, 'email', e.target.checked)} disabled={saving === `${category}-email`} />
                  </TableCell>
                  <TableCell align="center">
                    <Switch checked={pref.inapp_enabled} onChange={(e) => handleToggle(category, 'inapp', e.target.checked)} disabled={saving === `${category}-inapp`} />
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>
    </LayoutCard>
  );
}
