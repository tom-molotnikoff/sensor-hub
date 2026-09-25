import type { ReactNode } from 'react';
import { AppBar as MuiAppBar, Box, IconButton, Toolbar, Typography } from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';

interface AppBarProps {
  title: string;
  onMenuClick: () => void;
  children?: ReactNode;
}

export default function AppBar({ title, onMenuClick, children }: AppBarProps) {
  return (
    <MuiAppBar position="sticky" data-ui="app-bar">
      <Toolbar className="dark" sx={{ gap: 1, '& .MuiIconButton-root:hover': { bgcolor: 'nav.hover' } }}>
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
