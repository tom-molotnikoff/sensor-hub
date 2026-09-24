import { Children, Fragment, isValidElement, type ReactNode } from 'react';
import { Divider, Typography } from '@mui/material';
import type { PropertyGroup } from '../gen/aliases';
import Card from '../ui/Card';
import Stack from '../ui/Stack';

interface PropertyGroupSectionProps {
  group: PropertyGroup;
  children: ReactNode;
}

export default function PropertyGroupSection({ group, children }: PropertyGroupSectionProps) {
  return (
    <Card id={group.id} title={group.label}>
      <Stack>
        <Typography variant="body2" color="text.secondary">
          {group.description}
        </Typography>
        {Children.toArray(children).map((child, index) => (
          <Fragment key={isValidElement(child) ? child.key : index}>
            {index > 0 && <Divider />}
            {child}
          </Fragment>
        ))}
      </Stack>
    </Card>
  );
}
