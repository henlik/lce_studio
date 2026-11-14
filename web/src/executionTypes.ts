import type {StateMachineLock} from './stateTypes';
import type {ComponentBundle} from './behaviorTypes';
import type {SharedScenario,ScenarioOutcome} from './systemScenario';
import type {SystemDesign,SystemResolution} from './systemTypes';
export interface ValueType {type:string;recordRef?:{contractId:string;version:string;digest:string;record:string}}
export interface ExecutionInput extends ValueType {taskId:string;name:string;fields?:{name:string;type:string}[]}
export interface ExecutionPlan {engineVersion:string;tasks:{id:string;componentId:string;slot:string;name:string;bundle:ComponentBundle;state?:{entryCommand?:string;provider:{id:string;kind:string;version:string};machine:StateMachineLock}}[];links:{id:string;source:string;target:string;output:string;input:string;connectionId?:string}[];order:string[];inputs:ExecutionInput[]}
export interface Preparation {plan:ExecutionPlan;resolution:SystemResolution;designHash:string}
export interface StateEvidence {command?:string;provider:{id:string;kind:string;version:string};step:{replyLost?:boolean;status:string;beforeRevision:number;afterRevision:number;before:unknown;after:unknown;receipt?:unknown};instance:{state:unknown;revision:number;receipts:unknown[];effects:unknown[]}}
export interface ExecutionEvent {scenarioStep?:number;state?:StateEvidence;sequence:number;kind:string;taskId:string;target?:string;linkId?:string;connectionId?:string;values:Record<string,unknown>}
export interface ReplayVerification {id:string;executionId:string;createdAt:string;verifierVersion:string;snapshotHash:string;status:'matched'|'mismatch'|'unverified';message:string;checks:{path:string;status:string;message:string}[];recordedReportHash:string;replayedReportHash?:string;report?:ExecutionEvidence['report'];differences:{path:string;recorded:string;replayed:string}[];differencesTruncated:boolean}
export type ReplaySummary=Pick<ReplayVerification,'id'|'createdAt'|'status'|'message'>;
export interface ExecutionEvidence extends Preparation {scenario?:SharedScenario;id:string;createdAt:string;design:SystemDesign;inputs:unknown[];inputHash:string;report:{scenario?:ScenarioOutcome;engineVersion:string;mode:string;claimsEvaluated:boolean;events:ExecutionEvent[];results:{scenarioStep?:number;status?:string;state?:StateEvidence;taskId:string;outputs:Record<string,unknown>;operations:number;invocations:number;bundleHash:string}[];limitations:string[]}}
export function inputKey(p:ExecutionInput){return JSON.stringify([p.taskId,p.name]);}
function scalar(type:string,value:unknown):Record<string,unknown>{
 if(type==='string'){if(typeof value!=='string')throw new Error('A string is required');return {type,string:value};}
 if(type==='boolean'){if(value!=='true'&&value!=='false'&&typeof value!=='boolean')throw new Error('Choose true or false');return {type,boolean:value===true||value==='true'};}
 if(type==='integer'){if(value===''||(typeof value==='string'&&!value.trim())||value===null||value===undefined)throw new Error('Enter an integer');if(typeof value!=='number'&&typeof value!=='string')throw new Error('Enter an integer');const n=Number(value);if(!Number.isSafeInteger(n)||typeof value==='boolean')throw new Error('Enter a safe integer');return {type,integer:n};}
 throw new Error('Unsupported input type '+type);
}
export function encodeExecutionValue(p:ExecutionInput,value:unknown):Record<string,unknown>{
 if(p.type!=='record'&&p.type!=='collection')return scalar(p.type,value);
 if(!p.recordRef)throw new Error('Missing record identity');
 const record=(data:unknown)=>{
  if(!data||typeof data!=='object'||Array.isArray(data))throw new Error('A record object is required');
  const raw=data as Record<string,unknown>,fields=p.fields||[];
  if(Object.keys(raw).some(k=>!fields.some(f=>f.name===k)))throw new Error('Unknown record field');
  const typed=Object.fromEntries(fields.map(f=>[f.name,scalar(f.type,raw[f.name])]));
  return {type:'record',recordRef:p.recordRef,record:typed};
 };
 if(p.type==='record')return record(value);
 const data=typeof value==='string'?JSON.parse(value):value;
 if(!Array.isArray(data))throw new Error('Enter a JSON array of records');
 return {type:'collection',recordRef:p.recordRef,collection:data.map(record)};
}
export function executionInputs(ports:ExecutionInput[],values:Record<string,unknown>){
 const tasks=new Map<string,Record<string,unknown>>();
 for(const p of ports){const row=tasks.get(p.taskId)||{};try{row[p.name]=encodeExecutionValue(p,values[inputKey(p)]);}catch(e){throw new Error(p.name+': '+(e as Error).message);}tasks.set(p.taskId,row);}
 return [...tasks].map(([taskId,values])=>({taskId,values}));
}
