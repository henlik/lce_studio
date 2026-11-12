<script setup lang="ts">
import { typeKey, typeLabel, type BehaviorPort } from "../behaviorTypes";
import type { WorkflowVariable } from "../workflowTypes";
const props = defineProps<{
  assign: Record<string, string>;
  variables: WorkflowVariable[];
  outputs: BehaviorPort[];
  disabled?: boolean;
}>();
function update(name: string, e: Event) {
  const v = (e.target as HTMLSelectElement).value;
  if (v) props.assign[name] = v;
  else delete props.assign[name];
}
</script>
<template>
  <div>
    <p class="wf-help">
      Copy returned fields into workflow variables. Unmapped variables keep
      their previous values.
    </p>
    <label v-for="v in variables" :key="v.name" class="field"
      >Set {{ v.name }} · {{ typeLabel(v)
      }}<select
        :value="assign[v.name] || ''"
        :aria-label="`Assign ${v.name}`"
        :disabled="disabled"
        @change="update(v.name, $event)"
      >
        <option value="">Keep previous value</option>
        <option
          v-if="
            assign[v.name] && !outputs.some((p) => p.name === assign[v.name])
          "
          :value="assign[v.name]"
        >
          Unresolved: {{ assign[v.name] }}
        </option>
        <option
          v-for="p in outputs"
          :key="p.name"
          :value="p.name"
          :disabled="typeKey(p) !== typeKey(v)"
        >
          {{ p.name }} · {{ typeLabel(p) }}
        </option>
      </select></label
    >
    <div
      v-for="k in Object.keys(assign).filter(
        (k) => !variables.some((v) => v.name === k),
      )"
      :key="k"
      class="wf-warning"
    >
      Unknown variable {{ k }} ← {{ assign[k]
      }}<button
        class="button quiet"
        :disabled="disabled"
        @click="delete assign[k]"
      >
        Remove assignment
      </button>
    </div>
  </div>
</template>
