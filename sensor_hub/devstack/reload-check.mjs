import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

const edits = 20;
const timeoutMs = 30_000;
const sensorHub = path.resolve(import.meta.dirname, '..');
const healthUrl = 'http://localhost:8080/api/health';
const viteUrl = 'http://localhost:3000';

const goAnchor = 'gin.H{"status": "ok"}';
const goTarget = {
  file: path.join(sensorHub, 'api/health_api.go'),
  edit: (original, token) => original.replace(goAnchor, `gin.H{"status": "ok", "reload_check": "${token}"}`),
};

const tsxTarget = {
  file: path.join(sensorHub, 'ui/sensor_hub_ui/src/SensorHub.tsx'),
  modulePath: '/src/SensorHub.tsx',
  edit: (original, token) => `${original}// reload-check ${token}\n`,
};

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function poll(check) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await check().catch(() => false)) return true;
    await sleep(25);
  }
  return false;
}

async function goReloaded(token) {
  const response = await fetch(healthUrl);
  const body = await response.json();
  return body.reload_check === token;
}

function connectVite() {
  const updates = [];
  const socket = new WebSocket(viteUrl, 'vite-hmr');
  socket.onmessage = (message) => {
    const payload = JSON.parse(message.data);
    if (payload.type === 'full-reload') updates.push({ at: Date.now(), path: tsxTarget.modulePath });
    for (const update of payload.updates ?? []) updates.push({ at: Date.now(), path: update.path });
  };
  return new Promise((resolve, reject) => {
    socket.onopen = () => resolve({ socket, updates });
    socket.onerror = () => reject(new Error(`cannot open the Vite HMR socket at ${viteUrl}`));
  });
}

async function tsxReloaded(vite, token, since) {
  const pushed = vite.updates.some((update) => update.at >= since && update.path === tsxTarget.modulePath);
  if (!pushed) return false;
  const served = await fetch(`${viteUrl}${tsxTarget.modulePath}?t=${Date.now()}`).then((r) => r.text());
  return served.includes(token);
}

async function run(label, target, reloaded) {
  const original = readFileSync(target.file, 'utf8');
  const results = [];
  try {
    for (let i = 1; i <= edits; i++) {
      const token = `${label}-${Date.now()}-${i}`;
      const start = Date.now();
      writeFileSync(target.file, target.edit(original, token));
      const ok = await poll(() => reloaded(token, start));
      const seconds = ((Date.now() - start) / 1000).toFixed(2);
      results.push(ok);
      console.log(`${label} edit ${String(i).padStart(2)}/${edits}  ${ok ? 'reloaded' : 'MISSED  '}  ${seconds}s`);
    }
  } finally {
    writeFileSync(target.file, original);
  }
  return results.filter(Boolean).length;
}

function restoreOnInterrupt(...targets) {
  const originals = targets.map((target) => [target.file, readFileSync(target.file, 'utf8')]);
  process.on('SIGINT', () => {
    for (const [file, content] of originals) writeFileSync(file, content);
    process.exit(130);
  });
}

if (!readFileSync(goTarget.file, 'utf8').includes(goAnchor)) {
  console.error(`${goTarget.file} no longer contains ${goAnchor}; update reload-check.mjs`);
  process.exit(2);
}
restoreOnInterrupt(goTarget, tsxTarget);

const goPassed = await run('go ', goTarget, goReloaded);
await sleep(3000);

const vite = await connectVite();
await fetch(`${viteUrl}${tsxTarget.modulePath}`);
const tsxPassed = await run('tsx', tsxTarget, (token, since) => tsxReloaded(vite, token, since));
vite.socket.close();

console.log(`\ngo: ${goPassed}/${edits} reloaded, tsx: ${tsxPassed}/${edits} reloaded`);
process.exit(goPassed === edits && tsxPassed === edits ? 0 : 1);
