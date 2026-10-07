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

export const operatorNames: Record<NonNullable<AutomationTrigger['operator']>, string> = {
  falls_below: 'falls below',
  rises_above: 'rises above',
  becomes: 'becomes',
};

function describeReading(
  { sensor_id, measurement_type, operator, threshold, rearm_margin, value, hold_seconds }: AutomationTrigger,
  sensorName: (id: number) => string,
): string {
  if (sensor_id === undefined) return 'when a sensor reading changes';
  const series = `${sensorName(sensor_id)}${measurement_type ? ` ${measurement_type}` : ''}`;
  const condition = operator === 'becomes' ? `becomes ${value ?? '?'}` : `${operatorNames[operator ?? 'falls_below']} ${threshold ?? '?'}`;
  const margin = operator !== 'becomes' && rearm_margin !== undefined ? ` (margin ${rearm_margin})` : '';
  const hold = hold_seconds ? ` for at least ${formatDuration(hold_seconds)}` : '';
  return `when ${series} ${condition}${margin}${hold}`;
}

function describeTrigger(trigger: AutomationTrigger, sensorName: (id: number) => string): string {
  if (trigger.type === 'interval') return trigger.seconds === undefined ? 'every so often' : `every ${formatDuration(trigger.seconds)}`;
  if (trigger.type === 'reading') return describeReading(trigger, sensorName);
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
  const when = triggers.length > 0 ? triggers.map((trigger) => describeTrigger(trigger, sensorName)).join(' or ') : 'with no trigger';
  const then = steps.length > 0 ? steps.map((step) => describeStep(step, sensorName)).join(', then ') : 'do nothing';
  return `${when.charAt(0).toUpperCase()}${when.slice(1)}, ${then}.`;
}

export function formatHubTime(iso: string, zone: string): string {
  return DateTime.fromISO(iso, { zone: 'utc' }).setZone(zone).toFormat('ccc d LLL, HH:mm');
}

export function describeNext({ status, next_fire_at, hub_timezone, triggers }: Automation): string {
  if (status === 'off' || status === 'broken') return '-';
  if (next_fire_at) return formatHubTime(next_fire_at, hub_timezone);
  return triggers.some((trigger) => trigger.type === 'reading') ? 'on reading' : '-';
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
    case 'cancelled':
      return run.current_step === 0 ? `cancelled before step 1 of ${total}` : `cancelled on step ${run.current_step} of ${total}`;
    case 'skipped':
      return 'already running';
  }
}

export const showsFailedFlag = (automation: Automation) => automation.last_run_failed && automation.status === 'armed';

export function describeStatusDetail(automation: Automation): string {
  if (automation.status === 'broken') return automation.status_reason ?? '';
  return showsFailedFlag(automation) ? 'last run failed' : '';
}

export const automationStatus: Record<Automation['status'], { label: string; key: StatusKey }> = {
  off: { label: 'Off', key: 'unknown' },
  armed: { label: 'Armed', key: 'ok' },
  running: { label: 'Running', key: 'info' },
  broken: { label: 'Broken', key: 'bad' },
};

export const runStatus: Record<AutomationRun['status'], StatusKey> = {
  running: 'info',
  waiting: 'info',
  succeeded: 'ok',
  failed: 'bad',
  cancelled: 'unknown',
  missed: 'unknown',
  skipped: 'unknown',
};

const savedLists: Record<string, { name: string; fields: Record<string, string> }> = {
  triggers: {
    name: 'Trigger',
    fields: {
      at: 'time',
      days: 'weekdays',
      seconds: 'interval',
      sensor_id: 'sensor',
      measurement_type: 'measurement',
      operator: 'condition',
      rearm_margin: 're-arm margin',
      hold_seconds: '"for at least"',
    },
  },
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
