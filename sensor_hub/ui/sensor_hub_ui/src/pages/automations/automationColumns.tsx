import type { Automation } from '../../gen/aliases';
import type { DataTableColumn } from '../../ui/DataTable';
import { EnabledSwitch, NameAndSummary, StatusWithFlag, type SensorName, type Toggle } from './AutomationCells';
import { automationStatus, describeAutomation, formatHubTime, showsFailedFlag } from './automationText';

type Column = DataTableColumn<Automation>;

const enabledColumn = (onToggle: Toggle): Column => ({
  field: 'enabled',
  headerName: '',
  width: 88,
  sortable: false,
  disableColumnMenu: true,
  compact: 'hidden',
  renderCell: ({ row }) => <EnabledSwitch automation={row} onToggle={onToggle} />,
});

const nameColumn = (sensorName: SensorName): Column => ({
  field: 'name',
  headerName: 'Automation',
  flex: 1,
  minWidth: 240,
  sortable: false,
  compact: 'title',
  renderCell: ({ row }) => <NameAndSummary automation={row} sensorName={sensorName} />,
});

const summaryColumn = (sensorName: SensorName): Column => ({
  field: 'summary',
  headerName: 'Summary',
  compact: 'meta',
  wide: 'hidden',
  valueGetter: (_, row) => describeAutomation(row, sensorName),
});

const failedColumn: Column = {
  field: 'last_run_failed',
  headerName: 'Last run',
  compact: 'meta',
  wide: 'hidden',
  valueGetter: (_, row) => (showsFailedFlag(row) ? 'last run failed' : ''),
};

const statusColumn: Column = {
  field: 'status',
  headerName: 'Status',
  width: 220,
  sortable: false,
  compact: 'status',
  statusOf: (row) => automationStatus[row.status].key,
  valueFormatter: (status: Automation['status']) => automationStatus[status].label,
  renderCell: ({ row }) => <StatusWithFlag automation={row} />,
};

const nextColumn: Column = {
  field: 'next_fire_at',
  headerName: 'Next',
  width: 160,
  compact: 'hidden',
  valueGetter: (_, row) => (row.status !== 'off' && row.next_fire_at ? formatHubTime(row.next_fire_at, row.hub_timezone) : '-'),
};

export function automationColumns(sensorName: SensorName, onToggle: Toggle | undefined): Column[] {
  const leading = onToggle ? [enabledColumn(onToggle)] : [];
  return [...leading, nameColumn(sensorName), summaryColumn(sensorName), failedColumn, statusColumn, nextColumn];
}
