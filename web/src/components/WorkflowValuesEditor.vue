<script setup lang="ts">
import { computed } from "vue";
import {
  typedDefault,
  type BehaviorPort,
  type TypedValue,
} from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import TypedValueEditor from "./TypedValueEditor.vue";
const props = defineProps<{
  ports: BehaviorPort[];
  values: Record<string, TypedValue>;
  contracts: DataContractLock[];
  label: string;
  disabled?: boolean;
  partial?: boolean;
}>();
const extras = computed(() =>
  Object.keys(props.values).filter(
    (k) => !props.ports.some((p) => p.name === k),
  ),
);
function set(name: string, v?: TypedValue) {
  if (v) props.values[name] = v;
  else delete props.values[name];
}
</script>
<template>
  <div class="wf-values">
    <div v-for="p in ports" :key="p.name" class="wf-value-row">
      <label v-if="partial" class="wf-check"
        ><input
          type="checkbox"
          :checked="Object.hasOwn(values, p.name)"
          :disabled="disabled"
          @change="
            ($event.target as HTMLInputElement).checked
              ? set(p.name, typedDefault(p.type, p.recordRef, contracts))
              : set(p.name)
          "
        />Assert {{ p.name }}</label
      ><TypedValueEditor
        v-if="!partial || Object.hasOwn(values, p.name)"
        :model-value="values[p.name]"
        :type="p.type"
        :record-ref="p.recordRef"
        :contracts="contracts"
        :label="`${label} ${p.name}`"
        :disabled="disabled"
        @update:model-value="set(p.name, $event)"
      />
    </div>
    <div v-for="name in extras" :key="name" class="wf-warning">
      Unresolved value {{ name }} is retained.
      <pre>{{ JSON.stringify(values[name], null, 2) }}</pre>
      <button
        class="button quiet"
        :disabled="disabled"
        @click="delete values[name]"
      >
        Remove {{ name }}
      </button>
    </div>
    <p v-if="!ports.length && !extras.length" class="wf-help">
      No values declared.
    </p>
  </div>
</template>
<style scoped>
.wf-values {
  display: grid;
  gap: 10px;
}
.wf-value-row {
  min-width: 0;
}
.wf-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #526f82;
}
.wf-check input {
  width: 16px;
  height: 16px;
}
.wf-warning pre {
  max-height: 140px;
  overflow: auto;
}
</style>
