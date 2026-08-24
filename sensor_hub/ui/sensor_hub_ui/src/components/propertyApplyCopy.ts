import type { PropertyDefinition } from '../gen/aliases';

const ACTION_CHIP_LABELS: Record<string, string> = {
  'service-restart': 'Service restart required',
  'oauth-reload': 'OAuth reload required',
};

const ACTION_APPLY_SEGMENTS: Record<string, string> = {
  'service-restart': 'applies after a service restart',
  'oauth-reload': 'applies after an OAuth reload',
};

// An action id the frontend has no copy for must never read as live - fall back to the raw id.
export function actionChipLabel(action: string): string {
  return ACTION_CHIP_LABELS[action] ?? `Requires: ${action}`;
}

function actionApplySegment(action: string): string {
  return ACTION_APPLY_SEGMENTS[action] ?? `applies after ${action}`;
}

// Presentation copy about specific properties, keyed by property key. The page must
// hold no structural knowledge of properties, but consequence wording is frontend copy.
export const CONSEQUENCE_NOTES: Record<string, string> = {
  'mqtt.broker.enabled': 'Changing this disconnects connected sensors.',
  'mqtt.broker.port': 'Changing this disconnects connected sensors.',
  'oauth.credentials.file.path': 'Apply this with Reload Config on the Notifications page.',
  'oauth.token.file.path': 'Apply this with Reload Config on the Notifications page.',
};

export function applyAction(definition: PropertyDefinition): string | undefined {
  const prefix = 'action:';
  return definition.apply.startsWith(prefix) ? definition.apply.slice(prefix.length) : undefined;
}

// The apply-state fragment of a modified row's helper line. Live properties get none.
export function applySegment(definition: PropertyDefinition, serverValue: string): string | undefined {
  if (definition.apply === 'next-cycle') {
    if (serverValue === '') return 'applies from the next cycle';
    // Both next-cycle properties are their own interval, so the saved value is the cycle length.
    // Registry units (seconds, minutes, hours, days) all pluralize regularly.
    const unit = definition.unit && (serverValue === '1' ? definition.unit.replace(/s$/, '') : definition.unit);
    const cycle = unit ? `${serverValue} ${unit}` : serverValue;
    return `applies from the next cycle (currently ${cycle})`;
  }
  const action = applyAction(definition);
  return action ? actionApplySegment(action) : undefined;
}
