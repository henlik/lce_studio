#!/usr/bin/env node
// Independent reference cases for the real v0.2 HTTP API. Run against a
// disposable PostgreSQL database: projects are removed, run evidence is retained.
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';

const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
const id = `continuum-acceptance-${randomUUID()}`;
let saved = false;
let checks = 0;
const report = message => { checks++; console.log(`PASS ${message}`); };
const clone = value => structuredClone(value);

async function request(path, { method = 'GET', body, expected = 200 } = {}) {
  const response = await fetch(`${base}${path}`, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(30000),
  });
  const text = await response.text();
  assert.equal(response.status, expected, `${method} ${path}: ${text.slice(0, 700)}`);
  if (expected === 204) { assert.equal(text, ''); return; }
  assert.match(response.headers.get('content-type') || '', /application\/json/);
  return JSON.parse(text);
}

const validate = (project, draft = false) => request(`/api/validate${draft ? '?draft=true' : ''}`, { method: 'POST', body: project });
const run = project => request('/api/runs', { method: 'POST', body: { project }, expected: 201 });
const host = (project, hostId) => project.infrastructure.hosts.find(value => value.id === hostId);
const node = (project, type) => project.nodes.find(value => value.type === type);
const place = (project, nodeId, hostId) => { project.infrastructure.placements.find(value => value.nodeId === nodeId).hostId = hostId; };
const close = (actual, expected, message) => assert.ok(Math.abs(actual - expected) < 1e-10, `${message}: expected ${expected}, got ${actual}`);

function fixture(legacy) {
  const project = clone(legacy);
  project.schemaVersion = 2;
  project.id = id;
  project.name = 'Continuum analytical reference';
  project.nodes.forEach(value => { value.config.memoryMb = 64; });
  node(project, 'service').config.processingMs = 10;
  node(project, 'service').config.capacity = 3;
  node(project, 'database').config.processingMs = 5;
  node(project, 'database').config.capacity = 3;
  node(project, 'database').config.deduplicate = true;
  project.edges.forEach(value => { value.latencyMs = 0; });
  project.requirements.find(value => value.rule === 'latency_p95').targetMs = 10000;
  project.scenario = { requestCount: 1, intervalMs: 1, timeoutMs: 1000, maxRetries: 0, dropFirstResponse: false, jitterMs: 0, seed: 42 };
  project.infrastructure = {
    sites: [
      { id: 'site-device', name: 'Device site', latitude: -1, longitude: 20 },
      { id: 'site-edge', name: 'Edge site', latitude: 0, longitude: 25 },
      { id: 'site-cloud', name: 'Regional site', latitude: 50, longitude: 4 },
    ],
    hosts: [
      { id: 'host-device', name: 'Device', siteId: 'site-device', tier: 'device', cpuCores: 1, memoryMb: 256, idleWatts: 0, activeWatts: 0, outages: [] },
      { id: 'host-edge', name: 'Edge', siteId: 'site-edge', tier: 'edge', cpuCores: 1, memoryMb: 512, idleWatts: 0, activeWatts: 0, outages: [] },
      { id: 'host-cloud', name: 'Cloud', siteId: 'site-cloud', tier: 'cloud', cpuCores: 2, memoryMb: 512, idleWatts: 0, activeWatts: 0, outages: [] },
    ],
    links: [
      { id: 'link-device-edge', sourceHostId: 'host-device', targetHostId: 'host-edge', latencyMs: 5, bandwidthKbps: 8000, outages: [] },
      { id: 'link-device-cloud', sourceHostId: 'host-device', targetHostId: 'host-cloud', latencyMs: 50, bandwidthKbps: 8000, outages: [] },
      { id: 'link-edge-cloud', sourceHostId: 'host-edge', targetHostId: 'host-cloud', latencyMs: 10, bandwidthKbps: 8000, outages: [] },
    ],
    placements: project.nodes.map(value => ({ nodeId: value.id, hostId: value.type === 'client' ? 'host-device' : 'host-edge' })),
    messageBytes: 1000,
  };
  return project;
}

async function reject(project, { draft = false } = {}) {
  const invalid = await validate(project, draft);
  assert.equal(invalid.valid, false);
  assert.ok(invalid.issues.length > 0);
  if (!draft) await request('/api/runs', { method: 'POST', body: { project }, expected: 422 });
}

try {
  assert.equal((await request('/api/health')).status, 'ok');
  const legacy = await request('/api/example');
  assert.equal(legacy.schemaVersion, 1);
  assert.equal(legacy.infrastructure, undefined);
  const template = await request('/api/example?template=continuum');
  assert.equal(template.schemaVersion, 2);
  assert.ok(template.infrastructure.hosts.length >= 3);
  assert.deepEqual(await validate(template), { valid: true, issues: [] });
  await request('/api/example?template=unknown-template', { expected: 400 });
  const project = fixture(legacy);
  assert.deepEqual(await validate(project), { valid: true, issues: [] });
  report('legacy and continuum examples expose their actual supported schemas');

  const edge = await run(project);
  // 10+5 ms processing; two transfers, each ceil(1000*8/8000)+5 = 6 ms.
  assert.equal(edge.metrics.completed, 1);
  assert.equal(edge.metrics.writes, 1);
  assert.equal(edge.metrics.p95LatencyMs, 27);
  const edgeLink = edge.infrastructure.links.find(value => value.linkId === 'link-device-edge');
  assert.equal(edgeLink.transfers, 2);
  assert.equal(edgeLink.bytesTransferred, 2000);
  assert.equal(edgeLink.droppedTransfers, 0);
  const cloudProject = clone(project);
  place(cloudProject, node(project, 'service').id, 'host-cloud');
  place(cloudProject, node(project, 'database').id, 'host-cloud');
  const cloud = await run(cloudProject);
  assert.equal(cloud.metrics.p95LatencyMs, 117); // 15 + 2*(1+50).
  assert.notEqual(cloud.modelHash, edge.modelHash);
  assert.equal(cloud.infrastructure.links.find(value => value.linkId === 'link-device-cloud').bytesTransferred, 2000);
  report('moving an unchanged application from edge to cloud changes latency from 27 to 117 ms');

  const cpu = clone(project);
  cpu.scenario.requestCount = 3;
  cpu.infrastructure.messageBytes = 1;
  cpu.infrastructure.links.forEach(value => { value.latencyMs = 0; value.bandwidthKbps = 10000000; });
  node(cpu, 'service').config.processingMs = 100;
  node(cpu, 'database').config.processingMs = 0;
  place(cpu, node(cpu, 'database').id, 'host-cloud');
  const oneCore = await run(cpu);
  host(cpu, 'host-edge').cpuCores = 2;
  const twoCores = await run(cpu);
  // Last request starts at t=2. Its service finishes at t=301 or t=201,
  // followed by 3 ms of remaining transmission. p95 is therefore 302 or 202.
  assert.equal(oneCore.metrics.p95LatencyMs, 302);
  assert.equal(twoCores.metrics.p95LatencyMs, 202);
  assert.equal(oneCore.infrastructure.hosts.find(value => value.hostId === 'host-edge').busyCoreMs, 300);
  assert.equal(twoCores.infrastructure.hosts.find(value => value.hostId === 'host-edge').busyCoreMs, 300);
  report('shared hardware CPU capacity changes queue delay with equal total processing work');

  const bandwidth = clone(project);
  bandwidth.scenario.requestCount = 2;
  bandwidth.scenario.timeoutMs = 10000;
  bandwidth.nodes.forEach(value => { value.config.processingMs = 0; });
  bandwidth.infrastructure.links[0].latencyMs = 0;
  bandwidth.infrastructure.links[0].bandwidthKbps = 8;
  const serialized = await run(bandwidth);
  // One half-duplex link serializes two 1000-byte sends and two replies:
  // replies arrive at 3000 and 4000 ms, with requests starting at 0 and 1 ms.
  assert.equal(serialized.metrics.completed, 2);
  assert.equal(serialized.metrics.p95LatencyMs, 3999);
  assert.equal(serialized.infrastructure.links[0].transfers, 4);
  assert.equal(serialized.infrastructure.links[0].bytesTransferred, 4000);
  report('bidirectional transfers share link bandwidth instead of receiving independent full-capacity links');

  const outage = clone(project);
  host(outage, 'host-edge').outages = [{ startMs: 10, endMs: 50 }];
  const interrupted = await run(outage);
  assert.equal(interrupted.metrics.completed, 0);
  assert.equal(interrupted.metrics.writes, 0);
  assert.equal(interrupted.infrastructure.hosts.find(value => value.hostId === 'host-edge').interruptedJobs, 1);
  outage.scenario.timeoutMs = 60;
  outage.scenario.maxRetries = 1;
  const recovered = await run(outage);
  assert.equal(recovered.metrics.completed, 1);
  assert.equal(recovered.metrics.writes, 1);
  assert.equal(recovered.metrics.attempts, 2);
  assert.equal(recovered.metrics.p95LatencyMs, 87); // retry at60 + healthy27.
  assert.ok(recovered.events.some(value => value.hostId === 'host-edge' && !value.nodeId));
  report('power loss interrupts volatile work and bounded retries recover after power returns');

  const boundary = clone(project);
  boundary.infrastructure.placements.forEach(value => { value.hostId = 'host-edge'; });
  boundary.nodes.forEach(value => { value.config.processingMs = 0; });
  boundary.scenario.requestCount = 2;
  boundary.scenario.intervalMs = 6;
  host(boundary, 'host-edge').outages = [{ startMs: 0, endMs: 6 }];
  const boundaryResult = await run(boundary);
  assert.equal(boundaryResult.metrics.completed, 1);
  assert.equal(boundaryResult.metrics.failed, 1);
  assert.equal(boundaryResult.metrics.writes, 1);
  report('outage boundaries are half-open: down at the start and available exactly at the end');

  const networkOutage = clone(project);
  networkOutage.infrastructure.links[0].outages = [{ startMs: 2, endMs: 20 }];
  const dropped = await run(networkOutage);
  assert.equal(dropped.metrics.completed, 0);
  assert.equal(dropped.metrics.writes, 0);
  assert.equal(dropped.infrastructure.links[0].droppedTransfers, 1);
  assert.equal(dropped.infrastructure.links[0].bytesTransferred, 0);
  assert.ok(dropped.events.some(value => value.linkId === 'link-device-edge' && !value.nodeId));
  report('a network outage during transit drops the request rather than producing a hidden successful write');

  const energy = clone(project);
  energy.infrastructure.placements.forEach(value => { value.hostId = 'host-edge'; });
  node(energy, 'service').config.processingMs = 100;
  node(energy, 'database').config.processingMs = 0;
  Object.assign(host(energy, 'host-edge'), { cpuCores: 2, idleWatts: 10, activeWatts: 30, outages: [{ startMs: 200, endMs: 400 }] });
  Object.assign(host(energy, 'host-device'), { idleWatts: 3, activeWatts: 3 });
  Object.assign(host(energy, 'host-cloud'), { idleWatts: 7, activeWatts: 7 });
  const powered = await run(energy);
  const edgeEnergy = powered.infrastructure.hosts.find(value => value.hostId === 'host-edge');
  assert.equal(powered.metrics.durationMs, 1000);
  assert.equal(edgeEnergy.uptimeMs, 800);
  assert.equal(edgeEnergy.busyCoreMs, 100);
  // Edge idle energy:10W*800ms; dynamic:20W*(1/2)*100ms; two unused hosts:10W*1000ms.
  close(edgeEnergy.energyWh, 9000 / 3600000, 'edge energy');
  close(powered.infrastructure.energyWh, 19000 / 3600000, 'total energy including unused powered hosts');
  report('energy integrates CPU occupancy, idle hosts and powered-off intervals over the full run horizon');

  const tooSmall = clone(project);
  host(tooSmall, 'host-edge').memoryMb = 127; // Two resident reservations of64 MiB.
  await reject(tooSmall);
  const dangling = clone(project);
  dangling.infrastructure.placements[0].hostId = 'missing-host';
  await reject(dangling, { draft: true });
  const disconnected = clone(project);
  disconnected.infrastructure.links = [];
  await reject(disconnected);
  const overlap = clone(project);
  host(overlap, 'host-edge').outages = [{ startMs: 10, endMs: 30 }, { startMs: 20, endMs: 40 }];
  await reject(overlap, { draft: true });
  const invalidCoordinates = clone(project);
  invalidCoordinates.infrastructure.sites[0].latitude = 91;
  await reject(invalidCoordinates, { draft: true });
  const duplicatePlacement = clone(project);
  duplicatePlacement.infrastructure.placements.push(clone(duplicatePlacement.infrastructure.placements[0]));
  await reject(duplicatePlacement, { draft: true });
  const disguisedV1 = clone(project);
  disguisedV1.schemaVersion = 1;
  await reject(disguisedV1);
  const draft = clone(project);
  draft.infrastructure.placements = [];
  assert.deepEqual(await validate(draft, true), { valid: true, issues: [] });
  await reject(draft);
  report('memory admission, references, direct routes, outage windows and schema declarations are validated');

  const seeded = clone(project);
  seeded.scenario.requestCount = 4;
  seeded.scenario.jitterMs = 7;
  const first = await run(seeded);
  const second = await run(seeded);
  assert.deepEqual(second.metrics, first.metrics);
  assert.deepEqual(second.infrastructure, first.infrastructure);
  assert.deepEqual(second.verdicts, first.verdicts);
  assert.deepEqual(second.events, first.events);
  assert.equal(second.modelHash, first.modelHash);
  for (let i = 0; i < first.events.length; i++) {
    assert.equal(first.events[i].sequence, i + 1);
    if (i > 0) assert.ok(first.events[i].timeMs >= first.events[i - 1].timeMs);
  }
  report('the same model and seed reproduce resource metrics and the ordered event trace');

  assert.deepEqual(await request('/api/projects', { method: 'POST', body: project }), project);
  saved = true;
  assert.deepEqual(await request(`/api/projects/${id}`), project);
  await request('/api/projects', { method: 'POST', body: cloudProject });
  assert.deepEqual(await request(`/api/runs/${edge.id}`), edge);
  assert.deepEqual(await request(`/api/runs/${cloud.id}`), cloud);
  assert.notDeepEqual(edge.project.infrastructure.placements, cloud.project.infrastructure.placements);
  const history = await request(`/api/runs?projectId=${id}`);
  assert.ok(history.some(value => value.id === edge.id));
  assert.ok(history.some(value => value.id === cloud.id));
  await request(`/api/projects/${id}`, { method: 'DELETE', expected: 204 });
  saved = false;
  assert.deepEqual(await request(`/api/runs/${edge.id}`), edge);
  report('saved placement alternatives retain immutable infrastructure snapshots and survive project deletion');

  console.log(`\n${checks} continuum acceptance checks passed against ${base}.`);
} finally {
  if (saved) await request(`/api/projects/${id}`, { method: 'DELETE', expected: 204 });
}
