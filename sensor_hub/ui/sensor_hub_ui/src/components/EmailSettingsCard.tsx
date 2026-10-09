import { useCallback, useEffect, useState } from 'react';
import {
  Alert, Button, Chip, CircularProgress, FormControl, InputAdornment, InputLabel, MenuItem, Select, TextField, Tooltip, Typography,
} from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import { apiClient } from '../gen/client';
import { unwrap } from '../api/unwrap';
import type { EmailSettings } from '../gen/aliases';
import { logger } from '../tools/logger';
import { NEEDS_REENTRY_LABEL, NEEDS_REENTRY_TEXT, needsReentry } from '../tools/secretStatus';
import Card from '../ui/Card';
import Inline from '../ui/Inline';
import Stack from '../ui/Stack';

type Security = EmailSettings['security'];

const SECURITY_LABELS: Record<Security, string> = {
  starttls: 'STARTTLS',
  implicit_tls: 'Implicit TLS',
  none: 'None (unencrypted)',
};

// The hub sends nothing until a host is set and a password it can decrypt is stored.
function readyToSend(settings: EmailSettings): boolean {
  return settings.host.trim() !== '' && settings.password_status === 'set';
}

function errorMessage(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback;
}

// The SMTP server alert and notification emails go through. The password is
// write-only: the hub never sends it back. A stored password shows as an empty
// field reading "unchanged". Leaving it empty keeps it, typing replaces it, and
// Clear sends an empty password, which removes it.
export default function EmailSettingsCard() {
  const [settings, setSettings] = useState<EmailSettings | null>(null);
  const [host, setHost] = useState('');
  const [port, setPort] = useState(587);
  const [security, setSecurity] = useState<Security>('starttls');
  const [username, setUsername] = useState('');
  const [fromAddress, setFromAddress] = useState('');
  const [password, setPassword] = useState('');
  const [clearPassword, setClearPassword] = useState(false);
  const [saving, setSaving] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Seeds the form from the saved settings. The password field always starts empty.
  const showSaved = useCallback((saved: EmailSettings) => {
    setSettings(saved);
    setHost(saved.host);
    setPort(saved.port);
    setSecurity(saved.security);
    setUsername(saved.username);
    setFromAddress(saved.from_address);
    setPassword('');
    setClearPassword(false);
  }, []);

  const load = useCallback(
    () => unwrap(apiClient.GET('/email/smtp'))
      .then(showSaved)
      .catch((e: unknown) => {
        setError(errorMessage(e, 'Failed to load email settings'));
        logger.error('Failed to load email settings', e);
      }),
    [showSaved],
  );

  useEffect(() => { void load(); }, [load]);

  const passwordStored = !!settings && settings.password_status !== undefined && settings.password_status !== 'unset';

  const passwordField = (): Pick<EmailSettings, 'password'> => {
    if (password) return { password };
    if (clearPassword) return { password: '' };
    return {};
  };

  const handleSave = async () => {
    setError(null);
    setSuccess(null);
    setSaving(true);
    const body: EmailSettings = {
      host, port, security, username, from_address: fromAddress,
      ...passwordField(),
    };
    try {
      showSaved(await unwrap(apiClient.PUT('/email/smtp', { body })));
      setSuccess('Email settings saved.');
    } catch (e: unknown) {
      setError(errorMessage(e, 'Failed to save email settings'));
      logger.error('Failed to save email settings', e);
    } finally {
      setSaving(false);
    }
  };

  // The test goes through the saved settings, and its outcome is recorded as a
  // send, so the settings are reloaded afterwards to show it.
  const handleSendTest = async () => {
    setError(null);
    setSuccess(null);
    setSending(true);
    try {
      const { message } = await unwrap(apiClient.POST('/email/smtp/test'));
      setSuccess(message);
    } catch (e: unknown) {
      setError(errorMessage(e, 'Failed to send the test email'));
      logger.error('Failed to send the test email', e);
    } finally {
      setSending(false);
      await load();
    }
  };

  let passwordHelper = '';
  if (clearPassword && !password) passwordHelper = 'The stored password will be removed.';
  else if (settings && needsReentry(settings) && !password) passwordHelper = NEEDS_REENTRY_TEXT;
  else if (passwordStored && !password) passwordHelper = 'Leave empty to keep the stored password.';

  const busy = saving || sending;
  const ready = !!settings && readyToSend(settings);

  return (
    <Card title="Email">
      {!settings ? (
        error ? <Alert severity="error">{error}</Alert> : <Inline><CircularProgress /></Inline>
      ) : (
        <Stack>
          <Typography variant="sectionTitle">Status</Typography>
          <Inline>
            <Chip
              icon={ready ? <CheckCircleIcon /> : <ErrorIcon />}
              label={ready ? 'Ready to send' : 'Not ready to send'}
              color={ready ? 'success' : 'default'}
              variant="outlined"
            />
            {needsReentry(settings) && (
              <Tooltip title={NEEDS_REENTRY_TEXT}>
                <Chip label={NEEDS_REENTRY_LABEL} color="error" />
              </Tooltip>
            )}
          </Inline>
          {!ready && (
            <Typography variant="body2" sx={{ color: 'text.secondary' }}>
              Alert and notification emails are sent once a host is set and a password is stored.
            </Typography>
          )}
          <Typography variant="body2" sx={{ color: 'text.secondary' }}>
            Last sent: {settings.last_sent_at ? new Date(settings.last_sent_at).toLocaleString() : 'Never'}
          </Typography>
          {settings.last_error && <Alert severity="warning">Last error: {settings.last_error}</Alert>}

          <Typography variant="sectionTitle">SMTP server</Typography>
          <TextField fullWidth label="Host" value={host} onChange={e => setHost(e.target.value)} required />
          <TextField
            fullWidth
            label="Port"
            type="number"
            value={port}
            onChange={e => setPort(Number(e.target.value))}
            helperText="Usually 587 for STARTTLS and 465 for implicit TLS."
          />
          <FormControl fullWidth>
            <InputLabel id="email-security-label">Security</InputLabel>
            <Select
              labelId="email-security-label"
              value={security}
              label="Security"
              onChange={e => setSecurity(e.target.value as Security)}
            >
              {(Object.keys(SECURITY_LABELS) as Security[]).map((mode) => (
                <MenuItem key={mode} value={mode}>{SECURITY_LABELS[mode]}</MenuItem>
              ))}
            </Select>
          </FormControl>
          {security === 'none' && (
            <Alert severity="warning">
              With no security, the password and every message cross the network unencrypted.
            </Alert>
          )}
          <TextField fullWidth label="Username" value={username} onChange={e => setUsername(e.target.value)}
            helperText="Leave empty if the server does not need a login." />
          <TextField
            fullWidth
            label="Password"
            type="password"
            value={password}
            placeholder={passwordStored && !clearPassword ? 'unchanged' : undefined}
            onChange={e => { setPassword(e.target.value); setClearPassword(false); }}
            helperText={passwordHelper}
            slotProps={{
              inputLabel: passwordStored ? { shrink: true } : undefined,
              input: passwordStored ? {
                endAdornment: (
                  <InputAdornment position="end">
                    <Button size="small" onClick={() => { setPassword(''); setClearPassword(true); }} disabled={clearPassword && !password}>
                      Clear
                    </Button>
                  </InputAdornment>
                ),
              } : undefined,
            }}
          />
          <TextField fullWidth label="From address" value={fromAddress} onChange={e => setFromAddress(e.target.value)}
            placeholder="alerts@example.com" helperText="The address emails are sent from." />

          {error && <Alert severity="error" onClose={() => setError(null)}>{error}</Alert>}
          {success && <Alert severity="success" onClose={() => setSuccess(null)}>{success}</Alert>}

          <Inline>
            <Button variant="contained" onClick={handleSave} disabled={busy}>
              {saving ? 'Saving...' : 'Save'}
            </Button>
            <Tooltip title="Sends a test email to your own address through the saved settings.">
              <span>
                <Button variant="outlined" onClick={handleSendTest} disabled={busy}>
                  {sending ? 'Sending...' : 'Send test email'}
                </Button>
              </span>
            </Tooltip>
          </Inline>
        </Stack>
      )}
    </Card>
  );
}
