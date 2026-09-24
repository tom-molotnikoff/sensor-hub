import { useState } from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Alert,
  Typography,
  IconButton,
  InputAdornment,
  Tooltip,
} from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { apiClient } from '../gen/client';
import Stack from '../ui/Stack';

type CreateApiKeyResponse = { key?: string; message?: string };

interface CreateApiKeyDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => Promise<void>;
}

export default function CreateApiKeyDialog({ open, onClose, onCreated }: CreateApiKeyDialogProps) {
  const [name, setName] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [createdKey, setCreatedKey] = useState<CreateApiKeyResponse | null>(null);
  const [copied, setCopied] = useState(false);

  const resetForm = () => {
    setName('');
    setExpiresAt('');
    setError(null);
    setCreatedKey(null);
    setCopied(false);
  };

  const handleCreate = async () => {
    if (!name.trim()) {
      setError('Name is required');
      return;
    }
    setError(null);
    try {
      const req: { name: string; expires_at?: string } = { name: name.trim() };
      if (expiresAt) {
        req.expires_at = new Date(expiresAt).toISOString();
      }
      const { data: response } = await apiClient.POST('/api-keys', { body: req as never });
      setCreatedKey(response as CreateApiKeyResponse ?? null);
      await onCreated();
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'message' in err) {
        setError((err as { message: string }).message);
      } else {
        setError('Failed to create API key');
      }
    }
  };

  const handleCopy = async () => {
    if (createdKey) {
      await navigator.clipboard.writeText(createdKey.key ?? '');
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  return (
    <Dialog open={open} onClose={createdKey ? undefined : handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{createdKey ? 'API Key Created' : 'Create API Key'}</DialogTitle>
      <DialogContent>
        {createdKey ? (
          <Stack>
            <Alert severity="warning">
              Copy this key now — it will not be shown again.
            </Alert>
            <TextField
              fullWidth
              label="API Key"
              value={createdKey.key}
              slotProps={{
                input: {
                  readOnly: true,
                  endAdornment: (
                    <InputAdornment position="end">
                      <Tooltip title={copied ? 'Copied!' : 'Copy to clipboard'}>
                        <IconButton onClick={handleCopy} edge="end">
                          <ContentCopyIcon />
                        </IconButton>
                      </Tooltip>
                    </InputAdornment>
                  ),
                },
              }}
              sx={{ fontFamily: 'monospace' }}
            />
            <Typography variant="caption" sx={{ color: "text.secondary" }}>
              Store this key securely. You won't be able to see it again.
            </Typography>
          </Stack>
        ) : (
          <Stack>
            {error && <Alert severity="error">{error}</Alert>}
            <TextField
              fullWidth
              label="Key Name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. My CLI Key"
              autoFocus
            />
            <TextField
              fullWidth
              label="Expires At (optional)"
              type="datetime-local"
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
              slotProps={{
                inputLabel: { shrink: true },
              }}
              helperText="Leave empty for a key that never expires"
            />
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        {createdKey ? (
          <Button variant="contained" onClick={handleClose}>Done</Button>
        ) : (
          <>
            <Button onClick={handleClose}>Cancel</Button>
            <Button variant="contained" onClick={handleCreate}>Create</Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
