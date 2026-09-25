import type { ReactNode } from 'react';
import { AppBar as MuiAppBar, Box, IconButton, Toolbar, Typography } from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';
import { charcoalSurface } from './charcoalSurface';

interface AppBarProps {
  title: string;
  onMenuClick: () => void;
  children?: ReactNode;
}

export default function AppBar({ title, onMenuClick, children }: AppBarProps) {
  return (
    <MuiAppBar position="sticky" enableColorOnDark data-ui="app-bar" sx={charcoalSurface.paint}>
      <Toolbar className={charcoalSurface.content.className} sx={[charcoalSurface.content.sx, { gap: 1 }]}>
        <IconButton edge="start" color="inherit" aria-label="menu" onClick={onMenuClick} sx={{ marginRight: 1 }}>
          <MenuIcon />
        </IconButton>
        <Typography
          variant="pageTitle"
          color="inherit"
          noWrap
          data-ui="app-bar-title"
          sx={{ flex: '1 1 auto', minWidth: 0, textAlign: 'end' }}
        >
          {title}
        </Typography>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>{children}</Box>
      </Toolbar>
    </MuiAppBar>
  );
}
