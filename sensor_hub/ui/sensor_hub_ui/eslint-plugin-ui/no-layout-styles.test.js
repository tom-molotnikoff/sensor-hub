import rule from './no-layout-styles.js';
import { ruleTester } from './rule-tester.js';

const bannedKeys = [
  'display', 'flex', 'flexDirection', 'flexGrow', 'width', 'height', 'minWidth', 'minHeight', 'maxWidth', 'maxHeight',
  'gap', 'rowGap', 'columnGap', 'gridTemplateColumns', 'gridColumn', 'position', 'inset', 'overflow', 'overflowX',
  'overflowY', 'margin', 'marginTop', 'marginX', 'padding', 'paddingLeft', 'paddingY',
  'm', 'mt', 'mr', 'mb', 'ml', 'mx', 'my', 'p', 'pt', 'pr', 'pb', 'pl', 'px', 'py',
];

const layoutKey = (key) => [{ messageId: 'layoutKey', data: { key } }];

ruleTester.run('no-layout-styles', rule, {
  valid: [
    ...bannedKeys.map((key) => `const chart = { ${key}: 1 }; <Chart data={{ ${key}: 1 }} />`),
    ...bannedKeys.map((key) => `<TextField slotProps={{ htmlInput: { ${key}: 1 } }} />`),
    '<Typography sx={{ color: "text.secondary", "&:hover": { color: "primary.main" } }} />',
    '<Chip style={{ cursor: "pointer" }} />',
    '<Dialog PaperProps={{ elevation: 0 }} />',
    'const extra = { color: "red" }; <Box sx={{ ...extra }} />',
  ],
  invalid: [
    ...bannedKeys.map((key) => ({ code: `<Box sx={{ ${key}: 1 }} />`, errors: layoutKey(key) })),
    ...bannedKeys.map((key) => ({ code: `<div style={{ "${key}": 1 }} />`, errors: layoutKey(key) })),
    { code: '<Dialog slotProps={{ paper: { sx: { width: 1 } } }} />', errors: layoutKey('width') },
    { code: '<Menu PaperProps={{ style: { maxHeight: 200 } }} />', errors: layoutKey('maxHeight') },
    { code: '<Select MenuProps={{ slotProps: { paper: { sx: { mt: 1 } } } }} />', errors: layoutKey('mt') },
    { code: '<Popover slotProps={{ paper: () => ({ sx: { p: 2 } }) }} />', errors: layoutKey('p') },
    { code: '<Box sx={[{ display: "flex" }, open && { gap: 1 }]} />', errors: [...layoutKey('display'), ...layoutKey('gap')] },
    { code: '<Box sx={(theme) => ({ padding: theme.spacing(1) })} />', errors: layoutKey('padding') },
    { code: '<Box sx={wide ? { width: 1 } : { height: 1 }} />', errors: [...layoutKey('width'), ...layoutKey('height')] },
    { code: '<Box sx={{ "& .MuiCard-root": { p: 0 } }} />', errors: layoutKey('p') },
    { code: '<Box sx={{ color: "primary.main", flex: { xs: 1, md: 2 } }} />', errors: layoutKey('flex') },
    { code: 'const card = { mb: 2 }; <Box sx={card} />', errors: layoutKey('mb') },
    { code: 'const base = { position: "relative" }; <Box sx={{ ...base, color: "red" }} />', errors: layoutKey('position') },
    { code: '<Box sx={{ overflow: "auto" } as const} />', errors: layoutKey('overflow') },
  ],
});
