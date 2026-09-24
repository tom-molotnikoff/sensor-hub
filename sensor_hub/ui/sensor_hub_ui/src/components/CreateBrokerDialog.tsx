import { useState } from 'react';
import {
  Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle,
  TextField, FormControlLabel, Switch, FormControl, InputLabel, Select, MenuItem,
} from '@mui/material';
import { apiClient } from '../gen/client';
import { logger } from '../tools/logger';
import Stack from '../ui/Stack';

type CreateBrokerPayload = {
  name: string;
  type: string;
  host: string;
  port: number;
  username?: string;
  password?: string;
  client_id?: string;
  enabled: boolean;
};

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: () => Promise<void>;
}

export default function CreateBrokerDialog({ open, onClose, onCreated }: Props) {
  const [name, setName] = useState('');
  const [type, setType] = useState('external');
  const [host, setHost] = useState('');
  const [port, setPort] = useState(1883);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [clientId, setClientId] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [error, setError] = useState('');

  const reset = () => {
    setName(''); setType('external'); setHost(''); setPort(1883);
    setUsername(''); setPassword(''); setClientId(''); setEnabled(true); setError('');
  };

  const handleCreate = async () => {
    setError('');
    const payload: CreateBrokerPayload = {
      name, type, host, port, enabled,
      ...(username && { username }),
      ...(password && { password }),
      ...(clientId && { client_id: clientId }),
    };
    try {
      await apiClient.POST('/mqtt/brokers', { body: payload as never });
      reset();
      onClose();
      await onCreated();
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || 'Failed to create broker';
      setError(msg);
      logger.error('Failed to create broker', e);
    }
  };

  const handleCancel = () => { reset(); onClose(); };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>Add MQTT Broker</DialogTitle>
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
          <TextField fullWidth label="Password" type="password" value={password} onChange={e => setPassword(e.target.value)} />
          <TextField fullWidth label="Client ID" value={clientId} onChange={e => setClientId(e.target.value)}
            helperText="Optional. Auto-generated if blank." />
          <FormControlLabel control={<Switch checked={enabled} onChange={e => setEnabled(e.target.checked)} />} label="Enabled" />
          {error && <Alert severity="error">{error}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel}>Cancel</Button>
        <Button variant="contained" onClick={handleCreate} disabled={!name}>Create</Button>
      </DialogActions>
    </Dialog>
  );
}
