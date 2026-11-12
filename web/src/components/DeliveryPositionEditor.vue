<script setup lang="ts">
import { ref, watch } from "vue";
const props = defineProps<{
  modelValue: number;
  identity: number;
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: number] }>();
const text = ref(""),
  error = ref("");
const buffers = new Map<number, { text: string; error: string }>();
let activeIdentity: number | undefined;
watch(
  () => [props.modelValue, props.identity] as const,
  ([value, identity]) => {
    if (activeIdentity !== identity) {
      if (activeIdentity !== undefined)
        buffers.set(activeIdentity, { text: text.value, error: error.value });
      activeIdentity = identity;
      const previous = buffers.get(identity);
      if (!Number.isFinite(value) && previous) {
        text.value = previous.text;
        error.value = previous.error;
        return;
      }
    } else if (error.value && !Number.isFinite(value)) return;
    text.value = String(value);
    error.value = Number.isFinite(value)
      ? ""
      : "Enter a finite decimal coordinate within ±1,000,000.";
  },
  { immediate: true },
);
function update(e: Event) {
  text.value = (e.target as HTMLInputElement).value;
  const value = Number(text.value);
  if (
    !/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/.test(text.value) ||
    !Number.isFinite(value) ||
    Math.abs(value) > 1000000
  ) {
    error.value = "Use a decimal coordinate within ±1,000,000.";
    emit("update:modelValue", NaN);
    return;
  }
  error.value = "";
  emit("update:modelValue", value);
}
</script>
<template>
  <label class="field"
    >{{ label
    }}<input
      type="text"
      inputmode="decimal"
      :value="text"
      :aria-label="label"
      :aria-invalid="!!error"
      :disabled="disabled"
      @input="update"
    /><small v-if="error" class="state-help">{{ error }}</small></label
  >
</template>
