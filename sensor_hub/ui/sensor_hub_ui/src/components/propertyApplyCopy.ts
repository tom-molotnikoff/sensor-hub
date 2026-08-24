import type { PropertyDefinition } from '../gen/aliases';

export const ACTION_CHIP_LABELS: Record<string, string> = {
  'service-restart': 'Service restart required',
  'oauth-reload': 'OAuth reload required',
};

const ACTION_APPLY_SEGMENTS: Record<string, string> = {
  'service-restart': 'applies after a service restart',
  'oauth-reload': 'applies after an OAuth reload',
};

// Presentation copy about specific properties, keyed by property key. The page must
// hold no structural knowledge of properties, but consequence wording is frontend copy.
export const CONSEQUENCE_NOTES: Record<string, string> = {
  'mqtt.broker.enabled': 'Changing this disconnects connected sensors.',
  'mqtt.broker.port': 'Changing this disconnects connected sensors.',
  'oauth.credentials.file.path': 'Apply this with Reload OAuth on the Notifications page.',
  'oauth.token.file.path': 'Apply this with Reload OAuth on the Notifications page.',
};

export function applyAction(definition: PropertyDefinition): string | undefined {
  const prefix = 'action:';
  return definition.apply.startsWith(prefix) ? definition.apply.slice(prefix.length) : undefined;
}

// The apply-state fragment of a modified row's helper line. Live properties get none.
export function applySegment(definition: PropertyDefinition, serverValue: string): string | undefined {
  if (definition.apply === 'next-cycle') {
    // Both next-cycle properties are their own interval, so the saved value is the cycle length.
    const cycle = definition.unit ? `${serverValue} ${definition.unit}` : serverValue;
    return `applies from the next cycle (currently ${cycle})`;
  }
  const action = applyAction(definition);
  return action ? ACTION_APPLY_SEGMENTS[action] : undefined;
}
