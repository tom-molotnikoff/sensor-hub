import { useMemo, useState } from 'react';
import { Box, Button, InputAdornment, LinearProgress, TextField, Typography } from '@mui/material';
import SearchIcon from '@mui/icons-material/Search';
import { useGridApiRef } from '@mui/x-data-grid';
import StatusPill from './StatusPill';
import { displayedValue, type DataTableColumn, type GridApiRef, type TableRow } from './columns';
import { responsivePixels } from '../tiers';
import { density } from '../theme/tokens';

const pageSize = 10;

interface CompactListProps<R extends TableRow> {
  rows: readonly R[];
  columns: readonly DataTableColumn<R>[];
  loading: boolean;
  onRowClick?: (row: R, anchor: HTMLElement) => void;
}

export default function CompactList<R extends TableRow>({ rows, columns, loading, onRowClick }: CompactListProps<R>) {
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
          return (
            <Box
              component="li"
              key={row.id}
              data-ui="data-table-row"
              sx={{ borderBottom: 1, borderColor: 'divider', '&:last-of-type': { borderBottom: 0 } }}
            >
              <Box
                component={onRowClick ? 'button' : 'div'}
                type={onRowClick ? 'button' : undefined}
                onClick={onRowClick && ((event: React.MouseEvent<HTMLElement>) => onRowClick(row, event.currentTarget))}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: responsivePixels(density.gap),
                  width: '100%',
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
                <Box sx={{ flex: '1 1 auto', minWidth: 0 }}>
                  <Typography variant="body" noWrap data-ui="data-table-title">
                    {displayedValue(row, title, apiRef)}
                  </Typography>
                  {metaLine && (
                    <Typography variant="bodySmall" noWrap sx={{ color: 'text.secondary' }} data-ui="data-table-meta">
                      {metaLine}
                    </Typography>
                  )}
                </Box>
                {status && <StatusPill status={status.statusOf(row)} label={displayedValue(row, status, apiRef)} />}
              </Box>
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
