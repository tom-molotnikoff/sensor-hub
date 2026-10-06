import type { Automation, AutomationInput, AutomationStep, AutomationTrigger, Capability, Sensor } from '../../gen/aliases';

type Keyed<T> = T & { key: number };

export type DraftTrigger = Keyed<AutomationTrigger>;
export type DraftStep = Keyed<AutomationStep>;

export interface Draft {
  name: string;
  enabled: boolean;
  triggers: DraftTrigger[];
  steps: DraftStep[];
}

let nextKey = 0;
const keyed = <T extends object>(item: T): Keyed<T> => ({ ...item, key: nextKey++ });

export const newTrigger = (): DraftTrigger => keyed({ type: 'schedule', at: '19:00', days: ['mon', 'tue', 'wed', 'thu', 'fri'] });

export const newStep = (): DraftStep => keyed({ type: 'set' });

export function draftOf(automation: Automation | undefined): Draft {
  if (!automation) return { name: '', enabled: true, triggers: [newTrigger()], steps: [newStep()] };
  return {
    name: automation.name,
    enabled: automation.enabled,
    triggers: automation.triggers.map(keyed),
    steps: automation.steps.map(keyed),
  };
}

export function inputOf({ name, enabled, triggers, steps }: Draft): AutomationInput {
  return {
    name,
    enabled,
    triggers: triggers.map(({ type, at, days }) => ({ type, at, days })),
    steps: steps.map(({ type, sensor_id, property, value }) => ({ type, sensor_id, property, value })),
  };
}

export function moved<T>(items: readonly T[], from: number, to: number): T[] {
  const result = [...items];
  const [item] = result.splice(from, 1);
  result.splice(to, 0, item);
  return result;
}

export const writableCapabilities = (sensor: Sensor | undefined): Capability[] => sensor?.capabilities ?? [];

export function defaultValue(capability: Capability | undefined): string | undefined {
  switch (capability?.type) {
    case 'binary':
      return capability.value_on;
    case 'numeric':
      return String(capability.min ?? 0);
    case 'enum':
      return capability.values?.[0];
    default:
      return undefined;
  }
}
