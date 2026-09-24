import { useMemo, useState } from 'react';
import { Box, Button, InputAdornment, LinearProgress, TextField, Typography } from '@mui/material';
import SearchIcon from '@mui/icons-material/Search';
import { useGridApiRef } from '@mui/x-data-grid';
import RowActionsMenu from './RowActionsMenu';
import StatusPill from './StatusPill';
import { displayedValue, type DataTableColumn, type GridApiRef, type RowAction, type TableRow } from './columns';
import { responsivePixels } from '../tiers';
import { density } from '../theme/tokens';

const pageSize = 10;

interface CompactListProps<R extends TableRow> {
  rows: readonly R[];
  columns: readonly DataTableColumn<R>[];
  loading: boolean;
  onRowClick?: (row: R, anchor: HTMLElement) => void;
  rowActions?: (row: R) => RowAction[];
}

export default function CompactList<R extends TableRow>({ rows, columns, loading, onRowClick, rowActions }: CompactListProps<R>) {
  const apiRef = useGridApiRef() as GridApiRef;
  const [query, setQuery] = useState('');
  const [shown, setShown] = useState(pageSize);

  const title = columns.find((column) => column.compact === 'title')!;
  const meta = columns.filter((column) => column.compact === 'meta');
  const status = columns.find((column) => column.compact === 'status');

  const matching = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase();
    if (!needle) return rows;
    return rows.filter((row) =>
      columns.some((column) => displayedValue(row, column, apiRef).toLocaleLowerCase().includes(needle)),
    );
  }, [rows, columns, query, apiRef]);

  const remaining = matching.length - shown;

  return (
    <Box data-ui="data-table" sx={{ display: 'flex', flexDirection: 'column', gap: responsivePixels(density.gap), minWidth: 0 }}>
      <TextField
        size="small"
        fullWidth
        placeholder="Search"
        value={query}
        onChange={(event) => {
          setQuery(event.target.value);
          setShown(pageSize);
        }}
        slotProps={{
          htmlInput: { 'aria-label': 'Search' },
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
          },
        }}
      />
      {loading && <LinearProgress />}
      <Box component="ul" data-ui="data-table-rows" sx={{ listStyle: 'none', margin: 0, padding: 0, minWidth: 0 }}>
        {matching.slice(0, shown).map((row) => {
          const metaLine = meta.map((column) => displayedValue(row, column, apiRef)).filter(Boolean).join(' · ');
          const titleText = displayedValue(row, title, apiRef);
          return (
            <Box
              component="li"
              key={row.id}
              data-ui="data-table-row"
              sx={{
                display: 'flex',
                alignItems: 'center',
                minWidth: 0,
                borderBottom: 1,
                borderColor: 'divider',
                '&:last-of-type': { borderBottom: 0 },
              }}
            >
              <Box
                component={onRowClick ? 'button' : 'div'}
                type={onRowClick ? 'button' : undefined}
                onClick={onRowClick && ((event: React.MouseEvent<HTMLElement>) => onRowClick(row, event.currentTarget))}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: responsivePixels(density.gap),
                  flex: '1 1 auto',
                  minWidth: 0,
                  paddingY: 1,
                  paddingX: 0,
                  border: 0,
                  background: 'none',
                  color: 'inherit',
                  font: 'inherit',
                  textAlign: 'start',
                  cursor: onRowClick ? 'pointer' : 'default',
                }}
              >
                <Box component="span" sx={{ flex: '1 1 auto', minWidth: 0 }}>
                  <Typography variant="body" component="span" noWrap sx={{ display: 'block' }} data-ui="data-table-title">
                    {titleText}
                  </Typography>
                  {metaLine && (
                    <Typography variant="bodySmall" component="span" noWrap sx={{ display: 'block', color: 'text.secondary' }} data-ui="data-table-meta">
                      {metaLine}
                    </Typography>
                  )}
                </Box>
                {status && <StatusPill status={status.statusOf(row)} label={displayedValue(row, status, apiRef)} />}
              </Box>
              {rowActions && <RowActionsMenu label={titleText} actions={rowActions(row)} />}
            </Box>
          );
        })}
      </Box>
      {remaining > 0 && (
        <Button variant="text" onClick={() => setShown((count) => count + pageSize)}>
          Show more ({remaining})
        </Button>
      )}
    </Box>
  );
}
