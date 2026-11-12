<script setup lang="ts">
import { Plus, Trash2 } from "lucide-vue-next";

interface Window {
  startMs: number;
  endMs: number;
}
const props = defineProps<{
  modelValue: Window[];
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: Window[]] }>();
function update(index: number, key: "startMs" | "endMs", event: Event) {
  const value = Number((event.target as HTMLInputElement).value);
  emit(
    "update:modelValue",
    props.modelValue.map((w, i) => (i === index ? { ...w, [key]: value } : w)),
  );
}
function add() {
  const startMs = props.modelValue.length
    ? Math.max(...props.modelValue.map((w) => w.endMs)) + 500
    : 500;
  emit("update:modelValue", [
    ...props.modelValue,
    { startMs, endMs: startMs + 500 },
  ]);
}
</script>

<template>
  <div class="outage-windows">
    <div class="outage-title">
      <strong>{{ label }}</strong
      ><button
        class="button quiet"
        type="button"
        @click="add"
        :disabled="disabled || modelValue.length >= 20"
      >
        <Plus :size="13" /> Add outage
      </button>
    </div>
    <p v-if="!modelValue.length" class="outage-empty">
      Always available. Add an interval to interrupt this resource.
    </p>
    <div v-for="(window, index) in modelValue" :key="index" class="outage-row">
      <label class="field"
        >Start · ms<input
          type="number"
          :value="window.startMs"
          min="0"
          max="3599999"
          step="1"
          :aria-label="`${label} ${index + 1} start milliseconds`"
          :disabled="disabled"
          @input="update(index, 'startMs', $event)"
      /></label>
      <label class="field"
        >End · ms<input
          type="number"
          :value="window.endMs"
          min="1"
          max="3600000"
          step="1"
          :aria-label="`${label} ${index + 1} end milliseconds`"
          :disabled="disabled"
          @input="update(index, 'endMs', $event)"
      /></label>
      <button
        class="icon-button"
        type="button"
        :aria-label="`Remove ${label.toLowerCase()} ${index + 1}`"
        :disabled="disabled"
        @click="
          emit(
            'update:modelValue',
            modelValue.filter((_, i) => i !== index),
          )
        "
      >
        <Trash2 :size="15" />
      </button>
    </div>
    <p v-if="modelValue.length" class="outage-caption">
      Unavailable from the start up to, but not including, the end. Intervals
      must not overlap.
    </p>
  </div>
</template>

<style scoped>
.outage-windows {
  border-top: 1px solid var(--line);
  padding-top: 14px;
  margin-top: 10px;
}
.outage-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}
.outage-title strong {
  font-size: 12px;
}
.outage-title .button {
  padding: 6px 8px;
  font-size: 11px;
}
.outage-row {
  display: grid;
  grid-template-columns: 1fr 1fr 28px;
  align-items: end;
  gap: 9px;
  margin-bottom: 9px;
}
.outage-row .field {
  margin: 0;
  font-size: 10px;
}
.outage-row .icon-button {
  margin-bottom: 3px;
}
.outage-empty,
.outage-caption {
  font-size: 11px;
  line-height: 1.6;
  color: #6e7e91;
  margin: 0 0 8px;
}
.outage-caption {
  margin-top: 8px;
}
</style>
