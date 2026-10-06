import { useMemo } from 'react';
import { useNavigate } from 'react-router';
import { Alert, Button, IconButton, Switch, Tooltip, Typography } from '@mui/material';
import AutoModeIcon from '@mui/icons-material/AutoMode';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutlineOutlined';
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined';
import type { Automation } from '../../gen/aliases';
import { useAutomations, useSetAutomationEnabled } from '../../hooks/useAutomations';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import ActionBar from '../../ui/ActionBar';
import Card from '../../ui/Card';
import DataTable, { type DataTableColumn } from '../../ui/DataTable';
import StatusPill from '../../ui/dataTable/StatusPill';
import EmptyState from '../../ui/EmptyState';
import Inline from '../../ui/Inline';
import Page from '../../ui/Page';
import { useTier } from '../../ui/tiers';
import { automationStatus, describeAutomation, formatHubTime } from './automationText';
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

function FailedFlag() {
  return (
    <Inline>
      <ErrorOutlineIcon fontSize="small" color="error" />
      <Typography variant="bodySmall" color="error">
        last run failed
      </Typography>
    </Inline>
  );
}

const showsFailedFlag = (automation: Automation) => automation.last_run_failed && automation.status === 'armed';

export default function AutomationsPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const wide = useTier() === 'wide';
  const sensorName = useSensorName();
  const { data: automations, isPending, error } = useAutomations();
  const setEnabled = useSetAutomationEnabled();

  const canEdit = hasPerm(user, 'manage_automations') && hasPerm(user, 'control_sensors');
  const zone = automations?.[0]?.hub_timezone;
  const openNew = () => navigate('/automations/new');
  const newButton = canEdit && (
    <Button variant="contained" onClick={openNew}>
      New automation
    </Button>
  );

  const columns = useMemo(
    () =>
      [
        ...(canEdit
          ? [
              {
                field: 'enabled',
                headerName: '',
                width: 88,
                sortable: false,
                disableColumnMenu: true,
                compact: 'hidden',
                renderCell: ({ row }) => (
                  <Switch
                    checked={row.enabled}
                    slotProps={{ input: { 'aria-label': `${row.name} enabled` } }}
                    onClick={(event) => event.stopPropagation()}
                    onChange={(_, enabled) => setEnabled.mutate({ id: row.id, enabled })}
                  />
                ),
              } satisfies DataTableColumn<Automation>,
            ]
          : []),
        {
          field: 'name',
          headerName: 'Automation',
          flex: 1,
          minWidth: 240,
          sortable: false,
          compact: 'title',
          renderCell: ({ row }) => (
            <div>
              <Typography variant="body" noWrap>
                {row.name}
              </Typography>
              <Typography variant="bodySmall" color="text.secondary" noWrap>
                {describeAutomation(row, sensorName)}
              </Typography>
            </div>
          ),
        },
        {
          field: 'summary',
          headerName: 'Summary',
          compact: 'meta',
          wide: 'hidden',
          valueGetter: (_, row) => describeAutomation(row, sensorName),
        },
        {
          field: 'last_run_failed',
          headerName: 'Last run',
          compact: 'meta',
          wide: 'hidden',
          valueGetter: (_, row) => (showsFailedFlag(row) ? 'last run failed' : ''),
        },
        {
          field: 'status',
          headerName: 'Status',
          width: 220,
          sortable: false,
          compact: 'status',
          statusOf: (row) => automationStatus[row.status].key,
          valueFormatter: (status: Automation['status']) => automationStatus[status].label,
          renderCell: ({ row }) => (
            <Inline>
              <StatusPill status={automationStatus[row.status].key} label={automationStatus[row.status].label} />
              {showsFailedFlag(row) && <FailedFlag />}
            </Inline>
          ),
        },
        {
          field: 'next_fire_at',
          headerName: 'Next',
          width: 160,
          compact: 'hidden',
          valueGetter: (_, row) => (row.status !== 'off' && row.next_fire_at ? formatHubTime(row.next_fire_at, row.hub_timezone) : '-'),
        },
      ] satisfies DataTableColumn<Automation>[],
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
      {setEnabled.error && <Alert severity="error">{setEnabled.error.message}</Alert>}
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
