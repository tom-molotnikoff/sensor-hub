import { useState } from 'react';
import { Navigate, useNavigate, useParams } from 'react-router';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import type { Automation } from '../../gen/aliases';
import { useAutomation, useDeleteAutomation, useSaveAutomation } from '../../hooks/useAutomations';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import Card from '../../ui/Card';
import StatusPill from '../../ui/dataTable/StatusPill';
import Inline from '../../ui/Inline';
import Page from '../../ui/Page';
import PageGrid from '../../ui/PageGrid';
import Stack from '../../ui/Stack';
import { StickyFooter } from '../../ui/Sticky';
import { useTier } from '../../ui/tiers';
import { draftOf, inputOf, type Draft } from './automationDraft';
import { automationStatus, describeAutomation, formatHubTime } from './automationText';
import RecentRunsCard from './RecentRunsCard';
import ThenCard from './ThenCard';
import { useSensorName } from './useSensorName';
import WhenCard from './WhenCard';

function PlainWordsCard({ draft, saved }: { draft: Draft; saved: Automation | undefined }) {
  const sensorName = useSensorName();
  const next = saved?.next_fire_at;

  return (
    <Card title="In plain words" actions={saved && <StatusPill status={automationStatus[saved.status].key} label={automationStatus[saved.status].label} />}>
      <Stack>
        <Typography variant="body">{describeAutomation(draft, sensorName)}</Typography>
        {saved && (
          <Typography variant="bodySmall" color="text.secondary">
            Next: {next && saved.status !== 'off' ? `${formatHubTime(next, saved.hub_timezone)} (${saved.hub_timezone})` : '-'}
          </Typography>
        )}
      </Stack>
    </Card>
  );
}

interface DeleteDialogProps {
  name: string;
  open: boolean;
  error: Error | null;
  onDelete: () => void;
  onClose: () => void;
}

function DeleteDialog({ name, open, error, onDelete, onClose }: DeleteDialogProps) {
  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>Delete automation</DialogTitle>
      <DialogContent>
        <Stack>
          <Typography variant="body">
            Delete <strong>{name}</strong> and its run history? Commands it already sent stay in command history.
          </Typography>
          {error && <Alert severity="error">{error.message}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" color="error" onClick={onDelete}>
          Delete
        </Button>
      </DialogActions>
    </Dialog>
  );
}

interface AutomationEditorProps {
  saved: Automation | undefined;
  canEdit: boolean;
  canDelete: boolean;
}

function AutomationEditor({ saved, canEdit, canDelete }: AutomationEditorProps) {
  const navigate = useNavigate();
  const wide = useTier() === 'wide';
  const [draft, setDraft] = useState(() => draftOf(saved));
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const save = useSaveAutomation();
  const remove = useDeleteAutomation();

  const update = (changes: Partial<Draft>) => setDraft((current) => ({ ...current, ...changes }));
  const title = draft.name || (saved ? saved.name : 'New automation');
  const back = (
    <IconButton aria-label="Back to automations" edge="start" onClick={() => navigate('/automations')}>
      <ArrowBackIcon />
    </IconButton>
  );
  const enabledSwitch = canEdit && (
    <FormControlLabel
      label="Enabled"
      control={<Switch checked={draft.enabled} onChange={(_, enabled) => update({ enabled })} />}
    />
  );
  const deleteButton = canDelete && saved && (
    <Button color="error" onClick={() => setConfirmingDelete(true)}>
      Delete
    </Button>
  );
  const saveButton = canEdit && (
    <Button
      variant="contained"
      disabled={save.isPending}
      onClick={() =>
        save.mutate(
          { id: saved?.id, input: inputOf(draft) },
          { onSuccess: (automation) => !saved && navigate(`/automations/${automation.id}`, { replace: true }) },
        )
      }
    >
      Save
    </Button>
  );
  const nameField = canEdit && (
    <TextField label="Name" value={draft.name} onChange={(event) => update({ name: event.target.value })} />
  );

  const plainWords = <PlainWordsCard draft={draft} saved={saved} />;
  const when = <WhenCard triggers={draft.triggers} readOnly={!canEdit} onChange={(triggers) => update({ triggers })} />;
  const then = <ThenCard steps={draft.steps} readOnly={!canEdit} onChange={(steps) => update({ steps })} />;
  const runs = <RecentRunsCard automationId={saved?.id} zone={saved?.hub_timezone ?? 'UTC'} />;

  return (
    <Page
      title={title}
      beforeTitle={back}
      actions={
        <>
          {enabledSwitch}
          {deleteButton}
          {saveButton}
        </>
      }
    >
      {!wide && (
        <Inline>
          {back}
          {enabledSwitch}
        </Inline>
      )}
      {save.error && <Alert severity="error">{save.error.message}</Alert>}
      {wide ? (
        <PageGrid>
          <PageGrid.Item span={{ wide: 8 }}>
            <Stack>
              {nameField}
              {when}
              {then}
            </Stack>
          </PageGrid.Item>
          <PageGrid.Item span={{ wide: 4 }}>
            <Stack>
              {plainWords}
              {runs}
            </Stack>
          </PageGrid.Item>
        </PageGrid>
      ) : (
        <Stack>
          {nameField}
          {plainWords}
          {when}
          {then}
          {runs}
        </Stack>
      )}
      {!wide && (deleteButton || saveButton) && (
        <StickyFooter>
          {deleteButton}
          {saveButton}
        </StickyFooter>
      )}
      {saved && (
        <DeleteDialog
          name={saved.name}
          open={confirmingDelete}
          error={remove.error}
          onClose={() => setConfirmingDelete(false)}
          onDelete={() => remove.mutate(saved.id, { onSuccess: () => navigate('/automations') })}
        />
      )}
    </Page>
  );
}

export default function AutomationEditorPage() {
  const { id } = useParams();
  const { user } = useAuth();
  const automationId = id === 'new' ? undefined : Number(id);
  const { data: saved, error, isPending } = useAutomation(automationId);
  const canEdit = hasPerm(user, 'manage_automations') && hasPerm(user, 'control_sensors');
  const canDelete = hasPerm(user, 'manage_automations');

  if (automationId === undefined && !canEdit) return <Navigate to="/automations" replace />;
  if (automationId !== undefined && (isPending || error)) {
    return (
      <Page title="Automation" loading={isPending}>
        {error && <Alert severity="error">{error.message}</Alert>}
      </Page>
    );
  }
  return <AutomationEditor key={automationId ?? 'new'} saved={saved} canEdit={canEdit} canDelete={canDelete} />;
}
