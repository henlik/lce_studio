<script setup lang="ts">
import { ref, watch } from "vue";
import type { ScalarType, TypedValue } from "../behaviorTypes";
const props = defineProps<{
  modelValue?: TypedValue;
  type: ScalarType;
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: TypedValue | undefined];
}>();
const integerText = ref("");
const integerError = ref("");
watch(
  () => [props.modelValue?.integer, props.modelValue?.type, props.type],
  () => {
    if (
      integerError.value &&
      props.modelValue?.type === "integer" &&
      props.modelValue.integer === undefined
    )
      return;
    integerText.value =
      props.modelValue?.type === "integer" &&
      props.modelValue.integer !== undefined
        ? String(props.modelValue.integer)
        : "";
    integerError.value = "";
  },
  { immediate: true },
);
function update(event: Event) {
  const raw = (event.target as HTMLInputElement).value;
  if (props.type === "integer") {
    integerText.value = raw;
    if (!/^[+-]?\d+$/.test(raw) || !Number.isSafeInteger(Number(raw))) {
      integerError.value =
        raw === ""
          ? "Enter an integer; no value is currently supplied."
          : "Use whole decimal digits within ±9007199254740991. Fractions, exponents and rounding are not accepted.";
      emit("update:modelValue", { type: "integer" });
      return;
    }
    integerError.value = "";
  }
  if (props.type === "boolean" && raw === "") {
    emit("update:modelValue", { type: "boolean" });
    return;
  }
  emit("update:modelValue", {
    type: props.type,
    [props.type]:
      props.type === "boolean"
        ? raw === "true"
        : props.type === "integer"
          ? Number(raw)
          : raw,
  });
}
</script>
<template>
  <label class="field typed-value"
    ><span
      >{{ label }} <small>{{ type }}</small></span
    >
    <select
      v-if="type === 'boolean'"
      :value="modelValue?.type === type ? String(modelValue.boolean) : ''"
      :aria-label="label"
      :disabled="disabled"
      @change="update"
    >
      <option value="">Missing value</option>
      <option value="true">true</option>
      <option value="false">false</option>
    </select>
    <input
      v-else-if="type === 'integer'"
      type="text"
      inputmode="numeric"
      :value="integerText"
      :aria-label="label"
      :aria-invalid="!!integerError"
      :disabled="disabled"
      @input="update"
    />
    <textarea
      v-else
      rows="2"
      :value="modelValue?.type === type ? modelValue.string : ''"
      :aria-label="label"
      :disabled="disabled"
      @input="update"
    />
    <em v-if="modelValue && modelValue.type !== type"
      >Stored value is {{ modelValue.type }}. Enter a {{ type }} value to
      replace it.</em
    >
    <em v-if="integerError" role="status">{{ integerError }}</em>
  </label>
</template>
<style scoped>
.typed-value {
  margin: 0;
  min-width: 0;
}
.typed-value > span {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  overflow-wrap: anywhere;
}
.typed-value small {
  color: #657e8f;
  font-size: 12px;
  font-weight: 400;
}
.typed-value textarea {
  resize: vertical;
  min-height: 52px;
  padding: 8px;
  font-size: 12px;
}
.typed-value em {
  font-size: 12px;
  color: #b17c4f;
  font-style: normal;
  margin-top: 5px;
}
</style>
