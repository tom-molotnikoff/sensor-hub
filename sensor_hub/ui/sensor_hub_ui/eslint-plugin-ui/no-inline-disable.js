const directive = /^\s*(eslint-disable(?:-next-line|-line)?|eslint)\s+([\s\S]*)$/;
const uiRule = /(^|[\s,])ui\//;

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
          const match = directive.exec(comment.value);
          if (match && uiRule.test(match[2].split(/\s--\s/)[0])) {
            context.report({ loc: comment.loc, messageId: 'disabled' });
          }
        }
      },
    };
  },
};
