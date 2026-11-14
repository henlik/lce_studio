import type {ExecutionPlan} from './executionTypes';
import type {TypedValue} from './behaviorTypes';
export interface StateCall {taskId:string;commandId:string;expectedRevision:number|null;fault?:string}
export interface SharedStep {kind:'invoke'|'restart';taskId?:string;inputs?:{taskId:string;values:Record<string,TypedValue>}[];calls?:StateCall[];expectedStatuses:Record<string,string>}
export interface SharedScenario {name:string;steps:SharedStep[];expected:{taskId:string;state:Record<string,TypedValue>;revision:number|null;receipts:number|null;effects:number|null}[]}
export interface ScenarioOutcome {verified:boolean;passed:boolean;assertions:{path:string;passed:boolean;message:string}[];final:unknown[]}
export const scenarioStatuses=['completed','committed','denied','replayed','id_conflict','revision_conflict','rolled_back','evaluation_error','reply_lost','skipped','restarted'];
export function stockScenario(plan:ExecutionPlan):SharedScenario {
 const owner=plan.tasks.find(t=>t.state?.machine.machine.id==='studio.example-stock-owner');
 const port=plan.inputs.find(p=>p.name==='request'&&p.type==='record');
 const summary=plan.tasks.find(t=>t.componentId==='summary');
 if(!owner?.state||!port?.recordRef||!summary||plan.tasks.length!==3)throw new Error('Install the stock execution example before using this scenario.');
 const input=(quantity:number)=>[{taskId:port.taskId,values:{request:{type:'record' as const,recordRef:port.recordRef,record:{tenantId:{type:'string' as const,string:'tenantDefault'},resourceId:{type:'string' as const,string:'resourceDefault'},quantity:{type:'integer' as const,integer:quantity}}}}}];
 const invoke=(commandId:string,revision:number,quantity:number,status:string,summaryStatus='completed',fault=''):SharedStep=>({kind:'invoke',inputs:input(quantity),calls:[{taskId:owner.id,commandId,expectedRevision:revision,fault}],expectedStatuses:{[owner.id]:status,[summary.id]:summaryStatus}});
 // These outcomes are handwritten, never copied from a run's observations.
 return {name:'Stock retries, conflict and restart',steps:[invoke('reserveA',0,2,'committed'),invoke('reserveA',0,2,'replayed'),invoke('stale',0,1,'revision_conflict','skipped'),invoke('reserveB',1,1,'reply_lost','skipped','after_commit_before_reply'),{kind:'restart',taskId:owner.id,expectedStatuses:{[owner.id]:'restarted'}},invoke('reserveB',1,1,'replayed')],expected:[{taskId:owner.id,revision:2,receipts:2,effects:2,state:{tenantId:{type:'string',string:'tenantDefault'},resourceId:{type:'string',string:'resourceDefault'},available:{type:'integer',integer:2},acceptedSinceRestart:{type:'integer',integer:0}}}]};
}
