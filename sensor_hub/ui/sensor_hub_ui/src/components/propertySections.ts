import type { PropertyDefinition, PropertyDefinitionsResponse, PropertyGroup } from '../gen/aliases';

export interface PropertySection {
  group: PropertyGroup;
  fields: PropertyDefinition[];
}

const UNGROUPED: PropertyGroup = {
  id: 'ungrouped',
  label: 'Ungrouped',
  description: 'Properties the definitions do not describe.',
  order: Number.MAX_SAFE_INTEGER,
};

function undescribed(key: string): PropertyDefinition {
  return {
    key,
    label: key,
    description: '',
    type: 'string',
    default: '',
    group: UNGROUPED.id,
    apply: 'live',
    readOnly: false,
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
      fields: defined.filter((definition) => definition.group === group.id),
    }));

  const ungrouped = [
    ...defined.filter((definition) => !groupIds.has(definition.group)),
    ...[...valueKeys].sort().filter((key) => !definedKeys.has(key)).map(undescribed),
  ];
  if (ungrouped.length > 0) sections.push({ group: UNGROUPED, fields: ungrouped });

  return sections;
}
