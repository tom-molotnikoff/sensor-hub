import { describe, expect, it } from 'vitest';
import type { Capability, Sensor } from '../gen/aliases';
import { capabilitiesOfType, capabilityStep, normalizeCapabilityProperty, sensorsWithCapabilitiesOfType } from './capabilityConfig';

function makeCapability(overrides: Partial<Capability> = {}): Capability {
  return {
    property: 'state',
    type: 'binary',
    value_on: 'ON',
    value_off: 'OFF',
    ...overrides,
  };
}

function makeSensor(overrides: Partial<Sensor> = {}): Sensor {
  return {
    id: 7,
    name: 'office-plug',
    external_id: 'office-plug',
    sensor_driver: 'zigbee2mqtt',
    config: {},
    metadata: {},
    capabilities: [makeCapability()],
    health_status: 'good',
    health_reason: 'ok',
    enabled: true,
    status: 'active',
    retention_hours: null,
    ...overrides,
  };
}

const brightness = makeCapability({ property: 'brightness', type: 'numeric', min: 0, max: 254, value_on: undefined, value_off: undefined });

describe('capabilityConfig', () => {
  it('offers as targets only the sensors with a capability of the widget\'s type', () => {
    const sensors = [
      makeSensor(),
      makeSensor({ id: 8, name: 'attic-bulb', capabilities: [makeCapability(), brightness] }),
    ];

    expect(sensorsWithCapabilitiesOfType(sensors, 'binary').map((sensor) => sensor.name)).toEqual(['office-plug', 'attic-bulb']);
    expect(sensorsWithCapabilitiesOfType(sensors, 'numeric').map((sensor) => sensor.name)).toEqual(['attic-bulb']);
  });

  it('keeps a property the sensor still offers and otherwise falls back to the preferred one, then the first', () => {
    const numeric = capabilitiesOfType(makeSensor({
      capabilities: [makeCapability(), makeCapability({ property: 'color_temp', type: 'numeric' }), brightness],
    }), 'numeric');

    expect(numeric.map((capability) => capability.property)).toEqual(['color_temp', 'brightness']);
    expect(normalizeCapabilityProperty('color_temp', numeric, 'brightness')).toBe('color_temp');
    expect(normalizeCapabilityProperty('effect', numeric, 'brightness')).toBe('brightness');
    expect(normalizeCapabilityProperty('effect', numeric, 'state')).toBe('color_temp');
    expect(normalizeCapabilityProperty('brightness', [], 'brightness')).toBe('');
  });

  it('reads the step from the exposes, but not from a feature nested under a composite property', () => {
    const sensor = makeSensor({
      metadata: {
        exposes: [
          { type: 'light', features: [
            { type: 'numeric', property: 'brightness', value_min: 0, value_max: 254, value_step: 2 },
            { type: 'composite', property: 'level_config', features: [
              { type: 'numeric', property: 'current_level_startup', value_step: 5 },
            ] },
          ] },
          { type: 'numeric', property: 'color_temp', value_min: 250, value_max: 454 },
        ],
      },
    });

    expect(capabilityStep(sensor, 'brightness')).toBe(2);
    expect(capabilityStep(sensor, 'color_temp')).toBeUndefined();
    expect(capabilityStep(sensor, 'current_level_startup')).toBeUndefined();
    expect(capabilityStep(makeSensor(), 'brightness')).toBeUndefined();
  });
});
