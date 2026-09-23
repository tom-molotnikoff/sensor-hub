export function propertyName(property) {
  if (property.computed) return property.key.type === 'Literal' ? String(property.key.value) : null;
  if (property.key.type === 'Identifier') return property.key.name;
  if (property.key.type === 'Literal') return String(property.key.value);
  return null;
}

function resolveIdentifier(context, identifier) {
  let scope = context.sourceCode.getScope(identifier);
  while (scope) {
    const variable = scope.set.get(identifier.name);
    if (variable) {
      const definition = variable.defs[0];
      if (definition?.type === 'Variable' && definition.parent.kind === 'const' && definition.node.init) {
        return definition.node.init;
      }
      return null;
    }
    scope = scope.upper;
  }
  return null;
}

function resolveMember(context, member) {
  if (member.computed || member.property.type !== 'Identifier') return null;
  const [owner] = objectsIn(context, member.object);
  const property = owner?.properties.find(
    (candidate) => candidate.type === 'Property' && propertyName(candidate) === member.property.name,
  );
  return property?.value ?? null;
}

function returnedExpressions(body) {
  if (body.type !== 'BlockStatement') return [body];
  return body.body.filter((statement) => statement.type === 'ReturnStatement' && statement.argument).map((statement) => statement.argument);
}

function objectsIn(context, node, seen = new Set()) {
  if (!node || seen.has(node)) return [];
  seen.add(node);
  switch (node.type) {
    case 'ObjectExpression':
      return [node];
    case 'ArrayExpression':
      return node.elements.flatMap((element) => objectsIn(context, element, seen));
    case 'ConditionalExpression':
      return [...objectsIn(context, node.consequent, seen), ...objectsIn(context, node.alternate, seen)];
    case 'LogicalExpression':
      return [...objectsIn(context, node.left, seen), ...objectsIn(context, node.right, seen)];
    case 'ArrowFunctionExpression':
    case 'FunctionExpression':
      return returnedExpressions(node.body).flatMap((expression) => objectsIn(context, expression, seen));
    case 'TSAsExpression':
    case 'TSSatisfiesExpression':
    case 'TSNonNullExpression':
      return objectsIn(context, node.expression, seen);
    case 'SpreadElement':
      return objectsIn(context, node.argument, seen);
    case 'Identifier':
      return objectsIn(context, resolveIdentifier(context, node), seen);
    case 'MemberExpression':
      return objectsIn(context, resolveMember(context, node), seen);
    default:
      return [];
  }
}

export function forEachProperty(context, value, visit, seen = new Set()) {
  for (const object of objectsIn(context, value, seen)) {
    for (const property of object.properties) {
      if (property.type === 'SpreadElement') {
        forEachProperty(context, property.argument, visit, seen);
        continue;
      }
      if (visit(property) !== false) forEachProperty(context, property.value, visit, seen);
    }
  }
}
