import { useEffect, useState, useCallback } from 'react';
import { Button, Box, Typography, Alert, CircularProgress, Chip, Dialog, DialogTitle, DialogContent, DialogActions, TextField } from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import RefreshIcon from '@mui/icons-material/Refresh';
import SyncIcon from '@mui/icons-material/Sync';
import { apiClient } from '../gen/client';
import type { OAuthAuthorizeResponse } from '../gen/aliases';

interface OAuthStatus {
  configured: boolean;
  needs_auth: boolean;
  token_valid: boolean;
  token_expiry?: string;
  refresher_active: boolean;
  last_refresh_at?: string;
  last_error?: string;
}
import LayoutCard from '../tools/LayoutCard';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { useIsMobile } from '../hooks/useMobile';
import {TypographyH2} from "../tools/Typography.tsx";

export default function OAuthConfigCard() {
  const [status, setStatus] = useState<OAuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [authorizing, setAuthorizing] = useState(false);
  const [reloading, setReloading] = useState(false);
  const [codeDialogOpen, setCodeDialogOpen] = useState(false);
  const [authCode, setAuthCode] = useState('');
  const [pendingState, setPendingState] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const { user } = useAuth();
  const isMobile = useIsMobile();

  const loadStatus = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const { data: s } = await apiClient.GET('/oauth/status');
      setStatus(s as unknown as OAuthStatus ?? null);
    } catch (err: unknown) {
      const e = err as { message?: string };
      setError(e.message || 'Failed to load OAuth status');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void Promise.resolve().then(loadStatus); }, [loadStatus]);

  const handleStartAuthorize = async () => {
    try {
      setAuthorizing(true);
      setError(null);
      const { data } = await apiClient.GET('/oauth/authorize');
      const oauthData = data as OAuthAuthorizeResponse;
      setPendingState(oauthData.state);
      window.open(oauthData.auth_url, '_blank', 'width=600,height=700');
      setCodeDialogOpen(true);
    } catch (err: unknown) {
      const e = err as { message?: string };
      setError(e.message || 'Failed to start authorization');
    } finally {
      setAuthorizing(false);
    }
  };

  const handleSubmitCode = async () => {
    if (!authCode.trim() || !pendingState) {
      setError('Please enter the authorization code');
      return;
    }
    try {
      setSubmitting(true);
      setError(null);
      await apiClient.POST('/oauth/submit-code', { body: { code: authCode.trim(), state: pendingState } as never });
      setSuccess('OAuth authorization successful! Token has been saved.');
      setCodeDialogOpen(false);
      setAuthCode('');
      setPendingState(null);
      await loadStatus();
    } catch (err: unknown) {
      const e = err as { message?: string };
      setError(e.message || 'Failed to exchange authorization code');
    } finally {
      setSubmitting(false);
    }
  };

  const handleReload = async () => {
    try {
      setReloading(true);
      setError(null);
      setSuccess(null);
      await apiClient.POST('/oauth/reload', {});
      setSuccess('OAuth configuration reloaded from disk.');
      await loadStatus();
    } catch (err: unknown) {
      const e = err as { message?: string };
      setError(e.message || 'Failed to reload OAuth configuration');
    } finally {
      setReloading(false);
    }
  };

  const handleCloseCodeDialog = () => {
    setCodeDialogOpen(false);
    setAuthCode('');
    setPendingState(null);
  };

  const canManage = !!user && hasPerm(user, 'manage_oauth');

  return (
    <>
      <LayoutCard variant="secondary" changes={{ alignItems: 'stretch', height: '100%', width: '100%' }}>
        <Box
          sx={{
            display: "flex",
            flexDirection: isMobile ? 'column' : 'row',
            alignItems: isMobile ? 'flex-start' : 'center',
            justifyContent: "space-between",
            gap: 2,
            mb: 2
          }}>
          <TypographyH2>OAuth Configuration</TypographyH2>
          <Box
            sx={{
              display: "flex",
              flexDirection: isMobile ? 'column' : 'row',
              gap: 1,
              width: isMobile ? '100%' : 'auto'
            }}>
            <Button variant="outlined" startIcon={<SyncIcon />} onClick={handleReload} disabled={loading || reloading || !canManage} title="Reload credentials.json from disk" fullWidth={isMobile}>
              {reloading ? 'Reloading...' : 'Reload Config'}
            </Button>
            <Button variant="outlined" startIcon={<RefreshIcon />} onClick={loadStatus} disabled={loading} fullWidth={isMobile}>
              Refresh
            </Button>
          </Box>
        </Box>

        {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>{error}</Alert>}
        {success && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>{success}</Alert>}

        {loading && !status ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}><CircularProgress /></Box>
        ) : status ? (
          <Box>
            <Typography variant="h6" gutterBottom>Status</Typography>
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 2, mb: 3 }}>
              <Chip icon={status.configured ? <CheckCircleIcon /> : <ErrorIcon />} label={status.configured ? 'Credentials Configured' : 'Not Configured'} color={status.configured ? 'success' : 'error'} variant="outlined" />
              {status.needs_auth && <Chip icon={<ErrorIcon />} label="Needs Authorization" color="warning" variant="outlined" />}
              {!status.needs_auth && <Chip icon={status.token_valid ? <CheckCircleIcon /> : <ErrorIcon />} label={status.token_valid ? 'Token Valid' : 'Token Invalid/Expired'} color={status.token_valid ? 'success' : 'warning'} variant="outlined" />}
              <Chip label={status.refresher_active ? 'Auto-refresh Active' : 'Auto-refresh Inactive'} color={status.refresher_active ? 'info' : 'default'} variant="outlined" />
            </Box>

            {status.token_expiry && <Typography variant="body2" gutterBottom sx={{
              color: "text.secondary"
            }}>Token Expiry: {new Date(status.token_expiry).toLocaleString()}</Typography>}
            {status.last_refresh_at && <Typography variant="body2" gutterBottom sx={{
              color: "text.secondary"
            }}>Last Refresh: {new Date(status.last_refresh_at).toLocaleString()}</Typography>}
            {status.last_error && <Alert severity="warning" sx={{ mt: 2, mb: 2 }}>Last Error: {status.last_error}</Alert>}

            <Box sx={{ mt: 4 }}>
              <Typography variant="h6" gutterBottom>{status.needs_auth ? 'Authorize Gmail Access' : 'Re-authorize Gmail Access'}</Typography>
              <Typography
                variant="body2"
                sx={{
                  color: "text.secondary",
                  mb: 2
                }}>
                {status.needs_auth
                  ? 'OAuth credentials are configured but no token exists. Click the button below to authorize access to Gmail for sending emails.'
                  : 'If your OAuth token has expired or you need to re-authorize, click the button below to start the Google authorization flow.'}
                {' '}This will open a new window where you can sign in with your Google account.
                After authorizing, Google will display an authorization code that you will need to copy and paste here.
              </Typography>
              <Button variant="contained" onClick={handleStartAuthorize} disabled={!canManage || authorizing || !status.configured}>
                {authorizing ? 'Opening...' : 'Authorize with Google'}
              </Button>
              {!status.configured && <Typography variant="body2" color="error" sx={{ mt: 1 }}>OAuth credentials file not found. Please configure credentials.json first.</Typography>}
            </Box>
          </Box>
        ) : null}
      </LayoutCard>
      <Dialog open={codeDialogOpen} onClose={handleCloseCodeDialog} maxWidth="sm" fullWidth>
        <DialogTitle>Enter Authorization Code</DialogTitle>
        <DialogContent>
          <Typography sx={{ mb: 2 }}>
            A new window has opened for Google authorization. After you sign in and grant access,
            Google will display an authorization code. Copy that code and paste it below.
          </Typography>
          <TextField autoFocus fullWidth label="Authorization Code" value={authCode} onChange={(e) => setAuthCode(e.target.value)} placeholder="Paste the authorization code here" disabled={submitting} sx={{ mt: 1 }} />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseCodeDialog} disabled={submitting}>Cancel</Button>
          <Button variant="contained" onClick={handleSubmitCode} disabled={submitting || !authCode.trim()}>
            {submitting ? 'Submitting...' : 'Submit Code'}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
