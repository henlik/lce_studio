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

async function artifactRequest(path,body,statuses=[200]){
 const r=await fetch(`${base}/api${path}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body),signal:AbortSignal.timeout(60000)});
 const value=await r.json();assert.ok(statuses.includes(r.status),JSON.stringify(value));return value;
}
const behaviorPackage={schemaVersion:2,id:`acceptance.copy-${randomUUID()}`,version:'1.0.0',name:'Boolean copy',description:'Synthetic typed link fixture',author:'Acceptance',license:'MIT',kernel:'composite',capabilities:['design','evaluate'],behavior:{inputs:[{name:'value',type:'boolean'}],outputs:[{name:'result',type:'boolean'}],nodes:[{id:'copy',op:'identity',inputs:{value:{input:'value'}}}],bindings:{result:{node:'copy',port:'result'}}}};
const bundle=await artifactRequest('/components/prepare',{package:behaviorPackage,dependencies:[]});
await artifactRequest('/components/bundles',bundle,[200,201]);
const linked={schemaVersion:1,id:`acceptance.linked-${randomUUID()}`,version:'1.0.0',name:'Linked copies',description:'Static compatibility, no joint execution',aspects:['source','target'].map(slot=>({slot,kind:'component',id:bundle.root.packageId,version:bundle.root.version,digest:bundle.root.digest})),links:[{id:'copy',source:{slot:'source',interface:'outputs',port:'result'},target:{slot:'target',interface:'inputs',port:'value'}}]};
const checked=await request('/definitions/validate','POST',linked);
assert.equal(checked.valid,true);assert.equal(checked.ports.length,4);assert.ok(checked.dependencies.length>=4);
await request('/definitions','POST',linked);
const wrongLink=structuredClone(linked);wrongLink.links[0].target.interface='outputs';wrongLink.links[0].target.port='result';
assert.equal((await request('/definitions/validate','POST',wrongLink)).valid,false);
await request('/definitions','POST',wrongLink,422);
const duplicateProducer=structuredClone(linked);duplicateProducer.links.push({...duplicateProducer.links[0],id:'second'});
assert.equal((await request('/definitions/validate','POST',duplicateProducer)).valid,false);
console.log('PASS typed aspect links, dependency inspection, direction checks and multiple-producer rejection');

const connectionFixture=await request('/connection-example','POST',{});
const connectionReport=await request('/resolve','POST',connectionFixture.design);
assert.equal(connectionReport.valid,true);assert.equal(connectionReport.compatibility,'checked');assert.equal(connectionReport.ports.length,4);
assert.ok(connectionReport.dependencies.some(d=>d.id==='example.encounter-contract'));
const mismatchReport=await request('/resolve','POST',connectionFixture.mismatch);
assert.equal(mismatchReport.valid,false);assert.equal(mismatchReport.compatibility,'invalid');assert.ok(mismatchReport.issues.some(i=>i.path==='connections[0].targetPort'));
const incomplete=structuredClone(connectionFixture.design);incomplete.contract.claims=[];delete incomplete.connections[0].targetPort;
assert.equal((await request('/interfaces','POST',incomplete)).ports.length,4);
const legacyReport=await request('/resolve','POST',await request('/example'));
assert.equal(legacyReport.compatibility,'incomplete');assert.equal(legacyReport.connections[0].status,'untyped');
await request('/conformance','POST',{design:connectionFixture.design},422);
await request('/'+connectionFixture.design.id,'PUT',connectionFixture.design);
assert.deepEqual(await request('/'+connectionFixture.design.id),connectionFixture.design);
console.log('PASS reception-to-consultation typed connections, contract-version rejection, incomplete-draft port inspection and legacy disclosure');

const executionPlan=await request('/execution/prepare','POST',{design:connectionFixture.design});
assert.equal(executionPlan.plan.tasks.length,2);assert.equal(executionPlan.plan.inputs.length,1);
const inputPort=executionPlan.plan.inputs[0];
const executionInputs=[{taskId:inputPort.taskId,values:{[inputPort.name]:{type:'record',recordRef:inputPort.recordRef,record:{encounterId:{type:'string',string:'synthetic-encounter-001'},patientId:{type:'string',string:'synthetic-patient-001'}}}}}];
const execution=await request('/execution/run','POST',{design:connectionFixture.design,inputs:executionInputs});
assert.equal(execution.report.mode,'pure-dataflow');assert.equal(execution.report.claimsEvaluated,false);
assert.deepEqual(execution.report.events.map(e=>e.kind),['started','completed','transfer','started','completed']);
assert.deepEqual(execution.report.results[1].outputs.encounter,executionInputs[0].values.encounter);
assert.deepEqual(await request('/executions/'+execution.id),execution);
assert.ok((await request('/executions?systemId='+connectionFixture.design.id)).some(e=>e.id===execution.id));
await request('/execution/run','POST',{design:connectionFixture.design,inputs:[]},422);
await request('/execution/prepare','POST',{design:connectionFixture.mismatch},422);
await request('/execution/prepare','POST',{design:await request('/example')},422);
console.log('PASS bounded pure execution, routed encounter values, ordered token trace, immutable snapshots and unsupported-path rejection');
