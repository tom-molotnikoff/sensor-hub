import { Switch, Typography } from '@mui/material';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutlineOutlined';
import type { Automation } from '../../gen/aliases';
import StatusPill from '../../ui/dataTable/StatusPill';
import Inline from '../../ui/Inline';
import { automationStatus, describeAutomation, showsFailedFlag } from './automationText';

export type SensorName = (id: number) => string;
export type Toggle = (automation: Automation, enabled: boolean) => void;

export function EnabledSwitch({ automation, onToggle }: { automation: Automation; onToggle: Toggle }) {
  return (
    <Switch
      checked={automation.enabled}
      slotProps={{ input: { 'aria-label': `${automation.name} enabled` } }}
      onClick={(event) => event.stopPropagation()}
      onChange={(_, enabled) => onToggle(automation, enabled)}
    />
  );
}

export function NameAndSummary({ automation, sensorName }: { automation: Automation; sensorName: SensorName }) {
  return (
    <div>
      <Typography variant="body" noWrap>
        {automation.name}
      </Typography>
      <Typography variant="bodySmall" color="text.secondary" noWrap>
        {describeAutomation(automation, sensorName)}
      </Typography>
    </div>
  );
}

export function StatusWithFlag({ automation }: { automation: Automation }) {
  const { key, label } = automationStatus[automation.status];
  if (automation.status === 'broken') {
    return (
      <div>
        <StatusPill status={key} label={label} />
        <Typography variant="bodySmall" color="text.secondary" noWrap title={automation.status_reason ?? undefined}>
          {automation.status_reason}
        </Typography>
      </div>
    );
  }
  return (
    <Inline>
      <StatusPill status={key} label={label} />
      {showsFailedFlag(automation) && (
        <Inline>
          <ErrorOutlineIcon fontSize="small" color="error" />
          <Typography variant="bodySmall" color="error">
            last run failed
          </Typography>
        </Inline>
      )}
    </Inline>
  );
}
