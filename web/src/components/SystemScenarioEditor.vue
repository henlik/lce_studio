<script setup lang="ts">
import {computed,ref} from 'vue';
import type {ExecutionPlan,ExecutionEvidence} from '../executionTypes';
import type {SystemDesign} from '../systemTypes';
import type {TypedValue} from '../behaviorTypes';
import {stockScenario,scenarioStatuses,type SharedScenario,type SharedStep} from '../systemScenario';
import {api,APIError} from '../api';
import {useWorkspaceDraft} from '../composables/useWorkspaceDraft';
import {useActionFeedback} from '../composables/useNotifications';
import TypedValueEditor from './TypedValueEditor.vue';
import DraftStatus from './DraftStatus.vue';
const props=defineProps<{plan:ExecutionPlan;design:SystemDesign}>();
const emit=defineEmits<{result:[ExecutionEvidence]}>();
const scenario=ref<SharedScenario>({name:'New shared scenario',steps:[],expected:[]});
const signature=ref(JSON.stringify(props.plan));
const recovery=useWorkspaceDraft('system-scenario-'+props.design.id,{scenario,signature});
const stale=computed(()=>signature.value!==JSON.stringify(props.plan));
const owners=computed(()=>props.plan.tasks.filter(t=>t.state));
const contracts=computed(()=>props.plan.tasks.flatMap(t=>t.state?[...t.state.machine.machine.dataContracts,...t.state.machine.machine.commands.flatMap(c=>c.decision.packages.flatMap(p=>p.package.dataContracts||[]))]:t.bundle.packages.flatMap(p=>p.package.dataContracts||[])));
const busy=ref(''),error=ref(''),notice=ref('');useActionFeedback('Shared scenario',busy,notice,error);
const name=(id:string)=>props.plan.tasks.find(t=>t.id===id)?.name||id;
function reset(){scenario.value={name:'New shared scenario',steps:[],expected:[]};signature.value=JSON.stringify(props.plan);}
function example(){try{scenario.value=stockScenario(props.plan);signature.value=JSON.stringify(props.plan);error.value='';}catch(e){error.value=(e as Error).message;}}
function add(){const byTask=new Map<string,Record<string,TypedValue>>();for(const p of props.plan.inputs){if(!byTask.has(p.taskId))byTask.set(p.taskId,{});}
 scenario.value.steps.push({kind:'invoke',inputs:[...byTask].map(([taskId,values])=>({taskId,values})),calls:owners.value.map(t=>({taskId:t.id,commandId:'request'+(scenario.value.steps.length+1),expectedRevision:0,fault:''})),expectedStatuses:{}});}
function restart(){if(owners.value[0])scenario.value.steps.push({kind:'restart',taskId:owners.value[0].id,expectedStatuses:{}});}
function move(i:number,delta:number){const j=i+delta;if(j<0||j>=scenario.value.steps.length)return;[scenario.value.steps[i],scenario.value.steps[j]]=[scenario.value.steps[j]!,scenario.value.steps[i]!];}
function expectation(id:string){const t=owners.value.find(t=>t.id===id);if(!t?.state)return;scenario.value.expected.push({taskId:id,state:JSON.parse(JSON.stringify(Object.fromEntries(t.state.machine.machine.state.map(s=>[s.name,s.initial])))),revision:0,receipts:0,effects:0});}
function setStatus(s:SharedStep,id:string,event:Event){const value=(event.target as HTMLSelectElement).value;if(value)s.expectedStatuses[id]=value;else delete s.expectedStatuses[id];}
async function run(){if(busy.value||stale.value)return;busy.value='Running shared scenario';error.value='';notice.value='';const snapshot=JSON.stringify({design:props.design,scenario:scenario.value});try{await recovery.ready;const result=await api<ExecutionEvidence>('/systems/execution/run','POST',JSON.parse(snapshot));emit('result',result);notice.value=result.report.scenario?.passed?'Declared scenario expectations passed':'Review scenario verdicts';}catch(e){error.value=(e as Error).message;if(e instanceof APIError)error.value+=' '+e.issues.map(i=>i.path+': '+i.message).join('; ');}finally{busy.value='';}}
</script>
<template>
<section class="scenario-editor" :inert="recovery.status.value==='loading'" :aria-busy="!!busy">
 <h3>Shared scenario</h3><p>State persists across these ordered steps within this run. Set expected revisions explicitly. This explores one interleaving, not every possible race.</p>
 <DraftStatus :draft="recovery"/><p v-if="stale" role="alert">The prepared graph changed. Reset or reload the example before editing this scenario.</p>
 <div class="actions"><button @click="reset" :disabled="!!busy">New empty scenario</button><button @click="example" :disabled="!!busy">Load stock retry scenario</button></div>
 <fieldset :disabled="!!busy||stale"><label>Scenario name<input v-model="scenario.name"/></label>
 <div class="actions"><button @click="add" :disabled="scenario.steps.length>=64">Add graph invocation</button><button @click="restart" :disabled="scenario.steps.length>=64">Add owner restart</button></div>
 <article v-for="(s,i) in scenario.steps" :key="i"><h4>{{i+1}} · {{s.kind==='restart'?'Restart owner':'Invoke graph'}}</h4>
  <div class="actions"><button @click="move(i,-1)" :disabled="i===0">Move up</button><button @click="move(i,1)" :disabled="i===scenario.steps.length-1">Move down</button><button @click="scenario.steps.splice(i+1,0,JSON.parse(JSON.stringify(s)))" :disabled="scenario.steps.length>=64">Duplicate / retry</button><button @click="scenario.steps.splice(i,1)">Remove</button></div>
  <label v-if="s.kind==='restart'">Owner<select v-model="s.taskId" @change="s.expectedStatuses={}"><option v-for="t in owners" :key="t.id" :value="t.id">{{t.name}}</option></select></label>
  <template v-else><section v-for="call in s.calls" :key="call.taskId"><h5>{{name(call.taskId)}}</h5><label>Command ID<input v-model="call.commandId" maxlength="64"/></label><label>Expected revision<input type="number" min="0" step="1" v-model.number="call.expectedRevision"/></label><label>Fault<select v-model="call.fault"><option value="">None</option><option value="before_commit">Before commit</option><option value="after_commit_before_reply">After commit, before reply</option></select></label></section>
   <section v-for="input in s.inputs" :key="input.taskId"><TypedValueEditor v-for="port in plan.inputs.filter(p=>p.taskId===input.taskId)" :key="port.name" :model-value="input.values[port.name]" :type="port.type as any" :record-ref="port.recordRef" :contracts="contracts" :label="name(input.taskId)+' / '+port.name" @update:model-value="value=>{if(value)input.values[port.name]=value;else delete input.values[port.name]}"/></section>
  </template>
  <details><summary>Expected task outcomes (optional)</summary><label v-for="t in (s.kind==='restart'?owners.filter(t=>t.id===s.taskId):plan.tasks)" :key="t.id">{{t.name}}<select :value="s.expectedStatuses[t.id]||''" @change="setStatus(s,t.id,$event)"><option value="">No assertion</option><option v-for="status in scenarioStatuses" :key="status">{{status}}</option></select></label></details>
 </article>
 <h4>Expected final state</h4><p>Enter expected values before running. New expectations start from the definition's initial state, never from observed results.</p><button v-for="t in owners.filter(t=>!scenario.expected.some(e=>e.taskId===t.id))" :key="t.id" @click="expectation(t.id)">Add expectation: {{t.name}}</button>
 <article v-for="(e,i) in scenario.expected" :key="e.taskId"><h5>{{name(e.taskId)}}</h5><label>Revision<input type="number" min="0" v-model.number="e.revision"/></label><label>Receipt count<input type="number" min="0" v-model.number="e.receipts"/></label><label>Effect count<input type="number" min="0" v-model.number="e.effects"/></label><TypedValueEditor v-for="slot in owners.find(t=>t.id===e.taskId)?.state?.machine.machine.state||[]" :key="slot.name" :model-value="e.state[slot.name]" :type="slot.type" :record-ref="slot.recordRef" :contracts="contracts" :label="slot.name" @update:model-value="value=>{if(value)e.state[slot.name]=value;else delete e.state[slot.name]}"/><button @click="scenario.expected.splice(i,1)">Remove expectation</button></article>
 <button @click="run" :disabled="!scenario.steps.length">Run shared scenario</button>
 </fieldset><p role="status">{{busy}}</p><p v-if="error" role="alert">{{error}}</p>
</section>
</template>
<style scoped>
.scenario-editor{margin:20px 0;min-width:0}.scenario-editor fieldset{min-width:0;border:1px solid #cbdce3;padding:12px;border-radius:8px}.scenario-editor article{border:1px solid #cbdce3;border-radius:8px;padding:14px;margin:12px 0;min-width:0}.scenario-editor label{display:grid;gap:5px;margin:8px 0;font-size:12px}.scenario-editor input,.scenario-editor select{min-width:0;width:100%;box-sizing:border-box;padding:8px;border:1px solid #cbdce3;border-radius:5px}.actions{display:flex;gap:6px;flex-wrap:wrap}.scenario-editor p{overflow-wrap:anywhere}
</style>
