import type { PropertyDefinition, PropertyDefinitionsResponse, PropertyGroup } from '../gen/aliases';

export interface PropertyRow {
  definition: PropertyDefinition;
  described: boolean;
}

export interface PropertySection {
  group: PropertyGroup;
  rows: PropertyRow[];
}

const UNGROUPED: PropertyGroup = {
  id: 'ungrouped',
  label: 'Ungrouped',
  description: 'Properties the definitions do not describe.',
  order: Number.MAX_SAFE_INTEGER,
};

function undescribed(key: string): PropertyRow {
  return {
    definition: {
      key,
      label: key,
      description: '',
      type: 'string',
      default: '',
      group: UNGROUPED.id,
      apply: 'live',
      readOnly: false,
    },
    described: false,
  };
}

export function buildSections(
  definitions: PropertyDefinitionsResponse | null,
  valueKeys: string[],
): PropertySection[] {
  const defined = definitions?.definitions ?? [];
  const groups = definitions?.groups ?? [];
  const groupIds = new Set(groups.map((group) => group.id));
  const definedKeys = new Set(defined.map((definition) => definition.key));

  const sections = [...groups]
    .sort((a, b) => a.order - b.order)
    .map((group) => ({
      group,
      rows: defined
        .filter((definition) => definition.group === group.id)
        .map((definition) => ({ definition, described: true })),
    }));

  const ungrouped: PropertyRow[] = [
    ...defined
      .filter((definition) => !groupIds.has(definition.group))
      .map((definition) => ({ definition, described: true })),
    ...[...valueKeys].sort().filter((key) => !definedKeys.has(key)).map(undescribed),
  ];
  if (ungrouped.length > 0) sections.push({ group: UNGROUPED, rows: ungrouped });

  return sections;
}
