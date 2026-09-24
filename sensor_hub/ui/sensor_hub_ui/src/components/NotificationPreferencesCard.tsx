import { useState } from 'react';
import { Typography, Switch, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Alert } from '@mui/material';
import { useNotifications } from '../providers/NotificationContext';
import type { NotificationCategory, ChannelPreference } from '../gen/aliases';
import Card from '../ui/Card';
import Stack from '../ui/Stack';

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

function buildPrefMap(preferences: ChannelPreference[]): Record<NotificationCategory, ChannelPreference> {
  const prefMap = {} as Record<NotificationCategory, ChannelPreference>;
  CATEGORIES.forEach(({ category }) => {
    const existing = preferences.find(p => p.category === category);
    prefMap[category] = existing || { category, email_enabled: true, inapp_enabled: true };
  });
  return prefMap;
}

export default function NotificationPreferencesCard() {
  const { preferences, updatePreference } = useNotifications();
  const [localPrefs, setLocalPrefs] = useState<Record<NotificationCategory, ChannelPreference>>(() => buildPrefMap(preferences));
  const [saving, setSaving] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Re-sync the editable copy when the server preferences change (adjust-during-render).
  const [prevPreferences, setPrevPreferences] = useState(preferences);
  if (prevPreferences !== preferences) {
    setPrevPreferences(preferences);
    setLocalPrefs(buildPrefMap(preferences));
  }

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
    <Card title="Notification Preferences">
      <Stack>
        <Typography variant="body2" sx={{ color: "text.secondary" }}>
          Configure which notifications you receive via email and in-app notifications.
        </Typography>
        {error && <Alert severity="error" onClose={() => setError(null)}>{error}</Alert>}
        <TableContainer>
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
      </Stack>
    </Card>
  );
}
