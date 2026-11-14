#!/usr/bin/env node
// Run against a disposable studio database. Synthetic design/evidence remain.
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';

const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
async function request(path, method = 'GET', body, status = 200) {
  const response = await fetch(`${base}/api/systems${path}`, {
    method, headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined, signal: AbortSignal.timeout(60000),
  });
  const result = await response.json();
  assert.equal(response.status, status, JSON.stringify(result));
  return result;
}

const design = await request('/example');
design.id = `acceptance.${randomUUID()}`;
assert.equal((await request('/validate', 'POST', design)).valid, true);
await request(`/${design.id}`, 'PUT', design);
assert.deepEqual(await request(`/${design.id}`), design);
const evidence = await request('/conformance', 'POST', { design });
assert.equal(evidence.report.passed, true);
assert.deepEqual(evidence.design, design);
assert.equal(evidence.report.scenarios.length, 7);
const scenario = id => evidence.report.scenarios.find(s => s.id === id);
for (const id of ['normal', 'lost-response', 'concurrent-duplicates', 'restart', 'database-unavailable']) {
  assert.ok(scenario(id).implementation.verdicts.every(v => v.status === 'satisfied'), id);
}
assert.ok(scenario('faulty-dedup').implementation.verdicts.some(v => v.status === 'violated'));
assert.ok(scenario('missing-evidence').implementation.verdicts.every(v => v.status === 'inconclusive'));
assert.ok(evidence.report.scenarios.every(s => s.model.verdicts.every(v => v.status === 'satisfied')));
assert.deepEqual(await request(`/evidence/${evidence.id}`), evidence);
assert.ok((await request(`/evidence?systemId=${design.id}`)).some(e => e.id === evidence.id));

design.name = 'Edited after execution';
await request(`/${design.id}`, 'PUT', design);
assert.notEqual((await request(`/evidence/${evidence.id}`)).design.name, design.name);
const invalid = structuredClone(design);
invalid.contract.claims[0].kind = 'arbitrary-code';
assert.equal((await request('/validate', 'POST', invalid)).valid, false);
console.log('PASS unified system persistence, seven conformance scenarios, immutable evidence, and unsupported claim rejection');
