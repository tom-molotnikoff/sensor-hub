import { useState } from 'react';
import {
  Alert,
  Button,
  Chip,
  type SnackbarCloseReason,
  Snackbar,
  Menu,
  MenuItem,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import VpnKeyOffIcon from '@mui/icons-material/VpnKeyOff';
import Card from '../ui/Card';
import DataTable from '../ui/DataTable';
import EmptyState from '../ui/EmptyState';
import CreateApiKeyDialog from './CreateApiKeyDialog';
import { apiClient } from '../gen/client';
import type { ApiKey } from '../gen/aliases';

interface ApiKeysCardProps {
  apiKeys: ApiKey[];
  loaded: boolean;
  onRefresh: () => Promise<void>;
}

type ApiKeyRow = {
  id: number;
  name: string;
  key_prefix: string;
  revoked: boolean;
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
};

function formatDate(dateStr: string | null): string {
  if (!dateStr) return '—';
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

type KeyStatus = 'Revoked' | 'Expired' | 'Active';

function keyStatus(row: ApiKeyRow): KeyStatus {
  if (row.revoked) return 'Revoked';
  if (row.expires_at && new Date(row.expires_at) < new Date()) return 'Expired';
  return 'Active';
}

const statusStyles = {
  Revoked: { chip: 'error', pill: 'bad' },
  Expired: { chip: 'warning', pill: 'warn' },
  Active: { chip: 'success', pill: 'ok' },
} as const;

export default function ApiKeysCard({ apiKeys, loaded, onRefresh }: ApiKeysCardProps) {
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<ApiKeyRow | null>(null);
  const [confirmAction, setConfirmAction] = useState<'revoke' | 'delete' | null>(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [alertSeverity, setAlertSeverity] = useState<'success' | 'error'>('success');
  const [alertMessage, setAlertMessage] = useState('');

  const handleSnackbarClose = (
    _event: React.SyntheticEvent | Event,
    reason?: SnackbarCloseReason,
  ) => {
    if (reason === 'clickaway') return;
    setSnackbarOpen(false);
  };

  const showAlert = (severity: 'success' | 'error', message: string) => {
    setAlertSeverity(severity);
    setAlertMessage(message);
    setSnackbarOpen(true);
  };

  const handleRowClick = (row: ApiKeyRow, anchor: HTMLElement) => {
    setSelectedRow(row);
    setMenuAnchorEl(anchor);
  };

  const handleMenuClose = () => {
    setMenuAnchorEl(null);
  };

  const handleRevoke = async () => {
    setConfirmAction(null);
    if (!selectedRow) return;
    try {
      await apiClient.POST('/api-keys/{id}/revoke', { params: { path: { id: selectedRow.id } } });
      showAlert('success', 'API key revoked');
      await onRefresh();
    } catch (err: unknown) {
      showAlert('error', err instanceof Error ? err.message : 'Failed to revoke key');
    }
    setSelectedRow(null);
  };

  const handleDelete = async () => {
    setConfirmAction(null);
    if (!selectedRow) return;
    try {
      await apiClient.DELETE('/api-keys/{id}', { params: { path: { id: selectedRow.id } } });
      showAlert('success', 'API key deleted');
      await onRefresh();
    } catch (err: unknown) {
      showAlert('error', err instanceof Error ? err.message : 'Failed to delete key');
    }
    setSelectedRow(null);
  };

  const rows: ApiKeyRow[] = apiKeys.filter(k => k.id !== undefined).map((key) => ({
    id: key.id!,
    name: key.name ?? '',
    key_prefix: key.key_prefix ?? '',
    revoked: key.revoked ?? false,
    expires_at: key.expires_at ?? null,
    last_used_at: key.last_used_at ?? null,
    created_at: key.created_at ?? '',
  }));

  return (
    <Card
      title="API Keys"
      actions={
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => setCreateDialogOpen(true)}>
          Create API Key
        </Button>
      }
    >
      {loaded && rows.length === 0 ? (
        <EmptyState
          icon={<VpnKeyOffIcon fontSize="large" />}
          title="No API keys"
          description="Create an API key to use the CLI tool or integrate with LLM assistants."
          actionLabel="Create your first API key"
          onAction={() => setCreateDialogOpen(true)}
        />
      ) : (
        <>
          <DataTable
            rows={rows}
            loading={!loaded}
            onRowClick={handleRowClick}
            columns={[
              { field: 'name', headerName: 'Name', flex: 1.5, minWidth: 120, compact: 'title' },
              { field: 'key_prefix', headerName: 'Key Prefix', flex: 1, minWidth: 100, compact: 'meta' },
              {
                field: 'status',
                headerName: 'Status',
                flex: 0.8,
                minWidth: 100,
                sortable: false,
                compact: 'status',
                statusOf: (row) => statusStyles[keyStatus(row)].pill,
                valueGetter: (_value, row) => keyStatus(row),
                renderCell: ({ row }) => {
                  const status = keyStatus(row);
                  return <Chip label={status} color={statusStyles[status].chip} size="small" />;
                },
              },
              {
                field: 'created_at',
                headerName: 'Created',
                flex: 1.2,
                minWidth: 140,
                compact: 'hidden',
                valueFormatter: (value: string) => formatDate(value),
              },
              {
                field: 'expires_at',
                headerName: 'Expires',
                flex: 1.2,
                minWidth: 140,
                compact: 'meta',
                valueFormatter: (value: string | null) => value ? `Expires ${formatDate(value)}` : 'Never expires',
              },
              {
                field: 'last_used_at',
                headerName: 'Last Used',
                flex: 1.2,
                minWidth: 140,
                compact: 'hidden',
                valueFormatter: (value: string | null) => value ? formatDate(value) : 'Never',
              },
            ]}
          />
          <Menu
            anchorEl={menuAnchorEl}
            open={Boolean(menuAnchorEl)}
            onClose={handleMenuClose}
          >
            {selectedRow && !selectedRow.revoked && (
              <MenuItem onClick={() => { handleMenuClose(); setConfirmAction('revoke'); }}>
                Revoke
              </MenuItem>
            )}
            <MenuItem onClick={() => { handleMenuClose(); setConfirmAction('delete'); }}>
              Delete
            </MenuItem>
          </Menu>
        </>
      )}

      <CreateApiKeyDialog
        open={createDialogOpen}
        onClose={() => setCreateDialogOpen(false)}
        onCreated={onRefresh}
      />

      <Dialog
        open={confirmAction !== null}
        onClose={() => { setConfirmAction(null); setSelectedRow(null); }}
      >
        <DialogTitle>
          {confirmAction === 'revoke' ? 'Revoke API Key' : 'Delete API Key'}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {confirmAction === 'revoke'
              ? `Are you sure you want to revoke "${selectedRow?.name}"? This key will no longer be usable.`
              : `Are you sure you want to permanently delete "${selectedRow?.name}"? This cannot be undone.`
            }
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setConfirmAction(null); setSelectedRow(null); }}>Cancel</Button>
          <Button
            variant="contained"
            color="error"
            onClick={confirmAction === 'revoke' ? handleRevoke : handleDelete}
          >
            {confirmAction === 'revoke' ? 'Revoke' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={snackbarOpen}
        autoHideDuration={3000}
        onClose={handleSnackbarClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert onClose={handleSnackbarClose} severity={alertSeverity}>
          {alertMessage}
        </Alert>
      </Snackbar>
    </Card>
  );
}
