<script setup lang="ts">
import { computed } from "vue";
import {
  recordRef as makeRecordRef,
  type DataContractLock,
  type DataTypeRef,
} from "../dataTypes";
import TypedValueEditor from "./TypedValueEditor.vue";
import {
  scalarTypes,
  typedDefault,
  type BehaviorBinding,
  type BehaviorPort,
  type BindingSource,
  type ValueType,
  compatibleType,
  typeKey,
  typeLabel,
  type ValueShape,
} from "../behaviorTypes";
const props = defineProps<{
  modelValue?: BehaviorBinding;
  expectedType: ValueType | "T";
  recordRef?: DataTypeRef;
  contracts?: DataContractLock[];
  structured?: boolean;
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
    recordRef: p.recordRef,
  })),
  ...props.sources.map((p) => ({
    value: `node:${p.node}:${p.port}`,
    label: p.label,
    type: p.type,
    recordRef: p.recordRef,
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
        props.recordRef,
        props.contracts,
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
const literalChoices = computed(() => {
  const choices: ValueShape[] =
    props.expectedType === "T" ? scalarTypes.map((type) => ({ type })) : [];
  if (
    props.structured ||
    props.expectedType === "record" ||
    props.expectedType === "collection"
  ) {
    for (const lock of props.contracts || [])
      for (const record of lock.contract.records)
        for (const type of ["record", "collection"] as const) {
          const shape = { type, recordRef: makeRecordRef(lock, record.name) };
          if (
            props.expectedType === "T" ||
            compatibleType(
              { type: props.expectedType, recordRef: props.recordRef },
              shape,
            )
          )
            choices.push(shape);
        }
  }
  return choices;
});
function setLiteralType(event: Event) {
  const choice = literalChoices.value.find(
    (c) => typeKey(c) === (event.target as HTMLSelectElement).value,
  );
  if (!choice || choice.type === "T") return;
  emit("update:modelValue", {
    literal: typedDefault(choice.type, choice.recordRef, props.contracts),
  });
}
</script>
<template>
  <div class="binding-editor">
    <label class="field"
      >{{ label }}
      <span class="binding-type">{{
        expectedType === "T"
          ? "matching type"
          : recordRef
            ? `${expectedType}<${recordRef.record}>`
            : expectedType
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
          :disabled="!compatibleType({ type: expectedType, recordRef }, option)"
        >
          {{ option.label }} · {{ option.type
          }}{{
            option.recordRef
              ? `<${option.recordRef.record}> @ ${option.recordRef.version} · ${option.recordRef.digest.slice(0, 8)}`
              : ""
          }}
        </option>
      </select></label
    >
    <div v-if="modelValue?.literal" class="literal-editor">
      <label
        v-if="
          expectedType === 'T' ||
          ((expectedType === 'record' || expectedType === 'collection') &&
            !recordRef)
        "
        class="field"
        >Literal type<select
          :value="typeKey(modelValue.literal)"
          :aria-label="`${label} literal type`"
          :disabled="disabled"
          @change="setLiteralType"
        >
          <option
            v-if="
              !literalChoices.some(
                (c) => typeKey(c) === typeKey(modelValue!.literal!),
              )
            "
            :value="typeKey(modelValue.literal)"
          >
            Choose an exact record type
          </option>
          <option
            v-for="choice in literalChoices"
            :key="typeKey(choice)"
            :value="typeKey(choice)"
          >
            {{ typeLabel(choice) }}
          </option>
        </select></label
      ><TypedValueEditor
        :model-value="modelValue.literal"
        :type="expectedType === 'T' ? modelValue.literal.type : expectedType"
        :label="`${label} literal`"
        :record-ref="recordRef || modelValue.literal.recordRef"
        :contracts="contracts"
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
