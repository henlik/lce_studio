import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { workflowStages, stageBoundary, stageProgress, stagePath } from '../src/workflowStages.ts';
import { inspectionVerdict } from '../src/inspection.ts';
import type { WorkflowDefinition } from '../src/workflowTypes.ts';
const source = JSON.parse(readFileSync(new URL('../../examples/reception/hospital-reception.workflow.json', import.meta.url),'utf8')) as WorkflowDefinition;
test('stage boundaries preserve branches, cross-stage recovery, and original definition', () => {
 const w = structuredClone(source);
 for (const n of w.nodes) n.stage = ['lookupIdentity','classifyLookup','identityRoute','registerPatient','confirmIdentity','identityReview','validateIdentity'].includes(n.id) ? 'Arrival/Identity' : 'Other';
 const before = JSON.stringify(w);
 const root = workflowStages(w);
 assert.equal(root.reduce((sum,g) => sum+g.nodes.length,0),w.nodes.length);
 const boundary = stageBoundary(w,'Arrival');
 assert.equal(boundary.ids.length,7);
 assert.ok(boundary.incoming.length>0);
 assert.ok(boundary.outgoing.some(r => r.to === 'lookupAffiliation'));
 assert.equal(workflowStages(w,'Arrival')[0]?.path,'Arrival/Identity');
 assert.equal(JSON.stringify(w),before);
});
test('re-entry is current again; unvisited alternatives never count as completed', () => {
 const visits = [{nodeId:'a'}, {nodeId:'b'}, {nodeId:'a'}];
 assert.equal(stageProgress(['a'],visits,0),'current');
 assert.equal(stageProgress(['a'],visits,1),'visited');
 assert.equal(stageProgress(['a'],visits,2),'current');
 assert.equal(stageProgress(['never'],visits,2),'not visited');
 assert.equal(stagePath(' Arrival / Queue '),'Arrival/Queue');
});
test('inspection distinguishes expected rejection from missing or wrong evidence', () => {
 const item = {verdictPath:'result.valid',expected:false} as any;
 assert.equal(inspectionVerdict(item,{result:{valid:false}}).passed,true);
 assert.equal(inspectionVerdict(item,{result:{valid:true}}).passed,false);
 assert.equal(inspectionVerdict(item,{}).passed,false);
});
