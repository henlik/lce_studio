<script setup lang="ts">
import {ref,watch} from 'vue';
import {api} from '../api';
import type {ExecutionEvidence,ReplayVerification,ReplaySummary} from '../executionTypes';
import {useActionFeedback} from '../composables/useNotifications';
const props=defineProps<{evidence:ExecutionEvidence;disabled:boolean}>();
const result=ref<ReplayVerification|null>(null),history=ref<ReplaySummary[]>([]);
const busy=ref(''),notice=ref(''),error=ref('');
let generation=0;
useActionFeedback('Replay verification',busy,notice,error);
const label=(status:string)=>status==='matched'?'Reproduced exactly':status==='mismatch'?'Mismatch detected':'Replay unverified';
async function action(text:string,fn:(id:string)=>Promise<{result?:ReplayVerification;history?:ReplaySummary[]}>){
 if(busy.value||props.disabled)return;
 const token=++generation,id=props.evidence.id;
 busy.value=text;notice.value='';error.value='';
 try{const response=await fn(id);if(token!==generation||props.evidence.id!==id)return;if(response.result){result.value=response.result;notice.value=label(response.result.status);}if(response.history)history.value=response.history;}
 catch(e){if(token===generation)error.value=(e as Error).message;}
 finally{if(token===generation)busy.value='';}
}
function historyPath(id:string){return '/systems/executions/'+encodeURIComponent(id)+'/replays';}
async function loadHistory(){await action('Loading replay verifications',async id=>({history:await api<ReplaySummary[]>(historyPath(id))}));}
async function verify(){await action('Replaying saved execution',async id=>{
 const replay=await api<ReplayVerification>(historyPath(id),'POST',{});
 // The saved verification is retained even if a later history refresh fails.
 return {result:replay,history:[{id:replay.id,createdAt:replay.createdAt,status:replay.status,message:replay.message},...history.value].slice(0,50)};
});}
async function open(id:string){await action('Opening replay verification',async()=>({result:await api<ReplayVerification>('/systems/execution-replays/'+encodeURIComponent(id))}));}
function download(){if(!result.value)return;const url=URL.createObjectURL(new Blob([JSON.stringify(result.value,null,2)],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download='system-replay-verification.json';a.click();URL.revokeObjectURL(url);}
watch(()=>props.evidence.id,()=>{generation++;result.value=null;history.value=[];busy.value='';notice.value='';error.value='';});
</script>
<template>
 <section class="replay-panel" :aria-busy="!!busy" aria-label="Saved execution replay verification">
  <h4>Reproduce this execution</h4>
  <p>Replay the saved plan and inputs using their pinned definitions, independently of the current design and catalog. Each verification is saved separately.</p>
  <div class="replay-actions"><button :disabled="disabled||!!busy" @click="verify">Verify replay</button><button :disabled="disabled||!!busy" @click="loadHistory">Previous verifications</button><span role="status">{{busy}}</span></div>
  <p v-if="error" role="alert" class="replay-error">{{error}}</p>
  <div v-if="history.length" class="replay-history"><button v-for="item in history" :key="item.id" :disabled="disabled||!!busy" @click="open(item.id)">{{new Date(item.createdAt).toLocaleString()}} · {{label(item.status)}}</button></div>
  <section v-if="result" class="replay-result" :class="result.status" aria-live="polite">
   <h4>{{label(result.status)}}</h4><p>{{result.message}}</p>
   <p v-if="evidence.report.scenario"><strong>Original scenario: {{!evidence.report.scenario.verified?'unverified (no expectations)':evidence.report.scenario.passed?'expectations passed':'expectations failed'}}.</strong> Reproducing a failed or unverified result does not turn it into a passed scenario.</p>
   <p>Replay checks model reproducibility. It does not verify a deployed implementation or evaluate system contract claims. Hashes are consistency checks, not signed provenance.</p>
   <ul><li v-for="check in result.checks" :key="check.path"><strong>{{check.status}}</strong> · <code>{{check.path}}</code> · {{check.message}}</li></ul>
   <div v-if="result.differences.length" class="difference-table"><table><caption>Recorded versus replayed report</caption><thead><tr><th>Location</th><th>Recorded</th><th>Replayed</th></tr></thead><tbody><tr v-for="diff in result.differences" :key="diff.path"><td><code>{{diff.path}}</code></td><td>{{diff.recorded}}</td><td>{{diff.replayed}}</td></tr></tbody></table><p v-if="result.differencesTruncated">Showing the first 32 differences. Export the verification to inspect its complete replay report.</p></div>
   <details><summary>Verification identity and replay report</summary><p>Execution: {{result.executionId}}<br/>Verifier: {{result.verifierVersion}}<br/>Snapshot: {{result.snapshotHash}}<br/>Recorded report: {{result.recordedReportHash}}<br/>Replayed report: {{result.replayedReportHash||'Not produced'}}</p><pre v-if="result.report">{{JSON.stringify(result.report,null,2)}}</pre></details>
   <button @click="download">Export replay verification</button>
  </section>
 </section>
</template>
<style scoped>
.replay-panel{border:1px solid #cbdce3;border-radius:8px;padding:14px;margin:16px 0;min-width:0}.replay-panel h4{margin:0 0 10px}.replay-panel p,.replay-panel li{font-size:12px;line-height:1.6;overflow-wrap:anywhere}.replay-actions,.replay-history{display:flex;gap:8px;flex-wrap:wrap;align-items:center;margin:12px 0}.replay-panel button{max-width:100%;white-space:normal;overflow-wrap:anywhere}.replay-result{border-left:4px solid #a58b45;padding:12px;background:#fffbef;min-width:0}.replay-result.matched{border-color:#2d8065;background:#f1faf5}.replay-result.mismatch,.replay-error{border-color:#b04444;background:#fff2f1;color:#8c2c2c}.replay-panel ul{padding-left:20px}.difference-table{overflow:auto;max-width:100%}table{width:100%;border-collapse:collapse;font-size:12px;table-layout:fixed}caption{text-align:left;font-weight:600;padding:8px}th,td{text-align:left;vertical-align:top;overflow-wrap:anywhere;border:1px solid #d2dfe4;padding:7px}pre{white-space:pre-wrap;overflow-wrap:anywhere;max-height:320px;overflow:auto;font-size:11px}.replay-panel details{margin:12px 0}
</style>
