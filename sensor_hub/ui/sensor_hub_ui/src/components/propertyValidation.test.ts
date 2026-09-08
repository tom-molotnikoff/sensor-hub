import { describe, expect, it } from 'vitest';
import type { PropertyDefinition } from '../gen/aliases';
import { asRejection, propertyErrors } from './propertyValidation';

function makeDefinition(overrides: Partial<PropertyDefinition> = {}): PropertyDefinition {
  return {
    key: 'sensor.collection.interval',
    label: 'Collection interval',
    description: 'How often every enabled sensor is polled.',
    type: 'int',
    default: '300',
    group: 'sensors',
    apply: 'live',
    readOnly: false,
    ...overrides,
  };
}

function errorFor(definition: PropertyDefinition, value: string): string | undefined {
  return propertyErrors([definition], { [definition.key]: value }, null).fields.get(definition.key);
}

describe('propertyErrors', () => {
  it('rejects an int value that is not a whole number', () => {
    const definition = makeDefinition();

    expect(errorFor(definition, '30o')).toBe('Must be a whole number');
    expect(errorFor(definition, '1.5')).toBe('Must be a whole number');
    expect(errorFor(definition, ' 12')).toBe('Must be a whole number');
    expect(errorFor(definition, '1e3')).toBe('Must be a whole number');
    expect(errorFor(definition, '-12')).toBeUndefined();
    expect(errorFor(definition, '+12')).toBeUndefined();
  });

  it('rejects an emptied int, which the backend would silently store as zero', () => {
    expect(errorFor(makeDefinition({ validate: 'positive' }), '')).toBe('Must be a whole number');
  });

  it('applies positive and non_negative to int values', () => {
    const positive = makeDefinition({ validate: 'positive' });
    expect(errorFor(positive, '0')).toBe('Must be greater than 0');
    expect(errorFor(positive, '-1')).toBe('Must be greater than 0');
    expect(errorFor(positive, '1')).toBeUndefined();

    const nonNegative = makeDefinition({ validate: 'non_negative' });
    expect(errorFor(nonNegative, '-1')).toBe('Must be 0 or more');
    expect(errorFor(nonNegative, '0')).toBeUndefined();
  });

  it('applies non_empty to string values', () => {
    const definition = makeDefinition({ key: 'weather.location.name', type: 'string', validate: 'non_empty' });

    expect(errorFor(definition, '')).toBe('Must not be empty');
    expect(errorFor(definition, 'Manchester')).toBeUndefined();
  });

  it('leaves a bool and a read-only property unvalidated', () => {
    expect(errorFor(makeDefinition({ type: 'bool' }), 'neither')).toBeUndefined();
    expect(errorFor(makeDefinition({ readOnly: true, validate: 'positive' }), '-1')).toBeUndefined();
  });

  it('validates only what has been edited, so untouched server values cannot block a save', () => {
    const definition = makeDefinition({ validate: 'positive' });

    expect(propertyErrors([definition], {}, null).fields.size).toBe(0);
  });

  it('attaches a rejection naming a rendered field to that field', () => {
    const definition = makeDefinition();
    const rejection = { key: definition.key, message: 'invalid sensor.collection.interval value: 0' };

    const errors = propertyErrors([definition], {}, rejection);

    expect(errors.fields.get(definition.key)).toBe(rejection.message);
    expect(errors.page).toBeNull();
  });

  it('shows a rejection at page level when its key names no rendered field', () => {
    const errors = propertyErrors([makeDefinition()], {}, { key: 'ghost.property', message: 'invalid ghost.property value: x' });

    expect(errors.fields.size).toBe(0);
    expect(errors.page).toBe('invalid ghost.property value: x');
  });

  it('shows a rejection carrying no key at page level', () => {
    const errors = propertyErrors([makeDefinition()], {}, { message: 'Invalid request body' });

    expect(errors.fields.size).toBe(0);
    expect(errors.page).toBe('Invalid request body');
  });
});

describe('asRejection', () => {
  it('reads the message and key of a properties error body', () => {
    expect(asRejection({ message: 'invalid mqtt.broker.port value: 0', key: 'mqtt.broker.port' })).toEqual({
      message: 'invalid mqtt.broker.port value: 0',
      key: 'mqtt.broker.port',
    });
  });

  it('leaves the key undefined when the body carries none', () => {
    expect(asRejection({ message: 'Invalid request body' })).toEqual({ message: 'Invalid request body' });
  });

  it('falls back to a readable message for anything else', () => {
    expect(asRejection('network down')).toEqual({ message: 'network down' });
    expect(asRejection({ status: 500 })).toEqual({ message: '{"status":500}' });
    expect(asRejection(undefined)).toEqual({ message: 'undefined' });
  });
});
