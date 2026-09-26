import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import path from 'node:path';

const devstack = import.meta.dirname;
const api = 'http://localhost:8080/api';
const ui = 'http://localhost:3000';
const startTimeoutMs = 15 * 60_000;
const liveReadingTimeoutMs = 60_000;
const toggleTimeoutMs = 15_000;
const mqttDriver = 'mqtt-zigbee2mqtt';
const httpMockPort = 5000;
const switchablePlug = 'office-plug';
const widgetSettleTimeoutMs = 60_000;
const activeDashboardKey = 'sensor-hub-active-dashboard-id';
const outsideServiceWidgets = new Set(['weather-forecast']);

const { chromium } = createRequire(path.join(devstack, '../ui/sensor_hub_ui/package.json'))('@playwright/test');

const logins = [
  { username: 'admin', password: 'adminpassword', role: 'admin' },
  { username: 'user', password: 'userpassword', role: 'user' },
  { username: 'viewer', password: 'viewerpassword', role: 'viewer' },
];

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function compose(...args) {
  return execFileSync('docker', ['compose', ...args], { cwd: devstack, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
}

function containerState(service) {
  const id = compose('ps', '-a', '-q', service).trim();
  if (!id) throw new Error(`no ${service} container`);
  return JSON.parse(execFileSync('docker', ['inspect', '--format', '{{json .State}}', id], { encoding: 'utf8' }));
}

async function waitForHealth() {
  const deadline = Date.now() + startTimeoutMs;
  while (Date.now() < deadline) {
    const healthy = await fetch(`${api}/health`).then((response) => response.ok).catch(() => false);
    if (healthy) return;
    await sleep(1000);
  }
  throw new Error(`${api}/health did not answer within ${startTimeoutMs / 60_000} minutes`);
}

async function currentUser(headers) {
  const response = await fetch(`${api}/auth/me`, { headers });
  if (!response.ok) throw new Error(`/auth/me answered ${response.status}`);
  return (await response.json()).user;
}

async function signIn({ username, password }) {
  const response = await fetch(`${api}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!response.ok) throw new Error(`login as ${username} answered ${response.status}`);
  const session = response.headers.getSetCookie().map((cookie) => cookie.split(';')[0]).join('; ');
  return { headers: { Cookie: session }, mustChangePassword: (await response.json()).must_change_password };
}

async function checkLogin(login) {
  const { headers, mustChangePassword } = await signIn(login);
  if (mustChangePassword) throw new Error('asked to change the password');
  const user = await currentUser(headers);
  if (!user.roles.includes(login.role)) throw new Error(`has roles ${user.roles.join(', ')}, expected ${login.role}`);
}

let adminKey = '';

async function checkApiKey() {
  const key = compose('logs', '--no-log-prefix', 'seed').match(/admin_api_key=(shk_[0-9a-f]+)/)?.[1];
  if (!key) throw new Error('the seed logs print no admin API key');
  const user = await currentUser({ 'X-API-Key': key });
  if (user.username !== 'admin') throw new Error(`the key authenticates as ${user.username}`);
  adminKey = key;
}

async function request(path, { method = 'GET', body } = {}) {
  const response = await fetch(`${api}${path}`, {
    method,
    headers: { 'X-API-Key': adminKey, 'Content-Type': 'application/json' },
    body: body && JSON.stringify(body),
  });
  if (!response.ok) throw new Error(`${method} ${path} answered ${response.status}`);
  return response.json();
}

const nextWholeSecond = () => new Date(Math.ceil(Date.now() / 1000) * 1000);

async function readingsSince(since, sensor, type) {
  const query = new URLSearchParams({ start: since.toISOString(), end: new Date(Date.now() + 60_000).toISOString(), sensor, aggregation: 'raw' });
  if (type) query.set('type', type);
  return (await request(`/readings/between?${query}`)).readings;
}

async function approvedSensors() {
  return (await request('/sensors')).filter((sensor) => sensor.status === 'active' && sensor.enabled);
}

async function checkSubscription() {
  const subscriptions = await request('/mqtt/subscriptions');
  if (!subscriptions.some((subscription) => subscription.topic_pattern === 'zigbee2mqtt/#' && subscription.enabled)) {
    throw new Error('no enabled zigbee2mqtt/# subscription');
  }
}

async function checkHTTPMocks() {
  const mocks = compose('config', '--services').split('\n').filter((service) => service.startsWith('mock-http-'));
  const sensors = await approvedSensors();
  const since = new Date(Date.now() - 10 * 60_000);
  for (const mock of mocks) {
    const sensor = sensors.find((candidate) => candidate.config.url === `http://${mock}:${httpMockPort}`);
    if (!sensor) throw new Error(`no approved sensor polls ${mock}`);
    if ((await readingsSince(since, sensor.name)).length === 0) throw new Error(`${sensor.name} has no reading`);
  }
}

async function checkLiveMQTTReadings() {
  const since = nextWholeSecond();
  let waiting = (await approvedSensors()).filter((sensor) => sensor.sensor_driver === mqttDriver).map((sensor) => sensor.name);
  if (waiting.length === 0) throw new Error('no approved MQTT sensors');
  const deadline = Date.now() + liveReadingTimeoutMs;
  while (waiting.length > 0 && Date.now() < deadline) {
    await sleep(1000);
    const stillWaiting = [];
    for (const name of waiting) {
      if ((await readingsSince(since, name)).length === 0) stillWaiting.push(name);
    }
    waiting = stillWaiting;
  }
  if (waiting.length > 0) throw new Error(`no live reading within ${liveReadingTimeoutMs / 1000} s for ${waiting.join(', ')}`);
}

async function checkNoPendingSensors() {
  const pending = await request('/sensors/status/pending');
  if (pending.length > 0) throw new Error(`pending: ${pending.map((sensor) => sensor.name).join(', ')}`);
}

async function checkPlugToggles() {
  const plug = await request(`/sensors/${switchablePlug}`);
  const current = (await readingsSince(new Date(Date.now() - 60_000), switchablePlug, 'state')).at(-1);
  if (!current) throw new Error(`${switchablePlug} has reported no state in the last minute`);
  const [command, expected] = current.text_state === 'true' ? ['OFF', 'false'] : ['ON', 'true'];
  const since = new Date(Math.floor(Date.now() / 1000) * 1000);
  await request(`/sensors/${plug.id}/command`, { method: 'POST', body: { property: 'state', value: command } });
  const deadline = Date.now() + toggleTimeoutMs;
  while (Date.now() < deadline) {
    if ((await readingsSince(since, switchablePlug, 'state')).some((reading) => reading.text_state === expected)) return;
    await sleep(500);
  }
  throw new Error(`the hub shows no ${command} state within ${toggleTimeoutMs / 1000} s of the command`);
}

function widgetProblem(frame) {
  const state = frame.dataset.widgetState;
  if (state === 'error') return 'shows an error';
  if (state !== 'populated' || frame.querySelector('[aria-busy=true]')) return `is still ${state === 'populated' ? 'loading' : state}`;
  const empty = frame.querySelector('[data-ui=empty-state]');
  if (empty) return `shows the empty state "${empty.innerText.trim()}"`;
  const valueless = [...frame.querySelectorAll('[data-ui=metric-value], [data-ui=stat] *')]
    .some((element) => element.children.length === 0 && element.textContent.trim() === '\u2014');
  if (valueless) return 'shows no value';
  if (frame.querySelector('[aria-checked=mixed]')) return 'shows no state';
  const tiles = [...frame.querySelectorAll('[data-ui=tile]')];
  if (tiles.length > 0 && !tiles.some((tile) => getComputedStyle(tile).color === 'rgb(255, 255, 255)')) return 'has no tile with data';
  if (!frame.querySelector('[data-ui=frame-content]')?.innerText.trim()) return 'is blank';
  return null;
}

async function widgetProblems(page, dashboard) {
  await page.addInitScript(([key, id]) => localStorage.setItem(key, id), [activeDashboardKey, String(dashboard.id)]);
  await page.goto(ui);
  const widgets = JSON.parse(dashboard.config).widgets.filter((widget) => !outsideServiceWidgets.has(widget.type));
  let problems = [];
  const deadline = Date.now() + widgetSettleTimeoutMs;
  do {
    problems = [];
    for (const widget of widgets) {
      const frame = page.locator(`[data-widget-id="${widget.id}"] [data-ui=frame]`);
      if (await frame.count() === 0) {
        problems.push(`${widget.type} is not on the page`);
        continue;
      }
      await frame.scrollIntoViewIfNeeded();
      const problem = await frame.evaluate(widgetProblem);
      if (problem) problems.push(`${widget.type} ${problem}`);
    }
    if (problems.length > 0) await sleep(1000);
  } while (problems.length > 0 && Date.now() < deadline);
  return problems;
}

async function checkDashboardsRenderWithData() {
  const browser = await chromium.launch();
  const problems = [];
  let rendered = 0;
  try {
    for (const { username, password } of logins) {
      const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
      const login = await context.request.post(`${api}/auth/login`, { data: { username, password } });
      if (!login.ok()) throw new Error(`${username} could not log in: ${login.status()}`);
      const dashboards = await (await context.request.get(`${api}/dashboards`)).json();
      for (const dashboard of dashboards) {
        const page = await context.newPage();
        for (const problem of await widgetProblems(page, dashboard)) problems.push(`${username} on ${dashboard.name}: ${problem}`);
        await page.close();
        rendered++;
      }
      await context.close();
    }
  } finally {
    await browser.close();
  }
  if (rendered === 0) throw new Error('no seeded user can see a dashboard');
  if (problems.length > 0) throw new Error(problems.join('; '));
}

async function checkSeededDashboards() {
  const dashboards = await request('/dashboards');
  const names = dashboards.map((dashboard) => dashboard.name);
  for (const name of ['Home', 'Climate', 'Devices']) {
    if (!names.includes(name)) throw new Error(`admin has no ${name} dashboard`);
  }
  const home = dashboards.find((dashboard) => dashboard.name === 'Home');
  if (!home.is_default) throw new Error('Home is not the default dashboard');
}

async function checkViewerHasASharedDashboard() {
  const { headers } = await signIn(logins.find((login) => login.role === 'viewer'));
  const response = await fetch(`${api}/dashboards`, { headers });
  if (!response.ok) throw new Error(`viewer's /dashboards answered ${response.status}`);
  if ((await response.json()).length === 0) throw new Error('no dashboard is shared with viewer');
}

function checkSeedRanFirst() {
  const seed = containerState('seed');
  const hub = containerState('sensor-hub');
  if (seed.ExitCode !== 0) throw new Error(`seed exited ${seed.ExitCode}`);
  if (new Date(seed.FinishedAt) > new Date(hub.StartedAt)) throw new Error('sensor-hub started before the seed finished');
}

const checks = [
  ['seed completed before sensor-hub started', checkSeedRanFirst],
  ...logins.map((login) => [`${login.username} logs in as ${login.role} without a password change`, () => checkLogin(login)]),
  ['the admin API key in the seed logs authenticates', checkApiKey],
  ['the zigbee2mqtt/# subscription is enabled', checkSubscription],
  ['every HTTP mock is polled by an approved sensor with a reading', checkHTTPMocks],
  ['every approved MQTT device gets a new live reading', checkLiveMQTTReadings],
  ['no pending sensors exist', checkNoPendingSensors],
  [`the ${switchablePlug} toggles through the hub`, checkPlugToggles],
  ['Home, Climate and Devices exist and Home is the default', checkSeededDashboards],
  ['a dashboard is shared with viewer', checkViewerHasASharedDashboard],
  ['every widget on every dashboard a seeded user can see renders with data', checkDashboardsRenderWithData],
];

console.log('resetting the stack to an empty volume and starting it');
compose('down', '-v');
let failures = 0;
try {
  compose('up', '--build', '-d');
  await waitForHealth();
  for (const [name, check] of checks) {
    try {
      await check();
      console.log(`pass  ${name}`);
    } catch (error) {
      failures++;
      console.log(`FAIL  ${name}: ${error.message}`);
    }
  }
} finally {
  compose('down');
}

console.log(failures === 0 ? '\nall checks passed' : `\n${failures} of ${checks.length} checks failed`);
process.exit(failures === 0 ? 0 : 1);
