import type { GridColDef, GridRowId, GridValidRowModel, GridValueGetter } from '@mui/x-data-grid';
import type { StatusKey } from '../theme';

export type GridApiRef = Parameters<GridValueGetter>[3];

export type TableRow = GridValidRowModel & { id: GridRowId };

type CompactRole<R extends TableRow> =
  | { compact: 'title' | 'meta' | 'hidden' }
  | { compact: 'status'; statusOf: (row: R) => StatusKey };

export type DataTableColumn<R extends TableRow> = GridColDef<R> & CompactRole<R>;

type Tally<C extends readonly unknown[], Role, Found extends unknown[] = []> = C extends readonly [
  infer Head,
  ...infer Tail,
]
  ? Tally<Tail, Role, Head extends { compact: Role } ? [...Found, Head] : Found>
  : Found['length'];

export type ColumnRules<C extends readonly unknown[]> = number extends C['length']
  ? C
  : Tally<C, 'title'> extends 1
    ? Tally<C, 'meta'> extends 0 | 1 | 2
      ? Tally<C, 'status'> extends 0 | 1
        ? C
        : 'DataTable columns need at most one status column'
      : 'DataTable columns need at most two meta columns'
    : 'DataTable columns need exactly one title column';

const limits = { title: [1, 1], meta: [0, 2], status: [0, 1] } as const;

export function assertColumnRoles<R extends TableRow>(columns: readonly DataTableColumn<R>[]) {
  for (const [role, [min, max]] of Object.entries(limits)) {
    const count = columns.filter((column) => column.compact === role).length;
    if (count < min || count > max) {
      throw new Error(`DataTable needs ${min === max ? `exactly ${min}` : `${min} to ${max}`} ${role} columns, got ${count}`);
    }
  }
}

export function displayedValue<R extends TableRow>(
  row: R,
  column: DataTableColumn<R>,
  apiRef: GridApiRef,
): string {
  const raw: unknown = row[column.field];
  const value: unknown = column.valueGetter ? column.valueGetter(raw as never, row, column, apiRef) : raw;
  if (column.valueFormatter) return String(column.valueFormatter(value as never, row, column, apiRef) ?? '');
  if (value == null) return '';
  if (value instanceof Date) return column.type === 'date' ? value.toLocaleDateString() : value.toLocaleString();
  if (typeof value === 'number' && column.type === 'number') return value.toLocaleString();
  return String(value);
}
