import { execFileSync } from 'node:child_process';

const devstack = import.meta.dirname;
const api = 'http://localhost:8080/api';
const startTimeoutMs = 15 * 60_000;

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

async function checkLogin({ username, password, role }) {
  const response = await fetch(`${api}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!response.ok) throw new Error(`login answered ${response.status}`);
  if ((await response.json()).must_change_password) throw new Error('asked to change the password');
  const session = response.headers.getSetCookie().map((cookie) => cookie.split(';')[0]).join('; ');
  const user = await currentUser({ Cookie: session });
  if (!user.roles.includes(role)) throw new Error(`has roles ${user.roles.join(', ')}, expected ${role}`);
}

async function checkApiKey() {
  const key = compose('logs', '--no-log-prefix', 'seed').match(/admin_api_key=(shk_[0-9a-f]+)/)?.[1];
  if (!key) throw new Error('the seed logs print no admin API key');
  const user = await currentUser({ 'X-API-Key': key });
  if (user.username !== 'admin') throw new Error(`the key authenticates as ${user.username}`);
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
