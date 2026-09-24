import type { ReactNode } from 'react';
import { Box, Typography } from '@mui/material';
import { useBounded } from './useBounded';
import { useInsetApplied } from './inset';
import { responsivePixels } from './tiers';
import { density } from './theme/tokens';

export interface Tile {
  key: string | number;
  label: ReactNode;
  fill: string;
  ink: string;
}

interface TileGridProps {
  columns: number;
  tiles: Tile[];
  caption?: string;
}

const tileGap = 4;
const smallestTile = 12;
const largestLooseTile = 40;

function tileSide(columns: number, rows: number, bounded: boolean) {
  const fromWidth = `calc((100cqw - ${(columns - 1) * tileGap}px) / ${columns})`;
  if (!bounded) return `min(${largestLooseTile}px, ${fromWidth})`;
  const fromHeight = `calc((100cqh - ${(rows - 1) * tileGap}px) / ${rows})`;
  return `max(${smallestTile}px, min(${fromWidth}, ${fromHeight}))`;
}

export default function TileGrid({ columns, tiles, caption }: TileGridProps) {
  const bounded = useBounded();
  const insetApplied = useInsetApplied();
  const rows = Math.max(1, Math.ceil(tiles.length / columns));

  return (
    <Box
      data-ui="tile-grid"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 0.5,
        padding: bounded && !insetApplied ? responsivePixels(density.card) : 1,
        boxSizing: 'border-box',
        minWidth: 0,
        ...(bounded && { height: '100%', minHeight: 0, overflow: 'hidden' }),
      }}
    >
      {caption && (
        <Typography variant="caption" color="text.secondary" align="center" noWrap>
          {caption}
        </Typography>
      )}
      <Box
        data-ui="tile-grid-area"
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          minWidth: 0,
          ...(bounded ? { flex: '1 1 0', minHeight: 0, containerType: 'size' } : { containerType: 'inline-size' }),
        }}
      >
        <Box
          sx={{
            '--tile': tileSide(columns, rows, bounded),
            display: 'grid',
            gridTemplateColumns: `repeat(${columns}, var(--tile))`,
            gridAutoRows: 'var(--tile)',
            gap: `${tileGap}px`,
          }}
        >
          {tiles.map((tile) => (
            <Box
              key={tile.key}
              data-ui="tile"
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                borderRadius: 0.5,
                bgcolor: tile.fill,
                color: tile.ink,
                fontSize: 'max(9px, calc(var(--tile) * 0.35))',
                fontWeight: 'fontWeightBold',
              }}
            >
              {tile.label}
            </Box>
          ))}
        </Box>
      </Box>
    </Box>
  );
}
