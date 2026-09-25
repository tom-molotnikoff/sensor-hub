import { propertyName } from './style-objects.js';

const colourLiteral = /(?:^|[^\w&])(?<colour>#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})(?![\w-])|\brgba?\([^)]*\)?)/;
const numericText = /^\s*[-+]?(\d|\.\d)/;
const typeKeys = /^(fontSize|fontWeight|lineHeight)$/;

function isNumeric(node) {
  if (!node) return false;
  switch (node.type) {
    case 'Literal':
      return typeof node.value === 'number' || (typeof node.value === 'string' && numericText.test(node.value));
    case 'TemplateLiteral':
      return numericText.test(node.quasis[0].value.cooked ?? '');
    case 'UnaryExpression':
      return isNumeric(node.argument);
    case 'ConditionalExpression':
      return isNumeric(node.consequent) || isNumeric(node.alternate);
    case 'LogicalExpression':
      return isNumeric(node.left) || isNumeric(node.right);
    case 'ObjectExpression':
      return node.properties.some((property) => property.type === 'Property' && isNumeric(property.value));
    case 'JSXExpressionContainer':
    case 'TSAsExpression':
    case 'TSSatisfiesExpression':
      return isNumeric(node.expression);
    default:
      return false;
  }
}

export default {
  meta: {
    type: 'problem',
    docs: { description: 'Colours, type sizes and line heights come from the theme' },
    messages: {
      colour: 'Raw colour "{{colour}}". Use a theme palette path instead.',
      typeScale: 'Numeric {{key}}. Use a typography variant or theme value instead.',
    },
    schema: [],
  },
  create(context) {
    const checkText = (node, text) => {
      const colour = typeof text === 'string' ? colourLiteral.exec(text)?.groups.colour : undefined;
      if (colour) context.report({ node, messageId: 'colour', data: { colour } });
    };
    return {
      Literal(node) {
        checkText(node, node.value);
      },
      TemplateElement(node) {
        checkText(node, node.value.cooked);
      },
      Property(node) {
        const key = propertyName(node);
        if (key && typeKeys.test(key) && isNumeric(node.value)) {
          context.report({ node: node.key, messageId: 'typeScale', data: { key } });
        }
      },
      JSXAttribute(node) {
        if (node.name.type === 'JSXIdentifier' && typeKeys.test(node.name.name) && isNumeric(node.value)) {
          context.report({ node: node.name, messageId: 'typeScale', data: { key: node.name.name } });
        }
      },
    };
  },
};
