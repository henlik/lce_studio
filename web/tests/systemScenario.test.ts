import test from 'node:test';
import assert from 'node:assert/strict';
import {stockScenario} from '../src/systemScenario.ts';
import type {ExecutionPlan} from '../src/executionTypes.ts';

const plan={tasks:[{id:'request',componentId:'request'},{id:'owner',componentId:'stock',state:{machine:{machine:{id:'studio.example-stock-owner'}}}},{id:'summary',componentId:'summary'}],inputs:[{taskId:'request',name:'request',type:'record',recordRef:{contractId:'reservation',version:'1.0.0',digest:'a'.repeat(64),record:'Request'}}]} as unknown as ExecutionPlan;
test('retry scenario keeps repeated command identity but independently editable request snapshots',()=>{
 const scenario=stockScenario(plan);
 assert.equal(scenario.steps[0]!.calls![0]!.commandId,scenario.steps[1]!.calls![0]!.commandId);
 scenario.steps[0]!.inputs![0]!.values.request!.record!.quantity!.integer=99;
 assert.equal(scenario.steps[1]!.inputs![0]!.values.request!.record!.quantity!.integer,2);
 assert.equal(scenario.expected[0]!.state.available!.integer,2);
 assert.equal(scenario.steps[3]!.expectedStatuses.summary,'skipped');
 assert.equal(scenario.steps[4]!.kind,'restart');
 assert.equal(scenario.steps[5]!.calls![0]!.expectedRevision,1);
});
test('stock scenario is explicit and refuses unrelated prepared graphs',()=>{
 assert.throws(()=>stockScenario({...plan,tasks:[]}));
 assert.throws(()=>stockScenario({...plan,inputs:[]}));
});
