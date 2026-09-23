const directive = /^\s*(eslint-disable(?:-next-line|-line)?|eslint)(?:\s+([\s\S]*))?$/;
const uiRule = /(^|[\s,])ui\//;

function disablesUiRules(comment) {
  const match = directive.exec(comment.value);
  if (!match) return false;
  const rules = (match[2] ?? '').replace(/(^|\s)--(\s[\s\S]*)?$/, '').trim();
  return uiRule.test(rules) || (match[1] !== 'eslint' && rules === '');
}

export default {
  meta: {
    type: 'problem',
    docs: { description: 'ui/* rules cannot be switched off inline' },
    messages: {
      disabled: 'ui/* rules cannot be switched off inline. Fix the violation instead.',
    },
    schema: [],
  },
  create(context) {
    return {
      Program() {
        for (const comment of context.sourceCode.getAllComments()) {
          if (disablesUiRules(comment)) context.report({ loc: comment.loc, messageId: 'disabled' });
        }
      },
    };
  },
};
