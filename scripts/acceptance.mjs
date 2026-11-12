#!/usr/bin/env node
// Exercises the real Go HTTP API. Use a disposable database when running this
// suite: it removes its own project but run snapshots are intentionally retained.
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';

const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
const id = `acceptance-${randomUUID()}`;
let saved = false;
let checks = 0;
const report = message => { checks++; console.log(`PASS ${message}`); };

async function request(path, { method = 'GET', body, raw, headers = {}, expected = 200 } = {}) {
  const response = await fetch(`${base}${path}`, {
    method,
    headers: { ...((body !== undefined || raw !== undefined) ? { 'Content-Type': 'application/json' } : {}), ...headers },
    body: raw ?? (body === undefined ? undefined : JSON.stringify(body)),
    signal: AbortSignal.timeout(30000),
  });
  const text = await response.text();
  assert.equal(response.status, expected, `${method} ${path}: ${text.slice(0,600)}`);
  if (expected === 204) { assert.equal(text, ''); return; }
  assert.match(response.headers.get('content-type') || '', /application\/json/, `${path} should return JSON`);
  return JSON.parse(text);
}

const run = project => request('/api/runs', { method: 'POST', body: { project }, expected: 201 });
const verdict = (result, rule) => {
  const item = result.verdicts.find(value => value.rule === rule);
  assert.ok(item, `Missing verdict ${rule}`);
  return item;
};

try {
  assert.equal((await request('/api/health')).status, 'ok');
  const example = await request('/api/example');
  const project = structuredClone(example);
  project.id = id;
  project.name = 'Acceptance test project';

  const validation = await request('/api/validate', { method: 'POST', body: project });
  assert.deepEqual(validation, { valid: true, issues: [] });
  report('example is executable and validates without issues');

  assert.deepEqual(await request('/api/projects', { method: 'POST', body: project }), project);
  saved = true;
  assert.deepEqual(await request(`/api/projects/${id}`), project);
  assert.ok((await request('/api/projects')).some(value => value.id === id));
  report('project save, detail and project listing agree');

  const happy = await run(project);
  assert.equal(happy.metrics.completed, project.scenario.requestCount);
  assert.equal(happy.metrics.failed, 0);
  assert.equal(happy.metrics.writes, project.scenario.requestCount);
  assert.equal(happy.metrics.duplicateWrites, 0);
  assert.equal(happy.metrics.deduplicated, project.scenario.requestCount);
  assert.equal(happy.metrics.attempts, project.scenario.requestCount * 2);
  assert.ok(happy.verdicts.every(value => value.passed));
  assert.ok(happy.events.some(value => value.kind === 'response_dropped'));
  assert.ok(happy.events.some(value => value.kind === 'retry_sent'));
  assert.ok(happy.events.some(value => value.kind === 'deduplicated'));
  for (let i = 0; i < happy.events.length; i++) {
    assert.equal(happy.events[i].sequence, i + 1);
    if (i) assert.ok(happy.events[i].timeMs >= happy.events[i - 1].timeMs);
  }
  report('lost-response demo completes with one write per logical request');

  const unsafe = structuredClone(project);
  unsafe.nodes.find(value => value.type === 'database').config.deduplicate = false;
  const duplicates = await run(unsafe);
  assert.equal(duplicates.metrics.writes, project.scenario.requestCount * 2);
  assert.equal(duplicates.metrics.duplicateWrites, project.scenario.requestCount);
  assert.equal(verdict(duplicates, 'no_duplicates').passed, false);
  assert.equal(verdict(duplicates, 'all_complete').passed, true);
  report('disabling deduplication causes an observable failed requirement');

  const noRetry = structuredClone(project);
  noRetry.scenario.maxRetries = 0;
  const exhausted = await run(noRetry);
  assert.equal(exhausted.metrics.completed, 0);
  assert.equal(exhausted.metrics.failed, project.scenario.requestCount);
  assert.equal(exhausted.metrics.writes, project.scenario.requestCount);
  assert.equal(verdict(exhausted, 'all_complete').passed, false);
  assert.equal(verdict(exhausted, 'latency_p95').passed, false);
  report('exhausted retries fail completion even when writes committed');

  const analytical = structuredClone(project);
  analytical.scenario.requestCount = 1;
  analytical.scenario.dropFirstResponse = false;
  const one = await run(analytical);
  assert.equal(one.metrics.p95LatencyMs, 60); // 15 + 5 processing + 2 * (10 + 10) transit.
  assert.equal(one.metrics.averageLatencyMs, 60);
  assert.equal(one.metrics.attempts, 1);
  report('single-request latency matches the independently calculated 60 ms');

  const queue = structuredClone(analytical);
  queue.scenario.requestCount = 3;
  queue.scenario.intervalMs = 1;
  queue.scenario.timeoutMs = 1000;
  queue.edges.forEach(value => { value.latencyMs = 0; });
  queue.nodes.find(value => value.type === 'database').config.processingMs = 0;
  const service = queue.nodes.find(value => value.type === 'service');
  service.config.processingMs = 100;
  service.config.capacity = 1;
  const serial = await run(queue);
  assert.equal(serial.metrics.p95LatencyMs, 298);
  service.config.capacity = 2;
  const concurrent = await run(queue);
  assert.equal(concurrent.metrics.p95LatencyMs, 198);
  assert.ok(serial.events.some(value => value.kind === 'queued'));
  report('queue capacity changes latency as analytically expected: 298 to 198 ms');

  const seeded = structuredClone(project);
  seeded.scenario.jitterMs = 13;
  const firstSeeded = await run(seeded);
  const secondSeeded = await run(seeded);
  assert.deepEqual(secondSeeded.events, firstSeeded.events);
  assert.deepEqual(secondSeeded.metrics, firstSeeded.metrics);
  assert.deepEqual(secondSeeded.verdicts, firstSeeded.verdicts);
  assert.equal(secondSeeded.modelHash, firstSeeded.modelHash);
  assert.notEqual(secondSeeded.id, firstSeeded.id);
  report('same model and seed reproduce trace, metrics and requirement verdicts');

  const changed = structuredClone(project);
  changed.name = 'Edited after experiment';
  changed.nodes[0].position.x += 25;
  await request('/api/projects', { method: 'POST', body: changed });
  const restored = await request(`/api/runs/${happy.id}`);
  assert.deepEqual(restored, happy);
  assert.deepEqual(restored.project, project);
  const history = await request(`/api/runs?projectId=${id}`);
  assert.ok(history.some(value => value.id === happy.id));
  assert.ok(history.every(value => value.projectId === id));
  report('saved experiment keeps its original model after the editable project changes');

  const draft = structuredClone(project);
  draft.edges = [];
  await request('/api/projects', { method: 'POST', body: draft });
  assert.equal((await request('/api/validate', { method: 'POST', body: draft })).valid, false);
  await request('/api/runs', { method: 'POST', body: { project: draft }, expected: 422 });
  report('incomplete topology can be saved as a draft but cannot be simulated');

  const dangling = structuredClone(project);
  dangling.edges[0].target = 'missing-node';
  const invalid = await request('/api/validate', { method: 'POST', body: dangling });
  assert.equal(invalid.valid, false);
  assert.ok(invalid.issues.some(value => value.path.includes('target')));
  await request('/api/runs', { method: 'POST', body: { project: dangling }, expected: 422 });
  const cycle = structuredClone(project);
  cycle.edges.push({ id: 'back-edge', source: 'database', target: 'client', latencyMs: 1 });
  assert.equal((await request('/api/validate', { method: 'POST', body: cycle })).valid, false);
  await request('/api/runs', { method: 'POST', body: { project: cycle }, expected: 422 });
  report('dangling references and cycles cannot reach the simulator');

  await request('/api/validate', { method: 'POST', raw: '{', expected: 400 });
  await request('/api/validate', { method: 'POST', raw: `${JSON.stringify(project)} {}`, expected: 400 });
  await request('/api/validate', { method: 'POST', body: { ...project, unknownField: true }, expected: 400 });
  const negative = structuredClone(project);
  negative.scenario.requestCount = -1;
  await request('/api/runs', { method: 'POST', body: { project: negative }, expected: 422 });
  await request('/api/does-not-exist', { expected: 404 });
  await request('/api/projects', { method: 'POST', body: project, headers: { Origin: 'https://unrelated.example' }, expected: 403 });
  report('malformed JSON, extra values, unknown fields and cross-origin writes are rejected');

  await request(`/api/projects/${id}`, { method: 'DELETE', expected: 204 });
  saved = false;
  await request(`/api/projects/${id}`, { expected: 404 });
  assert.equal((await request(`/api/runs/${happy.id}`)).modelHash, happy.modelHash);
  report('deleting the test project preserves immutable run evidence');
  console.log(`\n${checks} acceptance checks passed against ${base}.`);
} finally {
  if (saved) {
    await request(`/api/projects/${id}`, { method: 'DELETE', expected: 204 });
  }
}
