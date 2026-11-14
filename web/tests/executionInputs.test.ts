import test from 'node:test';
import assert from 'node:assert/strict';
import {encodeExecutionValue,executionInputs,inputKey,type ExecutionInput} from '../src/executionTypes.ts';
const p:ExecutionInput={taskId:'task',name:'encounter',type:'record',recordRef:{contractId:'example.encounter',version:'1.0.0',digest:'a'.repeat(64),record:'Encounter'},fields:[{name:'patientId',type:'string'},{name:'urgent',type:'boolean'},{name:'priority',type:'integer'}]};
test('execution inputs preserve exact record identity and encode scalar fields',()=>{
 const value=encodeExecutionValue(p,{patientId:'synthetic',urgent:'false',priority:'2'});
 assert.deepEqual(value,{type:'record',recordRef:p.recordRef,record:{patientId:{type:'string',string:'synthetic'},urgent:{type:'boolean',boolean:false},priority:{type:'integer',integer:2}}});
 assert.deepEqual(executionInputs([p],{[inputKey(p)]:{patientId:'synthetic',urgent:false,priority:2}}),[{taskId:'task',values:{encounter:value}}]);
});
test('missing fields, unknown fields, wrong scalar values and unsafe integers are rejected',()=>{
 for(const value of [{},{patientId:'p',urgent:'',priority:1},{patientId:'p',urgent:true,priority:' '},{patientId:'p',urgent:true,priority:[]},{patientId:'p',urgent:true,priority:9007199254740992},{patientId:'p',urgent:true,priority:2,extra:1}])assert.throws(()=>encodeExecutionValue(p,value));
});
test('record collections are typed and inputs to independent tasks are kept separate',()=>{
 const collection=encodeExecutionValue({...p,type:'collection'},'[{"patientId":"p","urgent":true,"priority":1}]');
 assert.equal(collection.type,'collection');assert.equal((collection.collection as unknown[]).length,1);
 const other={...p,taskId:'other'};
 const inputs=executionInputs([p,other],{[inputKey(p)]:{patientId:'one',urgent:true,priority:1},[inputKey(other)]:{patientId:'two',urgent:false,priority:2}});
 assert.equal(inputs.length,2);assert.notDeepEqual(inputs[0]!.values,inputs[1]!.values);
});
