<script setup lang="ts">
import {computed,ref} from 'vue';
import type {SystemCatalogEntry} from '../systemTypes';
const props=defineProps<{entries:SystemCatalogEntry[];selectedName?:string;busy:boolean}>();
const emit=defineEmits<{refresh:[];add:[entry:SystemCatalogEntry];attach:[entry:SystemCatalogEntry]}>();
const search=ref('');
const kinds=['definition','component','data','state','messages','workflow'];
const filtered=computed(()=>props.entries.filter(e=>(e.name+' '+e.id+' '+e.version).toLowerCase().includes(search.value.toLowerCase())));
</script>
<template>
 <aside class="system-library" aria-label="System reference library">
  <div class="library-heading"><strong>Reference library</strong><button :disabled="busy" @click="emit('refresh')">Refresh</button></div>
  <label>Search definitions<input v-model="search" type="search" placeholder="Name, ID or version"/></label>
  <p>Exact installed versions and built-in adapters. Select a component to attach data, state, message or workflow references.</p>
  <p v-if="!filtered.length">No matching definitions. Install a package in its component editor, then refresh.</p>
  <section v-for="kind in kinds" :key="kind" v-show="filtered.some(e=>e.kind===kind)"><h3>{{kind}}</h3>
   <article v-for="entry in filtered.filter(e=>e.kind===kind)" :key="entry.kind+entry.id+entry.version">
    <strong>{{entry.name||entry.id}}</strong><small>{{entry.id}} · {{entry.version}}</small><small>{{entry.source==='builtin-adapter'?'Built-in reference adapter':'Installed package'}}</small>
    <p v-if="!entry.ready" class="catalog-error">{{entry.issue}}</p>
    <button v-if="kind==='component'||kind==='definition'" :disabled="!entry.ready||busy" @click="emit('add',entry)">Add component</button>
    <button v-else :disabled="!entry.ready||busy||!selectedName" @click="emit('attach',entry)">{{selectedName?'Attach to '+selectedName:'Select a component first'}}</button>
   </article>
  </section>
 </aside>
</template>
<style scoped>
.system-library{background:#fff;border:1px solid #d5e3e9;border-radius:8px;padding:12px;min-width:0;max-height:650px;overflow:auto}.library-heading{display:flex;justify-content:space-between;align-items:center;gap:8px;font-size:12px}.system-library label{display:grid;gap:5px;font-size:11px;margin:12px 0}.system-library input{width:100%;box-sizing:border-box;min-width:0;border:1px solid #cadbe3;border-radius:5px;padding:8px}.system-library p{font-size:11px;line-height:1.5;color:#66818e}.system-library h3{text-transform:capitalize;font-size:12px;margin:16px 0 8px}.system-library article{border-top:1px solid #e0e8ed;padding:10px 0;display:grid;gap:6px;overflow-wrap:anywhere;font-size:12px}.system-library small{font-size:10px;color:#66818e}.system-library button{border:1px solid #bdd6ca;border-radius:5px;background:#eff8f3;color:#286b50;padding:7px;font-size:11px;cursor:pointer;white-space:normal}.system-library button:disabled{opacity:.5;cursor:default}.catalog-error{color:#a03a34!important}
</style>
