import { useState } from 'react';
import type { WidgetProps } from '../types';
import { useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useAuth } from '../../providers/AuthContext';
import { hasPerm } from '../../tools/Utils';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';
import { capabilityStep } from '../capabilityConfig';
import { useSensorCommand, type CommandPhase } from '../useSensorCommand';
import CommandNotice from '../CommandNotice';
import ValueSlider from '../../ui/ValueSlider';

const outcomeCaption: Partial<Record<CommandPhase, string>> = {
  acknowledged: 'Acknowledged',
  failed: 'Command failed',
  timed_out: 'Command timed out',
  unsent: 'Command not sent',
};

export default function SensorSliderWidget({ config }: WidgetProps) {
  const { sensors } = useSensorContext();
  const { user } = useAuth();
  const reportUpdate = useReportWidgetUpdate();
  useWidgetStateReport(useCurrentReadingsReady() ? 'populated' : 'loading');

  const sensorId = config.sensorId as number | undefined;
  const property = config.property as string | undefined;
  const sensor = sensorId ? sensors.find((candidate) => candidate.id === sensorId) : undefined;
  const capability = sensor && property
    ? sensor.capabilities?.find((each) => each.type === 'numeric' && each.property === property)
    : undefined;
  const [sentValue, setSentValue] = useState<number | null>(null);
  const command = useSensorCommand({ sensor, property, onDataUpdate: reportUpdate });
  const reading = sensor && property ? command.readings[sensor.name]?.[property] : undefined;

  if (!sensor || !property || !capability) {
    return <NeedsConfiguration message="Select a controllable sensor and numeric property" />;
  }

  const pending = command.phase === 'pending' && sentValue !== null;
  const value = pending ? sentValue : reading?.numeric_value ?? null;
  const caption = pending
    ? `Setting to ${sentValue}…`
    : outcomeCaption[command.phase] ?? (reading ? undefined : 'Waiting for a reading');

  const commit = (next: number) => {
    reportUpdate(new Date());
    setSentValue(next);
    void command.send(String(next));
  };

  return (
    <>
      <ValueSlider
        value={value}
        min={capability.min ?? 0}
        max={capability.max ?? 100}
        step={capabilityStep(sensor, property) ?? 1}
        unit={capability.unit}
        label={`Set ${sensor.name} ${property}`}
        caption={caption}
        readOnly={!hasPerm(user, 'control_sensors')}
        onCommit={commit}
      />
      <CommandNotice command={command} />
    </>
  );
}
