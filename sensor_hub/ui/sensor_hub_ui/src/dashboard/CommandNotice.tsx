import { Alert, Snackbar } from '@mui/material';
import type { SensorCommand } from './useSensorCommand';

export default function CommandNotice({ command }: { command: SensorCommand }) {
  return (
    <Snackbar
      open={command.notice != null}
      autoHideDuration={3000}
      onClose={command.dismissNotice}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
    >
      <Alert severity="error" onClose={command.dismissNotice}>
        {command.notice}
      </Alert>
    </Snackbar>
  );
}
