import path from 'node:path';
import { ESLint } from 'eslint';
import { describe, expect, it } from 'vitest';
import { layoutComponents } from './index.js';

const root = path.resolve(import.meta.dirname, '..');
const eslint = new ESLint({ cwd: root });
const guard = new ESLint({
  cwd: root,
  overrideConfigFile: path.join(root, 'eslint.inline-guard.config.js'),
  allowInlineConfig: false,
  warnIgnored: false,
});

async function rulesReported(file, code, linter = eslint) {
  const [result] = await linter.lintText(code, { filePath: path.join(root, file) });
  return result?.messages.map((message) => message.ruleId) ?? [];
}

describe('restricted imports', () => {
  for (const name of layoutComponents) {
    it(`rejects ${name} from @mui/material outside src/ui`, async () => {
      expect(await rulesReported('src/components/Probe.tsx', `import { ${name} } from '@mui/material'; export default ${name};`))
        .toContain('no-restricted-imports');
    });

    it(`rejects @mui/material/${name} outside src/ui`, async () => {
      expect(await rulesReported('src/components/Probe.tsx', `import ${name} from '@mui/material/${name}'; export default ${name};`))
        .toContain('no-restricted-imports');
    });

    it(`allows ${name} from @mui/material inside src/ui`, async () => {
      expect(await rulesReported('src/ui/Probe.tsx', `import { ${name} } from '@mui/material'; export default ${name};`))
        .not.toContain('no-restricted-imports');
    });
  }

  it.each([
    "import { Box } from '@mui/system'; export default Box;",
    "import Box from '@mui/system/Box'; export default Box;",
    "import Box from '@mui/material/Box/Box'; export default Box;",
    "import Grid from '@mui/material/PigmentGrid'; export default Grid;",
  ])('rejects deep and system layout imports outside src/ui: %s', async (code) => {
    expect(await rulesReported('src/components/Probe.tsx', code)).toContain('no-restricted-imports');
  });

  it('allows other MUI components outside src/ui', async () => {
    expect(await rulesReported('src/components/Probe.tsx', "import { Button, Typography } from '@mui/material'; export { Button, Typography };"))
      .not.toContain('no-restricted-imports');
  });

  const gridLayout = "import { GridLayout } from 'react-grid-layout'; import 'react-grid-layout/css/styles.css'; export default GridLayout;";

  it('rejects react-grid-layout outside src/ui and the dashboard engine', async () => {
    expect(await rulesReported('src/dashboard/widgets/Probe.tsx', gridLayout)).toContain('no-restricted-imports');
  });

  it('allows react-grid-layout in the dashboard engine', async () => {
    expect(await rulesReported('src/dashboard/DashboardEngine.tsx', gridLayout)).not.toContain('no-restricted-imports');
  });

  it('allows react-grid-layout inside src/ui', async () => {
    expect(await rulesReported('src/ui/Probe.tsx', gridLayout)).not.toContain('no-restricted-imports');
  });

  const responsiveContainer = "import { ResponsiveContainer } from 'recharts'; export default ResponsiveContainer;";

  it('rejects ResponsiveContainer outside ChartArea', async () => {
    expect(await rulesReported('src/components/Probe.tsx', responsiveContainer)).toContain('no-restricted-imports');
    expect(await rulesReported('src/ui/Probe.tsx', responsiveContainer)).toContain('no-restricted-imports');
    expect(await rulesReported('src/dashboard/DashboardEngine.tsx', responsiveContainer)).toContain('no-restricted-imports');
  });

  it('allows ResponsiveContainer in ChartArea', async () => {
    expect(await rulesReported('src/ui/ChartArea.tsx', responsiveContainer)).not.toContain('no-restricted-imports');
  });

  it('allows the rest of recharts anywhere', async () => {
    expect(await rulesReported('src/components/Probe.tsx', "import { LineChart } from 'recharts'; export default LineChart;"))
      .not.toContain('no-restricted-imports');
  });
});

describe('rule scope', () => {
  const layout = 'export const probe = <div style={{ display: "flex" }} />;';
  const visuals = 'export const probe = "#fff";';

  it('applies the layout rule outside src/ui', async () => {
    expect(await rulesReported('src/pages/Probe.tsx', layout)).toContain('ui/no-layout-styles');
  });

  it('leaves layout to src/ui', async () => {
    expect(await rulesReported('src/ui/Probe.tsx', layout)).not.toContain('ui/no-layout-styles');
  });

  it('leaves test files alone', async () => {
    expect(await rulesReported('src/pages/Probe.test.tsx', `${layout} export const colour = "#fff";`)).toEqual([]);
  });

  it('applies the raw visuals rule outside src/ui/theme, src/ui included', async () => {
    expect(await rulesReported('src/pages/Probe.tsx', visuals)).toContain('ui/no-raw-visuals');
    expect(await rulesReported('src/ui/Probe.tsx', visuals)).toContain('ui/no-raw-visuals');
  });

  it('leaves raw visuals to src/ui/theme', async () => {
    expect(await rulesReported('src/ui/theme/Probe.ts', visuals)).not.toContain('ui/no-raw-visuals');
  });
});

describe('inline disable guard', () => {
  const selfDisabling = '/* eslint-disable ui/no-inline-disable, ui/no-layout-styles */\nexport const probe = 1;';

  it('reports a directive that also switches the guard off', async () => {
    expect(await rulesReported('src/pages/Probe.tsx', selfDisabling, guard)).toEqual(['ui/no-inline-disable']);
  });

  it.each([
    '/* eslint-disable */\nexport const probe = 1;',
    'export const probe = 1; // eslint-disable-line',
    '// eslint-disable-next-line -- layout is fine here\nexport const probe = 1;',
  ])('reports a blanket directive: %s', async (code) => {
    expect(await rulesReported('src/pages/Probe.tsx', code, guard)).toEqual(['ui/no-inline-disable']);
  });

  it('leaves directives in src/ui alone', async () => {
    expect(await rulesReported('src/ui/Probe.tsx', selfDisabling, guard)).toEqual([]);
  });

  it('leaves directives for other rules alone', async () => {
    expect(await rulesReported('src/pages/Probe.tsx', '// eslint-disable-next-line no-console\nexport const probe = 1;', guard))
      .toEqual([]);
  });
});
