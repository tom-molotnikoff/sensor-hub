import { ThemeProvider } from '@mui/material';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import DataTable, { type DataTableColumn } from './DataTable';
import { theme } from './theme';

interface Row {
  id: number;
  name: string;
  driver: string;
  enabled: boolean;
  reason: string;
}

const columns = [
  { field: 'name', headerName: 'Name', compact: 'title' },
  { field: 'driver', headerName: 'Driver', compact: 'meta' },
  {
    field: 'enabled',
    headerName: 'Enabled',
    type: 'boolean',
    compact: 'meta',
    valueFormatter: (value: boolean) => (value ? 'enabled' : 'disabled'),
  },
  { field: 'reason', headerName: 'Reason', compact: 'status', statusOf: (row: Row) => (row.enabled ? 'ok' : 'bad') },
] as const satisfies readonly DataTableColumn<Row>[];

function rowsOf(count: number): Row[] {
  return Array.from({ length: count }, (_, index) => ({
    id: index + 1,
    name: `sensor-${String(index + 1).padStart(2, '0')}`,
    driver: index % 2 ? 'zigbee' : 'http',
    enabled: index !== 2,
    reason: index === 5 ? 'Battery low' : 'fine',
  }));
}

function renderTable(rows: Row[], onRowClick?: (row: Row, anchor: HTMLElement) => void) {
  return render(
    <ThemeProvider theme={theme}>
      <DataTable rows={rows} columns={columns} onRowClick={onRowClick} />
    </ThemeProvider>,
  );
}

function titles() {
  return Array.from(document.querySelectorAll('[data-ui=data-table-title]'), (title) => title.textContent);
}

function search(text: string) {
  fireEvent.change(screen.getByRole('textbox', { name: 'Search' }), { target: { value: text } });
}

describe('DataTable on compact', () => {
  it('shows each row as a title, a meta line and a status pill instead of a grid', () => {
    const { container } = renderTable(rowsOf(3));

    expect(container.querySelector('.MuiDataGrid-root')).toBeNull();
    const third = screen.getAllByRole('listitem')[2];
    expect(within(third).getByText('sensor-03')).toHaveClass('MuiTypography-body');
    expect(within(third).getByText('http · disabled')).toHaveClass('MuiTypography-bodySmall');
    expect(third.querySelector('[data-ui=status-pill]')).toHaveAttribute('data-status', 'bad');
    expect(third.querySelector('[data-ui=status-pill]')).toHaveTextContent('fine');
  });

  it('narrows rows to any column whose displayed value contains the search, ignoring case', () => {
    renderTable(rowsOf(8));

    search('DISABLED');
    expect(titles()).toEqual(['sensor-03']);
    search('battery');
    expect(titles()).toEqual(['sensor-06']);
    search('zig');
    expect(titles()).toEqual(['sensor-02', 'sensor-04', 'sensor-06', 'sensor-08']);
  });

  it('keeps the rows in their given order and has no sort, filter, export or column controls', () => {
    const rows = rowsOf(4).reverse();
    renderTable(rows);

    expect(titles()).toEqual(rows.map((row) => row.name));
    expect(screen.queryAllByRole('button')).toEqual([]);
    expect(screen.queryByRole('columnheader')).toBeNull();
  });

  it('shows ten rows at a time and starts again from ten when the search changes', () => {
    renderTable(rowsOf(23));

    expect(titles()).toHaveLength(10);
    fireEvent.click(screen.getByRole('button', { name: 'Show more (13)' }));
    expect(titles()).toHaveLength(20);
    fireEvent.click(screen.getByRole('button', { name: 'Show more (3)' }));
    expect(titles()).toHaveLength(23);
    expect(screen.queryByRole('button', { name: /Show more/ })).toBeNull();

    search('sensor');
    expect(titles()).toHaveLength(10);
  });

  it('hands a tapped row and its element to the row click handler', () => {
    const onRowClick = vi.fn();
    renderTable(rowsOf(2), onRowClick);

    const button = screen.getByRole('button', { name: /sensor-02/ });
    fireEvent.click(button);
    expect(onRowClick).toHaveBeenCalledWith(rowsOf(2)[1], button);
  });
});

describe('DataTable on wide', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('renders a DataGrid', () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: true,
      media: query,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
    }));
    const { container } = renderTable(rowsOf(3));

    expect(container.querySelector('.MuiDataGrid-root')).not.toBeNull();
    expect(screen.queryByRole('textbox', { name: 'Search' })).toBeNull();
  });
});

describe('DataTable columns', () => {
  const rows = rowsOf(1);

  it('reject a literal array that breaks the compact role rules', () => {
    const tables = [
      // @ts-expect-error DataTable needs a title column
      <DataTable rows={rows} columns={[{ field: 'name', compact: 'meta' }]} />,
      // @ts-expect-error DataTable takes one title column
      <DataTable rows={rows} columns={[{ field: 'name', compact: 'title' }, { field: 'driver', compact: 'title' }]} />,
      <DataTable
        rows={rows}
        // @ts-expect-error DataTable takes at most two meta columns
        columns={[
          { field: 'name', compact: 'title' },
          { field: 'driver', compact: 'meta' },
          { field: 'enabled', compact: 'meta' },
          { field: 'reason', compact: 'meta' },
        ]}
      />,
      <DataTable
        rows={rows}
        // @ts-expect-error DataTable takes at most one status column
        columns={[
          { field: 'name', compact: 'title' },
          { field: 'driver', compact: 'status', statusOf: () => 'ok' },
          { field: 'reason', compact: 'status', statusOf: () => 'bad' },
        ]}
      />,
    ];
    expect(tables).toHaveLength(4);
  });

  it('throw for an array built at runtime that breaks the rules', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    const built: DataTableColumn<Row>[] = [...columns, { field: 'driver', headerName: 'Again', compact: 'title' }];

    expect(() =>
      render(
        <ThemeProvider theme={theme}>
          <DataTable rows={rows} columns={built} />
        </ThemeProvider>,
      ),
    ).toThrow(Error);
  });

  it('accept no styling overrides', () => {
    const overrides = [
      // @ts-expect-error DataTable takes no sx
      <DataTable rows={rows} columns={columns} sx={{ height: 400 }} />,
      // @ts-expect-error DataTable takes no style
      <DataTable rows={rows} columns={columns} style={{ height: 400 }} />,
      // @ts-expect-error DataTable takes no className
      <DataTable rows={rows} columns={columns} className="tall" />,
    ];
    expect(overrides).toHaveLength(3);
  });
});
