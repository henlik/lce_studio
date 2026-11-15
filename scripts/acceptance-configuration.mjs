#!/usr/bin/env node
// Use a disposable studio database, as for the other acceptance scripts.
import assert from 'node:assert/strict';
const base=(process.env.STUDIO_URL||'http://127.0.0.1:8080').replace(/\/$/,'');
async function request(path,body,status=200){const r=await fetch(base+'/api/configuration'+path,{method:body===undefined?'GET':'POST',headers:body===undefined?{}:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(60000)});const value=await r.json();assert.equal(r.status,status,JSON.stringify(value).slice(0,2000));return value;}
const project=await request('/example');
const c=await request('/compile',{project});assert.equal(c.valid,true);assert.equal(c.artifacts.length,56);assert.equal(c.experiments.length,43);
assert.deepEqual([...new Set(c.artifacts.map(a=>a.kind))].sort(),['behavior','data','definition','messages','state','system','timing','workflow']);
const report=await request('/run',{project});assert.equal(report.passed,true);assert.equal(report.cases.length,43);
const format=await request('/format',{project,expectedHash:c.sourceHash,file:project.entry});assert.equal(format.compilation.valid,true);
await request('/format',{project,expectedHash:'stale'},409);
const pkg=await request('/pack',{project,id:'acceptance.configuration',version:'1.0.0',test:true});assert.equal(pkg.report.passed,true);
assert.deepEqual(await request('/replay',{package:pkg}),pkg.report);
const first=await request('/install',{package:pkg});assert.ok(first.installed+first.existing>0);
assert.equal((await request('/install',{package:pkg})).installed,0);
const stored=await request('/package?id=acceptance.configuration&version=1.0.0');assert.equal(stored.digest,pkg.digest);
assert.equal((await request('/compare',{previous:pkg,package:stored})).compatible,true);
const tampered=structuredClone(pkg);tampered.artifacts[0].body.name='Forged';await request('/verify',{package:tampered},422);
const missing=structuredClone(project);delete missing.lock;delete missing.files[Object.keys(missing.files).find(f=>f!==missing.entry)];assert.equal((await request('/compile',{project:missing})).valid,false);
console.log('Configuration acceptance passed: 56 artifacts, 43 experiments, all eight kinds, exact replay, atomic installation and negative controls.');
const nested=await request('/example?template=hierarchy');
const nc=await request('/compile',{project:nested});assert.equal(nc.valid,true);assert.equal(nc.workspace.roots.length,1);assert.equal(nc.artifacts.length,4);
const queue=nc.artifacts.find(a=>a.body.name==='Queue management');
const target={kind:queue.kind,id:queue.id,version:queue.version,digest:queue.digest};
const scoped=await request('/run',{project:nested,scope:{target,descendants:true}});assert.equal(scoped.passed,true);assert.equal(scoped.cases.length,2);assert.deepEqual(scoped.scope.target,target);
const organized=await request('/organize',{project:nested,expectedHash:nc.sourceHash});assert.equal(organized.compilation.valid,true);assert.equal(Object.keys(organized.project.files).length,9);
assert.equal((await request('/run',{project:organized.project})).cases.length,4);
await request('/organize',{project:nested,expectedHash:'stale'},409);
console.log('Hierarchy acceptance passed: three levels, four independent testbenches, exact scoped runs and semantics-preserving folders.');
