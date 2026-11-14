import test from 'node:test';
import assert from 'node:assert/strict';
import {stockScenario,multiCommandScenario,ownerCommandStep} from '../src/systemScenario.ts';
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
test('owner commands use only the chosen owner and start with empty typed inputs',()=>{
 const multi=structuredClone(plan);multi.tasks[1]!.state!.machine.machine.commands=[{name:'reserve'},{name:'adjust'}] as any;
 const step=ownerCommandStep(multi,'owner','adjust','adjustA');
 assert.equal(step.kind,'command');assert.deepEqual(step.inputs,[{taskId:'owner',values:{}}]);assert.equal(step.calls![0]!.command,'adjust');assert.deepEqual(step.expectedStatuses,{});
 assert.throws(()=>ownerCommandStep(multi,'request','adjust'));assert.throws(()=>ownerCommandStep(multi,'owner','unknown'));
});
test('multi-command reference has independent inputs and handwritten owner expectations',()=>{
 const multi=structuredClone(plan);multi.tasks[1]!.state!.entryCommand='reserve';multi.tasks[1]!.state!.machine.machine.commands=[{name:'reserve'},{name:'adjust'}] as any;
 const scenario=multiCommandScenario(multi);assert.equal(scenario.steps.length,10);assert.equal(scenario.steps[1]!.calls![0]!.command,'adjust');assert.equal(scenario.steps[5]!.calls![0]!.command,'reserve');
 assert.equal(scenario.steps[6]!.expectedStatuses.owner,'id_conflict');assert.equal(scenario.expected[0]!.state.available!.integer,7);assert.equal(scenario.expected[0]!.receipts,4);assert.equal(scenario.expected[0]!.revision,3);
 scenario.steps[1]!.inputs![0]!.values.count!.integer=999;assert.equal(scenario.steps[2]!.inputs![0]!.values.count!.integer,8);
 multi.tasks[1]!.state!.entryCommand='adjust';assert.throws(()=>multiCommandScenario(multi));
});
