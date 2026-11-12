<script setup lang="ts">
import { computed } from "vue";
import type { BehaviorPort } from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import { freshStateName } from "../stateTypes";
import type { WorkflowBinding, WorkflowVariable } from "../workflowTypes";
import WorkflowBindingEditor from "./WorkflowBindingEditor.vue";
const props = defineProps<{
  bindings: Record<string, WorkflowBinding>;
  ports?: BehaviorPort[];
  inputs: BehaviorPort[];
  variables: WorkflowVariable[];
  contracts: DataContractLock[];
  label: string;
  free?: boolean;
  disabled?: boolean;
}>();
const rows = computed(() =>
  props.free
    ? Object.keys(props.bindings).map((name) => ({ name, port: undefined }))
    : (props.ports || []).map((p) => ({ name: p.name, port: p })),
);
const extras = computed(() =>
  props.free
    ? []
    : Object.keys(props.bindings).filter(
        (k) => !props.ports?.some((p) => p.name === k),
      ),
);
function set(k: string, v?: WorkflowBinding) {
  if (v) props.bindings[k] = v;
  else if (props.free) props.bindings[k] = {};
  else delete props.bindings[k];
}
function rename(old: string, e: Event) {
  const k = (e.target as HTMLInputElement).value;
  if (k && k !== old && !Object.hasOwn(props.bindings, k)) {
    props.bindings[k] = props.bindings[old];
    delete props.bindings[old];
  }
}
function add() {
  props.bindings[freshStateName("value", Object.keys(props.bindings))] = {};
}
</script>
<template>
  <div>
    <div v-for="row in rows" :key="row.name" class="wf-map-row">
      <label v-if="free" class="field"
        >Field name<input
          :value="row.name"
          :aria-label="`${label} field ${row.name}`"
          :disabled="disabled"
          @change="rename(row.name, $event)" /></label
      ><WorkflowBindingEditor
        :model-value="bindings[row.name]"
        :port="row.port"
        :inputs="inputs"
        :variables="variables"
        :contracts="contracts"
        :label="`${label} ${row.name}`"
        :disabled="disabled"
        @update:model-value="set(row.name, $event)"
      /><button
        v-if="free"
        class="button quiet"
        :disabled="disabled"
        @click="delete bindings[row.name]"
      >
        Remove {{ row.name }}
      </button>
    </div>
    <div v-for="name in extras" :key="name" class="wf-warning">
      Unresolved binding {{ name }} is retained.
      <code>{{ JSON.stringify(bindings[name]) }}</code
      ><button
        class="button quiet"
        :disabled="disabled"
        @click="delete bindings[name]"
      >
        Remove {{ name }}
      </button>
    </div>
    <button
      v-if="free"
      class="button secondary"
      :disabled="disabled"
      @click="add"
    >
      Add {{ label }} field
    </button>
  </div>
</template>
<style scoped>
.wf-map-row {
  border-top: 1px solid #e4ecef;
  padding-top: 12px;
  margin: 10px 0;
}
.wf-map-row code {
  overflow-wrap: anywhere;
}
</style>
