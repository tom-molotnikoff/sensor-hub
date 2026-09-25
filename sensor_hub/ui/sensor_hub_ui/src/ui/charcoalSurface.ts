export const charcoalSurface = {
  paint: { bgcolor: 'nav.bg', backgroundImage: 'none' },
  content: {
    className: 'dark',
    sx: { color: 'nav.text', '& .MuiIconButton-root:hover': { bgcolor: 'nav.hover' } },
  },
} as const;
