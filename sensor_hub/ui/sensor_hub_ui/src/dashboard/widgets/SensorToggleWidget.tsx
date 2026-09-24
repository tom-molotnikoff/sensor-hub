import { useRef, useState } from 'react';
import { Alert, Snackbar } from '@mui/material';
import type { WidgetProps } from '../types';
import type { Capability, CommandStatusMessage } from '../../gen/aliases';
import { apiClient } from '../../gen/client';
import { requestScheduler } from '../../scheduler/requestScheduler';
import { useCurrentReadings, useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';
import SlideSwitch from '../../ui/SlideSwitch';

function resolveBinaryCapability(
  capabilities: Capability[] | undefined,
  property: string,
): Capability | undefined {
  return capabilities?.find((capability) => capability.type === 'binary' && capability.property === property);
}

function normalizeBinaryStateToken(value: string | null | undefined): string | null {
  if (value == null) return null;

  const normalized = value.trim().toLowerCase();
  if (!normalized) return null;

  if (['on', 'true', '1', 'enabled', 'enable'].includes(normalized)) {
    return 'on';
  }

  if (['off', 'false', '0', 'disabled', 'disable'].includes(normalized)) {
    return 'off';
  }

  return normalized;
}

function resolveCheckedState(
  value: string | null | undefined,
  valueOn: string,
  valueOff: string,
): boolean | null {
  const currentToken = normalizeBinaryStateToken(value);
  const onToken = normalizeBinaryStateToken(valueOn);
  const offToken = normalizeBinaryStateToken(valueOff);

  if (currentToken == null || onToken == null || offToken == null) {
    return null;
  }

  if (currentToken === onToken) return true;
  if (currentToken === offToken) return false;
  return null;
}

export default function SensorToggleWidget({ config }: WidgetProps) {
  const { sensors } = useSensorContext();
  const { user } = useAuth();
  const reportUpdate = useReportWidgetUpdate();
  useWidgetStateReport(useCurrentReadingsReady() ? 'populated' : 'loading');

  const sensorId = config.sensorId as number | undefined;
  const property = config.property as string | undefined;
  const sensor = sensorId ? sensors.find((candidate) => candidate.id === sensorId) : undefined;
  const capability = sensor && property ? resolveBinaryCapability(sensor.capabilities, property) : undefined;
  const valueOn = capability?.value_on ?? 'ON';
  const valueOff = capability?.value_off ?? 'OFF';
  const [optimisticValue, setOptimisticValue] = useState<string | null>(null);
  const pendingCommandRef = useRef<{ id: number; previousValue: string | null } | null>(null);
  const [snackbarMessage, setSnackbarMessage] = useState<string | null>(null);

  // useCurrentReadings keeps callbacks in refs, so this needs no memoization.

  const handleCommandStatus = (message: CommandStatusMessage) => {
    const pendingCommand = pendingCommandRef.current;
    if (!pendingCommand || !sensor || !property) return;
    if (message.id !== pendingCommand.id || message.sensor_id !== sensor.id || message.property !== property) return;

    if (message.status === 'failed' || message.status === 'timed_out') {
      setOptimisticValue(pendingCommand.previousValue);
      setSnackbarMessage(message.status === 'timed_out' ? 'Command timed out' : 'Command failed');
      reportUpdate(new Date());
    }

    pendingCommandRef.current = null;
  };

  const readings = useCurrentReadings({ onDataUpdate: reportUpdate, onCommandStatus: handleCommandStatus });
  const reading = sensor && property ? readings[sensor.name]?.[property] : undefined;

  // Drop the optimistic value once the server confirms it (adjust-during-render).

  if (
    optimisticValue != null
    && resolveCheckedState(optimisticValue, valueOn, valueOff) != null
    && resolveCheckedState(optimisticValue, valueOn, valueOff) === resolveCheckedState(reading?.text_state, valueOn, valueOff)
  ) {
    setOptimisticValue(null);
  }

  const effectiveValue = optimisticValue ?? reading?.text_state ?? null;
  const resolvedCheckedState = resolveCheckedState(effectiveValue, valueOn, valueOff);
  const canControl = hasPerm(user, 'control_sensors');
  const canInteract = canControl && resolvedCheckedState != null;

  if (!sensor || !property || !capability) {
    return <NeedsConfiguration message="Select a controllable sensor and binary property" />;
  }

  const commitCheckedState = async (nextChecked: boolean) => {
    if (!canInteract) return;

    const previousValue = reading?.text_state ?? null;
    const nextValue = nextChecked ? valueOn : valueOff;
    if (nextValue === effectiveValue) return;

    setOptimisticValue(nextValue);
    reportUpdate(new Date());

    // Send the command immediately and pause low-priority background polls for its duration,

    // so the command (and its confirmation) aren't queued behind the read-only chart flood.

    const { data, error } = await requestScheduler.runWithPreemption(() => apiClient.POST('/sensors/{id}/command', {
      params: { path: { id: sensor.id } },
      body: { property, value: nextValue },
    }));

    if (error) {
      setOptimisticValue(previousValue);
      setSnackbarMessage('Failed to send command');
      return;
    }

    if (data) {
      pendingCommandRef.current = { id: data.id, previousValue };
    }
  };

  return (
    <>
      <SlideSwitch
        checked={resolvedCheckedState}
        readOnly={!canControl}
        label={`Toggle ${sensor.name} ${property}`}
        onChange={(next) => void commitCheckedState(next)}
      />
      <Snackbar
        open={snackbarMessage != null}
        autoHideDuration={3000}
        onClose={() => setSnackbarMessage(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity="error" onClose={() => setSnackbarMessage(null)}>
          {snackbarMessage}
        </Alert>
      </Snackbar>
    </>
  );
}
