<script setup lang="ts">
import {computed} from 'vue';
import {Folder,FileCode2} from 'lucide-vue-next';
const props=defineProps<{files:string[];prefix?:string;active:string;entry:string}>();
const emit=defineEmits<{open:[file:string]}>();
const prefix=computed(()=>props.prefix||'');
const items=computed(()=>[...new Set(props.files.filter(f=>f.startsWith(prefix.value)).map(f=>f.slice(prefix.value.length).split('/')[0]))].sort());
const isFolder=(item:string)=>props.files.some(f=>f.startsWith(prefix.value+item+'/'));
</script>
<template><div class="file-tree"><template v-for="item in items" :key="item"><details v-if="isFolder(item)" :open="active.startsWith(prefix+item+'/')"><summary><Folder :size="13"/>{{item}}</summary><ConfigurationFileTree :files="files" :prefix="prefix+item+'/'" :active="active" :entry="entry" @open="emit('open',$event)"/></details><button v-else :title="prefix+item" :aria-current="prefix+item===active?'page':undefined" @click="emit('open',prefix+item)"><FileCode2 :size="13"/><span>{{item}}{{prefix+item===entry?' · entry':''}}</span></button></template></div></template>
<style scoped>
.file-tree{min-width:0}details .file-tree{margin-left:11px;padding-left:6px;border-left:1px solid #cbdbe2}summary{font-size:11px;padding:6px 3px;color:#516e7e;cursor:pointer}summary svg{vertical-align:middle;margin-right:5px}button{display:flex;align-items:center;gap:6px;width:100%;text-align:left;color:#38586b;font:11px/1.5 ui-monospace,monospace;background:transparent;padding:6px 4px;border:0;border-radius:4px;min-width:0}button span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}button svg{flex:none}button:hover,button[aria-current=page]{background:#dcebef}button:focus-visible,summary:focus-visible{outline:2px solid #247d99}
</style>
