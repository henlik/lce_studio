<script setup lang="ts">
import {computed,ref,watch} from 'vue';
import {api,APIError} from '../api';
import type {SystemDesign} from '../systemTypes';
import {inputKey,executionInputs,type Preparation,type ExecutionEvidence,type ExecutionInput} from '../executionTypes';
import {useWorkspaceDraft} from '../composables/useWorkspaceDraft';
import {useActionFeedback} from '../composables/useNotifications';
import DraftStatus from './DraftStatus.vue';
const props=defineProps<{design:SystemDesign}>();
const formKey=(p:ExecutionInput)=>JSON.stringify([props.design.id,inputKey(p)]);
const emit=defineEmits<{locate:[path:string]}>();
const preparation=ref<Preparation|null>(null),preparedDesign=ref(''),evidence=ref<ExecutionEvidence|null>(null);
const values=ref<Record<string,any>>({});
const recovery=useWorkspaceDraft('system-execution-inputs',{values});
const busy=ref(''),error=ref(''),notice=ref(''),step=ref(0);
const issues=ref<{path:string;message:string}[]>([]);
const orderedTasks=computed(()=>{const plan=evidence.value?.plan;return plan?plan.order.map(id=>plan.tasks.find(t=>t.id===id)!).filter(Boolean):[];});
const history=ref<{id:string;createdAt:string;name:string;taskCount:number}[]>([]);
useActionFeedback('System execution',busy,notice,error);
const stamp=computed(()=>JSON.stringify(props.design));
const ready=computed(()=>preparation.value&&preparedDesign.value===stamp.value);
const current=computed(()=>evidence.value?.report.events[step.value]);
const stale=computed(()=>evidence.value&&JSON.stringify(evidence.value.design)!==stamp.value);
const tasks=computed(()=>evidence.value?.plan.tasks||preparation.value?.plan.tasks||[]);
const activeTask=computed(()=>current.value?.kind==='transfer'?current.value.target:current.value?.taskId);
const name=(id:string)=>tasks.value.find(t=>t.id===id)?.name||id;
watch(()=>props.design.id,()=>{history.value=[];});
async function action(label:string,fn:()=>Promise<void>){if(busy.value)return;busy.value=label;error.value='';notice.value='';issues.value=[];try{await recovery.ready;await fn();}catch(e){error.value=(e as Error).message;if(e instanceof APIError)issues.value=e.issues;}finally{busy.value='';}}
async function prepare(){await action('Preparing pure execution',async()=>{
 const snapshot=stamp.value;const p=await api<Preparation>('/systems/execution/prepare','POST',{design:JSON.parse(snapshot)});
 if(stamp.value!==snapshot){notice.value='Design changed; prepare again';return;}
 preparation.value=p;preparedDesign.value=snapshot;evidence.value=null;
 for(const port of p.plan.inputs){const key=formKey(port);if(port.type==='record'){const previous=values.value[key];values.value[key]=Object.fromEntries((port.fields||[]).map(f=>[f.name,previous&&typeof previous==='object'&&!Array.isArray(previous)?previous[f.name]??'':'']));}else if(values.value[key]===undefined||typeof values.value[key]==='object')values.value[key]=port.type==='collection'?'[]':'';}
 notice.value='Plan prepared. Supply every external input; linked inputs are routed automatically.';
 });}
async function run(){if(!ready.value||!preparation.value)return;await action('Executing pure component graph',async()=>{
 const inputs=executionInputs(preparation.value!.plan.inputs,Object.fromEntries(preparation.value!.plan.inputs.map(p=>[inputKey(p),values.value[formKey(p)]])));
 const result=await api<ExecutionEvidence>('/systems/execution/run','POST',{design:JSON.parse(preparedDesign.value),inputs});
 evidence.value=result;step.value=0;notice.value='Pure model execution saved. Step through the token trace.';
 });}
function sample(){if(!preparation.value)return;for(const p of preparation.value.plan.inputs){if(p.type==='record'&&p.fields?.length===2&&p.fields.every(f=>f.type==='string'&&['encounterId','patientId'].includes(f.name)))values.value[formKey(p)]={encounterId:'synthetic-encounter-001',patientId:'synthetic-patient-001'};}notice.value='Synthetic encounter values filled for matching example inputs';}
async function loadHistory(){await action('Loading execution history',async()=>{const id=props.design.id;const rows=await api<typeof history.value>('/systems/executions?systemId='+encodeURIComponent(id));if(props.design.id===id)history.value=rows;});}
async function load(id:string){await action('Opening saved execution',async()=>{evidence.value=await api<ExecutionEvidence>('/systems/executions/'+encodeURIComponent(id));step.value=0;});}
function download(){if(!evidence.value)return;const url=URL.createObjectURL(new Blob([JSON.stringify(evidence.value,null,2)],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download='system-execution.json';a.click();URL.revokeObjectURL(url);}
</script>
<template>
<section class="execution-panel" :inert="recovery.status.value==='loading'" :aria-busy="!!busy">
 <h2>Execute pure components</h2><p class="scope">One deterministic run through an acyclic dataflow graph. No external effects, resource timing or claim evaluation. State, messages, workflows and provider bindings require future execution adapters.</p>
 <div class="actions"><button :disabled="!!busy" @click="prepare">Prepare execution</button><button :disabled="!!busy||!ready" @click="run">Run pure model</button><button :disabled="!!busy" @click="loadHistory">Saved executions</button><DraftStatus :draft="recovery"/><span role="status">{{busy}}</span></div>
 <div v-if="error" role="alert" class="error"><p>{{error}}</p><p v-for="issue in issues" :key="issue.path+issue.message">{{issue.path}}: {{issue.message}} <button v-if="issue.path.startsWith('components')||issue.path.startsWith('connections')||issue.path.startsWith('deploymentPlans')" @click="emit('locate',issue.path)">Locate in design</button></p></div>
 <p v-if="preparation&&!ready" class="scope">The design changed. Prepare again before running.</p>
 <div v-if="history.length" class="history"><button v-for="item in history" :key="item.id" :disabled="!!busy" @click="load(item.id)">{{new Date(item.createdAt).toLocaleString()}} · {{item.name}} · {{item.taskCount}} tasks</button></div>
 <section v-if="ready&&preparation" class="input-form"><h3>External inputs</h3><p>Only inputs without a producer are shown. Fields are required.</p><button @click="sample">Use synthetic encounter</button>
  <p v-if="!preparation.plan.inputs.length">This plan has no external inputs.</p>
  <fieldset v-for="port in preparation.plan.inputs" :key="inputKey(port)" :class="{invalid:issues.some(i=>i.path.includes(port.taskId)&&i.path.includes(port.name))}"><legend>{{preparation.plan.tasks.find(t=>t.id===port.taskId)?.name}} / {{port.name}} · {{port.type}}</legend>
   <small v-if="port.recordRef">{{port.recordRef.contractId}}@{{port.recordRef.version}} · {{port.recordRef.record}}</small>
   <template v-if="port.type==='record'"><label v-for="field in port.fields" :key="field.name">{{field.name}} *<select v-if="field.type==='boolean'" v-model="values[formKey(port)][field.name]"><option value="">Choose…</option><option value="true">true</option><option value="false">false</option></select><input v-else v-model="values[formKey(port)][field.name]" :type="field.type==='integer'?'number':'text'"/></label></template>
   <label v-else-if="port.type==='collection'">JSON array of records *<textarea v-model="values[formKey(port)]" rows="4"/></label>
   <label v-else>{{port.name}} *<select v-if="port.type==='boolean'" v-model="values[formKey(port)]"><option value="">Choose…</option><option value="true">true</option><option value="false">false</option></select><input v-else v-model="values[formKey(port)]" :type="port.type==='integer'?'number':'text'"/></label>
  </fieldset>
 </section>
 <section v-if="evidence" class="trace-view"><h3>Token trace · pure model</h3><p>The trace uses saved inputs. Editing the form affects only the next run.</p><p v-if="stale" class="scope">Saved evidence belongs to a different design snapshot. The trace below uses that snapshot.</p>
  <div class="actions"><button :disabled="step===0" @click="step--">Previous</button><input aria-label="Trace step" type="range" min="0" :max="evidence.report.events.length-1" v-model.number="step"/><button :disabled="step>=evidence.report.events.length-1" @click="step++">Next</button><span aria-live="polite">{{step+1}} / {{evidence.report.events.length}} · {{current?.kind}}</span><button @click="download">Export execution</button></div>
  <div class="task-strip"><button v-for="task in orderedTasks" :key="task.id" :class="{active:activeTask===task.id}" @click="step=Math.max(0,evidence!.report.events.findIndex(e=>e.taskId===task.id&&e.kind==='started'))"><strong>{{task.name}}</strong><small>{{task.slot}} · {{evidence.report.results.find(r=>r.taskId===task.id)?.operations}} operations</small></button></div>
  <div class="link-strip"><p v-for="link in evidence.plan.links" :key="link.id" :class="{active:current?.linkId===link.id}">{{name(link.source)}} / {{link.output}} → {{name(link.target)}} / {{link.input}}</p></div>
  <p v-if="current"><strong>{{name(current.taskId)}}</strong> · {{current.kind}}<template v-if="current.target"> → {{name(current.target)}}</template></p><pre>{{JSON.stringify(current?.values,null,2)}}</pre>
  <details><summary>Outputs and snapshot identity</summary><p>Design: {{evidence.designHash}}<br/>Inputs: {{evidence.inputHash}}<br/>Engine: {{evidence.report.engineVersion}} · Claims evaluated: {{evidence.report.claimsEvaluated}}</p><section v-for="result in evidence.report.results" :key="result.taskId"><h4>{{name(result.taskId)}}</h4><pre>{{JSON.stringify(result.outputs,null,2)}}</pre><small>Bundle {{result.bundleHash}}</small></section></details>
  <p v-for="limit in evidence.report.limitations" :key="limit" class="scope">{{limit}}</p>
 </section>
</section>
</template>
<style scoped>
.execution-panel{min-width:0}.execution-panel button{border:1px solid #c8dbe3;border-radius:6px;background:#fff;padding:8px 12px;color:#285c52;cursor:pointer;font:inherit;font-size:12px}.execution-panel button:hover{background:#eaf6f1}.execution-panel button:disabled{opacity:.5;cursor:default}.execution-panel button:focus-visible{outline:2px solid #247e65;outline-offset:2px}.execution-panel p{font-size:13px;line-height:1.6}.scope{padding:12px;background:#fff8e5;border-radius:6px;color:#77622f}.actions,.history{display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin:12px 0}.actions input[type=range]{width:min(230px,100%)}.input-form{max-width:800px}.input-form fieldset.invalid{border-color:#c64242;box-shadow:0 0 0 1px #c64242}.input-form fieldset{border:1px solid #cbdce3;border-radius:8px;margin:14px 0;min-width:0;padding:15px}.input-form label{display:grid;gap:7px;margin:10px 0;font-size:12px}.input-form input,.input-form select,.input-form textarea{width:100%;min-width:0;box-sizing:border-box;border:1px solid #cbdce3;border-radius:5px;padding:9px}.task-strip{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(220px,100%),1fr));gap:12px}.task-strip button{display:grid;gap:8px;text-align:left;padding:15px!important;overflow-wrap:anywhere}.active{outline:2px solid #2e8a6b;background:#e6f8ed!important}.link-strip p{padding:10px;border:1px solid #d6e5dd;border-radius:6px;overflow-wrap:anywhere}pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px;background:#f1f6f8;padding:14px;border-radius:6px;max-height:360px;overflow:auto}.trace-view details{overflow-wrap:anywhere}.error{color:#a13732;background:#fff0ef;padding:12px;border-radius:6px}.execution-panel small{overflow-wrap:anywhere}
</style>
