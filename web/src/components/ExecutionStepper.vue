<script setup lang="ts">
import { computed } from 'vue';
const props = defineProps<{ steps: {index: number; kind: string; status: string}[]; selected: number }>();
const emit = defineEmits<{ select: [index: number] }>();
const position = computed(() => Math.max(0,props.steps.findIndex(s => s.index === props.selected)));
function go(offset: number) { const step = props.steps[position.value + offset]; if (step) emit('select',step.index); }
</script>
<template>
  <div v-if="steps.length" class="execution-stepper" aria-label="Recorded execution navigation">
    <button :disabled="position === 0" @click="go(-1)">← Previous event</button>
    <label>Recorded event<select :value="selected" @change="emit('select',Number(($event.target as HTMLSelectElement).value))"><option v-for="(step,i) in steps" :key="step.index" :value="step.index">{{ i+1 }}. {{ step.kind }} · {{ step.status }}</option></select></label>
    <button :disabled="position === steps.length-1" @click="go(1)">Next event →</button>
    <span aria-live="polite">{{ position+1 }} / {{ steps.length }}</span>
    <small>Inspecting recorded evidence; navigation does not issue another command.</small>
  </div>
</template>
<style scoped>
.execution-stepper{display:flex;flex-wrap:wrap;align-items:center;gap:10px;padding:14px;border:1px solid #d5e4ec;background:#f3f9fd;border-radius:8px;margin:12px 0}.execution-stepper button,.execution-stepper select{background:white;border:1px solid #cedee9;padding:8px;border-radius:6px;color:#315e78;max-width:100%}.execution-stepper label{flex:1 1 180px;min-width:0;font-size:11px;color:#547487}.execution-stepper select{display:block;width:100%;margin-top:4px}.execution-stepper small{flex-basis:100%;font-size:11px;color:#6a8393}.execution-stepper button:disabled{opacity:.4}
</style>
