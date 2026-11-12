#!/usr/bin/env node
// Real HTTP acceptance cases with analytical references independent of engine
// implementation. Use a disposable database: immutable runs/catalog records stay.
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';

const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
const suffix = randomUUID();
const id = `workflow-acceptance-${suffix}`;
let saved = false;
let checks = 0;
const clone = value => structuredClone(value);
const report = message => { checks++; console.log(`PASS ${message}`); };

async function request(path, { method = 'GET', body, expected = 200, includeStatus = false } = {}) {
  const response = await fetch(`${base}${path}`, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(30000),
  });
  const text = await response.text();
  const statuses = Array.isArray(expected) ? expected : [expected];
  assert.ok(statuses.includes(response.status), `${method} ${path}: expected ${statuses}, got ${response.status}: ${text.slice(0, 700)}`);
  if (response.status === 204) { assert.equal(text, ''); return; }
  assert.match(response.headers.get('content-type') || '', /application\/json/);
  const value = JSON.parse(text);
  return includeStatus ? { status: response.status, value } : value;
}

const validate = (project, draft = false) => request(`/api/validate${draft ? '?draft=true' : ''}`, { method: 'POST', body: project });
const run = project => request('/api/runs', { method: 'POST', body: { project }, expected: 201 });
const install = (manifest, expected = 201, includeStatus = false) => request('/api/components', { method: 'POST', body: manifest, expected, includeStatus });
const byID = (project, nodeId) => project.nodes.find(value => value.id === nodeId);
const routeEvents = result => result.events.filter(value => value.kind === 'route_selected');
const routes = result => routeEvents(result).map(value => [value.requestId, value.edgeId]);
const eventCount = (result, kind, nodeId) => result.events.filter(value => value.kind === kind && value.nodeId === nodeId).length;

// These are precomputed contract vectors, not selected from observed run output.
// SHA-256("42\nrequest-NNN\nrouter"), first unsigned big-endian 64 bits,
// modulo 2 with edge IDs r-a/r-b yields the following twelve equal-weight routes.
const equalEdges = ['r-b', 'r-b', 'r-a', 'r-a', 'r-b', 'r-a', 'r-a', 'r-b', 'r-a', 'r-b', 'r-b', 'r-a'];
const weightedEdges = ['r-b', 'r-b', 'r-b', 'r-a', 'r-b', 'r-a', 'r-b', 'r-b', 'r-a', 'r-b', 'r-b', 'r-a'];
const expectedRoutes = edges => edges.map((edge, i) => [`request-${String(i + 1).padStart(3, '0')}`, edge]);

function fixture(legacy) {
  const project = clone(legacy);
  project.schemaVersion = 3;
  project.id = id;
  project.name = 'Exclusive workflow analytical reference';
  const makeNode = (nodeId, type, processingMs) => ({
    id: nodeId, type, name: nodeId, position: { x: 0, y: 0 },
    config: { processingMs, capacity: 10, deduplicate: type === 'database' },
  });
  project.nodes = [makeNode('client', 'client', 0), makeNode('router', 'router', 2), makeNode('worker-a', 'service', 10), makeNode('worker-b', 'service', 20), makeNode('db-a', 'database', 3), makeNode('db-b', 'database', 5)];
  project.edges = [
    { id: 'c-r', source: 'client', target: 'router', latencyMs: 1 },
    { id: 'r-a', source: 'router', target: 'worker-a', latencyMs: 2 },
    { id: 'r-b', source: 'router', target: 'worker-b', latencyMs: 4 },
    { id: 'a-db', source: 'worker-a', target: 'db-a', latencyMs: 3 },
    { id: 'b-db', source: 'worker-b', target: 'db-b', latencyMs: 6 },
  ];
  project.scenario = { requestCount: 12, intervalMs: 100, timeoutMs: 100, maxRetries: 0, dropFirstResponse: false, jitterMs: 0, seed: 42 };
  project.requirements.find(value => value.rule === 'latency_p95').targetMs = 1000;
  return project;
}

function infrastructure(project, separateBranch = false) {
  project.infrastructure = {
    sites: [{ id: 'site-local', name: 'Local test site', latitude: 0, longitude: 0 }],
    hosts: [
      { id: 'host-main', name: 'Main', siteId: 'site-local', tier: 'edge', cpuCores: 4, memoryMb: 1024, idleWatts: 0, activeWatts: 0, outages: [] },
      { id: 'host-a', name: 'Branch A', siteId: 'site-local', tier: 'edge', cpuCores: 4, memoryMb: 1024, idleWatts: 0, activeWatts: 0, outages: [] },
    ],
    links: [{ id: 'physical-a', sourceHostId: 'host-main', targetHostId: 'host-a', latencyMs: 0, bandwidthKbps: 10000000, outages: [] }],
    placements: project.nodes.map(n => ({ nodeId: n.id, hostId: separateBranch && ['worker-a', 'db-a'].includes(n.id) ? 'host-a' : 'host-main' })),
    messageBytes: 1,
  };
}

async function reject(project, draft = false) {
  const result = await validate(project, draft);
  assert.equal(result.valid, false);
  assert.ok(result.issues.length > 0);
  if (!draft) await request('/api/runs', { method: 'POST', body: { project }, expected: 422 });
}

// ASCII-only fixture manifests, with insertion order exactly matching the typed
// serialization specified in WORKFLOW_CONTRACT.md. This is an independent digest
// producer, including for profiles that are never installed in the catalog.
function packageFixture(packageId = `acceptance.worker-${suffix}`) {
  return {
    schemaVersion: 1, id: packageId, version: '1.0.0', name: 'Acceptance worker',
    description: 'Bounded worker reference', author: 'Acceptance suite', license: 'MIT',
    kernel: 'service', capabilities: ['design', 'simulate'],
    config: {
      processingMs: { default: 10, min: 0, max: 100 },
      capacity: { default: 10, min: 1, max: 100 },
      memoryMb: { default: 64, min: 1, max: 1024 },
      deduplicate: false,
    },
  };
}
const digest = manifest => createHash('sha256').update(JSON.stringify(manifest)).digest('hex');
function pin(project, manifest) {
  const hash = digest(manifest);
  project.packages = [{ package: clone(manifest), digest: hash }];
  for (const nodeId of ['worker-a', 'worker-b']) {
    byID(project, nodeId).component = { packageId: manifest.id, version: manifest.version, digest: hash };
  }
  return hash;
}

try {
  assert.equal((await request('/api/health')).status, 'ok');
  const legacy = await request('/api/example');
  assert.equal(legacy.schemaVersion, 1);
  assert.equal((await request('/api/example?template=continuum')).schemaVersion, 2);
  const example = await request('/api/example?template=branching');
  assert.equal(example.schemaVersion, 3);
  assert.ok(example.nodes.some(value => value.type === 'router'));
  assert.ok(example.packages.length >= 1);
  assert.deepEqual(await validate(example), { valid: true, issues: [] });
  const examples = await request('/api/components/examples');
  assert.ok(['service', 'database', 'router'].every(kernel => examples.some(value => value.package.kernel === kernel)));
  examples.forEach(value => assert.match(value.digest, /^[a-f0-9]{64}$/));
  const project = fixture(legacy);
  assert.deepEqual(await validate(project), { valid: true, issues: [] });
  report('all three project schemas and portable starter package definitions remain available');

  const baseline = await run(project);
  // A: 2+10+3 CPU + 2*(1+2+3) wire = 27ms.
  // B: 2+20+5 CPU + 2*(1+4+6) wire = 49ms. Six requests select each.
  assert.deepEqual(routes(baseline), expectedRoutes(equalEdges));
  assert.equal(baseline.metrics.completed, 12);
  assert.equal(baseline.metrics.writes, 12);
  assert.equal(baseline.metrics.p95LatencyMs, 49);
  assert.equal(baseline.metrics.averageLatencyMs, 38);
  assert.equal(eventCount(baseline, 'write', 'db-a'), 6);
  assert.equal(eventCount(baseline, 'write', 'db-b'), 6);
  assert.equal(baseline.infrastructure, undefined);
  assert.ok(baseline.events.every(value => value.hostId === undefined && value.linkId === undefined));
  report('exclusive routes match a fixed hash vector and independent 27/49 ms path latencies');

  const weighted = clone(project);
  weighted.edges.find(value => value.id === 'r-b').routeWeight = 3;
  const selected = await run(weighted);
  assert.deepEqual(routes(selected), expectedRoutes(weightedEdges));
  assert.equal(eventCount(selected, 'write', 'db-a'), 4);
  assert.equal(eventCount(selected, 'write', 'db-b'), 8);
  weighted.edges.reverse();
  weighted.scenario.jitterMs = 9;
  const reordered = await run(weighted);
  assert.deepEqual(routes(reordered), expectedRoutes(weightedEdges));
  const repeated = await run(weighted);
  assert.deepEqual(repeated.events, reordered.events);
  assert.deepEqual(repeated.metrics, reordered.metrics);
  report('weighted route choice is independent of edge array order and jitter timing');

  const convergent = clone(project);
  convergent.nodes = convergent.nodes.filter(value => value.id !== 'db-b');
  byID(convergent, 'db-a').config.processingMs = 5;
  convergent.edges.find(value => value.id === 'b-db').target = 'db-a';
  const merged = await run(convergent);
  assert.equal(merged.metrics.writes, 12);
  assert.equal(merged.metrics.completed, 12);
  assert.equal(merged.metrics.averageLatencyMs, 39); // Six paths at29 and six at49.
  assert.equal(eventCount(merged, 'write', 'db-a'), 12);
  assert.equal(merged.events.filter(value => value.kind === 'processing' && ['worker-a', 'worker-b'].includes(value.nodeId)).length, 12);
  assert.deepEqual(routes(merged), expectedRoutes(equalEdges));
  report('convergent paths execute one branch and commit once, without a parallel-join barrier');

  const retry = clone(project);
  retry.scenario.dropFirstResponse = true;
  retry.scenario.maxRetries = 1;
  const recovered = await run(retry);
  assert.equal(recovered.metrics.completed, 12);
  assert.equal(recovered.metrics.writes, 12);
  assert.equal(recovered.metrics.deduplicated, 12);
  assert.equal(recovered.metrics.attempts, 24);
  assert.equal(recovered.metrics.averageLatencyMs, 138);
  for (const [requestId, edgeId] of expectedRoutes(equalEdges)) {
    assert.deepEqual(routeEvents(recovered).filter(value => value.requestId === requestId).map(value => [value.attempt, value.edgeId]), [[1, edgeId], [2, edgeId]]);
  }
  byID(retry, 'db-b').config.deduplicate = false;
  const mixedDedup = await run(retry);
  assert.equal(mixedDedup.metrics.completed, 12);
  assert.equal(mixedDedup.metrics.writes, 18);
  assert.equal(mixedDedup.metrics.deduplicated, 6);
  assert.equal(mixedDedup.metrics.duplicateWrites, 6);
  assert.equal(eventCount(mixedDedup, 'duplicate_write', 'db-a'), 0);
  assert.equal(eventCount(mixedDedup, 'duplicate_write', 'db-b'), 6);
  report('retries preserve each logical route and honor the selected database deduplication policy');

  const contention = clone(project);
  contention.edges.forEach(value => { value.latencyMs = 0; });
  contention.nodes.forEach(value => { value.config.processingMs = value.type === 'service' ? 10 : 0; });
  Object.assign(contention.scenario, { requestCount: 3, intervalMs: 1 });
  infrastructure(contention);
  contention.infrastructure.hosts[0].cpuCores = 1;
  const singleCore = await run(contention);
  contention.infrastructure.hosts[0].cpuCores = 2;
  const twoCores = await run(contention);
  // B,B,A each demand10 core-ms. One shared core also admits zero-time
  // router/store work FIFO: completions10,30,30 => latencies10,29,28.
  // Two cores give completions10,11,20 => latencies10,10,18.
  assert.equal(singleCore.metrics.p95LatencyMs, 29);
  assert.equal(twoCores.metrics.p95LatencyMs, 18);
  assert.equal(singleCore.infrastructure.hosts[0].busyCoreMs, 30);
  assert.equal(twoCores.infrastructure.hosts[0].busyCoreMs, 30);
  report('different workflow branches contend for shared host cores with conserved processing work');

  const disconnected = clone(project);
  disconnected.edges.forEach(value => { value.latencyMs = 0; });
  disconnected.nodes.forEach(value => { value.config.processingMs = value.type === 'service' ? 10 : 0; });
  infrastructure(disconnected, true);
  disconnected.infrastructure.hosts[1].outages = [{ startMs: 0, endMs: 2000 }];
  const powerFailure = await run(disconnected);
  assert.equal(powerFailure.metrics.completed, 6);
  assert.equal(powerFailure.metrics.failed, 6);
  assert.equal(powerFailure.metrics.writes, 6);
  assert.equal(eventCount(powerFailure, 'write', 'db-a'), 0);
  assert.equal(eventCount(powerFailure, 'write', 'db-b'), 6);
  assert.equal(powerFailure.infrastructure.links[0].droppedTransfers, 6);
  disconnected.infrastructure.hosts[1].outages = [];
  disconnected.infrastructure.links[0].outages = [{ startMs: 0, endMs: 2000 }];
  const linkFailure = await run(disconnected);
  assert.equal(linkFailure.metrics.completed, 6);
  assert.equal(linkFailure.metrics.writes, 6);
  assert.equal(linkFailure.infrastructure.links[0].bytesTransferred, 0);
  assert.equal(linkFailure.infrastructure.links[0].droppedTransfers, 6);
  report('power or network failure on one branch leaves the other branch functional');

  const cycle = clone(project);
  cycle.edges.push({ id: 'cycle', source: 'worker-a', target: 'router', latencyMs: 0 });
  await reject(cycle, true);
  const misplacedWeight = clone(project);
  misplacedWeight.edges[0].routeWeight = 2;
  await reject(misplacedWeight, true);
  const excessiveWeight = clone(project);
  excessiveWeight.edges[1].routeWeight = 101;
  await reject(excessiveWeight, true);
  const duplicateEdge = clone(project);
  duplicateEdge.edges.push({ ...duplicateEdge.edges[1], id: 'duplicate-pair' });
  await reject(duplicateEdge, true);
  const unresolved = clone(project);
  unresolved.edges[1].target = 'missing-node';
  await reject(unresolved, true);
  const degenerateRouter = clone(project);
  degenerateRouter.edges = degenerateRouter.edges.filter(value => value.id !== 'r-a');
  await reject(degenerateRouter);
  const disguisedV1 = clone(project);
  disguisedV1.schemaVersion = 1;
  await reject(disguisedV1);
  report('cycles, duplicate edges, unresolved endpoints, invalid weights and unsupported schemas fail explicitly');

  const manifest = packageFixture();
  const installed = await install(manifest);
  assert.equal(installed.digest, digest(manifest));
  assert.deepEqual(installed.package, manifest);
  assert.ok(Number.isFinite(Date.parse(installed.installedAt)));
  const reverseKeys = Object.fromEntries(Object.entries(manifest).reverse());
  assert.deepEqual(await install(reverseKeys, 200), installed);
  const conflict = clone(manifest);
  conflict.description = 'Changed definition at an immutable version';
  await install(conflict, 409);
  assert.deepEqual(await install(manifest, 200), installed);
  const newer = clone(manifest);
  newer.version = '1.1.0';
  newer.config.processingMs.default = 20;
  const next = await install(newer);
  assert.notEqual(next.digest, installed.digest);
  const catalog = await request('/api/components');
  assert.deepEqual(catalog.filter(value => value.package.id === manifest.id).map(value => value.package.version), ['1.0.0', '1.1.0']);
  report('package versions are immutable, exact, content-addressed and independently installable');

  const racing = packageFixture(`acceptance.race-${suffix}`);
  const identical = await Promise.all(Array.from({ length: 3 }, () => install(racing, [200, 201], true)));
  assert.deepEqual(identical.map(value => value.status).sort(), [200, 200, 201]);
  identical.forEach(value => assert.deepEqual(value.value, identical[0].value));
  const raceA = packageFixture(`acceptance.conflict-${suffix}`);
  const raceB = clone(raceA);
  raceB.name = 'Concurrent conflicting definition';
  const competing = await Promise.all([raceA, raceB].map(value => install(value, [201, 409], true)));
  assert.deepEqual(competing.map(value => value.status).sort(), [201, 409]);
  const winner = competing.find(value => value.status === 201).value;
  assert.deepEqual((await request('/api/components')).find(value => value.package.id === raceA.id), winner);
  report('concurrent identical or conflicting installs preserve one immutable version');

  const unsupportedKernel = clone(manifest);
  unsupportedKernel.kernel = 'python';
  await install(unsupportedKernel, 422);
  const unsupportedCapability = clone(manifest);
  unsupportedCapability.capabilities.push('deploy');
  await install(unsupportedCapability, 422);
  const rangeVersion = clone(manifest);
  rangeVersion.version = '^1.0.0';
  await install(rangeVersion, 422);
  await install({ ...manifest, script: 'arbitrary executable content' }, 400);
  const portable = clone(project);
  const portableManifest = packageFixture(`acceptance.portable-${suffix}`);
  pin(portable, portableManifest);
  assert.ok(!(await request('/api/components')).some(value => value.package.id === portableManifest.id));
  assert.deepEqual(await validate(portable), { valid: true, issues: [] });
  const portableRun = await run(portable);
  assert.deepEqual(portableRun.metrics, baseline.metrics);
  const localDefinition = clone(project);
  pin(localDefinition, conflict);
  assert.deepEqual((await run(localDefinition)).metrics, baseline.metrics);
  assert.deepEqual(await install(manifest, 200), installed);
  const tampered = clone(portable);
  tampered.packages[0].package.name = 'Changed without a new content digest';
  await reject(tampered, true);
  const missingLock = clone(portable);
  delete missingLock.packages;
  await reject(missingLock);
  const wrongVersion = clone(portable);
  byID(wrongVersion, 'worker-a').component.version = '2.0.0';
  await reject(wrongVersion, true);
  const wrongDigest = clone(portable);
  byID(wrongDigest, 'worker-a').component.digest = '0'.repeat(64);
  await reject(wrongDigest, true);
  const outOfBounds = clone(portable);
  byID(outOfBounds, 'worker-a').config.processingMs = 101;
  await reject(outOfBounds);
  const wrongKernel = clone(portable);
  byID(wrongKernel, 'router').component = clone(byID(wrongKernel, 'worker-a').component);
  await reject(wrongKernel, true);
  report('portable embedded profiles run without installation; missing, tampered or incompatible definitions never fall back');

  assert.deepEqual(await request('/api/projects', { method: 'POST', body: portable }), portable);
  saved = true;
  const exported = JSON.parse(JSON.stringify(await request(`/api/projects/${id}`)));
  assert.deepEqual(exported, portable);
  const repeat = await run(exported);
  assert.equal(repeat.modelHash, portableRun.modelHash);
  assert.deepEqual(repeat.events, portableRun.events);
  assert.deepEqual(repeat.metrics, portableRun.metrics);
  const alternate = clone(exported);
  const changedManifest = clone(portableManifest);
  changedManifest.config.processingMs.default = 17;
  pin(alternate, changedManifest);
  const changed = await run(alternate);
  assert.notEqual(changed.modelHash, portableRun.modelHash);
  assert.deepEqual(changed.metrics, portableRun.metrics); // Existing explicit values are not overwritten by defaults.
  await request('/api/projects', { method: 'POST', body: alternate });
  assert.deepEqual(await request(`/api/runs/${portableRun.id}`), portableRun);
  const history = await request(`/api/runs?projectId=${id}`);
  assert.ok(history.some(value => value.id === portableRun.id));
  assert.ok(history.some(value => value.id === changed.id));
  await request(`/api/projects/${id}`, { method: 'DELETE', expected: 204 });
  saved = false;
  assert.deepEqual(await request(`/api/runs/${portableRun.id}`), portableRun);
  report('project export preserves pinned definitions and immutable run snapshots remain reproducible after later edits');

  console.log(`\n${checks} workflow acceptance checks passed against ${base}.`);
} finally {
  if (saved) await request(`/api/projects/${id}`, { method: 'DELETE', expected: 204 });
}
