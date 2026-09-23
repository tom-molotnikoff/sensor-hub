import { useColorScheme } from '@mui/material/styles';

/**
 * Returns true when the resolved colour scheme is dark.
 * Handles mode === 'system' by checking the OS preference via systemMode.
 */
export function useIsDark(): boolean {
  const { mode, systemMode } = useColorScheme();
  return mode === 'dark' || (mode === 'system' && systemMode === 'dark');
}
