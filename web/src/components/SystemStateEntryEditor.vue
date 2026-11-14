<script setup lang="ts">
import {computed,ref,watch} from 'vue';
import {api} from '../api';
import type {ComponentAspect} from '../definitionTypes';
import type {StateMachineLock} from '../stateTypes';
import {useActionFeedback} from '../composables/useNotifications';
const props=defineProps<{definition:{id:string;version:string};aspects:ComponentAspect[];entries:{slot:string;command:string}[]}>();
const emit=defineEmits<{'update:entries':[entries:{slot:string;command:string}[]]}>();
const choices=ref<{slot:string;commands:string[]}[]>([]),busy=ref(''),error=ref(''),notice=ref('');
const stamp=computed(()=>JSON.stringify([props.definition,props.aspects]));
useActionFeedback('State entry commands',busy,notice,error);
watch(stamp,()=>{choices.value=[];error.value='';notice.value='';});
async function load(){if(busy.value)return;const snapshot=stamp.value;busy.value='Loading pinned state commands';error.value='';notice.value='';try{
 const aspects=props.aspects.filter(a=>a.kind==='state');
 if(!aspects.length)throw new Error('No installed state aspects found. Refresh the design library and check the definition.');
 const rows=await Promise.all(aspects.map(async a=>{const r=await api<{body:{machine:StateMachineLock}}>('/systems/definitions/aspect','POST',a);return {slot:a.slot,commands:r.body.machine.machine.commands.map(c=>c.name)};}));
 if(snapshot===stamp.value){choices.value=rows;notice.value='Choose one entry per multi-command owner. Connections must use that command’s ports.';}
 }catch(e){if(snapshot===stamp.value)error.value=(e as Error).message;}finally{busy.value='';}}
function choose(slot:string,command:string){const entries=props.entries.filter(e=>e.slot!==slot);if(command)entries.push({slot,command});emit('update:entries',entries);}
</script>
<template><section class="state-entries"><h3>State graph entry commands</h3><p>Graph invocations call one selected command per state aspect. Other commands remain available as explicit scenario steps on the same owner.</p><button :disabled="!!busy" @click="load">Load command choices</button><span role="status">{{busy}}</span><p v-if="error" role="alert">{{error}}</p>
 <label v-for="choice in choices" :key="choice.slot">{{choice.slot}} · {{choice.commands.length>1?'Required entry command':'Entry command'}}<select :value="entries.find(e=>e.slot===choice.slot)?.command||''" @change="choose(choice.slot,($event.target as HTMLSelectElement).value)"><option value="">{{choice.commands.length===1?'Automatic: '+choice.commands[0]:'Choose a command…'}}</option><option v-if="entries.some(e=>e.slot===choice.slot&&!choice.commands.includes(e.command))" :value="entries.find(e=>e.slot===choice.slot)?.command">Unresolved: {{entries.find(e=>e.slot===choice.slot)?.command}}</option><option v-for="command in choice.commands" :key="command">{{command}}</option></select></label>
 <p v-for="entry in entries.filter(e=>!choices.some(c=>c.slot===e.slot))" :key="entry.slot">Saved selection: {{entry.slot}} → {{entry.command}} <button @click="choose(entry.slot,'')">Remove</button></p>
 </section></template>
<style scoped>.state-entries{min-width:0}.state-entries p{font-size:12px;line-height:1.5;overflow-wrap:anywhere}.state-entries label{display:grid;gap:6px;margin:10px 0}.state-entries select{min-width:0;width:100%;padding:8px}</style>
