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
  if (trigger.type === 'interval') return trigger.seconds === undefined ? 'every so often' : `every ${formatDuration(trigger.seconds)}`;
  return `at ${trigger.at ?? '--:--'} ${describeDays(trigger.days ?? [])}`;
}

const durationUnits: readonly [number, string][] = [
  [86_400, 'd'],
  [3_600, 'h'],
  [60, 'min'],
  [1, 's'],
];

export function formatDuration(totalSeconds: number): string {
  let rest = totalSeconds;
  const parts: string[] = [];
  for (const [size, unit] of durationUnits) {
    const count = Math.floor(rest / size);
    rest -= count * size;
    if (count > 0) parts.push(`${count} ${unit}`);
  }
  return parts.slice(0, 2).join(' ') || '0 s';
}

function describeStep({ type, sensor_id, property, value, seconds }: AutomationStep, sensorName: (id: number) => string): string {
  if (type === 'wait') return seconds === undefined ? 'wait' : `wait ${formatDuration(seconds)}`;
  if (sensor_id === undefined) return 'set a device';
  if (property === undefined) return `set ${sensorName(sensor_id)}`;
  return `set ${sensorName(sensor_id)} ${property} to ${value ?? '?'}`;
}

export function describeAutomation(
  { triggers, steps }: { triggers: readonly AutomationTrigger[]; steps: readonly AutomationStep[] },
  sensorName: (id: number) => string,
): string {
  const when = triggers.length > 0 ? triggers.map(describeTrigger).join(' or ') : 'with no trigger';
  const then = steps.length > 0 ? steps.map((step) => describeStep(step, sensorName)).join(', then ') : 'do nothing';
  return `${when.charAt(0).toUpperCase()}${when.slice(1)}, ${then}.`;
}

export function formatHubTime(iso: string, zone: string): string {
  return DateTime.fromISO(iso, { zone: 'utc' }).setZone(zone).toFormat('ccc d LLL, HH:mm');
}

export function describeRun(run: AutomationRun, zone: string): string {
  const total = run.steps.length;
  const count = `${total} ${total === 1 ? 'step' : 'steps'}`;
  switch (run.status) {
    case 'running':
      return `on step ${Math.max(run.current_step, 1)} of ${total}`;
    case 'waiting':
      return `waiting · step ${run.current_step} of ${total}${run.resume_at ? ` · resumes ${formatHubTime(run.resume_at, zone)}` : ''}`;
    case 'missed':
      return `hub was down - ${formatDuration(run.past_grace_seconds ?? 0)} past the grace window`;
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
  waiting: 'info',
  succeeded: 'ok',
  failed: 'bad',
  missed: 'unknown',
};

const savedLists: Record<string, { name: string; fields: Record<string, string> }> = {
  triggers: { name: 'Trigger', fields: { at: 'time', days: 'weekdays', seconds: 'interval' } },
  steps: { name: 'Step', fields: { sensor_id: 'device', seconds: 'wait' } },
};

// The API names the field as a 0-based JSON path, such as "steps[1].value"; the editor numbers cards from 1.
export function readableSaveError(message: string): string {
  const match = /^(triggers|steps)\[(\d+)\]\.(\w+)/.exec(message);
  if (!match) return message.charAt(0).toUpperCase() + message.slice(1);
  const [path, list, index, field] = match;
  const { name, fields } = savedLists[list];
  return `${name} ${Number(index) + 1} ${fields[field] ?? field}${message.slice(path.length)}`;
}
