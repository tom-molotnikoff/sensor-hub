import { useMemo } from 'react';
import { Box } from '@mui/material';
import { DataGrid, type GridColDef } from '@mui/x-data-grid';
import CompactList from './dataTable/CompactList';
import RowActionButtons from './dataTable/RowActionButtons';
import { assertColumnRoles, type ColumnRules, type DataTableColumn, type RowAction, type TableRow } from './dataTable/columns';
import { responsive, responsivePixels, useTier } from './tiers';
import { density } from './theme/tokens';
import { useBounded } from './useBounded';
import { useBleed } from './inset';

export type { DataTableColumn, RowAction } from './dataTable/columns';

interface DataTableProps<R extends TableRow, C extends readonly DataTableColumn<R>[]> {
  rows: readonly R[];
  columns: C & ColumnRules<C>;
  loading?: boolean;
  onRowClick?: (row: R, anchor: HTMLElement) => void;
  rowActions?: (row: R) => RowAction[];
}

export default function DataTable<R extends TableRow, const C extends readonly DataTableColumn<R>[]>({
  rows,
  columns,
  loading = false,
  onRowClick,
  rowActions,
}: DataTableProps<R, C>) {
  if (import.meta.env.DEV) assertColumnRoles(columns);
  const tier = useTier();
  const bounded = useBounded();
  useBleed(bounded);
  const gridColumns = useMemo<GridColDef<R>[]>(() => {
    if (!rowActions) return [...columns];
    const actionsColumn: GridColDef<R> = {
      field: 'actions',
      headerName: 'Actions',
      flex: 1,
      minWidth: 140,
      sortable: false,
      filterable: false,
      disableExport: true,
      renderCell: ({ row }) => <RowActionButtons actions={rowActions(row)} />,
    };
    return [...columns, actionsColumn];
  }, [columns, rowActions]);

  const table =
    tier === 'compact' ? (
      <CompactList rows={rows} columns={columns} loading={loading} onRowClick={onRowClick} rowActions={rowActions} />
    ) : (
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

  return bounded ? (
    <Box
      sx={{
        height: '100%',
        minHeight: 0,
        overflow: 'auto',
        paddingX: responsivePixels(density.card),
        paddingTop: responsive({ compact: `${density.card.compact}px`, wide: '0px' }),
        paddingBottom: 1,
      }}
    >
      {table}
    </Box>
  ) : (
    table
  );
}
