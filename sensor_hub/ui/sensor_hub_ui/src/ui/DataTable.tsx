import { useMemo } from 'react';
import { DataGrid } from '@mui/x-data-grid';
import CompactList from './dataTable/CompactList';
import { assertColumnRoles, type ColumnRules, type DataTableColumn, type TableRow } from './dataTable/columns';
import { useTier } from './tiers';

export type { DataTableColumn } from './dataTable/columns';

interface DataTableProps<R extends TableRow, C extends readonly DataTableColumn<R>[]> {
  rows: readonly R[];
  columns: C & ColumnRules<C>;
  loading?: boolean;
  onRowClick?: (row: R, anchor: HTMLElement) => void;
}

export default function DataTable<R extends TableRow, const C extends readonly DataTableColumn<R>[]>({
  rows,
  columns,
  loading = false,
  onRowClick,
}: DataTableProps<R, C>) {
  if (import.meta.env.DEV) assertColumnRoles(columns);
  const tier = useTier();
  const gridColumns = useMemo(() => [...columns], [columns]);

  if (tier === 'compact') {
    return <CompactList rows={rows} columns={columns} loading={loading} onRowClick={onRowClick} />;
  }

  return (
    <DataGrid
      data-ui="data-table"
      rows={rows}
      columns={gridColumns}
      loading={loading}
      showToolbar
      onRowClick={onRowClick && ((params, event) => onRowClick(params.row, event.currentTarget as HTMLElement))}
      sx={onRowClick && { '& .MuiDataGrid-row': { cursor: 'pointer' } }}
    />
  );
}
