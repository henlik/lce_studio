<script setup lang="ts">
import { computed } from "vue";
import {
  typedDefault,
  typeKey,
  typeLabel,
  type BehaviorPort,
  type ValueShape,
  type ValueType,
} from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import type { WorkflowBinding, WorkflowVariable } from "../workflowTypes";
import TypedValueEditor from "./TypedValueEditor.vue";
import StateTypeSelect from "./StateTypeSelect.vue";
const props = defineProps<{
  modelValue?: WorkflowBinding;
  port?: BehaviorPort;
  inputs: BehaviorPort[];
  variables: WorkflowVariable[];
  contracts: DataContractLock[];
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: WorkflowBinding | undefined];
}>();
const options = computed(() => [
  ...props.inputs.map((p) => ({
    key: `input:${p.name}`,
    label: `Input · ${p.name}`,
    shape: p,
  })),
  ...props.variables.map((p) => ({
    key: `variable:${p.name}`,
    label: `Variable · ${p.name}`,
    shape: p,
  })),
]);
const selected = computed(() =>
  props.modelValue?.input
    ? `input:${props.modelValue.input}`
    : props.modelValue?.variable
      ? `variable:${props.modelValue.variable}`
      : props.modelValue?.literal
        ? "literal"
        : "",
);
const shape = computed(
  () => props.port || props.modelValue?.literal || { type: "string" as const },
);
function choose(e: Event) {
  const key = (e.target as HTMLSelectElement).value;
  if (!key) emit("update:modelValue", undefined);
  else if (key === "literal")
    emit("update:modelValue", {
      literal: typedDefault(
        shape.value.type,
        shape.value.recordRef,
        props.contracts,
      ),
    });
  else {
    const kind = key.slice(0, key.indexOf(":")),
      name = key.slice(key.indexOf(":") + 1);
    emit(
      "update:modelValue",
      kind === "input" ? { input: name } : { variable: name },
    );
  }
}
function literalType(s: ValueShape) {
  emit("update:modelValue", {
    literal: typedDefault(s.type as ValueType, s.recordRef, props.contracts),
  });
}
</script>
<template>
  <div class="wf-binding">
    <label class="field"
      >{{ label }}<small v-if="port">{{ typeLabel(port) }}</small
      ><select
        :aria-label="`${label} source`"
        :value="selected"
        :disabled="disabled"
        @change="choose"
      >
        <option value="">Choose a source</option>
        <option value="literal">Typed literal</option>
        <option
          v-if="
            selected &&
            selected !== 'literal' &&
            !options.some((o) => o.key === selected)
          "
          :value="selected"
        >
          Unresolved: {{ selected }}
        </option>
        <option
          v-for="o in options"
          :key="o.key"
          :value="o.key"
          :disabled="!!port && typeKey(port) !== typeKey(o.shape)"
        >
          {{ o.label }} · {{ typeLabel(o.shape) }}
        </option>
      </select></label
    ><template v-if="modelValue?.literal"
      ><StateTypeSelect
        v-if="!port"
        :model-value="shape"
        :contracts="contracts"
        :label="`${label} literal type`"
        :disabled="disabled"
        @update:model-value="literalType" /><TypedValueEditor
        :model-value="modelValue.literal"
        :type="shape.type"
        :record-ref="shape.recordRef"
        :contracts="contracts"
        :label="`${label} literal`"
        :disabled="disabled"
        @update:model-value="
          emit('update:modelValue', $event ? { literal: $event } : undefined)
        "
    /></template>
  </div>
</template>
<style scoped>
.wf-binding {
  min-width: 0;
}
.wf-binding .field {
  margin-bottom: 9px;
}
.wf-binding small {
  font-size: 10px;
  overflow-wrap: anywhere;
}
</style>
