import type { Automation } from '../../gen/aliases';
import type { DataTableColumn } from '../../ui/DataTable';
import { EnabledSwitch, NameAndSummary, StatusWithFlag, type SensorName, type Toggle } from './AutomationCells';
import { automationStatus, describeAutomation, describeNext, describeStatusDetail } from './automationText';

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

const statusDetailColumn: Column = {
  field: 'status_reason',
  headerName: 'Status detail',
  compact: 'meta',
  wide: 'hidden',
  valueGetter: (_, row) => describeStatusDetail(row),
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
  valueGetter: (_, row) => describeNext(row),
};

export function automationColumns(sensorName: SensorName, onToggle: Toggle | undefined): Column[] {
  const leading = onToggle ? [enabledColumn(onToggle)] : [];
  return [...leading, nameColumn(sensorName), summaryColumn(sensorName), statusDetailColumn, statusColumn, nextColumn];
}
