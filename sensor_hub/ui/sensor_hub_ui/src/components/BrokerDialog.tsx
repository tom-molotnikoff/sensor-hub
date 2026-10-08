import { useState } from 'react';
import {
  Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle,
  TextField, FormControlLabel, Switch, FormControl, InputLabel, Select, MenuItem, InputAdornment,
} from '@mui/material';
import { apiClient } from '../gen/client';
import { unwrap } from '../api/unwrap';
import type { MQTTBroker } from '../gen/aliases';
import { logger } from '../tools/logger';
import Stack from '../ui/Stack';

type BrokerPayload = {
  name: string;
  type: string;
  host?: string;
  port?: number;
  username?: string;
  password?: string;
  client_id?: string;
  enabled: boolean;
};

interface Props {
  open: boolean;
  onClose: () => void;
  onSaved: () => Promise<void>;
  // The broker to edit; absent when adding one.
  broker?: MQTTBroker | null;
}

// The password is write-only: the hub never sends it back. A stored password
// shows as an empty field reading "unchanged". Leaving it empty keeps it,
// typing replaces it, and Clear sends an empty password, which removes it.
export default function BrokerDialog({ open, onClose, onSaved, broker }: Props) {
  const editing = !!broker;
  const [name, setName] = useState('');
  const [type, setType] = useState('external');
  const [host, setHost] = useState('');
  const [port, setPort] = useState(1883);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [clearPassword, setClearPassword] = useState(false);
  const [clientId, setClientId] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [error, setError] = useState('');

  // Re-seed the form whenever the dialog opens (adjust-during-render).
  const [prevOpen, setPrevOpen] = useState(false);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (open) {
      setName(broker?.name ?? '');
      setType(broker?.type ?? 'external');
      setHost(broker?.host ?? '');
      setPort(broker?.port ?? 1883);
      setUsername(broker?.username ?? '');
      setPassword('');
      setClearPassword(false);
      setClientId(broker?.client_id ?? '');
      setEnabled(broker?.enabled ?? true);
      setError('');
    }
  }

  const passwordStored = editing && broker?.password_status !== undefined && broker.password_status !== 'unset';

  const passwordField = (): Pick<BrokerPayload, 'password'> => {
    if (password) return { password };
    if (clearPassword) return { password: '' };
    return {};
  };

  const handleSave = async () => {
    setError('');
    const payload: BrokerPayload = {
      name, type, enabled,
      ...(type !== 'embedded' && { host, port }),
      ...(username && { username }),
      ...passwordField(),
      ...(clientId && { client_id: clientId }),
    };
    try {
      if (broker) {
        await unwrap(apiClient.PUT('/mqtt/brokers/{id}', { params: { path: { id: broker.id } }, body: payload as never }));
      } else {
        await unwrap(apiClient.POST('/mqtt/brokers', { body: payload as never }));
      }
      onClose();
      await onSaved();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Failed to save broker';
      setError(msg);
      logger.error('Failed to save broker', e);
    }
  };

  let passwordHelper = '';
  if (clearPassword && !password) passwordHelper = 'The stored password will be removed.';
  else if (passwordStored && !password) passwordHelper = 'Leave empty to keep the stored password.';

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{editing ? 'Edit MQTT Broker' : 'Add MQTT Broker'}</DialogTitle>
      <DialogContent>
        <Stack>
          <TextField fullWidth label="Name" value={name} onChange={e => setName(e.target.value)} required />
          <FormControl fullWidth>
            <InputLabel>Type</InputLabel>
            <Select value={type} label="Type" onChange={e => setType(e.target.value)}>
              <MenuItem value="external">External</MenuItem>
              <MenuItem value="embedded">Embedded</MenuItem>
            </Select>
          </FormControl>
          <TextField fullWidth label="Host" value={type === 'embedded' ? 'localhost' : host} onChange={e => setHost(e.target.value)}
            disabled={type === 'embedded'} helperText={type === 'embedded' ? 'Embedded brokers always use localhost' : ''} />
          <TextField fullWidth label="Port" type="number" value={port} onChange={e => setPort(Number(e.target.value))} />
          <TextField fullWidth label="Username" value={username} onChange={e => setUsername(e.target.value)} />
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
          <TextField fullWidth label="Client ID" value={clientId} onChange={e => setClientId(e.target.value)}
            helperText="Optional. Auto-generated if blank." />
          <FormControlLabel control={<Switch checked={enabled} onChange={e => setEnabled(e.target.checked)} />} label="Enabled" />
          {error && <Alert severity="error">{error}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={handleSave} disabled={!name}>{editing ? 'Save' : 'Create'}</Button>
      </DialogActions>
    </Dialog>
  );
}
