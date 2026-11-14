import type {DeliveryMessageExpectation,DeliveryMessage} from './deliveryTypes';
import type {ExecutionPlan} from './executionTypes';
import type {TypedValue} from './behaviorTypes';
export interface StateCall {command?:string;taskId:string;commandId:string;expectedRevision:number|null;fault?:string}
export interface MessageCall {channelId:string;routeId?:string;sourceCommandId?:string;sourceMessage?:string;alias?:string;expectedRevision?:number;fault?:string}
export interface SharedStep {kind:'invoke'|'restart'|'command'|'dispatch'|'deliver'|'retry'|'broker_restart';message?:MessageCall;expectedMessageStatus?:string;taskId?:string;inputs?:{taskId:string;values:Record<string,TypedValue>}[];calls?:StateCall[];expectedStatuses:Record<string,string>}
export interface SharedScenario {expectedMessages?:(DeliveryMessageExpectation & {channelId:string})[];name:string;steps:SharedStep[];expected:{taskId:string;state:Record<string,TypedValue>;revision:number|null;receipts:number|null;effects:number|null}[]}
export interface ScenarioOutcome {messages?:{channelId:string;messages:DeliveryMessage[]}[];verified:boolean;passed:boolean;assertions:{path:string;passed:boolean;message:string}[];final:unknown[]}
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

export function ownerCommandStep(plan:ExecutionPlan,taskId:string,command:string,commandId='command1'):SharedStep {
 const task=plan.tasks.find(t=>t.id===taskId);
 if(!task?.state?.machine.machine.commands.some(c=>c.name===command))throw new Error('Choose a command on an existing state owner.');
 return {kind:'command',taskId,inputs:[{taskId,values:{}}],calls:[{taskId,command,commandId,expectedRevision:0,fault:''}],expectedStatuses:{}};
}
export function multiCommandScenario(plan:ExecutionPlan):SharedScenario {
 const base=stockScenario(plan),owner=plan.tasks.find(t=>t.state?.machine.machine.id==='studio.example-stock-owner');
 if(!owner?.state||owner.state.entryCommand!=='reserve'||!owner.state.machine.machine.commands.some(c=>c.name==='adjust'))throw new Error('Install and prepare the multi-command stock example first.');
 const command=(name:string,id:string,revision:number,values:Record<string,TypedValue>,status:string,fault=''):SharedStep=>({kind:'command',taskId:owner.id,inputs:[{taskId:owner.id,values}],calls:[{taskId:owner.id,command:name,commandId:id,expectedRevision:revision,fault}],expectedStatuses:{[owner.id]:status}});
 const lost=JSON.parse(JSON.stringify(base.steps[3])) as SharedStep;lost.calls![0]!.expectedRevision=2;
 const retryValues=JSON.parse(JSON.stringify(lost.inputs![0]!.values));
 return {name:'Reserve and adjust one stock owner',steps:[base.steps[0]!,command('adjust','adjustA',1,{count:{type:'integer',integer:8}},'committed'),command('adjust','adjustA',1,{count:{type:'integer',integer:8}},'replayed'),lost,base.steps[4]!,command('reserve','reserveB',2,retryValues,'replayed'),command('adjust','reserveA',0,{count:{type:'integer',integer:4}},'id_conflict'),command('adjust','staleAdjust',2,{count:{type:'integer',integer:6}},'revision_conflict'),command('adjust','rollbackAdjust',3,{count:{type:'integer',integer:9}},'rolled_back','before_commit'),command('adjust','invalidAdjust',3,{count:{type:'integer',integer:-1}},'denied')],expected:[{...base.expected[0]!,revision:3,receipts:4,effects:2,state:{...base.expected[0]!.state,available:{type:'integer',integer:7}}}]};
}

export function messageScenario(plan:ExecutionPlan):SharedScenario {
 const channel=plan.channels?.find(c=>c.model.model.id==='studio.stock-ledger-delivery');
 const stock=plan.tasks.find(t=>t.state?.machine.machine.id==='studio.message-stock-owner');
 const ledger=plan.tasks.find(t=>t.state?.machine.machine.id==='lce.reservation-ledger');
 if(!channel||!stock?.state||!ledger?.state)throw new Error('Install and prepare the stock-to-ledger messages example first.');
 const request:TypedValue={type:'record',recordRef:stock.state.machine.machine.commands[0]!.inputs[0]!.recordRef,record:{tenantId:{type:'string',string:'tenantDefault'},resourceId:{type:'string',string:'resourceDefault'},quantity:{type:'integer',integer:2}}};
 const command=(id:string,fault:string,status:string):SharedStep=>({kind:'command',taskId:stock.id,inputs:[{taskId:stock.id,values:{request:JSON.parse(JSON.stringify(request))}}],calls:[{taskId:stock.id,command:'reserve',commandId:id,expectedRevision:0,fault}],expectedStatuses:{[stock.id]:status}});
 const dispatch=(id:string,alias:string,status:string):SharedStep=>({kind:'dispatch',message:{channelId:channel.id,routeId:'reservationToLedger',sourceCommandId:id,alias,expectedRevision:0},expectedStatuses:{},expectedMessageStatus:status});
 const deliver=(kind:'deliver'|'retry',fault:string,status:string):SharedStep=>({kind,message:{channelId:channel.id,alias:'reservationA',fault},expectedStatuses:{},expectedMessageStatus:status});
 const initial=(task:typeof stock)=>JSON.parse(JSON.stringify(Object.fromEntries(task.state!.machine.machine.state.map(s=>[s.name,s.initial])))) as Record<string,TypedValue>;
 return {name:'Stock to ledger: rollback, retry and lost acknowledgement',steps:[command('rollback','before_commit','rolled_back'),dispatch('rollback','noMessage','missing_intent'),command('reserveA','','committed'),dispatch('reserveA','reservationA','dispatched'),deliver('deliver','drop_before_receive','dropped'),deliver('retry','before_recipient_commit','recipient_unacknowledged'),deliver('retry','after_recipient_commit_before_ack','acknowledgement_lost'),{kind:'broker_restart',message:{channelId:channel.id},expectedMessageStatus:'broker_restarted',expectedStatuses:{}},{kind:'restart',taskId:ledger.id,expectedStatuses:{[ledger.id]:'restarted'}},{kind:'restart',taskId:stock.id,expectedStatuses:{[stock.id]:'restarted'}},deliver('retry','','acknowledged'),deliver('retry','','already_acknowledged'),command('reserveA','','replayed'),dispatch('reserveA','reservationA','dispatch_replayed')],expected:[{taskId:stock.id,state:{...initial(stock),available:{type:'integer',integer:3}},revision:1,receipts:1,effects:1},{taskId:ledger.id,state:{...initial(ledger),totalReserved:{type:'integer',integer:2}},revision:1,receipts:1,effects:0}],expectedMessages:[{channelId:channel.id,name:'noMessage',status:'absent',attempts:0},{channelId:channel.id,name:'reservationA',status:'acknowledged',attempts:4,accepted:true}]};
}
