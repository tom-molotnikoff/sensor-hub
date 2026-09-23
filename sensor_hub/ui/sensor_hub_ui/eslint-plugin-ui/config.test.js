import path from 'node:path';
import { ESLint } from 'eslint';
import { describe, expect, it } from 'vitest';

const root = path.resolve(import.meta.dirname, '..');
const eslint = new ESLint({ cwd: root });

async function rulesReported(file, code) {
  const [result] = await eslint.lintText(code, { filePath: path.join(root, file) });
  return result.messages.map((message) => message.ruleId);
}

const layoutComponents = ['Grid', 'Stack', 'Box', 'Paper', 'Card', 'useMediaQuery'];

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

  it('rejects Box from @mui/system outside src/ui', async () => {
    expect(await rulesReported('src/components/Probe.tsx', "import { Box } from '@mui/system'; export default Box;"))
      .toContain('no-restricted-imports');
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
  const disable = '// eslint-disable-next-line ui/no-layout-styles\nexport const probe = 1;';

  it('applies the layout and inline-disable rules outside src/ui', async () => {
    expect(await rulesReported('src/pages/Probe.tsx', layout)).toContain('ui/no-layout-styles');
    expect(await rulesReported('src/pages/Probe.tsx', disable)).toContain('ui/no-inline-disable');
  });

  it('leaves layout to src/ui', async () => {
    expect(await rulesReported('src/ui/Probe.tsx', layout)).not.toContain('ui/no-layout-styles');
    expect(await rulesReported('src/ui/Probe.tsx', disable)).not.toContain('ui/no-inline-disable');
  });

  it('applies the raw visuals rule outside src/ui/theme, src/ui included', async () => {
    expect(await rulesReported('src/pages/Probe.tsx', visuals)).toContain('ui/no-raw-visuals');
    expect(await rulesReported('src/ui/Probe.tsx', visuals)).toContain('ui/no-raw-visuals');
  });

  it('leaves raw visuals to src/ui/theme', async () => {
    expect(await rulesReported('src/ui/theme/Probe.ts', visuals)).not.toContain('ui/no-raw-visuals');
  });
});
