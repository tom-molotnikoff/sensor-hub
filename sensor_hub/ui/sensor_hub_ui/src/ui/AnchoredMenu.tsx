import type { ReactNode } from 'react';
import { Box, Divider, Menu, MenuItem, Typography, type PopoverOrigin } from '@mui/material';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';

const viewportMargin = 16;

const widths = { sm: 260, md: 360 } as const;

const placements = {
  'below-end': { anchor: { vertical: 'bottom', horizontal: 'right' }, transform: { vertical: 'top', horizontal: 'right' } },
  'above-start': { anchor: { vertical: 'top', horizontal: 'left' }, transform: { vertical: 'bottom', horizontal: 'left' } },
} as const satisfies Record<string, { anchor: PopoverOrigin; transform: PopoverOrigin }>;

interface AnchoredMenuProps {
  id?: string;
  'data-ui'?: string;
  anchorEl: HTMLElement | null;
  onClose: () => void;
  width: keyof typeof widths;
  placement?: keyof typeof placements;
  maxHeight?: number;
  spacedItems?: boolean;
  labelledBy?: string;
  variant?: 'menu' | 'selectedMenu';
  children?: ReactNode;
}

export default function AnchoredMenu({
  id,
  'data-ui': ui,
  anchorEl,
  onClose,
  width,
  placement = 'below-end',
  maxHeight,
  spacedItems = false,
  labelledBy,
  variant,
  children,
}: AnchoredMenuProps) {
  return (
    <Menu
      id={id}
      data-ui={ui}
      variant={variant}
      anchorEl={anchorEl}
      open={anchorEl !== null}
      onClose={onClose}
      marginThreshold={viewportMargin}
      anchorOrigin={placements[placement].anchor}
      transformOrigin={placements[placement].transform}
      slotProps={{
        list: { 'aria-labelledby': labelledBy },
        paper: {
          sx: {
            width: `min(${widths[width]}px, calc(100vw - ${viewportMargin * 2}px))`,
            maxHeight,
            ...(spacedItems && { '& .MuiMenuItem-root': { paddingY: 1.5 } }),
          },
        },
      }}
    >
      {children}
    </Menu>
  );
}

export function MenuDivider() {
  return <Divider component="li" />;
}

interface MenuNewTabLinkProps {
  href: string;
  onClick?: () => void;
  children?: ReactNode;
}

export function MenuNewTabLink({ href, onClick, children }: MenuNewTabLinkProps) {
  return (
    <li role="none">
      <MenuItem component="a" href={href} target="_blank" rel="noopener noreferrer" onClick={onClick}>
        {children}
        <OpenInNewIcon fontSize="small" color="action" titleAccess="opens in a new tab" />
      </MenuItem>
    </li>
  );
}

export function MenuHeading({ id, children }: { id: string; children?: ReactNode }) {
  return (
    <li role="none" data-ui="menu-heading">
      <Typography id={id} component="div" variant="caption" color="text.secondary" noWrap sx={{ paddingX: 2, paddingY: 1 }}>
        {children}
      </Typography>
    </li>
  );
}

interface MenuChoice<T extends string> {
  value: T;
  label: string;
  icon: ReactNode;
}

interface MenuSegmentsProps<T extends string> {
  label: string;
  choices: readonly MenuChoice<T>[];
  value: T | undefined;
  onChange: (value: T) => void;
}

export function MenuSegments<T extends string>({ label, choices, value, onChange }: MenuSegmentsProps<T>) {
  return (
    <li role="none">
      <Box
        role="group"
        aria-label={label}
        data-ui="menu-segments"
        sx={{ display: 'flex', marginX: 1, marginBottom: 1, border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden' }}
      >
        {choices.map((choice) => (
          <MenuItem
            key={choice.value}
            component="div"
            role="menuitemradio"
            aria-checked={choice.value === value}
            selected={choice.value === value}
            onClick={() => onChange(choice.value)}
            sx={{
              flex: '1 1 0',
              flexDirection: 'column',
              gap: 0.25,
              minHeight: 0,
              paddingX: 0.5,
              paddingY: 0.75,
              typography: 'caption',
              color: 'text.secondary',
              '& + &': { borderLeft: 1, borderColor: 'divider' },
              '&.Mui-selected': { color: 'primary.main' },
            }}
          >
            {choice.icon}
            {choice.label}
          </MenuItem>
        ))}
      </Box>
    </li>
  );
}
