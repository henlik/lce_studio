import { spawn } from 'node:child_process';
import { mkdir } from 'node:fs/promises';
import { setTimeout as delay } from 'node:timers/promises';

// The backend remains a real Go process; Vite only proxies /api to it.
const go = process.env.STUDIO_GO_BINARY || 'go';
await mkdir('.cache', { recursive: true });
const binary = process.platform === 'win32' ? '.cache/studio-dev.exe' : '.cache/studio-dev';
await new Promise((resolve, reject) => {
  const build = spawn(go, ['build', '-o', binary, './cmd/studio'], { stdio: 'inherit' });
  build.on('error', reject);
  build.on('exit', code => code === 0 ? resolve() : reject(new Error('Go build failed.')));
});
const backend = spawn(binary, [], {
  stdio: 'inherit',
  env: { ...process.env, STUDIO_ADDR: '127.0.0.1:8080' },
});
let frontend;
let stopping = false;
function stop(code = 0) {
  if (stopping) return;
  stopping = true;
  frontend?.kill('SIGTERM');
  backend.kill('SIGTERM');
  process.exitCode = code;
}
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => stop());
backend.on('error', err => { console.error('Cannot start Go backend:', err.message); stop(1); });
backend.on('exit', code => { if (!stopping) stop(code || 1); });
let ready = false;
for (let i = 0; i < 40 && !stopping; i++) {
  try {
    const response = await fetch('http://127.0.0.1:8080/api/health', { signal: AbortSignal.timeout(1000) });
    if (response.ok) { ready = true; break; }
  } catch {}
  await delay(500);
}
if (!ready) { console.error('Go backend did not become ready on port 8080.'); stop(1); }
else if (!stopping) {
  const args = process.argv.slice(2);
  if (!args.includes('--host')) args.push('--host', '127.0.0.1');
  if (!args.includes('--port')) args.push('--port', process.env.PORT || '5173');
  frontend = spawn('npm', ['--prefix', 'web', 'run', 'dev', '--', ...args], { stdio: 'inherit' });
  frontend.on('error', err => { console.error(err.message); stop(1); });
  frontend.on('exit', code => { if (!stopping) stop(code || 0); });
}
