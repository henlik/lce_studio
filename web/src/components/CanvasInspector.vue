<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue';
defineProps<{ title:string }>();
const emit=defineEmits<{close:[]}>();
const panel=ref<HTMLElement|null>(null);
let previous:HTMLElement|null=null;
onMounted(async()=>{ previous=document.activeElement as HTMLElement|null; await nextTick(); if(!panel.value?.contains(document.activeElement)) panel.value?.focus({preventScroll:true}); });
function close() { emit('close'); void nextTick(()=>previous?.isConnected && previous.focus({preventScroll:true})); }
</script>
<template>
  <aside ref="panel" class="canvas-context-inspector" tabindex="-1" :aria-label="title" @pointerdown.stop @keydown.esc.stop.prevent="close">
    <header><strong>{{ title }}</strong><button type="button" aria-label="Close canvas settings" @click="close">×</button></header>
    <div class="context-inspector-body"><slot /></div>
  </aside>
</template>
<style scoped>
.canvas-context-inspector { position:absolute; z-index:8; top:64px; right:12px; bottom:12px; width:min(400px,calc(100% - 24px)); display:flex; flex-direction:column; min-width:0; background:#fff; border:1px solid #85b6aa; border-radius:10px; box-shadow:0 12px 38px #234e6433; overflow:hidden; color:#355269; }
.canvas-context-inspector:focus { outline:none; }
header { display:flex; align-items:center; justify-content:space-between; padding:10px 14px; background:#eef8f5; border-bottom:1px solid #d2e7df; gap:8px; }
header strong { font-size:14px; overflow-wrap:anywhere; }
header button { border:0; background:transparent; color:#476c60; font-size:24px; cursor:pointer; }
.context-inspector-body { overflow:auto; min-height:0; padding:12px; overscroll-behavior:contain; }
.context-inspector-body :deep(.state-panel) { border:0; margin:0; padding:0; box-shadow:none; }
.context-inspector-body :deep(.state-grid) { grid-template-columns:minmax(0,1fr); }
.context-inspector-body :deep(fieldset),.context-inspector-body :deep(.field) { min-width:0; }
.context-inspector-body :deep(.state-card) { padding:10px; }
.context-inspector-body :deep(input),.context-inspector-body :deep(select) { max-width:100%; }
@media(max-width:640px) { .canvas-context-inspector { top:130px; right:8px; width:calc(100% - 16px); bottom:8px; } }
</style>
