<script setup lang="ts">
import { ref, watch } from "vue";
const props = defineProps<{
  modelValue?: number;
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: number] }>();
const text = ref(""),
  error = ref("");
watch(
  () => props.modelValue,
  (value) => {
    if (error.value && Number.isNaN(value)) return;
    text.value = value === undefined ? "" : String(value);
    error.value = "";
  },
  { immediate: true },
);
function update(e: Event) {
  text.value = (e.target as HTMLInputElement).value;
  const value = Number(text.value);
  if (!/^\d+$/.test(text.value) || !Number.isSafeInteger(value)) {
    error.value = "Use a nonnegative whole number within 9007199254740991.";
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
      inputmode="numeric"
      :value="text"
      :aria-label="label"
      :aria-invalid="!!error"
      :disabled="disabled"
      @input="update"
    /><small v-if="error" class="state-number-error">{{ error }}</small></label
  >
</template>
<style scoped>
.state-number-error {
  color: #a97249;
  font-size: 11px;
  line-height: 1.6;
}
</style>
