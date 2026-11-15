<script setup lang="ts">
import {computed,ref,watch} from 'vue';
import {api,APIError} from '../api';
import {normalizeSystemResponse,type SystemDesign} from '../systemTypes';
import {locateSource,sourceOffset,sourceCanApply,type SourceCompilation,type SourceDiagnostic} from '../sourceAuthoring';
import {useWorkspaceDraft} from '../composables/useWorkspaceDraft';
import {useActionFeedback} from '../composables/useNotifications';
import DraftStatus from './DraftStatus.vue';
const props=defineProps<{design:SystemDesign}>();
const emit=defineEmits<{apply:[SystemDesign];catalog:[]}>();
const source=ref(''),busy=ref(''),notice=ref(''),error=ref('');
const draft=useWorkspaceDraft('system-assembly-source-v1',{source});
useActionFeedback('Source authoring',busy,notice,error);
const compilation=ref<SourceCompilation|null>(null),diagnostics=ref<SourceDiagnostic[]>([]),prepared=ref(false),checkedSource=ref(''),checkedDesign=ref('');
const editor=ref<HTMLTextAreaElement|null>(null),fileInput=ref<HTMLInputElement|null>(null);
const ready=computed(()=>!!compilation.value?.design&&sourceCanApply(source.value,checkedSource.value,JSON.stringify(props.design),checkedDesign.value,prepared.value));
watch(source,()=>{compilation.value=null;prepared.value=false;diagnostics.value=[];notice.value='';});
async function action(label:string,fn:()=>Promise<void>){if(busy.value)return;busy.value=label;error.value='';notice.value='';try{await draft.ready;await fn();}catch(e){error.value=(e as Error).message;}finally{busy.value='';}}
async function load(){await action('Loading source example',async()=>{const r=await api<{source:string}>('/systems/source/example');source.value=r.source;});}
async function install(){await action('Installing M1 library',async()=>{await api('/systems/m1-example','POST');emit('catalog');notice.value='Immutable M1 dependencies installed. The canvas has not changed.';});}
async function compile(){await action('Compiling and checking source',async()=>{
 const text=source.value,stamp=JSON.stringify(props.design);prepared.value=false;compilation.value=null;diagnostics.value=[];
 const result=await api<SourceCompilation>('/systems/source/compile','POST',{source:text});
 if(text!==source.value||stamp!==JSON.stringify(props.design)){notice.value='Source or canvas changed during compilation. Check again.';return;}
 compilation.value=result;diagnostics.value=result.diagnostics;checkedSource.value=text;checkedDesign.value=stamp;
 if(!result.valid||!result.design)return;
 try{
  await api('/systems/execution/prepare','POST',{design:result.design});
  if(text!==source.value||stamp!==JSON.stringify(props.design)){notice.value='Source or canvas changed during checking. Check again.';return;}
  prepared.value=true;notice.value='Source, catalog pins, connections and providers checked. Review the model, then apply it to the canvas.';
 }catch(e){
  if(text!==source.value||stamp!==JSON.stringify(props.design))return;
  if(e instanceof APIError&&e.issues.length)diagnostics.value=e.issues.map(issue=>({...locateSource(result.locations,issue.path),path:issue.path,message:issue.message}));
  else throw e;
 }
 });}
function apply(){if(!ready.value||!compilation.value?.design)return;emit('apply',normalizeSystemResponse(compilation.value.design));prepared.value=false;}
function focus(d:SourceDiagnostic){if(!editor.value)return;const offset=sourceOffset(source.value,d);editor.value.focus();editor.value.setSelectionRange(offset,Math.min(offset+1,source.value.length));}
function download(){const url=URL.createObjectURL(new Blob([source.value],{type:'text/plain;charset=utf-8'}));const a=document.createElement('a');a.href=url;a.download='system.lce';a.click();URL.revokeObjectURL(url);}
async function importFile(e:Event){const input=e.target as HTMLInputElement,file=input.files?.[0];input.value='';if(!file)return;await action('Opening source file',async()=>{if(file.size>65536)throw new Error('Source exceeds 64 KiB.');source.value=await file.text();});}
</script>
<template><section class="source-editor" :aria-busy="!!busy"><h2>Source · experimental LCE Assembly 0.1</h2><p>VHDL-inspired component assembly compiled into the same system model. Component behavior remains defined in installed artifacts. This syntax is not IEEE VHDL.</p>
 <div class="actions"><button :disabled="!!busy" @click="load">Load M1 source</button><button :disabled="!!busy" @click="install">Install M1 library</button><button :disabled="!!busy||!source.trim()" @click="compile">Compile and check</button><button :disabled="!!busy||!ready" @click="apply">Apply to canvas</button><button :disabled="!!busy" @click="fileInput?.click()">Open .lce</button><button :disabled="!source" @click="download">Download source</button><input ref="fileInput" hidden type="file" accept=".lce,text/plain" @change="importFile"/></div>
 <DraftStatus :draft="draft"/><p role="status">{{busy||notice}}</p><p v-if="error" role="alert" class="error">{{error}}</p>
 <p>Applying replaces the current system draft; export that design first if you want a separate copy. Source is saved independently. Later canvas edits do not rewrite this text.</p>
 <label for="system-assembly-source">System source<textarea id="system-assembly-source" ref="editor" v-model="source" spellcheck="false" autocapitalize="off" autocomplete="off" :readonly="!!busy" placeholder="Load M1 source or open a .lce file to begin."/></label>
 <ul v-if="diagnostics.length" class="error" aria-label="Source diagnostics"><li v-for="(d,i) in diagnostics" :key="i"><button @click="focus(d)">Line {{d.line}}, column {{d.column}} · {{d.path}} {{d.message}}</button></li></ul>
 <section v-if="compilation?.design"><h3>Compiled model preview</h3><p>{{compilation.design.name}} · {{compilation.design.components.length}} components · {{compilation.design.connections.length}} data connections · {{compilation.design.executionProviders?.length||0}} providers</p><p v-if="prepared&&!ready">The canvas changed. Compile and check again before applying.</p><ul><li v-for="c in compilation.design.components" :key="c.id">{{c.name}} → {{c.definition.id}}@{{c.definition.version}}</li></ul><details><summary>Inspect generated system JSON</summary><pre>{{JSON.stringify(compilation.design,null,2)}}</pre></details></section>
 <details><summary>Supported experiment scope</summary><p>Declare installed component definitions, provider port maps, state entries, message/workflow owner mappings, correlated handoffs, data connections, claims and assumptions. Checks reuse the existing catalog and execution preparation. Generics, new behavior bodies, nested system definitions, deployment syntax and text testbenches are future work. Unsupported constructs are rejected.</p><p>After applying: open Execution, prepare, load the M1 acceptance suite and run its scenarios. Compilation and preparation do not execute the system.</p></details>
</section></template>
<style scoped>.source-editor{min-width:0;padding:16px}.actions{display:flex;flex-wrap:wrap;gap:8px}label{display:grid;gap:8px;margin-top:14px}textarea{box-sizing:border-box;width:100%;min-width:0;min-height:460px;resize:vertical;font:13px/1.6 monospace;tab-size:2;white-space:pre;overflow:auto;background:#142333;color:#e4f1f5;border:1px solid #416077;border-radius:8px;padding:16px}button{padding:8px;border:1px solid #bfd4cd;border-radius:5px;cursor:pointer}button:disabled{opacity:.5;cursor:default}.error{color:#a03333;background:#fff1f0;padding:12px}.error button{text-align:left;overflow-wrap:anywhere}p,li{line-height:1.6;overflow-wrap:anywhere}pre{white-space:pre-wrap;overflow-wrap:anywhere;max-height:400px;overflow:auto;font-size:12px}@media(max-width:700px){.source-editor{padding:8px}textarea{min-height:360px}}</style>
