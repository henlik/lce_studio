<script setup lang="ts">
import { computed } from "vue";
import TypedValueEditor from "./TypedValueEditor.vue";
import {
  scalarTypes,
  typedDefault,
  type BehaviorBinding,
  type BehaviorPort,
  type BindingSource,
  type ScalarType,
} from "../behaviorTypes";
const props = defineProps<{
  modelValue?: BehaviorBinding;
  expectedType: ScalarType | "T";
  label: string;
  inputs: BehaviorPort[];
  sources: BindingSource[];
  disabled?: boolean;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: BehaviorBinding | undefined];
}>();
const selected = computed(() =>
  props.modelValue?.input
    ? `input:${props.modelValue.input}`
    : props.modelValue?.node
      ? `node:${props.modelValue.node}:${props.modelValue.port}`
      : props.modelValue?.literal
        ? "literal"
        : "",
);
const options = computed(() => [
  ...props.inputs.map((p) => ({
    value: `input:${p.name}`,
    label: `Input · ${p.name}`,
    type: p.type,
  })),
  ...props.sources.map((p) => ({
    value: `node:${p.node}:${p.port}`,
    label: p.label,
    type: p.type,
  })),
]);
const unresolved = computed(
  () =>
    selected.value !== "" &&
    selected.value !== "literal" &&
    !options.value.some((o) => o.value === selected.value),
);
function choose(event: Event) {
  const value = (event.target as HTMLSelectElement).value;
  if (!value) {
    emit("update:modelValue", undefined);
    return;
  }
  if (value === "literal") {
    emit("update:modelValue", {
      literal: typedDefault(
        props.expectedType === "T" ? "integer" : props.expectedType,
      ),
    });
    return;
  }
  const [kind, first, second] = value.split(":");
  emit(
    "update:modelValue",
    kind === "input" ? { input: first } : { node: first, port: second },
  );
}
function setLiteralType(event: Event) {
  emit("update:modelValue", {
    literal: typedDefault(
      (event.target as HTMLSelectElement).value as ScalarType,
    ),
  });
}
</script>
<template>
  <div class="binding-editor">
    <label class="field"
      >{{ label }}
      <span class="binding-type">{{
        expectedType === "T" ? "matching scalar" : expectedType
      }}</span
      ><select
        :value="selected"
        :disabled="disabled"
        :aria-label="`${label} source`"
        @change="choose"
      >
        <option value="">Unbound — choose source</option>
        <option value="literal">Typed literal</option>
        <option v-if="unresolved" :value="selected">
          Unresolved: {{ selected }}
        </option>
        <option
          v-for="option in options"
          :key="option.value"
          :value="option.value"
          :disabled="
            expectedType !== 'T' &&
            option.type !== 'T' &&
            option.type !== expectedType
          "
        >
          {{ option.label }} · {{ option.type }}
        </option>
      </select></label
    >
    <div v-if="modelValue?.literal" class="literal-editor">
      <label v-if="expectedType === 'T'" class="field"
        >Literal type<select
          :value="modelValue.literal.type"
          :aria-label="`${label} literal type`"
          :disabled="disabled"
          @change="setLiteralType"
        >
          <option v-for="type in scalarTypes" :key="type">{{ type }}</option>
        </select></label
      ><TypedValueEditor
        :model-value="modelValue.literal"
        :type="expectedType === 'T' ? modelValue.literal.type : expectedType"
        :label="`${label} literal`"
        :disabled="disabled"
        @update:model-value="
          (value) =>
            emit('update:modelValue', value ? { literal: value } : undefined)
        "
      />
    </div>
  </div>
</template>
<style scoped>
.binding-editor {
  padding: 12px;
  background: #f8fafc;
  border: 1px solid #e9eef2;
  border-radius: 6px;
  margin-bottom: 10px;
}
.binding-editor .field {
  margin-bottom: 0;
  font-size: 12px;
}
.binding-type {
  color: #657e8f;
  font-size: 12px;
}
.literal-editor {
  display: grid;
  gap: 9px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid #e7ecef;
}
.binding-editor select {
  width: 100%;
  font-size: 12px;
}
</style>
