import rule from './no-inline-disable.js';
import { ruleTester } from './rule-tester.js';

const disabled = [{ messageId: 'disabled' }];

ruleTester.run('no-inline-disable', rule, {
  valid: [
    '// eslint-disable-next-line no-console\nconst a = 1;',
    '/* eslint-disable no-console -- see ui/README */\nconst a = 1;',
    '// the ui/no-layout-styles rule keeps layout in primitives\nconst a = 1;',
  ],
  invalid: [
    { code: '// eslint-disable-next-line ui/no-layout-styles\nconst a = 1;', errors: disabled },
    { code: 'const a = 1; // eslint-disable-line ui/no-raw-visuals', errors: disabled },
    { code: '/* eslint-disable ui/no-layout-styles */\nconst a = 1;', errors: disabled },
    { code: '/* eslint-disable no-console, ui/no-raw-visuals */\nconst a = 1;', errors: disabled },
    { code: '/* eslint ui/no-layout-styles: off */\nconst a = 1;', errors: disabled },
  ],
});
