import { useEffect, useState } from 'react';
import { Button } from '@mui/material';
import DeleteIcon from '@mui/icons-material/Delete';
import { apiClient } from '../gen/client';
import { logger } from '../tools/logger';
import Card from '../ui/Card';
import DataTable, { type RowAction } from '../ui/DataTable';

type Session = { id: number; created_at: string; expires_at: string; last_accessed_at: string; ip_address: string; user_agent: string; current?: boolean };

const getShortDeviceInfo = (userAgent: string): string => {
  if (!userAgent) return 'Unknown';
  if (userAgent.includes('iPhone')) return 'iPhone';
  if (userAgent.includes('iPad')) return 'iPad';
  if (userAgent.includes('Android')) return 'Android';
  if (userAgent.includes('Windows')) return 'Windows';
  if (userAgent.includes('Mac')) return 'Mac';
  if (userAgent.includes('Linux')) return 'Linux';
  return userAgent.substring(0, 20) + '...';
};

const formatTime = (value: string) => (value ? new Date(value).toLocaleString() : '');

export default function SessionsCard() {
  const [sessions, setSessions] = useState<Session[]>([]);

  const load = () =>
    apiClient.GET('/auth/sessions')
      .then(({ data: s }) => setSessions((s as Session[] | null) ?? []))
      .catch((e) => logger.error(e));

  useEffect(() => { void load(); }, []);

  const revoke = async (id: number) => {
    try {
      await apiClient.DELETE('/auth/sessions/{id}', { params: { path: { id } } });
      await load();
    } catch (e) { logger.error(e); }
  };

  const rowActions = (session: Session): RowAction[] => [
    {
      label: 'Revoke',
      icon: <DeleteIcon fontSize="small" />,
      color: 'error',
      disabled: session.current,
      onClick: () => { void revoke(session.id); },
    },
  ];

  return (
    <Card title="Active Sessions" actions={<Button variant="outlined" onClick={() => load()}>Refresh</Button>}>
      <DataTable
        rows={sessions}
        rowActions={rowActions}
        columns={[
          { field: 'id', headerName: 'ID', width: 80, compact: 'hidden' },
          {
            field: 'device',
            headerName: 'Device',
            width: 180,
            compact: 'title',
            valueGetter: (_value, row) => `${getShortDeviceInfo(row.user_agent)}${row.current ? ' (this session)' : ''}`,
          },
          { field: 'ip_address', headerName: 'IP', flex: 1, minWidth: 120, compact: 'meta' },
          { field: 'user_agent', headerName: 'User Agent', flex: 2, minWidth: 200, compact: 'hidden' },
          { field: 'created_at', headerName: 'Created', width: 180, compact: 'hidden', valueFormatter: formatTime },
          { field: 'last_accessed_at', headerName: 'Last Accessed', width: 180, compact: 'meta', valueFormatter: formatTime },
          { field: 'expires_at', headerName: 'Expires', width: 180, compact: 'hidden', valueFormatter: formatTime },
        ]}
      />
    </Card>
  );
}
