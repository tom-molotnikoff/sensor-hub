import tseslint from 'typescript-eslint';
import noInlineDisable from './no-inline-disable.js';
import noLayoutStyles from './no-layout-styles.js';
import noRawVisuals from './no-raw-visuals.js';

export const layoutComponents = ['Grid', 'Stack', 'Box', 'Paper', 'Card', 'useMediaQuery'];

const layoutImportPaths = [{ name: '@mui/material', importNames: layoutComponents }];
const layoutImportPatterns = [
  {
    group: ['material', 'system'].flatMap((mui) =>
      layoutComponents.flatMap((name) => [`@mui/${mui}/${name}`, `@mui/${mui}/${name}/*`]),
    ),
    message: 'Layout belongs to the src/ui primitives.',
  },
  { group: ['@mui/material/Pigment*'], message: 'Layout belongs to the src/ui primitives.' },
  { group: ['@mui/system'], importNames: layoutComponents, message: 'Layout belongs to the src/ui primitives.' },
];
const gridLayoutImportPatterns = [
  { group: ['react-grid-layout', 'react-grid-layout/*'], message: 'Only src/ui and the dashboard engine lay out grids.' },
];
const responsiveContainerImportPaths = [
  { name: 'recharts', importNames: ['ResponsiveContainer'], message: 'Size charts with ChartArea.' },
];

const restrictImports = ({ paths = [], patterns = [] }) => ['error', { paths, patterns }];

const sourceFiles = ['src/**/*.{ts,tsx}'];
const testFiles = ['src/**/*.test.{ts,tsx}'];

const plugin = {
  meta: { name: 'eslint-plugin-ui' },
  rules: {
    'no-inline-disable': noInlineDisable,
    'no-layout-styles': noLayoutStyles,
    'no-raw-visuals': noRawVisuals,
  },
  configs: {},
};

plugin.configs.recommended = [
  {
    files: sourceFiles,
    ignores: testFiles,
    plugins: { ui: plugin },
    rules: {
      'ui/no-layout-styles': 'error',
      'ui/no-raw-visuals': 'error',
      'no-restricted-imports': restrictImports({
        paths: [...layoutImportPaths, ...responsiveContainerImportPaths],
        patterns: [...layoutImportPatterns, ...gridLayoutImportPatterns],
      }),
    },
  },
  {
    files: ['src/dashboard/DashboardEngine.tsx'],
    rules: {
      'no-restricted-imports': restrictImports({
        paths: [...layoutImportPaths, ...responsiveContainerImportPaths],
        patterns: layoutImportPatterns,
      }),
    },
  },
  {
    files: ['src/ui/**'],
    rules: {
      'ui/no-layout-styles': 'off',
      'no-restricted-imports': restrictImports({ paths: responsiveContainerImportPaths }),
    },
  },
  {
    files: ['src/ui/ChartArea.tsx'],
    rules: { 'no-restricted-imports': 'off' },
  },
  {
    files: ['src/ui/theme/**'],
    rules: { 'ui/no-raw-visuals': 'off' },
  },
];

plugin.configs.inlineDisableGuard = [
  {
    files: sourceFiles,
    ignores: [...testFiles, 'src/ui/**'],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
    plugins: { ui: plugin },
    rules: { 'ui/no-inline-disable': 'error' },
  },
];

export default plugin;
