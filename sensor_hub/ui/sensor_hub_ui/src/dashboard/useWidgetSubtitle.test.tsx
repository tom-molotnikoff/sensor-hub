import { renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useWidgetSubtitle } from './useWidgetSubtitle';
import type { MeasurementTypeInfo, Sensor } from '../gen/aliases';

const sensors: Sensor[] = [];
const properties: Record<string, string> = {};
const measurementTypes: MeasurementTypeInfo[] = [];
const requestedTypes: boolean[] = [];

function sensor(id: number, name: string): Sensor {
  return {
    id,
    name,
    external_id: name,
    sensor_driver: 'zigbee2mqtt',
    config: {},
    metadata: {},
    capabilities: [],
    health_status: 'good',
    health_reason: 'ok',
    enabled: true,
    status: 'active',
    retention_hours: null,
  };
}

function measurementType(name: string, displayName: string): MeasurementTypeInfo {
  return {
    id: 1,
    name,
    display_name: displayName,
    category: 'numeric',
    unit: 'kWh',
    default_aggregation_function: 'avg',
    supported_aggregation_functions: ['avg'],
  };
}

vi.mock('../hooks/useSensorContext', () => ({
  useSensorContext: () => ({
    sensors,
  }),
}));

vi.mock('../hooks/useProperties', () => ({
  useProperties: () => properties,
}));

vi.mock('../hooks/useMeasurementTypes', () => ({
  useMeasurementTypes: (enabled: boolean) => {
    requestedTypes.push(enabled);
    return measurementTypes;
  },
}));

describe('useWidgetSubtitle', () => {
  beforeEach(() => {
    sensors.splice(0, sensors.length);
    Object.keys(properties).forEach((key) => delete properties[key]);
    measurementTypes.splice(0, measurementTypes.length, measurementType('energy', 'Energy'), measurementType('voltage', 'Voltage'));
    requestedTypes.splice(0, requestedTypes.length);
  });

  it('shows the measurement type and aggregation function on readings charts', () => {
    const { result } = renderHook(() => useWidgetSubtitle('readings-chart', { measurementType: 'energy', aggregationFunction: 'increase' }));

    expect(result.current).toBe('Energy · increase');
  });

  it('leaves the function off readings charts on auto', () => {
    const { result } = renderHook(() => useWidgetSubtitle('readings-chart', { measurementType: 'energy', aggregationFunction: '' }));

    expect(result.current).toBe('Energy');
  });

  it('falls back to the measurement type name before the types have loaded', () => {
    measurementTypes.splice(0, measurementTypes.length);

    const { result } = renderHook(() => useWidgetSubtitle('readings-chart', { measurementType: 'energy' }));

    expect(result.current).toBe('energy');
  });

  it('shows the measurement type, function and sensors on comparison charts', () => {
    sensors.splice(0, sensors.length, sensor(7, 'office-plug'), sensor(8, 'kitchen-plug'));

    const { result } = renderHook(() => useWidgetSubtitle('comparison-chart', { measurementType: 'voltage', aggregationFunction: 'max', sensorIds: [8, 7] }));

    expect(result.current).toBe('Voltage · max · kitchen-plug, office-plug');
  });

  it('only asks for measurement types on chart widgets', () => {
    renderHook(() => useWidgetSubtitle('gauge', { sensorId: 7 }));

    expect(requestedTypes).toEqual([false]);
  });

  it('includes the selected property for sensor toggle widgets', () => {
    sensors.splice(0, sensors.length, {
      id: 7,
      name: 'office-plug',
      external_id: 'office-plug',
      sensor_driver: 'zigbee2mqtt',
      config: {},
      metadata: {},
      capabilities: [],
      health_status: 'good',
      health_reason: 'ok',
      enabled: true,
      status: 'active',
      retention_hours: null,
    });

    const { result } = renderHook(() => useWidgetSubtitle('sensor-toggle', { sensorId: 7, property: 'state' }));

    expect(result.current).toBe('office-plug · state');
  });

  it('keeps the existing sensor-only subtitle for other sensor widgets', () => {
    sensors.splice(0, sensors.length, {
      id: 7,
      name: 'office-plug',
      external_id: 'office-plug',
      sensor_driver: 'zigbee2mqtt',
      config: {},
      metadata: {},
      capabilities: [],
      health_status: 'good',
      health_reason: 'ok',
      enabled: true,
      status: 'active',
      retention_hours: null,
    });

    const { result } = renderHook(() => useWidgetSubtitle('gauge', { sensorId: 7, property: 'state' }));

    expect(result.current).toBe('office-plug');
  });
});
