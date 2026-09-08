import type { PropertyDefinition } from '../gen/aliases';

export interface PropertyRejection {
  key?: string;
  message: string;
}

export interface PropertyErrors {
  fields: Map<string, string>;
  page: string | null;
}

const INTEGER = /^[+-]?\d+$/;

function ruleError(definition: PropertyDefinition, value: string): string | null {
  if (definition.readOnly) return null;

  if (definition.type === 'int') {
    if (!INTEGER.test(value)) return 'Must be a whole number';
    const parsed = Number(value);
    if (definition.validate === 'positive' && parsed <= 0) return 'Must be greater than 0';
    if (definition.validate === 'non_negative' && parsed < 0) return 'Must be 0 or more';
    return null;
  }

  if (definition.type === 'string' && definition.validate === 'non_empty' && value === '') {
    return 'Must not be empty';
  }

  return null;
}

export function asRejection(error: unknown): PropertyRejection {
  if (error && typeof error === 'object') {
    const body = error as { message?: unknown; key?: unknown };
    if (typeof body.message === 'string') {
      return { message: body.message, key: typeof body.key === 'string' ? body.key : undefined };
    }
  }
  if (typeof error === 'string') return { message: error };
  const serialised = JSON.stringify(error);
  return { message: typeof serialised === 'string' ? serialised : String(error) };
}

export function propertyErrors(
  definitions: PropertyDefinition[],
  edits: Record<string, string>,
  rejection: PropertyRejection | null,
): PropertyErrors {
  const fields = new Map<string, string>();

  for (const definition of definitions) {
    const value = edits[definition.key];
    if (value === undefined) continue;
    const message = ruleError(definition, value);
    if (message !== null) fields.set(definition.key, message);
  }

  if (rejection === null) return { fields, page: null };

  const { key, message } = rejection;
  if (key !== undefined && definitions.some((definition) => definition.key === key)) {
    fields.set(key, message);
    return { fields, page: null };
  }

  return { fields, page: message };
}
