import { useState } from 'react';
import { Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField } from '@mui/material';
import { apiClient } from '../gen/client';
import { unwrap } from '../api/unwrap';
import type { MQTTClientCreated } from '../gen/aliases';
import { logger } from '../tools/logger';
import Stack from '../ui/Stack';

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: (client: MQTTClientCreated) => Promise<void>;
}

export default function CreateMqttClientDialog({ open, onClose, onCreated }: Props) {
  const [name, setName] = useState('');
  const [topicPrefix, setTopicPrefix] = useState('');
  const [error, setError] = useState('');

  const reset = () => { setName(''); setTopicPrefix(''); setError(''); };

  const handleCreate = async () => {
    setError('');
    try {
      const created = await unwrap(apiClient.POST('/mqtt/clients', { body: { name, topic_prefix: topicPrefix } }));
      reset();
      onClose();
      await onCreated(created);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create MQTT client');
      logger.error('Failed to create MQTT client', e);
    }
  };

  const handleCancel = () => { reset(); onClose(); };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>Add MQTT Client</DialogTitle>
      <DialogContent>
        <Stack>
          <TextField fullWidth label="Name" value={name} onChange={e => setName(e.target.value)} required
            helperText="The MQTT username your device connects with." />
          <TextField fullWidth label="Topic prefix" value={topicPrefix} onChange={e => setTopicPrefix(e.target.value)} required
            placeholder="zigbee2mqtt/"
            helperText="The only topics this client may publish and subscribe to. Ends with /." />
          {error && <Alert severity="error">{error}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel}>Cancel</Button>
        <Button variant="contained" onClick={handleCreate} disabled={!name || !topicPrefix}>Create</Button>
      </DialogActions>
    </Dialog>
  );
}
