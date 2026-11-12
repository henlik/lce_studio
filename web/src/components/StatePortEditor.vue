<script setup lang="ts">
import type { BehaviorPort, ValueShape, ValueType } from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import { freshStateName, stateUIKey } from "../stateTypes";
import StateTypeSelect from "./StateTypeSelect.vue";
const props = defineProps<{
  ports: BehaviorPort[];
  contracts: DataContractLock[];
  label: string;
  disabled?: boolean;
}>();
function setType(port: BehaviorPort, shape: ValueShape) {
  port.type = shape.type as ValueType;
  if (shape.recordRef) port.recordRef = { ...shape.recordRef };
  else delete port.recordRef;
}
function add() {
  props.ports.push({
    name: freshStateName(
      "value",
      props.ports.map((p) => p.name),
    ),
    type: "string",
  });
}
</script>
<template>
  <div class="state-port-editor">
    <div
      v-for="(port, i) in ports"
      :key="stateUIKey(port)"
      class="state-port-row"
    >
      <label class="field"
        >Name<input
          v-model="port.name"
          :disabled="disabled"
          :aria-label="`${label} ${i + 1} name`" /></label
      ><label class="field"
        >Type<StateTypeSelect
          :model-value="port"
          :contracts="contracts"
          :label="`${label} ${i + 1} type`"
          :disabled="disabled"
          @update:model-value="setType(port, $event)" /></label
      ><button
        class="button quiet"
        :disabled="disabled"
        @click="ports.splice(i, 1)"
      >
        Remove {{ label }} {{ i + 1 }}
      </button>
    </div>
    <button
      class="button secondary"
      :disabled="disabled || ports.length >= 8"
      @click="add"
    >
      Add {{ label }}
    </button>
    <p class="state-help">
      Changing ports keeps existing bindings and values for explicit correction.
    </p>
  </div>
</template>
<style scoped>
.state-port-row {
  display: grid;
  grid-template-columns: 1fr 2fr auto;
  gap: 12px;
  align-items: end;
}
.state-port-row .button {
  margin-bottom: 16px;
}
@media (max-width: 800px) {
  .state-port-row {
    grid-template-columns: 1fr;
  }
  .state-port-row .button {
    margin-bottom: 5px;
  }
}
</style>
