import { describe, expect, it } from 'vitest';
import type { PropertyDefinition, PropertyDefinitionsResponse } from '../gen/aliases';
import { buildSections } from './propertySections';

function makeDefinition(overrides: Partial<PropertyDefinition> = {}): PropertyDefinition {
  return {
    key: 'sensor.discovery.skip',
    label: 'Skip sensor discovery',
    description: "Don't try to auto-discover sensors at startup.",
    type: 'bool',
    default: 'false',
    group: 'sensors',
    apply: 'live',
    readOnly: false,
    ...overrides,
  };
}

const response: PropertyDefinitionsResponse = {
  definitions: [
    makeDefinition(),
    makeDefinition({ key: 'database.path', label: 'Database file', group: 'advanced' }),
  ],
  groups: [
    { id: 'advanced', label: 'Advanced', description: 'Rarely-changed settings.', order: 2 },
    { id: 'sensors', label: 'Sensors & collection', description: 'How often sensors are polled.', order: 1 },
  ],
};

describe('buildSections', () => {
  it('orders the groups by order and puts each definition under the group it names', () => {
    const sections = buildSections(response, ['sensor.discovery.skip', 'database.path']);

    expect(sections.map((section) => section.group.id)).toEqual(['sensors', 'advanced']);
    expect(sections[0].fields.map((field) => field.key)).toEqual(['sensor.discovery.skip']);
    expect(sections[1].fields.map((field) => field.key)).toEqual(['database.path']);
  });

  it('puts a definition naming a group outside the groups list into Ungrouped rather than dropping it', () => {
    const stray = makeDefinition({ key: 'stray.property', label: 'Stray', group: 'nowhere' });
    const sections = buildSections(
      { ...response, definitions: [...response.definitions, stray] },
      [],
    );

    const ungrouped = sections[sections.length - 1];
    expect(ungrouped.group.id).toBe('ungrouped');
    expect(ungrouped.fields).toEqual([stray]);
  });

  it('puts a value with no definition into Ungrouped as an editable string carrying its raw key', () => {
    const sections = buildSections(response, ['sensor.discovery.skip', 'unknown.key']);

    const ungrouped = sections[sections.length - 1];
    expect(ungrouped.group.id).toBe('ungrouped');
    expect(ungrouped.fields).toEqual([
      {
        key: 'unknown.key',
        label: 'unknown.key',
        description: '',
        type: 'string',
        default: '',
        group: 'ungrouped',
        apply: 'live',
        readOnly: false,
      },
    ]);
  });

  it('collects every value into one Ungrouped section, sorted, when there are no definitions at all', () => {
    const sections = buildSections(null, ['sensor.discovery.skip', 'database.path']);

    expect(sections).toHaveLength(1);
    expect(sections[0].group.label).toBe('Ungrouped');
    expect(sections[0].fields.map((field) => field.key)).toEqual([
      'database.path',
      'sensor.discovery.skip',
    ]);
  });

  it('adds no Ungrouped section when every definition has a group and every value has a definition', () => {
    const sections = buildSections(response, ['sensor.discovery.skip', 'database.path']);

    expect(sections.map((section) => section.group.id)).not.toContain('ungrouped');
  });
});
