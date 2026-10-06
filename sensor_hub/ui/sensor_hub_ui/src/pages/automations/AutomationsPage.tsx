import { useMemo } from 'react';
import { useNavigate } from 'react-router';
import { Alert, Button, IconButton, Tooltip } from '@mui/material';
import AutoModeIcon from '@mui/icons-material/AutoMode';
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined';
import { useAutomations, useSetAutomationEnabled } from '../../hooks/useAutomations';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import ActionBar from '../../ui/ActionBar';
import Card from '../../ui/Card';
import DataTable from '../../ui/DataTable';
import EmptyState from '../../ui/EmptyState';
import Page from '../../ui/Page';
import { useTier } from '../../ui/tiers';
import { automationColumns } from './automationColumns';
import { useSensorName } from './useSensorName';

function ZoneInfo({ zone }: { zone: string }) {
  return (
    <Tooltip title={`Times are in the hub's timezone, ${zone}.`}>
      <IconButton size="small" aria-label="About these times">
        <InfoOutlinedIcon fontSize="small" />
      </IconButton>
    </Tooltip>
  );
}

export default function AutomationsPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const wide = useTier() === 'wide';
  const sensorName = useSensorName();
  const { data: automations, isPending, error } = useAutomations();
  const { mutate: setEnabled, error: toggleError } = useSetAutomationEnabled();

  const canEdit = hasPerm(user, 'manage_automations') && hasPerm(user, 'control_sensors');
  const zone = automations?.[0]?.hub_timezone;
  const openNew = () => navigate('/automations/new');
  const newButton = canEdit && (
    <Button variant="contained" onClick={openNew}>
      New automation
    </Button>
  );

  const columns = useMemo(
    () => automationColumns(sensorName, canEdit ? (automation, enabled) => setEnabled({ id: automation.id, enabled }) : undefined),
    [canEdit, sensorName, setEnabled],
  );

  return (
    <Page
      title="Automations"
      actions={
        <>
          {zone && <ZoneInfo zone={zone} />}
          {newButton}
        </>
      }
      loading={user === undefined}
    >
      {!wide && (newButton || zone) && <ActionBar trailing={<>{zone && <ZoneInfo zone={zone} />}{newButton}</>} />}
      {toggleError && <Alert severity="error">{toggleError.message}</Alert>}
      <Card>
        {error ? (
          <Alert severity="error">{error.message}</Alert>
        ) : !isPending && automations.length === 0 ? (
          <EmptyState
            icon={<AutoModeIcon fontSize="large" />}
            title="No automations yet"
            description="An automation switches devices for you at the times you choose."
            actionLabel={canEdit ? 'New automation' : undefined}
            onAction={canEdit ? openNew : undefined}
            size="lg"
          />
        ) : (
          <DataTable
            rows={automations ?? []}
            loading={isPending}
            columns={columns}
            onRowClick={(row) => navigate(`/automations/${row.id}`)}
          />
        )}
      </Card>
    </Page>
  );
}
