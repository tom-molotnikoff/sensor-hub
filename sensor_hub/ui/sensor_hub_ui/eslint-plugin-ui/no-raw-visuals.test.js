import rule from './no-raw-visuals.js';
import { ruleTester } from './rule-tester.js';

const colour = (value) => [{ messageId: 'colour', data: { colour: value } }];
const typeScale = (key) => [{ messageId: 'typeScale', data: { key } }];

ruleTester.run('no-raw-visuals', rule, {
  valid: [
    '<Typography color="text.secondary" />',
    '<Chip sx={{ bgcolor: "status.ok" }} />',
    '<Typography variant="cardTitle" />',
    '<Box sx={{ color: (theme) => theme.palette.text.primary }} />',
    '<SvgIcon fontSize="small" />',
    '<Typography sx={{ fontSize: "inherit", fontWeight: "fontWeightBold" }} />',
    '<Typography sx={{ lineHeight: "inherit" }} />',
    '<Typography sx={{ fontSize: theme.typography.body2.fontSize }} />',
    'const link = "/docs#add-sensor"; const entity = "&#123;";',
    'const border = `1px solid ${theme.palette.divider}`;',
    'const hash = "#abcdefg";',
  ],
  invalid: [
    { code: 'const c = "#fff";', errors: colour('#fff') },
    { code: 'const c = "#ffff";', errors: colour('#ffff') },
    { code: 'const c = "#D4451A";', errors: colour('#D4451A') },
    { code: 'const c = "#D4451A80";', errors: colour('#D4451A80') },
    { code: '<Box sx={{ border: "1px solid #ddd" }} />', errors: colour('#ddd') },
    { code: 'const shadow = `0 0 0 3px var(--mui-palette-primary-main, #1976d2)`;', errors: colour('#1976d2') },
    { code: '<Typography color="#333" />', errors: colour('#333') },
    { code: 'const c = "rgb(0, 0, 0)";', errors: colour('rgb(0, 0, 0)') },
    { code: 'const c = "rgba(212,69,26,0.06)";', errors: colour('rgba(212,69,26,0.06)') },
    { code: 'const c = `rgba(${r}, ${g}, ${b}, 0.5)`;', errors: colour('rgba(') },
    { code: '<Typography sx={{ fontSize: 12 }} />', errors: typeScale('fontSize') },
    { code: '<Typography sx={{ fontSize: "0.8rem" }} />', errors: typeScale('fontSize') },
    { code: '<Typography sx={{ fontSize: { xs: 12, md: 14 } }} />', errors: typeScale('fontSize') },
    { code: '<Typography sx={{ fontSize: compact ? 12 : "inherit" }} />', errors: typeScale('fontSize') },
    { code: '<Typography fontSize={12} />', errors: typeScale('fontSize') },
    { code: '<Typography sx={{ fontWeight: 600 }} />', errors: typeScale('fontWeight') },
    { code: '<Typography sx={{ fontWeight: "700" }} />', errors: typeScale('fontWeight') },
    { code: '<Typography fontWeight={500} />', errors: typeScale('fontWeight') },
    { code: '<Typography fontWeight="500" />', errors: typeScale('fontWeight') },
    { code: '<Typography sx={{ lineHeight: 1 }} />', errors: typeScale('lineHeight') },
    { code: '<Typography sx={{ lineHeight: "20px" }} />', errors: typeScale('lineHeight') },
    { code: '<Typography lineHeight={1.2} />', errors: typeScale('lineHeight') },
  ],
});
