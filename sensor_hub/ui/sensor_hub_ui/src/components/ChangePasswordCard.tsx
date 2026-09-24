import React, { useState } from 'react';
import { apiClient } from '../gen/client';
import { useNavigate } from 'react-router';
import { TextField, Button, Alert, CircularProgress } from '@mui/material';
import Card from '../ui/Card';
import Inline from '../ui/Inline';
import Stack from '../ui/Stack';

function extractErrorMessage(err: unknown): string | null {
  if (!err) return null;
  if (typeof err === 'string') return err;
  if (typeof err === 'object' && err !== null && 'message' in err) {
    const e = err as { message?: unknown };
    if (typeof e.message === 'string') return e.message;
    return String(e.message);
  }
  return String(err);
}

export default function ChangePasswordCard() {
  const [newPassword, setNewPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (newPassword !== confirm) {
      setError('Passwords do not match');
      return;
    }
    setLoading(true);
    try {
      await apiClient.PUT('/users/password', { body: { new_password: newPassword } as never });
      navigate('/');
    } catch (err: unknown) {
      const message = extractErrorMessage(err) || 'Failed to change password';
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card title="Change password">
      <form onSubmit={submit}>
        <Stack>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField label="New password" type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} fullWidth disabled={loading} required />
          <TextField label="Confirm password" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} fullWidth disabled={loading} required />
          <Inline>
            <Button type="submit" variant="contained" disabled={loading} startIcon={loading ? <CircularProgress color="inherit" size={18} /> : undefined}>
              {loading ? 'Saving...' : 'Change password'}
            </Button>
          </Inline>
        </Stack>
      </form>
    </Card>
  );
}
