import noInlineDisable from './no-inline-disable.js';
import noLayoutStyles from './no-layout-styles.js';
import noRawVisuals from './no-raw-visuals.js';

const layoutImports = [
  { name: '@mui/material', importNames: ['Grid', 'Stack', 'Box', 'Paper', 'Card', 'useMediaQuery'] },
  ...['Grid', 'Stack', 'Box', 'Paper', 'Card', 'useMediaQuery'].map((name) => ({ name: `@mui/material/${name}` })),
  { name: '@mui/system', importNames: ['Grid', 'Stack', 'Box', 'useMediaQuery'] },
];
const gridLayoutImports = [{ group: ['react-grid-layout', 'react-grid-layout/*'] }];
const responsiveContainerImports = [{ name: 'recharts', importNames: ['ResponsiveContainer'] }];

const restrictImports = (paths, patterns = []) => ['error', { paths, patterns }];

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
    files: ['src/**/*.{ts,tsx}'],
    plugins: { ui: plugin },
    rules: {
      'ui/no-inline-disable': 'error',
      'ui/no-layout-styles': 'error',
      'ui/no-raw-visuals': 'error',
      'no-restricted-imports': restrictImports([...layoutImports, ...responsiveContainerImports], gridLayoutImports),
    },
  },
  {
    files: ['src/dashboard/DashboardEngine.tsx'],
    rules: {
      'no-restricted-imports': restrictImports([...layoutImports, ...responsiveContainerImports]),
    },
  },
  {
    files: ['src/ui/**'],
    rules: {
      'ui/no-inline-disable': 'off',
      'ui/no-layout-styles': 'off',
      'no-restricted-imports': restrictImports(responsiveContainerImports),
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

export default plugin;
