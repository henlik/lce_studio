<script setup lang="ts">
import { computed, ref } from 'vue';
const props = defineProps<{ active: string; structure: string[]; behavior: string[]; execution: string[] }>();
const emit = defineEmits<{ select: [tab: string] }>();
const last = ref<Record<string, string>>({});
const modes = computed(() => [
  { name: 'Structure', tabs: props.structure, help: 'Boundary, parts and contracts' },
  { name: 'Behavior', tabs: props.behavior, help: 'Rules, routes and scenarios' },
  { name: 'Execution', tabs: props.execution, help: 'Run, inspect and replay evidence' },
]);
function select(mode: {name: string; tabs: string[]}) {
  for (const item of modes.value) if (item.tabs.includes(props.active)) last.value[item.name] = props.active;
  emit('select', last.value[mode.name] || mode.tabs[0]!);
}
</script>
<template>
  <nav class="workspace-modes" aria-label="Design perspective">
    <button v-for="mode in modes" :key="mode.name" :aria-pressed="mode.tabs.includes(active)" @click="select(mode)">
      <strong>{{ mode.name }}</strong><small>{{ mode.help }}</small>
    </button>
  </nav>
</template>
<style scoped>
.workspace-modes{display:flex;flex-wrap:wrap;gap:8px;padding:12px 0;min-width:0}.workspace-modes button{flex:1 1 150px;text-align:left;padding:12px 16px;border:1px solid #d7e3ec;border-radius:9px;background:#f7fafc;color:#426176;cursor:pointer}.workspace-modes button[aria-pressed=true]{background:#eaf5fc;border-color:#2984bc;color:#165e8c}.workspace-modes strong,.workspace-modes small{display:block}.workspace-modes small{font-size:11px;margin-top:5px;line-height:1.5}
</style>
