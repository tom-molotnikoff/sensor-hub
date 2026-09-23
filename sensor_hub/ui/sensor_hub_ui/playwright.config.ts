import os from 'node:os';
import path from 'node:path';
import { defineConfig, devices } from '@playwright/test';
import { routes } from './tests/layout/routes';

const port = 4180;
const sensorHubDir = path.resolve(import.meta.dirname, '../..');
const seedPath = path.join(os.tmpdir(), 'sensor-hub-layout', 'seed.db');
const fixtures = [...new Set(routes.flatMap((route) => route.fixtures))];

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  reporter: 'list',
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: [
      `go run ./testharness/cmd/seed -out ${seedPath} -readings 200000`,
      `go run -tags integration ./testharness/cmd/layout -seed ${seedPath} -ui ${path.join(import.meta.dirname, 'dist')} -addr 127.0.0.1:${port} -fixtures "${fixtures.join(',')}"`,
    ].join(' && '),
    cwd: sensorHubDir,
    wait: { stdout: /layout harness listening/ },
    timeout: 300_000,
    gracefulShutdown: { signal: 'SIGTERM', timeout: 5_000 },
  },
});
