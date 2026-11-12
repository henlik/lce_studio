<script setup lang="ts">
import { computed } from "vue";
import { typeKey, typeLabel, type ValueShape } from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import { stateTypeChoices } from "../stateTypes";
const props = defineProps<{
  modelValue: ValueShape;
  contracts: DataContractLock[];
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: ValueShape] }>();
const choices = computed(() => stateTypeChoices(props.contracts));
function update(e: Event) {
  const choice = choices.value.find(
    (c) => c.key === (e.target as HTMLSelectElement).value,
  );
  if (choice)
    emit("update:modelValue", {
      type: choice.type,
      ...(choice.recordRef ? { recordRef: { ...choice.recordRef } } : {}),
    });
}
</script>
<template>
  <select
    :value="typeKey(modelValue)"
    :aria-label="label"
    :disabled="disabled"
    @change="update"
  >
    <option
      v-if="!choices.some((c) => c.key === typeKey(modelValue))"
      :value="typeKey(modelValue)"
    >
      Unresolved: {{ typeLabel(modelValue) }}
    </option>
    <option v-for="c in choices" :key="c.key" :value="c.key">
      {{ c.label }}
    </option>
  </select>
</template>
