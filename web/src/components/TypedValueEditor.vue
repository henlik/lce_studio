<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  typedDefault,
  cloneBehavior,
  type ValueType,
  type TypedValue,
} from "../behaviorTypes";
import {
  resolveRecord,
  dataRefKey,
  type DataTypeRef,
  type DataContractLock,
} from "../dataTypes";
const props = defineProps<{
  modelValue?: TypedValue;
  type: ValueType;
  recordRef?: DataTypeRef;
  contracts?: DataContractLock[];
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: TypedValue | undefined];
}>();
const definition = computed(() =>
  resolveRecord(props.recordRef, props.contracts || []),
);
const rows = computed(() =>
  props.modelValue?.type === "collection" &&
  Array.isArray(props.modelValue.collection)
    ? props.modelValue.collection
    : [],
);
const refMismatch = computed(
  () =>
    props.modelValue &&
    ["record", "collection"].includes(props.type) &&
    dataRefKey(props.modelValue.recordRef) !== dataRefKey(props.recordRef),
);
function setField(name: string, value: TypedValue | undefined) {
  const fields = { ...(props.modelValue?.record || {}) };
  if (value) fields[name] = cloneBehavior(value);
  else delete fields[name];
  emit("update:modelValue", {
    type: "record",
    recordRef: props.recordRef ? { ...props.recordRef } : undefined,
    record: fields,
  });
}
function setRow(index: number, value: TypedValue | undefined) {
  const items = cloneBehavior(rows.value);
  if (value) items[index] = value;
  emit("update:modelValue", {
    type: "collection",
    recordRef: props.recordRef ? { ...props.recordRef } : undefined,
    collection: items,
  });
}
function addRow() {
  if (rows.value.length >= 128) return;
  emit("update:modelValue", {
    type: "collection",
    recordRef: props.recordRef ? { ...props.recordRef } : undefined,
    collection: [
      ...cloneBehavior(rows.value),
      typedDefault("record", props.recordRef, props.contracts),
    ],
  });
}
function removeRow(index: number) {
  const items = cloneBehavior(rows.value);
  items.splice(index, 1);
  emit("update:modelValue", {
    type: "collection",
    recordRef: props.recordRef ? { ...props.recordRef } : undefined,
    collection: items,
  });
}
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
  if (props.type === "record" || props.type === "collection") return;
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
  <div class="field typed-value">
    <span
      >{{ label }} <small>{{ type }}</small></span
    >
    <template v-if="type === 'record' || type === 'collection'">
      <small v-if="recordRef"
        >{{ recordRef.contractId }} @ {{ recordRef.version }} /
        {{ recordRef.record }} · {{ recordRef.digest.slice(0, 10) }}</small
      >
      <p v-if="!definition" class="structured-note">
        Select an embedded record type to edit this value.
      </p>
      <template v-else-if="type === 'record'"
        ><div class="record-fields">
          <TypedValueEditor
            v-for="field in definition.fields"
            :key="field.name"
            :type="field.type"
            :label="`${label}.${field.name}`"
            :model-value="modelValue?.record?.[field.name]"
            :disabled="disabled"
            @update:model-value="(value) => setField(field.name, value)"
          /></div
      ></template>
      <template v-else
        ><p class="structured-note">
          {{ rows.length }} ordered records · maximum 128
        </p>
        <div v-for="(row, index) in rows" :key="index" class="collection-row">
          <TypedValueEditor
            type="record"
            :record-ref="recordRef"
            :contracts="contracts"
            :label="`${label}[${index}]`"
            :model-value="row"
            :disabled="disabled"
            @update:model-value="(value) => setRow(index, value)"
          /><button
            type="button"
            class="button quiet"
            :disabled="disabled"
            @click="removeRow(index)"
          >
            Remove record {{ index + 1 }}
          </button>
        </div>
        <button
          type="button"
          class="button secondary"
          :disabled="disabled || rows.length >= 128"
          @click="addRow"
        >
          Add record to {{ label }}
        </button></template
      >
      <em v-if="refMismatch"
        >Stored value has a different exact record identity. Editing replaces it
        with the selected type.</em
      >
    </template>
    <select
      v-else-if="type === 'boolean'"
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
  </div>
</template>
<style scoped>
.record-fields {
  display: grid;
  gap: 12px;
  padding: 12px;
  border: 1px solid #d9e4eb;
  border-radius: 6px;
  background: #fafcfd;
}
.collection-row {
  padding: 12px;
  border: 1px solid #dce7ed;
  border-radius: 7px;
  margin: 10px 0;
}
.structured-note {
  font-size: 12px;
  color: #627b8c;
  line-height: 1.6;
}
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
