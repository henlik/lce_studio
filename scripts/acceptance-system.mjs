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
const resolution=await request('/resolve','POST',design);
assert.equal(resolution.valid,true);
assert.equal(resolution.references.length,2);
assert.ok(resolution.references.every(r=>r.digest.length===64&&r.source==='builtin-adapter'));
const missing=structuredClone(design);
missing.components[1].artifacts.push({kind:'data',id:'missing.reference',version:'1.0.0'});
assert.equal((await request('/validate','POST',missing)).valid,true);
assert.equal((await request('/resolve','POST',missing)).valid,false);
await request('/conformance','POST',{design:missing},422);
await request(`/${design.id}`, 'PUT', design);
assert.deepEqual(await request(`/${design.id}`), design);
const evidence = await request('/conformance', 'POST', { design });
assert.equal(evidence.report.passed, true);
assert.deepEqual(evidence.design, design);
assert.deepEqual(evidence.resolution,resolution);
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

// Ownership manifests use real installed artifacts, without claiming they execute jointly.
const examplesResponse=await fetch(`${base}/api/data/examples`);
assert.equal(examplesResponse.status,200);
const dataExamples=await examplesResponse.json();
const contract=dataExamples[0].contract;
const installedResponse=await fetch(`${base}/api/data/contracts`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(contract)});
assert.ok([200,201].includes(installedResponse.status),await installedResponse.text());
const entry=(await request('/catalog')).find(e=>e.kind==='data'&&e.id===contract.contract.id&&e.version===contract.contract.version);
assert.ok(entry?.ready);
const definition={schemaVersion:1,id:`acceptance.definition-${randomUUID()}`,version:'1.0.0',name:'Reception ownership',description:'Synthetic ownership acceptance',aspects:[{slot:'patients',kind:entry.kind,id:entry.id,version:entry.version,digest:entry.digest}]};
assert.equal((await request('/definitions/validate','POST',definition)).valid,true);
const installed=await request('/definitions','POST',definition);
assert.deepEqual(await request('/definitions','POST',definition),installed);
const editor=await request('/definitions/aspect','POST',definition.aspects[0]);
assert.equal(editor.workspace,'data');assert.deepEqual(editor.body.contract,contract);
await request('/definitions','POST',{...definition,name:'Cannot overwrite'},409);
const invalidPin=structuredClone(definition);invalidPin.aspects[0].digest='0'.repeat(64);
await request('/definitions','POST',invalidPin,422);
const composed=structuredClone(design);composed.components[0].definition={kind:'definition',id:definition.id,version:definition.version};
const composedResolution=await request('/resolve','POST',composed);
assert.equal(composedResolution.valid,true);assert.equal(composedResolution.references.length,3);
assert.equal(composedResolution.references[1].digest,entry.digest);
await request('/conformance','POST',{design:composed},422);
console.log('PASS immutable ownership, exact aspect editor payload, expanded resolution and unsupported joint execution rejection');
