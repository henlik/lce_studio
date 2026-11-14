import test from 'node:test';
import assert from 'node:assert/strict';
import {emptySystem,parseSystemImport,normalizeSystemResponse} from '../src/systemTypes.ts';

test('new systems contain no seeded design and have independent identities',()=>{
 const first=emptySystem(),second=emptySystem();
 assert.notEqual(first.id,second.id);
 assert.deepEqual(first.components,[]);
 assert.deepEqual(first.connections,[]);
 assert.deepEqual(first.deploymentPlans,[]);
 assert.deepEqual(first.contract.claims,[]);
 assert.deepEqual(first.contract.assumptions,[]);
});
test('system import roundtrips complete structured definitions',()=>{
 const design=emptySystem();
 design.components.push({id:'intake',name:'Intake',definition:{id:'reception.intake',version:'1.0.0'},artifacts:[],bindings:[]});
 design.contract.claims.push({id:'unique',kind:'exactly_one_record',description:'One encounter'});
 assert.deepEqual(parseSystemImport(JSON.stringify(design)),design);
});
test('system import rejects structurally unsafe or unrelated JSON before editor adoption',()=>{
 for(const source of ['null','[]','{}','{"schemaVersion":99}',JSON.stringify({...emptySystem(),components:[null]}),JSON.stringify({...emptySystem(),contract:{claims:[]}}),JSON.stringify({...emptySystem(),deploymentPlans:[{id:'x',name:'x',resources:null,placements:[]}]})])assert.throws(()=>parseSystemImport(source));
});

test('API nil slices normalize without accepting malformed scalar fields',()=>{
 const design=emptySystem();
 const normalized=normalizeSystemResponse({...design,components:[{id:'intake',name:'Intake',definition:{id:'x',version:'1.0.0'},artifacts:null,bindings:null}],connections:null,deploymentPlans:[{id:'local',name:'Local',resources:null,placements:null}],contract:{...design.contract,claims:null,assumptions:null}});
 assert.deepEqual(normalized.components[0]?.artifacts,[]);
 assert.deepEqual(normalized.components[0]?.bindings,[]);
 assert.deepEqual(normalized.connections,[]);
 assert.deepEqual(normalized.deploymentPlans[0]?.resources,[]);
 assert.deepEqual(normalized.contract.assumptions,[]);
 assert.throws(()=>normalizeSystemResponse({...design,id:null}));
 assert.throws(()=>normalizeSystemResponse({...design,components:[{}]}));
 assert.throws(()=>normalizeSystemResponse({...design,connections:'bad'}));
});

test('system import distinguishes component packages from unified definitions',()=>{
 const design=emptySystem();
 design.components.push({id:'intake',name:'Intake',definition:{kind:'definition',id:'hospital.reception',version:'1.0.0'},artifacts:[],bindings:[]});
 assert.deepEqual(parseSystemImport(JSON.stringify(design)),design);
 const invalid=JSON.parse(JSON.stringify(design));invalid.components[0].definition.kind='data';
 assert.throws(()=>parseSystemImport(JSON.stringify(invalid)));
});

test('typed system ports roundtrip and partial or malformed endpoint objects are rejected',()=>{
 const d=emptySystem();
 d.connections=[{id:'handoff',source:'reception',target:'consultation',kind:'data',sourcePort:{slot:'root',interface:'outputs',port:'encounter'},targetPort:{slot:'root',interface:'inputs',port:'encounter'}}];
 assert.deepEqual(parseSystemImport(JSON.stringify(d)),d);
 for(const malformed of [null,{},'encounter',{slot:'root',interface:'inputs',port:1}]){
  const candidate=JSON.parse(JSON.stringify(d));candidate.connections[0].targetPort=malformed;
  assert.throws(()=>parseSystemImport(JSON.stringify(candidate)));
 }
 const missing=JSON.parse(JSON.stringify(d));delete missing.connections[0].targetPort;
 assert.throws(()=>parseSystemImport(JSON.stringify(missing)));
});

test('offline providers survive imports and malformed provider shapes are rejected',()=>{
 const d=emptySystem();d.executionProviders=[{id:'state-model',kind:'modeled-state',version:'1.0.0'}];
 assert.deepEqual(parseSystemImport(JSON.stringify(d)),d);
 for(const providers of [null,{},[{}],[{id:'x',kind:'modeled-state',version:1}]])assert.throws(()=>parseSystemImport(JSON.stringify({...d,executionProviders:providers})));
});
