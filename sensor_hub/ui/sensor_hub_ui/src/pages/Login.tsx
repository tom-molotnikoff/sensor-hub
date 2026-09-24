import React, { useState } from 'react';
import { useNavigate } from 'react-router';
import { apiClient } from '../gen/client';
import type { LoginResponse } from '../gen/aliases';
import { setCsrfToken } from '../api/Csrf';
import { useAuth } from '../providers/AuthContext.tsx';
import { TextField, Button, Alert, CircularProgress } from '@mui/material';
import Card from '../ui/Card';
import Inline from '../ui/Inline';
import Stack from '../ui/Stack';
import StandalonePage from '../ui/StandalonePage';

export default function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const { refresh } = useAuth();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (loading) return;
    setLoading(true);
    setError(null);
    try {
      const { data, error } = await apiClient.POST('/auth/login', { body: { username, password } });
      if (error) throw { message: (error as { message?: string }).message || 'Login failed' };
      if (data?.csrf_token) setCsrfToken(data.csrf_token);
      const res: LoginResponse = data ?? {};
      try { await refresh(); } catch { /* ignore */ }
      if (res.must_change_password) {
        navigate('/account/change-password');
      } else {
        navigate('/');
      }
    } catch (err: unknown) {
      let msg = 'Login failed';
      if (err && typeof err === 'object' && 'message' in err) {
        const e = err as { message?: unknown };
        if (typeof e.message === 'string') msg = e.message;
      }
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <StandalonePage title="Sign in">
      <Card>
        <form onSubmit={submit}>
          <Stack>
            {error && <Alert severity="error">{error}</Alert>}
            <TextField
              label="Username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              fullWidth
              autoComplete="username"
              disabled={loading}
              required
            />
            <TextField
              label="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              fullWidth
              autoComplete="current-password"
              disabled={loading}
              required
            />
            <Inline>
              <Button
                type="submit"
                variant="contained"
                disabled={loading}
                startIcon={loading ? <CircularProgress color="inherit" size={18} /> : undefined}
              >
                {loading ? 'Signing in...' : 'Sign in'}
              </Button>
            </Inline>
          </Stack>
        </form>
      </Card>
    </StandalonePage>
  );
}
