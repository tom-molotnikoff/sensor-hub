import { useState } from 'react';
import type { WidgetProps } from '../types';
import type { Capability } from '../../gen/aliases';
import { useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';
import { useSensorCommand } from '../useSensorCommand';
import CommandNotice from '../CommandNotice';
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
  const command = useSensorCommand({ sensor, property, onDataUpdate: reportUpdate });
  const reading = sensor && property ? command.readings[sensor.name]?.[property] : undefined;

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
    await command.send(nextValue, () => setOptimisticValue(previousValue));
  };

  return (
    <>
      <SlideSwitch
        checked={resolvedCheckedState}
        readOnly={!canControl}
        label={`Toggle ${sensor.name} ${property}`}
        onChange={(next) => void commitCheckedState(next)}
      />
      <CommandNotice command={command} />
    </>
  );
}
