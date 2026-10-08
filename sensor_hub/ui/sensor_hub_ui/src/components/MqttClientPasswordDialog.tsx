import { useState } from 'react';
import {
  Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle,
  IconButton, InputAdornment, TextField, Tooltip,
} from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import type { MQTTClientCreated } from '../gen/aliases';
import Stack from '../ui/Stack';

interface Props {
  client: MQTTClientCreated | null;
  onClose: () => void;
}

/**
 * Shows a newly generated MQTT client password, once. The dialog has no
 * onClose, so Escape and clicking outside do nothing: only Close dismisses it.
 */
export default function MqttClientPasswordDialog({ client, onClose }: Props) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(client?.password ?? '');
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleClose = () => {
    setCopied(false);
    onClose();
  };

  return (
    <Dialog open={client !== null} maxWidth="sm" fullWidth>
      <DialogTitle>Password for {client?.name}</DialogTitle>
      <DialogContent>
        <Stack>
          <Alert severity="warning">This password is shown once. Put it in your device's MQTT config now.</Alert>
          <TextField
            fullWidth
            label="Password"
            value={client?.password ?? ''}
            slotProps={{
              input: {
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <Tooltip title={copied ? 'Copied!' : 'Copy to clipboard'}>
                      <IconButton onClick={handleCopy} edge="end" aria-label="Copy password">
                        <ContentCopyIcon />
                      </IconButton>
                    </Tooltip>
                  </InputAdornment>
                ),
              },
            }}
            sx={{ fontFamily: 'monospace' }}
          />
          <TextField fullWidth label="Username" value={client?.name ?? ''} slotProps={{ input: { readOnly: true } }} />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button variant="contained" onClick={handleClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
