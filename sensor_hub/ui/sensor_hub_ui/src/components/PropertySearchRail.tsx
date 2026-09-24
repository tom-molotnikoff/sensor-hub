import { List, ListItemButton, ListItemText, TextField, Typography } from '@mui/material';
import Inline from '../ui/Inline';
import Stack from '../ui/Stack';
import Sticky from '../ui/Sticky';

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
  stickyOffset: number;
}

export default function PropertySearchRail({
  groups,
  currentGroupId,
  search,
  onSearchChange,
  stickyOffset,
}: PropertySearchRailProps) {
  return (
    <Sticky offset={stickyOffset}>
      <nav aria-label="Property groups">
        <Stack>
          <TextField
            value={search}
            onChange={(event) => onSearchChange(event.target.value)}
            size="small"
            fullWidth
            placeholder="Search"
            slotProps={{ htmlInput: { 'aria-label': 'Search properties' } }}
          />
          <List dense disablePadding>
            {groups.map((group) => {
              const current = group.id === currentGroupId;
              return (
                <ListItemButton
                  key={group.id}
                  component="a"
                  href={`#${group.id}`}
                  selected={current}
                  aria-current={current ? 'true' : undefined}
                  data-testid={`rail-${group.id}`}
                  sx={{
                    borderRadius: 1,
                    borderLeft: 3,
                    borderLeftColor: current ? 'primary.main' : 'transparent',
                  }}
                >
                  <ListItemText
                    primary={group.label}
                    slotProps={{
                      primary: {
                        variant: current ? 'sectionTitle' : 'body',
                        color: current ? 'primary' : 'textPrimary',
                      },
                    }}
                  />
                  <Inline>
                    {group.editedCount > 0 && (
                      <Typography variant="caption" color="text.secondary" data-testid={`rail-edited-count-${group.id}`}>
                        {group.editedCount}
                      </Typography>
                    )}
                    {group.errorCount > 0 && (
                      <Typography
                        variant="caption"
                        color="error"
                        aria-label={`${group.errorCount} ${group.errorCount === 1 ? 'error' : 'errors'} in ${group.label}`}
                        data-testid={`rail-error-count-${group.id}`}
                      >
                        {group.errorCount}
                      </Typography>
                    )}
                  </Inline>
                </ListItemButton>
              );
            })}
          </List>
        </Stack>
      </nav>
    </Sticky>
  );
}
