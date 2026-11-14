#!/usr/bin/env node
import assert from 'node:assert/strict';
const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/,'');
async function request(path, body) {
 const response = await fetch(`${base}/api${path}`, {method:body ? 'POST':'GET',headers:body?{'Content-Type':'application/json'}:{},body:body?JSON.stringify(body):undefined,signal:AbortSignal.timeout(30000)});
 const result = await response.json();
 assert.ok(response.ok,`${path}: ${response.status} ${JSON.stringify(result).slice(0,1200)}`);
 return result;
}
const pack=await request('/inspection');
assert.ok(pack.cases.length>=30);
const seen=new Set();
for (const item of pack.cases) {
 assert.ok(!seen.has(item.id));seen.add(item.id);
 const result=await request(item.endpoint,item.body);
 const actual=item.verdictPath.split('.').reduce((v,key)=>v?.[key],result);
 assert.deepEqual(actual,item.expected,`${item.id}: ${item.name}: ${item.verdictPath}`);
 if (item.workspace==='components') assert.ok(result.cases.length>0, 'empty component test suite');
 if(item.workspace==='workflows') {
  assert.equal(result.result.verified,true);
  const original = await request(`/workflow/runs/${result.id}`);
  assert.deepEqual(original,result,'immutable workflow history');
 }
 console.log(`PASS ${item.id} ${item.name}`);
}
console.log(`Inspection lab: ${seen.size} independently checked exercises passed.`);
