import { Box } from '@mui/material';
import { useChartColours } from '../../theme/chartColours';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';
import { indeterminate } from './keyframes';
import LoaderShell from './LoaderShell';

/**
 * Loader for the uptime widget: a terracotta segment slides across a rounded
 * track (matching the real LinearProgress), with calm grey placeholders for the
 * percentage and caption. No number is shown.
 */
export default function IndeterminateBarLoader() {
  const colours = useChartColours();
  const reduced = usePrefersReducedMotion();

  return (
    <LoaderShell>
      <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 1.8, width: '100%' }}>
        <Box sx={{ width: '56%', maxWidth: 130, height: 12, borderRadius: 1, bgcolor: colours.noData }} />
        <Box sx={{ position: 'relative', width: '80%', height: 10, borderRadius: 5, bgcolor: colours.noData, overflow: 'hidden' }}>
          <Box
            data-testid="indeterminate-bar"
            sx={{
              position: 'absolute',
              top: 0,
              height: '100%',
              width: '38%',
              borderRadius: 5,
              bgcolor: colours.categorical[0],
              ...(reduced
                ? { left: 0 }
                : { animation: `${indeterminate} 1.5s cubic-bezier(.65,.05,.36,1) infinite` }),
            }}
          />
        </Box>
        <Box sx={{ width: '42%', maxWidth: 96, height: 8, borderRadius: 1, bgcolor: colours.noData }} />
      </Box>
    </LoaderShell>
  );
}
