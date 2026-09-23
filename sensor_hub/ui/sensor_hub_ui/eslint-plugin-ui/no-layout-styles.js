import { forEachProperty, propertyName } from './style-objects.js';

const layoutKey =
  /^(display|flex[A-Za-z]*|width|height|min[A-Z][A-Za-z]*|max[A-Z][A-Za-z]*|gap|rowGap|columnGap|grid[A-Za-z]*|position|inset[A-Za-z]*|overflow|overflowX|overflowY|margin[A-Za-z]*|padding[A-Za-z]*|[mp][trblxy]?)$/;

const styleName = /^(sx|style|changes)$|Style$/;
const propsName = /Props$/;

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
    const reported = new Set();

    const checkStyle = (value) =>
      forEachProperty(context, value, (property) => {
        const key = propertyName(property);
        if (!key || !layoutKey.test(key)) return true;
        if (!reported.has(property.key)) {
          reported.add(property.key);
          context.report({ node: property.key, messageId: 'layoutKey', data: { key } });
        }
        return false;
      });

    const checkProps = (value) =>
      forEachProperty(context, value, (property) => {
        const key = propertyName(property);
        if (!key || !styleName.test(key)) return true;
        checkStyle(property.value);
        return false;
      });

    return {
      JSXAttribute(node) {
        if (node.name.type !== 'JSXIdentifier' || node.value?.type !== 'JSXExpressionContainer') return;
        const name = node.name.name;
        if (styleName.test(name)) checkStyle(node.value.expression);
        else if (propsName.test(name)) checkProps(node.value.expression);
      },
    };
  },
};
