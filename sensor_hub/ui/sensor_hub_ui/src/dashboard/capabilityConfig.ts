import type { Capability, Sensor } from '../gen/aliases';

export type CapabilityType = Capability['type'];

export function capabilitiesOfType(sensor: Sensor | null | undefined, type: CapabilityType): Capability[] {
  return (sensor?.capabilities ?? []).filter((capability) => capability.type === type);
}

export function sensorsWithCapabilitiesOfType(sensors: Sensor[], type: CapabilityType): Sensor[] {
  return sensors.filter((sensor) => capabilitiesOfType(sensor, type).length > 0);
}

// Keeps the chosen property while the sensor still offers it, otherwise falls
// back to the preferred one and then to the first offered.
export function normalizeCapabilityProperty(property: unknown, capabilities: Capability[], preferred: unknown): string {
  const currentProperty = typeof property === 'string' ? property : '';
  if (capabilities.some((capability) => capability.property === currentProperty)) {
    return currentProperty;
  }

  return capabilities.find((capability) => capability.property === preferred)?.property
    ?? capabilities[0]?.property
    ?? '';
}

interface Expose {
  property?: unknown;
  value_step?: unknown;
  features?: unknown;
}

// The capability carries no step, so it is read from the Zigbee2MQTT exposes
// the hub stores in the sensor's metadata. As on the hub, the features of a
// composite with a property of its own are nested under it, not the property.
export function capabilityStep(sensor: Sensor | null | undefined, property: string): number | undefined {
  const find = (exposes: unknown): number | undefined => {
    if (!Array.isArray(exposes)) return undefined;
    for (const expose of exposes as Expose[]) {
      if (expose == null || typeof expose !== 'object') continue;
      if (expose.property === property) {
        return typeof expose.value_step === 'number' && expose.value_step > 0 ? expose.value_step : undefined;
      }
      if (expose.property === undefined || expose.property === '') {
        const nested = find(expose.features);
        if (nested !== undefined) return nested;
      }
    }
    return undefined;
  };
  return find(sensor?.metadata?.exposes);
}
