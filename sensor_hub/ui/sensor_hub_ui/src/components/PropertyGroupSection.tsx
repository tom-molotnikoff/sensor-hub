import { Divider, Paper, Stack, Typography } from '@mui/material';
import type { PropertyGroup } from '../gen/aliases';
import { TypographyH3 } from '../tools/Typography';

interface PropertyGroupSectionProps {
  group: PropertyGroup;
  children: React.ReactNode;
}

export default function PropertyGroupSection({ group, children }: PropertyGroupSectionProps) {
  return (
    <Paper id={group.id} sx={{ p: 2, width: '100%', scrollMarginTop: 80 }}>
      <TypographyH3 changes={{ margin: 0 }}>{group.label}</TypographyH3>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
        {group.description}
      </Typography>
      <Stack divider={<Divider />}>{children}</Stack>
    </Paper>
  );
}
