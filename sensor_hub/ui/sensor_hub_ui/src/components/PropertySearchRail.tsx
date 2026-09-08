import { Box, List, ListItemButton, ListItemText, TextField, Typography } from '@mui/material';
import { useIsMobile } from '../hooks/useMobile';

export interface RailGroup {
  id: string;
  label: string;
  editedCount: number;
  errorCount: number;
}

interface PropertySearchRailProps {
  groups: RailGroup[];
  currentGroupId?: string;
  search: string;
  onSearchChange: (value: string) => void;
  stickyTop: number;
}

export default function PropertySearchRail({
  groups,
  currentGroupId,
  search,
  onSearchChange,
  stickyTop,
}: PropertySearchRailProps) {
  const isMobile = useIsMobile();

  return (
    <Box
      component="nav"
      aria-label="Property groups"
      sx={{
        flex: isMobile ? '0 0 auto' : '0 0 200px',
        width: isMobile ? '100%' : undefined,
        ...(isMobile ? {} : { position: 'sticky', top: stickyTop, alignSelf: 'flex-start' }),
      }}
    >
      <TextField
        value={search}
        onChange={(event) => onSearchChange(event.target.value)}
        size="small"
        fullWidth
        placeholder="Search"
        slotProps={{ htmlInput: { 'aria-label': 'Search properties' } }}
      />
      <List dense sx={{ mt: 1 }}>
        {groups.map((group) => (
          <ListItemButton
            key={group.id}
            component="a"
            href={`#${group.id}`}
            selected={group.id === currentGroupId}
            aria-current={group.id === currentGroupId ? 'true' : undefined}
            data-testid={`rail-${group.id}`}
            sx={{
              borderRadius: 1,
              borderLeft: 3,
              borderLeftColor: group.id === currentGroupId ? 'primary.main' : 'transparent',
            }}
          >
            <ListItemText
              primary={group.label}
              slotProps={{
                primary: {
                  sx: {
                    fontSize: 15,
                    fontWeight: group.id === currentGroupId ? 700 : 500,
                    color: group.id === currentGroupId ? 'primary.main' : 'text.primary',
                  },
                },
              }}
            />
            {group.editedCount > 0 && (
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ ml: 1 }}
                data-testid={`rail-edited-count-${group.id}`}
              >
                {group.editedCount}
              </Typography>
            )}
            {group.errorCount > 0 && (
              <Typography
                variant="caption"
                color="error"
                sx={{ ml: 1 }}
                aria-label={`${group.errorCount} ${group.errorCount === 1 ? 'error' : 'errors'} in ${group.label}`}
                data-testid={`rail-error-count-${group.id}`}
              >
                {group.errorCount}
              </Typography>
            )}
          </ListItemButton>
        ))}
      </List>
    </Box>
  );
}
