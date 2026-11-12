<script setup lang="ts">
import { computed } from "vue";
import {
  typedDefault,
  typeKey,
  typeLabel,
  type BehaviorPort,
} from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import type { StateBinding, StateSlot } from "../stateTypes";
import TypedValueEditor from "./TypedValueEditor.vue";
const props = defineProps<{
  modelValue?: StateBinding;
  port: BehaviorPort;
  state: StateSlot[];
  inputs: BehaviorPort[];
  contracts: DataContractLock[];
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: StateBinding | undefined];
}>();
const options = computed(() => [
  ...props.state.map((s) => ({
    value: `state:${s.name}`,
    label: `Owned state · ${s.name} · ${typeLabel(s)}`,
    shape: s,
  })),
  ...props.inputs.map((s) => ({
    value: `input:${s.name}`,
    label: `Command input · ${s.name} · ${typeLabel(s)}`,
    shape: s,
  })),
]);
const selected = computed(() =>
  props.modelValue?.state
    ? `state:${props.modelValue.state}`
    : props.modelValue?.input
      ? `input:${props.modelValue.input}`
      : props.modelValue?.literal
        ? "literal"
        : "",
);
function choose(e: Event) {
  const s = (e.target as HTMLSelectElement).value;
  if (!s) emit("update:modelValue", undefined);
  else if (s === "literal")
    emit("update:modelValue", {
      literal: typedDefault(
        props.port.type,
        props.port.recordRef,
        props.contracts,
      ),
    });
  else {
    const [kind, name] = s.split(":");
    emit(
      "update:modelValue",
      kind === "state" ? { state: name } : { input: name },
    );
  }
}
</script>
<template>
  <div class="state-binding">
    <label class="field"
      >{{ label }} <small>{{ typeLabel(port) }}</small
      ><select
        :value="selected"
        :aria-label="`${label} source`"
        :disabled="disabled"
        @change="choose"
      >
        <option value="">Choose a source</option>
        <option value="literal">Typed literal</option>
        <option
          v-if="
            selected &&
            selected !== 'literal' &&
            !options.some((o) => o.value === selected)
          "
          :value="selected"
        >
          Unresolved: {{ selected }}
        </option>
        <option
          v-for="option in options"
          :key="option.value"
          :value="option.value"
          :disabled="typeKey(option.shape) !== typeKey(port)"
        >
          {{ option.label }}
        </option>
      </select></label
    ><TypedValueEditor
      v-if="modelValue?.literal"
      :model-value="modelValue.literal"
      :type="port.type"
      :record-ref="port.recordRef"
      :contracts="contracts"
      :label="`${label} literal`"
      :disabled="disabled"
      @update:model-value="
        emit('update:modelValue', $event ? { literal: $event } : undefined)
      "
    />
  </div>
</template>
<style scoped>
.state-binding {
  background: #f8fbfc;
  border: 1px solid #e0eaee;
  border-radius: 7px;
  padding: 12px;
  margin: 10px 0;
}
.state-binding small {
  font-weight: 400;
  overflow-wrap: anywhere;
}
.state-binding .field {
  margin-bottom: 8px;
}
</style>
