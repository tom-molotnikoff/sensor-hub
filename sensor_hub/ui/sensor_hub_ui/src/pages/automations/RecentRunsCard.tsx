import { DateTime } from 'luxon';
import { Alert, Typography } from '@mui/material';
import { useAutomationRuns } from '../../hooks/useAutomations';
import Card from '../../ui/Card';
import StatusPill from '../../ui/dataTable/StatusPill';
import Inline from '../../ui/Inline';
import Stack from '../../ui/Stack';
import { describeRun, formatHubTime, runStatus } from './automationText';

const historyDays = 30;

interface RecentRunsCardProps {
  automationId: number | undefined;
  zone: string;
}

export default function RecentRunsCard({ automationId, zone }: RecentRunsCardProps) {
  const { data: runs, error } = useAutomationRuns(automationId);
  const since = DateTime.now().minus({ days: historyDays });
  const recent = (runs ?? []).filter((run) => DateTime.fromISO(run.started_at) >= since);

  return (
    <Card title="Recent runs" actions={<Typography variant="bodySmall" color="text.secondary">last {historyDays} days</Typography>}>
      {error ? (
        <Alert severity="error">{error.message}</Alert>
      ) : recent.length === 0 ? (
        <Typography variant="body" color="text.secondary">
          No runs in the last {historyDays} days.
        </Typography>
      ) : (
        <Stack>
          {recent.map((run) => (
            <div key={run.id} data-ui="automation-run">
              <Inline>
                <Typography variant="body">{formatHubTime(run.due_at ?? run.started_at, zone)}</Typography>
                <StatusPill status={runStatus[run.status]} label={run.status} />
              </Inline>
              <Typography variant="bodySmall" color="text.secondary">
                {describeRun(run, zone)}
              </Typography>
            </div>
          ))}
        </Stack>
      )}
    </Card>
  );
}
