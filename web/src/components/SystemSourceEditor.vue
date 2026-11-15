<script setup lang="ts">
import {computed,ref,watch} from 'vue';
import {api,APIError} from '../api';
import {normalizeSystemResponse,type SystemDesign} from '../systemTypes';
import type {ComponentDefinition} from '../definitionTypes';
import {locateSource,sourceOffset,sourceCanApply,type SourceCompilation,type SourceDiagnostic,type SourceTestReport} from '../sourceAuthoring';
import {useWorkspaceDraft} from '../composables/useWorkspaceDraft';
import {useActionFeedback} from '../composables/useNotifications';
import DraftStatus from './DraftStatus.vue';
const props=withDefaults(defineProps<{design?:SystemDesign;mode?:'system'|'component'}>(),{mode:'system'});
const emit=defineEmits<{apply:[SystemDesign];catalog:[];installed:[ComponentDefinition]}>();
const source=ref(''),busy=ref(''),notice=ref(''),error=ref(''),report=ref<SourceTestReport|null>(null);
const draft=useWorkspaceDraft(props.mode==='component'?'component-source-v1':'system-assembly-source-v1',{source,report});
useActionFeedback('Source authoring',busy,notice,error);
const compilation=ref<SourceCompilation|null>(null),diagnostics=ref<SourceDiagnostic[]>([]),prepared=ref(false),checkedSource=ref(''),checkedDesign=ref('');
const editor=ref<HTMLTextAreaElement|null>(null),fileInput=ref<HTMLInputElement|null>(null);
const stamp=()=>JSON.stringify(props.design||{});
const checked=computed(()=>!!compilation.value?.valid&&source.value===checkedSource.value);
const ready=computed(()=>props.mode==='system'&&!!compilation.value?.design&&sourceCanApply(source.value,checkedSource.value,stamp(),checkedDesign.value,prepared.value));
const reportCurrent=computed(()=>!!report.value&&report.value.source===source.value);
const editorId='lce-'+props.mode+'-source';
watch(source,()=>{compilation.value=null;prepared.value=false;diagnostics.value=[];notice.value='';});
async function action(label:string,fn:()=>Promise<void>){if(busy.value)return;busy.value=label;error.value='';notice.value='';try{await draft.ready;await fn();}catch(e){error.value=(e as Error).message;}finally{busy.value='';}}
async function load(component:boolean){await action('Loading source example',async()=>{const r=await api<{source:string}>('/systems/source/'+(component?'component-example':'example'));source.value=r.source;});}
async function installM1(){await action('Installing M1 library',async()=>{await api('/systems/m1-example','POST');emit('catalog');notice.value='Immutable M1 dependencies installed.';});}
async function prepare(result:SourceCompilation,text:string,designStamp:string){
 if(!result.design||props.mode!=='system')return;
 try{
  await api('/systems/execution/prepare','POST',{design:result.design});
  if(text!==source.value||designStamp!==stamp()){notice.value='Source or canvas changed during checking. Check again.';return;}
  prepared.value=true;notice.value='Catalog pins, connections and providers checked. Review the model, then apply it to the canvas.';
 }catch(e){
  if(text!==source.value||designStamp!==stamp())return;
  if(e instanceof APIError&&e.issues.length)diagnostics.value=e.issues.map(issue=>({...locateSource(result.locations,issue.path),path:issue.path,message:issue.message}));
  else throw e;
 }
}
async function compile(){await action('Compiling and checking source',async()=>{
 const text=source.value,designStamp=stamp();prepared.value=false;compilation.value=null;diagnostics.value=[];
 const result=await api<SourceCompilation>('/systems/source/compile','POST',{source:text});
 if(text!==source.value||designStamp!==stamp()){notice.value='Source or canvas changed during compilation. Check again.';return;}
 compilation.value=result;diagnostics.value=result.diagnostics;checkedSource.value=text;checkedDesign.value=designStamp;
 if(!result.valid)return;
 if(result.component){notice.value='Component compiled. Review its artifacts, run the testbench, then install the version.';return;}
 if(props.mode==='component'){notice.value='Open system assembly files in Design → System architecture → Source.';return;}
 await prepare(result,text,designStamp);
});}
async function run(){await action('Running component testbench',async()=>{
 const text=source.value;const result=await api<SourceTestReport>('/systems/source/testbench','POST',{source:text});
 if(text!==source.value){notice.value='Source changed during execution. Run again.';return;}
 report.value=result;diagnostics.value=result.cases.flatMap(c=>c.diagnostics);
 notice.value=result.passed?`${result.cases.length} cases passed. Source and evidence are saved in this browser; download them for sharing.`:'Testbench failed. Select a diagnostic to inspect its expectation.';
});}
async function installComponent(){await action('Installing source component',async()=>{
 const text=source.value,designStamp=stamp();const result=await api<SourceCompilation>('/systems/source/install','POST',{source:text});
 emit('catalog');if(result.component)emit('installed',result.component.definition);
 if(text!==source.value){notice.value='Snapshot installed; newer source edits were retained.';return;}
 compilation.value=result;checkedSource.value=text;checkedDesign.value=designStamp;diagnostics.value=[];
 notice.value='Immutable component installed. Refresh an already-open Design library to see it.';
 await prepare(result,text,designStamp);
});}
function apply(){if(!ready.value||!compilation.value?.design)return;emit('apply',normalizeSystemResponse(compilation.value.design));prepared.value=false;}
function focus(d:SourceDiagnostic){if(!editor.value)return;const offset=sourceOffset(source.value,d);editor.value.focus();editor.value.setSelectionRange(offset,Math.min(offset+1,source.value.length));}
function download(text:string,name:string,type='text/plain;charset=utf-8'){const url=URL.createObjectURL(new Blob([text],{type}));const a=document.createElement('a');a.href=url;a.download=name;a.click();URL.revokeObjectURL(url);}
async function importFile(e:Event){const input=e.target as HTMLInputElement,file=input.files?.[0];input.value='';if(!file)return;await action('Opening source file',async()=>{if(file.size>65536)throw new Error('Source exceeds 64 KiB.');source.value=await file.text();});}
</script>
<template><section class="source-editor" :aria-busy="!!busy">
 <h2>Source &amp; testbench · experimental LCE 0.2</h2>
 <p>Define typed commands, owned state, decision rules and event intents once. The compiler generates contracts and wiring for the existing engines. This syntax is inspired by VHDL.</p>
 <div class="actions">
  <button :disabled="!!busy" @click="load(true)">Load complete stock component</button>
  <template v-if="mode==='system'"><button :disabled="!!busy" @click="load(false)">Load M1 assembly</button><button :disabled="!!busy" @click="installM1">Install M1 library</button></template>
  <button :disabled="!!busy||!source.trim()" @click="compile">Compile and check</button>
  <button :disabled="!!busy||!checked||!compilation?.testbench" @click="run">Run testbench</button>
  <button :disabled="!!busy||!checked||!compilation?.component" @click="installComponent">Install component version</button>
  <button v-if="mode==='system'" :disabled="!!busy||!ready" @click="apply">Apply to canvas</button>
  <button :disabled="!!busy" @click="fileInput?.click()">Open .lce</button>
  <button :disabled="!source" @click="download(source,mode==='component'?'component.lce':'system.lce')">Download source</button>
  <button v-if="report" @click="download(JSON.stringify(report,null,2),'component-testbench-evidence.json','application/json')">Download testbench evidence</button>
  <input ref="fileInput" hidden type="file" accept=".lce,text/plain" @change="importFile"/>
 </div>
 <DraftStatus :draft="draft"/><p role="status">{{busy||notice}}</p><p v-if="error" role="alert" class="error">{{error}}</p>
 <p v-if="mode==='system'">Applying replaces the current system draft. Export it first to keep a separate copy. Later canvas edits do not rewrite this text.</p>
 <label :for="editorId">{{mode==='component'?'Component and testbench':'Component or system'}} source<textarea :id="editorId" ref="editor" v-model="source" spellcheck="false" autocapitalize="off" autocomplete="off" :readonly="!!busy" placeholder="Load an example or open a .lce file to begin."/></label>
 <ul v-if="diagnostics.length" class="error" aria-label="Source diagnostics"><li v-for="(d,i) in diagnostics" :key="i"><button @click="focus(d)">Line {{d.line}}, column {{d.column}} · {{d.path}} {{d.message}}</button></li></ul>
 <section v-if="report" class="test-results" aria-label="Testbench results">
  <h3>{{report.passed?'Passed':'Failed'}} · {{report.cases.length}} test cases <span v-if="!reportCurrent">· Previous source version</span></h3>
  <p v-if="!reportCurrent">The source changed. These saved results describe the earlier source included in the evidence download.</p>
  <details v-for="(c,i) in report.cases" :key="i"><summary>{{c.passed?'PASS':'FAIL'}} · {{c.name}} · {{c.assertions}} checks</summary>
   <div class="table-scroll"><table><thead><tr><th>Step</th><th>Command</th><th>Outcome</th><th>Revision</th><th>Outputs</th></tr></thead><tbody><tr v-for="s in c.result.steps" :key="s.index"><td>{{s.index+1}}</td><td>{{s.kind==='restart'?'Restart':s.commandId}}</td><td>{{s.status}}{{s.replyLost?' · reply lost':''}}</td><td>{{s.beforeRevision}} → {{s.afterRevision}}</td><td><code>{{JSON.stringify(s.receipt?.outputs||{})}}</code></td></tr></tbody></table></div>
   <p v-for="(d,j) in c.diagnostics" :key="j" class="error"><button :disabled="!reportCurrent" @click="focus(d)">Line {{d.line}} · {{d.message}}</button></p>
   <details><summary>Final owned state and event intents</summary><pre>{{JSON.stringify(c.result.instances,null,2)}}</pre></details>
  </details>
  <p>Bounded offline model checks. A reply-lost step records the internal commit, not a received response. Event intents do not deliver broker messages.</p>
 </section>
 <section v-if="compilation?.component"><h3>Generated component</h3><p>{{compilation.component.definition.name}} · {{compilation.component.definition.version}} · {{compilation.component.machine.machine.commands.length}} commands · {{compilation.component.machine.machine.state.length}} state slots · {{compilation.component.machine.machine.effects.length}} events</p><p>Data schemas, pure decision packages, state bindings and exact artifact pins are generated together.</p><details><summary>Inspect generated artifacts and testbench</summary><pre>{{JSON.stringify({component:compilation.component,testbench:compilation.testbench},null,2)}}</pre></details></section>
 <section v-if="compilation?.design&&mode==='system'"><h3>Compiled system preview</h3><p>{{compilation.design.name}} · {{compilation.design.components.length}} components · {{compilation.design.connections.length}} data connections</p><p v-if="prepared&&!ready">The canvas changed. Compile and check again before applying.</p><details><summary>Inspect generated system JSON</summary><pre>{{JSON.stringify(compilation.design,null,2)}}</pre></details></section>
 <details><summary>Supported experiment scope</summary><p>Component bodies support scalar inputs, durable/volatile state, pure expressions, atomic commands and committed event intents. Testbenches support calls, retries, revision conflicts, fault injection, restarts and independent assertions. Each case begins with a fresh owner. Source and testbench evidence autosave in this browser.</p><p>System assembly still supports installed definitions, provider maps, owner mappings, correlated handoffs and connections. Generics, nested bodies, record expressions, text broker/workflow definitions, deployment syntax and bidirectional canvas editing remain future work.</p></details>
</section></template>
<style scoped>
.source-editor{min-width:0;padding:16px}.actions{display:flex;flex-wrap:wrap;gap:8px}label{display:grid;gap:8px;margin-top:14px}textarea{box-sizing:border-box;width:100%;min-width:0;min-height:460px;resize:vertical;font:13px/1.6 monospace;tab-size:2;white-space:pre;overflow:auto;background:#142333;color:#e4f1f5;border:1px solid #416077;border-radius:8px;padding:16px}button{padding:8px;border:1px solid #bfd4cd;border-radius:5px;cursor:pointer}button:disabled{opacity:.5;cursor:default}.error{color:#a03333;background:#fff1f0;padding:12px}.error button{text-align:left;overflow-wrap:anywhere}p,li{line-height:1.6;overflow-wrap:anywhere}pre{white-space:pre-wrap;overflow-wrap:anywhere;max-height:400px;overflow:auto;font-size:12px}.test-results{padding:16px;background:#f2f7f5;border-radius:8px;margin:16px 0}.test-results>details{padding:12px 0;border-bottom:1px solid #cfddd6}summary{cursor:pointer;overflow-wrap:anywhere}.table-scroll{overflow:auto;max-width:100%}table{border-collapse:collapse;width:100%;font-size:12px}td,th{padding:8px;text-align:left;border-bottom:1px solid #ddd}td code{overflow-wrap:anywhere;white-space:normal}@media(max-width:700px){.source-editor{padding:8px}textarea{min-height:360px}}
</style>
