import { forEachStyleProperty, objectsIn, propertyName } from './style-objects.js';

const layoutKey =
  /^(display|flex[A-Za-z]*|width|height|min[A-Z][A-Za-z]*|max[A-Z][A-Za-z]*|gap|rowGap|columnGap|grid[A-Za-z]*|position|inset[A-Za-z]*|overflow|overflowX|overflowY|margin[A-Za-z]*|padding[A-Za-z]*|[mp][trblxy]?)$/;

const styleAttribute = /^(sx|style)$/;
const propsAttribute = /Props$/;

export default {
  meta: {
    type: 'problem',
    docs: { description: 'Layout belongs to the src/ui primitives, not to style objects' },
    messages: {
      layoutKey: '"{{key}}" is a layout style. Lay components out with the src/ui primitives instead.',
    },
    schema: [],
  },
  create(context) {
    const checkStyle = (styleValue) =>
      forEachStyleProperty(context, styleValue, (property) => {
        const key = propertyName(property);
        if (key && layoutKey.test(key)) {
          context.report({ node: property.key, messageId: 'layoutKey', data: { key } });
          return false;
        }
      });

    const checkProps = (propsValue, seen = new Set()) => {
      for (const object of objectsIn(context, propsValue, seen)) {
        for (const property of object.properties) {
          if (property.type === 'SpreadElement') {
            checkProps(property.argument, seen);
            continue;
          }
          const key = propertyName(property);
          if (key && styleAttribute.test(key)) checkStyle(property.value);
          else checkProps(property.value, seen);
        }
      }
    };

    return {
      JSXAttribute(node) {
        if (node.name.type !== 'JSXIdentifier' || node.value?.type !== 'JSXExpressionContainer') return;
        const name = node.name.name;
        if (styleAttribute.test(name)) checkStyle(node.value.expression);
        else if (propsAttribute.test(name)) checkProps(node.value.expression);
      },
    };
  },
};
