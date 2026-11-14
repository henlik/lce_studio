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

const stateFixture=await request('/state-execution-example','POST',{});
const statePlan=await request('/execution/prepare','POST',{design:stateFixture.design});
assert.equal(statePlan.plan.tasks.length,3);assert.equal(statePlan.plan.inputs.length,1);
const statePort=statePlan.plan.inputs[0];
const stateInputs=[{taskId:statePort.taskId,values:{request:{type:'record',recordRef:statePort.recordRef,record:{tenantId:{type:'string',string:'tenantDefault'},resourceId:{type:'string',string:'resourceDefault'},quantity:{type:'integer',integer:2}}}}}];
const stateRun=await request('/execution/run','POST',{design:stateFixture.design,inputs:stateInputs});
assert.equal(stateRun.report.mode,'stateful-dataflow');assert.equal(stateRun.report.events.length,8);
const stateResult=stateRun.report.results.find(r=>r.state);
assert.equal(stateResult.state.step.status,'committed');assert.equal(stateResult.state.instance.revision,1);
assert.equal(stateResult.state.instance.state.available.integer,3);assert.equal(stateResult.state.instance.receipts.length,1);assert.equal(stateResult.state.instance.effects.length,1);
assert.equal(stateRun.report.results.at(-1).outputs.remaining.integer,3);
assert.deepEqual(await request('/executions/'+stateRun.id),stateRun);
await request('/'+stateFixture.design.id,'PUT',stateFixture.design);
assert.deepEqual(await request('/'+stateFixture.design.id),stateFixture.design);
stateInputs[0].values.request.record.quantity.integer=9;
const denied=await request('/execution/run','POST',{design:stateFixture.design,inputs:stateInputs});
assert.equal(denied.report.results.find(r=>r.state).state.step.status,'denied');assert.equal(denied.report.results.at(-1).outputs.remaining.integer,5);
const missingProvider=structuredClone(stateFixture.design);missingProvider.components[1].bindings=[];
await request('/execution/prepare','POST',{design:missingProvider},422);
const liveProvider=structuredClone(stateFixture.design);liveProvider.executionProviders[0].kind='postgresql';
await request('/execution/prepare','POST',{design:liveProvider},422);
console.log('PASS connected state commit/denial, provider rejection, owned revisions/receipts/intents and immutable snapshots');

const ownerId=statePlan.plan.tasks.find(t=>t.state).id;
const summaryId=statePlan.plan.tasks.find(t=>t.componentId==='summary').id;
const wave=(commandId,expectedRevision,quantity,status,downstream='completed',fault='')=>{
 const inputs=structuredClone(stateInputs);inputs[0].values.request.record.quantity.integer=quantity;
 return {kind:'invoke',inputs,calls:[{taskId:ownerId,commandId,expectedRevision,fault}],expectedStatuses:{[ownerId]:status,[summaryId]:downstream}};
};
const sharedScenario={name:'Stock shared scenario',steps:[wave('reserveA',0,2,'committed'),wave('reserveA',0,2,'replayed'),wave('stale',0,1,'revision_conflict','skipped'),wave('reserveB',1,1,'reply_lost','skipped','after_commit_before_reply'),{kind:'restart',taskId:ownerId,expectedStatuses:{[ownerId]:'restarted'}},wave('reserveB',1,1,'replayed')],expected:[{taskId:ownerId,revision:2,receipts:2,effects:2,state:{tenantId:{type:'string',string:'tenantDefault'},resourceId:{type:'string',string:'resourceDefault'},available:{type:'integer',integer:2},acceptedSinceRestart:{type:'integer',integer:0}}}]};
const sharedRun=await request('/execution/run','POST',{design:stateFixture.design,scenario:sharedScenario});
assert.equal(sharedRun.report.mode,'shared-scenario');assert.equal(sharedRun.report.scenario.verified,true);assert.equal(sharedRun.report.scenario.passed,true);assert.equal(sharedRun.report.claimsEvaluated,false);
assert.equal(sharedRun.report.scenario.final[0].instance.state.available.integer,2);
assert.ok(!sharedRun.report.events.some(e=>[3,4].includes(e.scenarioStep)&&e.kind==='transfer'&&e.taskId===ownerId));
assert.deepEqual(await request('/executions/'+sharedRun.id),sharedRun);
const wrongScenario=structuredClone(sharedScenario);wrongScenario.expected[0].state.available.integer=99;
const wrongRun=await request('/execution/run','POST',{design:stateFixture.design,scenario:wrongScenario});assert.equal(wrongRun.report.scenario.passed,false);
const invalidScenario=structuredClone(sharedScenario);delete invalidScenario.steps[0].calls[0].expectedRevision;
await request('/execution/run','POST',{design:stateFixture.design,scenario:invalidScenario},422);
await request('/execution/run','POST',{design:stateFixture.design,scenario:sharedScenario,inputs:stateInputs},400);
console.log('PASS shared-scenario state continuity, blocked reply transfers, independent failed verdicts and immutable snapshots');

for(const source of [execution,stateRun,sharedRun,wrongRun]){
 assert.equal(source.snapshotVersion,1);assert.equal(source.planHash.length,64);assert.equal(source.reportHash.length,64);
 const replay=await request('/executions/'+source.id+'/replays','POST',{});
 assert.equal(replay.status,'matched');assert.equal(replay.executionId,source.id);assert.equal(replay.report.claimsEvaluated,false);
 assert.deepEqual(replay.report,source.report);assert.equal(replay.recordedReportHash,replay.replayedReportHash);
 assert.deepEqual(await request('/execution-replays/'+replay.id),replay);
 assert.ok((await request('/executions/'+source.id+'/replays')).some(r=>r.id===replay.id));
 assert.deepEqual(await request('/executions/'+source.id),source);
}
await request('/executions/'+execution.id+'/replays','POST',{inputs:executionInputs},400);
await request('/executions/missing/replays','POST',{},404);
console.log('PASS immutable catalog-independent replay verification for pure, state, shared and failed scenarios');

const multiFixture=await request('/multi-command-example','POST',{});
const multiPlan=await request('/execution/prepare','POST',{design:multiFixture.design});
assert.equal(multiPlan.plan.engineVersion,'system-dataflow-3');
const multiOwner=multiPlan.plan.tasks.find(t=>t.state);
assert.equal(multiOwner.state.entryCommand,'reserve');assert.equal(multiOwner.state.machine.machine.commands.length,2);
const direct=(command,commandId,expectedRevision,values,status,fault='')=>({kind:'command',taskId:ownerId,calls:[{taskId:ownerId,command,commandId,expectedRevision,fault}],inputs:[{taskId:ownerId,values}],expectedStatuses:{[ownerId]:status}});
const lostMulti=wave('reserveB',2,1,'reply_lost','skipped','after_commit_before_reply');
const multiScenario={name:'Reserve and adjust one owner',steps:[wave('reserveA',0,2,'committed'),direct('adjust','adjustA',1,{count:{type:'integer',integer:8}},'committed'),direct('adjust','adjustA',1,{count:{type:'integer',integer:8}},'replayed'),lostMulti,{kind:'restart',taskId:ownerId},direct('reserve','reserveB',2,lostMulti.inputs[0].values,'replayed'),direct('adjust','reserveA',0,{count:{type:'integer',integer:4}},'id_conflict'),direct('adjust','staleAdjust',2,{count:{type:'integer',integer:6}},'revision_conflict'),direct('adjust','rollbackAdjust',3,{count:{type:'integer',integer:9}},'rolled_back','before_commit'),direct('adjust','invalidAdjust',3,{count:{type:'integer',integer:-1}},'denied')],expected:[{...sharedScenario.expected[0],revision:3,receipts:4,effects:2,state:{...sharedScenario.expected[0].state,available:{type:'integer',integer:7}}}]};
const multiRun=await request('/execution/run','POST',{design:multiFixture.design,scenario:multiScenario});
assert.equal(multiRun.report.engineVersion,'system-scenario-2');assert.equal(multiRun.report.scenario.passed,true);assert.equal(multiRun.report.claimsEvaluated,false);
assert.equal(multiRun.report.scenario.final[0].instance.state.available.integer,7);
assert.ok(!multiRun.report.events.some(e=>e.kind==='transfer'&&![1,4].includes(e.scenarioStep)));
assert.deepEqual(await request('/executions/'+multiRun.id),multiRun);
const multiReplay=await request('/executions/'+multiRun.id+'/replays','POST',{});assert.equal(multiReplay.status,'matched');assert.deepEqual(multiReplay.report,multiRun.report);
const absentEntry=structuredClone(multiFixture.design);delete absentEntry.components[1].stateEntries;await request('/execution/prepare','POST',{design:absentEntry},422);
const wrongEntry=structuredClone(multiFixture.design);wrongEntry.components[1].stateEntries[0].command='adjust';await request('/execution/prepare','POST',{design:wrongEntry},422);
const wrongCall=structuredClone(multiScenario);wrongCall.steps[1].calls[0].command='unknown';await request('/execution/run','POST',{design:multiFixture.design,scenario:wrongCall},422);
console.log('PASS selected graph entry, shared multi-command receipts/revisions, direct-call isolation, negative inputs and replay');

const messagesExample=await request('/message-example','POST');
const messagePlan=await request('/execution/prepare','POST',{design:messagesExample.design});
const messageStock=messagePlan.plan.tasks.find(t=>t.componentId==='stock');
const messageLedger=messagePlan.plan.tasks.find(t=>t.componentId==='ledger');
const channelId=messagePlan.plan.channels[0].id;
const quantity=(n)=>({type:'integer',integer:n});
const messageRequest={request:{type:'record',recordRef:messageStock.state.machine.machine.commands[0].inputs[0].recordRef,record:{tenantId:{type:'string',string:'tenantDefault'},resourceId:{type:'string',string:'resourceDefault'},quantity:quantity(2)}}};
const reservation=(id,fault,status)=>({kind:'command',taskId:messageStock.id,inputs:[{taskId:messageStock.id,values:structuredClone(messageRequest)}],calls:[{taskId:messageStock.id,command:'reserve',commandId:id,expectedRevision:0,fault}],expectedStatuses:{[messageStock.id]:status}});
const dispatchNotice=(id,alias,status)=>({kind:'dispatch',message:{channelId,routeId:'reservationToLedger',sourceCommandId:id,alias,expectedRevision:0},expectedMessageStatus:status});
const deliverNotice=(kind,fault,status)=>({kind,message:{channelId,alias:'reservationA',fault},expectedMessageStatus:status});
const baseState=(task)=>Object.fromEntries(task.state.machine.machine.state.map(s=>[s.name,s.initial]));
const messageScenario={name:'Stock-to-ledger acceptance',steps:[reservation('rollback','before_commit','rolled_back'),dispatchNotice('rollback','noMessage','missing_intent'),reservation('reserveA','','committed'),dispatchNotice('reserveA','reservationA','dispatched'),deliverNotice('deliver','drop_before_receive','dropped'),deliverNotice('retry','before_recipient_commit','recipient_unacknowledged'),deliverNotice('retry','after_recipient_commit_before_ack','acknowledgement_lost'),{kind:'broker_restart',message:{channelId},expectedMessageStatus:'broker_restarted'},{kind:'restart',taskId:messageLedger.id},{kind:'restart',taskId:messageStock.id},deliverNotice('retry','','acknowledged'),deliverNotice('retry','','already_acknowledged'),reservation('reserveA','','replayed'),dispatchNotice('reserveA','reservationA','dispatch_replayed')],expected:[{taskId:messageStock.id,state:{...baseState(messageStock),available:quantity(3)},revision:1,receipts:1,effects:1},{taskId:messageLedger.id,state:{...baseState(messageLedger),totalReserved:quantity(2)},revision:1,receipts:1,effects:0}],expectedMessages:[{channelId,name:'noMessage',status:'absent',attempts:0},{channelId,name:'reservationA',status:'acknowledged',attempts:4,accepted:true}]};
const messageRun=await request('/execution/run','POST',{design:messagesExample.design,scenario:messageScenario});
assert.equal(messageRun.report.engineVersion,'system-scenario-3');assert.equal(messageRun.report.scenario.passed,true);assert.equal(messageRun.report.claimsEvaluated,false);
assert.deepEqual(await request('/executions/'+messageRun.id),messageRun);
const lostAck=messageRun.report.events.find(e=>e.kind==='acknowledgement_lost');
assert.equal(lostAck.message.messages[0].status,'pending');assert.equal(lostAck.message.messages[0].acknowledgement,undefined);
for(const phase of ['intent_created','dispatched','delivered','recipient_committed','recipient_replayed','acknowledged','broker_restarted'])assert.ok(messageRun.report.events.some(e=>e.kind===phase),phase);
const messageReplay=await request('/executions/'+messageRun.id+'/replays','POST',{});assert.equal(messageReplay.status,'matched');
const wrongMessages=structuredClone(messageScenario);wrongMessages.expectedMessages[1].attempts=3;
assert.equal((await request('/execution/run','POST',{design:messagesExample.design,scenario:wrongMessages})).report.scenario.passed,false);
const wrongBinding=structuredClone(messagesExample.design);wrongBinding.components[0].messageOwners[0].componentId='ledger';
await request('/execution/prepare','POST',{design:wrongBinding},422);
const wrongTransport=structuredClone(messageScenario);delete wrongTransport.steps[3].message.expectedRevision;
await request('/execution/run','POST',{design:messagesExample.design,scenario:wrongTransport},422);
await request('/execution/run','POST',{design:messagesExample.design,inputs:[]},422);
console.log('PASS shared owner/broker delivery, rollback isolation, lost acknowledgements, durable restarts, duplicate suppression, journey trace and replay');

const m1=await request('/m1-example','POST');
const m1Plan=await request('/execution/prepare','POST',{design:m1.design});
assert.equal(m1Plan.plan.engineVersion,'system-dataflow-5');
assert.equal(m1Plan.providerChecks.length,4);assert.ok(m1Plan.providerChecks.every(c=>c.satisfied&&c.required.length));
const m1Cases=await request('/m1-scenarios','POST',{design:m1.design});assert.equal(m1Cases.length,6);
assert.deepEqual(m1Cases,m1.scenarios);
for(const c of m1Cases){
 const run=await request('/execution/run','POST',{design:m1.design,scenario:c.scenario});
 assert.equal(run.report.engineVersion,'system-scenario-4');assert.equal(run.report.scenario.passed,true,c.name);assert.equal(run.report.claimsEvaluated,false);
 assert.equal(run.report.scenario.workflows.length,1);
 assert.ok(run.report.events.some(e=>e.workflow));
 assert.deepEqual(await request('/executions/'+run.id),run);
 const replay=await request('/executions/'+run.id+'/replays','POST',{});
 assert.equal(replay.status,'matched',c.name);assert.deepEqual(replay.report,run.report);
 console.log('PASS M1: '+c.name);
}
const m1WrongCapability=structuredClone(m1.design);m1WrongCapability.components[0].bindings[0].requiredCapabilities=['live-network'];
await request('/execution/prepare','POST',{design:m1WrongCapability},422);
const m1WrongOwner=structuredClone(m1.design);m1WrongOwner.components[0].workflowOwners[0].componentId='ledger';
await request('/execution/prepare','POST',{design:m1WrongOwner},422);
const forgedM1=structuredClone(m1Cases[0].scenario);forgedM1.steps=forgedM1.steps.slice(0,2);
forgedM1.steps.push({kind:'workflow_event',workflow:{instanceId:'reservation',event:{id:'forged',kind:'response',taskId:'task2',values:{accepted:{type:'boolean',boolean:true}}}}});
await request('/execution/run','POST',{design:m1.design,scenario:forgedM1},422);
const wrongM1=structuredClone(m1Cases[0].scenario);wrongM1.expectedWorkflows[0].outcome='wrongOutcome';
assert.equal((await request('/execution/run','POST',{design:m1.design,scenario:wrongM1})).report.scenario.passed,false);
await request('/m1-scenarios','POST',{design:m1.design,unknown:true},400);
await request('/execution/run','POST',{design:m1.design,inputs:[]},422);
console.log('PASS M1 provider checks, shared ownership, correlated acknowledgements, negative assertions and immutable replay');
