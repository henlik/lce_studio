<script setup lang="ts">
import { computed, ref, watch, useId } from 'vue';
import DesignOrientation from './DesignOrientation.vue';
const props = withDefaults(defineProps<{
  workspace: string; active: string; structure?: string[]; behavior?: string[]; execution?: string[];
  title?: string; structureLabel?: string;
}>(), { structure: () => [], behavior: () => [], execution: () => [], structureLabel: 'Structure' });
const emit = defineEmits<{ select: [tab: string] }>();
const id = 'ribbon-' + useId().replace(/:/g, '');
const modes = computed(() => [
  {name:props.structureLabel, tabs:props.structure},
  {name:'Behavior', tabs:props.behavior}, {name:'Execution', tabs:props.execution},
].filter(m => m.tabs.length));
const perspective = computed(() => modes.value.find(m => m.tabs.includes(props.active))?.name || modes.value[0]?.name || 'File');
const selected = ref(perspective.value);
const collapsed = ref(false);
try { collapsed.value = localStorage.getItem('studio.ribbon.' + props.workspace) === 'collapsed'; } catch {}
watch(collapsed, value => { try { localStorage.setItem('studio.ribbon.' + props.workspace, value ? 'collapsed' : 'open'); } catch {} });
watch(perspective, value => { selected.value = value; });
const last = ref<Record<string,string>>({});
function choose(name:string) {
  const old = modes.value.find(m => m.tabs.includes(props.active));
  if(old) last.value[old.name] = props.active;
  selected.value = name; collapsed.value = false;
  const mode = modes.value.find(m => m.name === name);
  if(mode && !mode.tabs.includes(props.active)) emit('select',last.value[name] || mode.tabs[0]!);
}
function keys(event:KeyboardEvent) {
  if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key)) return;
  const buttons = Array.from((event.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('[role="tab"]'));
  const index = buttons.indexOf(event.target as HTMLButtonElement);
  if(index < 0) return;
  event.preventDefault();
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length-1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + buttons.length) % buttons.length;
  buttons[next]?.click(); buttons[next]?.focus();
}
defineExpose({ show:choose });
</script>
<template>
  <section class="studio-ribbon" :class="{collapsed}" :aria-label="workspace + ' commands'">
    <div v-if="title || $slots.status" class="ribbon-document"><strong>{{ title }}</strong><slot name="status" /></div>
    <div class="ribbon-navigation">
      <div role="tablist" :aria-label="workspace + ' ribbon'" class="ribbon-tablist" @keydown="keys">
        <button v-for="name in ['File', ...modes.map(m=>m.name), ...($slots.contract ? ['Contract'] : []), 'Help']" :key="name"
          :id="id + '-' + name" role="tab" :aria-selected="selected === name" :tabindex="selected === name ? 0 : -1"
          :aria-controls="id + '-panel'" @click="choose(name)">{{ name }}</button>
      </div>
      <button class="ribbon-toggle" :aria-expanded="!collapsed" :aria-controls="id + '-panel'" @click="collapsed=!collapsed">{{ collapsed ? 'Show commands' : 'Hide commands' }}</button>
    </div>
    <div v-show="!collapsed" :id="id + '-panel'" role="tabpanel" :aria-labelledby="id + '-' + selected" class="ribbon-content">
      <div v-show="selected === 'File'" class="ribbon-file"><slot name="file" /></div>
      <div v-show="selected === 'Contract'"><slot name="contract" /></div>
      <div v-show="selected === 'Help'" class="ribbon-help"><slot name="help" /><DesignOrientation :workspace="workspace" /></div>
      <div v-show="!['File','Help','Contract'].includes(selected)" class="ribbon-commands"><slot /><slot name="commands" /></div>
    </div>
  </section>
</template>
<style scoped>
.studio-ribbon { min-width:0; margin:0 0 12px; color:#355269; }
.ribbon-document { display:flex; align-items:center; flex-wrap:wrap; gap:8px 16px; margin-bottom:8px; min-width:0; }
.ribbon-document>strong { font-size:14px; overflow-wrap:anywhere; }
.ribbon-document :deep(.draft-status) { margin:0 0 0 auto; border:0; padding:4px 8px; font-size:11px; min-width:0; max-width:100%; box-sizing:border-box; }
.ribbon-document :deep(.draft-status > span) { min-width:0; overflow-wrap:anywhere; }
.ribbon-navigation { display:flex; min-width:0; align-items:center; border:1px solid #dce6ed; border-radius:8px 8px 0 0; background:#f8fbfc; }
.collapsed .ribbon-navigation { border-radius:8px; }
.ribbon-tablist { display:flex; min-width:0; overflow:auto; flex:1; }
.ribbon-tablist button { flex:none; white-space:nowrap; padding:11px 15px; font-size:12px; background:transparent; border:0; border-bottom:3px solid transparent; border-radius:0; color:#4b687a; cursor:pointer; }
.ribbon-tablist button[aria-selected=true] { color:#16715d; border-bottom-color:#23846e; background:#eaf6f1; }
.ribbon-tablist button:focus-visible,.ribbon-toggle:focus-visible { outline:2px solid #23846e; outline-offset:-3px; }
.ribbon-toggle { padding:8px 12px; font-size:11px; color:#567381; border:0; background:transparent; cursor:pointer; white-space:nowrap; }
.ribbon-content { background:#fff; padding:8px 12px; border:1px solid #dce6ed; border-top:0; border-radius:0 0 8px 8px; max-height:240px; overflow:auto; min-width:0; }
.ribbon-commands { display:flex; flex-wrap:wrap; align-items:center; gap:8px 16px; }
.ribbon-content :deep(nav),.ribbon-content :deep(.state-actions),.ribbon-content :deep(.wf-actions),.ribbon-content :deep(.workbench-actions) { margin:0; padding:0; border:0; gap:8px; }
.ribbon-content :deep(button) { font-size:12px; }
.ribbon-file :deep(.state-library),.ribbon-file :deep(.wf-library),.ribbon-file :deep(.workbench-library) { margin:8px 0 0; padding:8px 0; border:0; border-top:1px solid #e1eaee; border-radius:0; }
.ribbon-content :deep(.field) { min-width:0; }
.ribbon-help { font-size:12px; line-height:1.6; }
.ribbon-help :deep(h2) { font-size:16px; margin:4px 0; }
.ribbon-help :deep(.design-orientation) { margin:8px 0 0; }
@media(max-width:640px) {
 .ribbon-tablist button { padding:10px; }
 .ribbon-toggle { max-width:78px; white-space:normal; }
 .ribbon-content { max-height:210px; padding:8px; }
 .ribbon-document :deep(.draft-status) { margin-left:0; }
}
</style>
