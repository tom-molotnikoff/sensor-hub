import { useNavigate, Link as RouterLink } from 'react-router';
import { Alert, Link } from '@mui/material';
import type { CommandHistoryEntry } from '../gen/aliases';
import { useSensorCommands } from '../hooks/useSensorCommands';
import Card from '../ui/Card';
import DataTable, { type DataTableColumn } from '../ui/DataTable';
import StatusPill from '../ui/dataTable/StatusPill';
import EmptyState from '../ui/EmptyState';
import type { StatusKey } from '../ui/theme';
import { useTier } from '../ui/tiers';

const commandStatus: Record<CommandHistoryEntry['status'], { label: string; key: StatusKey }> = {
  sent: { label: 'Sent', key: 'info' },
  acknowledged: { label: 'Acknowledged', key: 'ok' },
  timed_out: { label: 'Timed out', key: 'warn' },
  failed: { label: 'Failed', key: 'bad' },
};

const sentBy = (entry: CommandHistoryEntry) => entry.automation?.name ?? entry.user?.username ?? '-';

const columns = [
  {
    field: 'command',
    headerName: 'Command',
    flex: 1,
    minWidth: 160,
    compact: 'title',
    valueGetter: (_, row) => `${row.property} = ${row.value}`,
  },
  {
    field: 'sent_at',
    headerName: 'Sent',
    width: 190,
    compact: 'meta',
    valueFormatter: (value: string) => new Date(value).toLocaleString(),
  },
  {
    field: 'sent_by',
    headerName: 'Sent by',
    flex: 1,
    minWidth: 160,
    compact: 'meta',
    valueGetter: (_, row) => sentBy(row),
    renderCell: ({ row }) =>
      row.automation ? (
        <Link component={RouterLink} to={`/automations/${row.automation.id}`} onClick={(event) => event.stopPropagation()}>
          {row.automation.name}
        </Link>
      ) : (
        sentBy(row)
      ),
  },
  {
    field: 'status',
    headerName: 'Status',
    width: 150,
    compact: 'status',
    statusOf: (row) => commandStatus[row.status].key,
    valueFormatter: (status: CommandHistoryEntry['status']) => commandStatus[status].label,
    renderCell: ({ row }) => <StatusPill status={commandStatus[row.status].key} label={commandStatus[row.status].label} />,
  },
] as const satisfies readonly DataTableColumn<CommandHistoryEntry>[];

export default function CommandHistoryCard({ sensorId }: { sensorId: number }) {
  const navigate = useNavigate();
  const compact = useTier() === 'compact';
  const { data: commands, isPending, error } = useSensorCommands(sensorId);

  return (
    <Card title="Command history">
      {error ? (
        <Alert severity="error">{error.message}</Alert>
      ) : !isPending && commands.length === 0 ? (
        <EmptyState title="No commands sent yet" />
      ) : (
        <DataTable
          rows={commands ?? []}
          loading={isPending}
          columns={columns}
          onRowClick={compact ? (row) => row.automation && navigate(`/automations/${row.automation.id}`) : undefined}
        />
      )}
    </Card>
  );
}
