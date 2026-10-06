import { DateTime } from 'luxon';
import type { Automation, AutomationRun, AutomationStep, AutomationTrigger } from '../../gen/aliases';
import type { StatusKey } from '../../ui/theme';

export type Weekday = NonNullable<AutomationTrigger['days']>[number];

export const weekdays: readonly { day: Weekday; letter: string; name: string }[] = [
  { day: 'mon', letter: 'M', name: 'Mon' },
  { day: 'tue', letter: 'T', name: 'Tue' },
  { day: 'wed', letter: 'W', name: 'Wed' },
  { day: 'thu', letter: 'T', name: 'Thu' },
  { day: 'fri', letter: 'F', name: 'Fri' },
  { day: 'sat', letter: 'S', name: 'Sat' },
  { day: 'sun', letter: 'S', name: 'Sun' },
];

const sameDays = (days: readonly Weekday[], expected: readonly Weekday[]) =>
  days.length === expected.length && expected.every((day) => days.includes(day));

function describeDays(days: readonly Weekday[]): string {
  if (days.length === 7) return 'every day';
  if (sameDays(days, ['mon', 'tue', 'wed', 'thu', 'fri'])) return 'on weekdays';
  if (sameDays(days, ['sat', 'sun'])) return 'at weekends';
  if (days.length === 0) return 'on no days';
  return `on ${weekdays.filter(({ day }) => days.includes(day)).map(({ name }) => name).join(', ')}`;
}

function describeTrigger(trigger: AutomationTrigger): string {
  return `${trigger.at ?? '--:--'} ${describeDays(trigger.days ?? [])}`;
}

function describeStep({ sensor_id, property, value }: AutomationStep, sensorName: (id: number) => string): string {
  if (sensor_id === undefined) return 'set a device';
  if (property === undefined) return `set ${sensorName(sensor_id)}`;
  return `set ${sensorName(sensor_id)} ${property} to ${value ?? '?'}`;
}

export function describeAutomation(
  { triggers, steps }: { triggers: readonly AutomationTrigger[]; steps: readonly AutomationStep[] },
  sensorName: (id: number) => string,
): string {
  const when = triggers.length > 0 ? `At ${triggers.map(describeTrigger).join(' or ')}` : 'With no trigger';
  const then = steps.length > 0 ? steps.map((step) => describeStep(step, sensorName)).join(', then ') : 'do nothing';
  return `${when}, ${then}.`;
}

export function formatHubTime(iso: string, zone: string): string {
  return DateTime.fromISO(iso, { zone: 'utc' }).setZone(zone).toFormat('ccc d LLL, HH:mm');
}

export function describeRun(run: AutomationRun): string {
  const total = run.steps.length;
  const count = `${total} ${total === 1 ? 'step' : 'steps'}`;
  switch (run.status) {
    case 'running':
      return `on step ${Math.max(run.current_step, 1)} of ${total}`;
    case 'succeeded':
      return `${count} · all succeeded`;
    case 'failed':
      return run.error ?? `failed on step ${run.current_step} of ${total}`;
  }
}

export const showsFailedFlag = (automation: Automation) => automation.last_run_failed && automation.status === 'armed';

export const automationStatus: Record<Automation['status'], { label: string; key: StatusKey }> = {
  off: { label: 'Off', key: 'unknown' },
  armed: { label: 'Armed', key: 'ok' },
  running: { label: 'Running', key: 'info' },
};

export const runStatus: Record<AutomationRun['status'], StatusKey> = {
  running: 'info',
  succeeded: 'ok',
  failed: 'bad',
};

const savedLists: Record<string, string> = { triggers: 'Trigger', steps: 'Step' };
const savedFields: Record<string, string> = { at: 'time', days: 'weekdays', sensor_id: 'device' };

// The API names the field as a 0-based JSON path, such as "steps[1].value"; the editor numbers cards from 1.
export function readableSaveError(message: string): string {
  const match = /^(triggers|steps)\[(\d+)\]\.(\w+)/.exec(message);
  if (!match) return message.charAt(0).toUpperCase() + message.slice(1);
  const [path, list, index, field] = match;
  return `${savedLists[list]} ${Number(index) + 1} ${savedFields[field] ?? field}${message.slice(path.length)}`;
}
