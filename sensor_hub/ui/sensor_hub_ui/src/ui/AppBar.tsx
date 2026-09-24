import type { MouseEvent, ReactNode } from 'react';
import { AppBar as MuiAppBar, Avatar, Box, IconButton, Toolbar, Typography } from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';

interface AppBarProps {
  title: string;
  brand?: string;
  onMenuClick: () => void;
  account: { initial: string; onClick: (event: MouseEvent<HTMLElement>) => void };
  children?: ReactNode;
}

export default function AppBar({ title, brand, onMenuClick, account, children }: AppBarProps) {
  return (
    <MuiAppBar position="sticky" data-ui="app-bar">
      <Toolbar sx={{ gap: 1 }}>
        <IconButton edge="start" color="inherit" aria-label="menu" onClick={onMenuClick} sx={{ marginRight: 1 }}>
          <MenuIcon />
        </IconButton>
        {brand && (
          <Typography variant="pageTitle" component="div" color="inherit" noWrap sx={{ flexShrink: 0 }}>
            {brand}
          </Typography>
        )}
        <Typography
          variant="pageTitle"
          color="inherit"
          noWrap
          data-ui="app-bar-title"
          sx={{ flex: '1 1 auto', minWidth: 0, textAlign: 'end' }}
        >
          {title}
        </Typography>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>
          {children}
          <IconButton color="inherit" aria-label="account" onClick={account.onClick}>
            <Avatar sx={{ width: 32, height: 32 }}>{account.initial}</Avatar>
          </IconButton>
        </Box>
      </Toolbar>
    </MuiAppBar>
  );
}
