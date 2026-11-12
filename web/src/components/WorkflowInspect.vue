<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { ComponentBundle, PrimitiveSpec } from "../behaviorTypes";
import {
  referenceKey,
  typeLabel,
  nodeInputs,
  nodeOutputs,
  valueText,
} from "../behaviorTypes";
import { bundleRoot, downloadWorkflow } from "../workflowTypes";
import BehaviorGraph from "./BehaviorGraph.vue";
const props = defineProps<{
  bundle: ComponentBundle;
  primitives: PrimitiveSpec[];
}>();
const path = ref<string[]>([]),
  selected = ref("");
watch(
  () => props.bundle,
  () => {
    path.value = [];
    selected.value = "";
  },
);
const root = computed(() => bundleRoot(props.bundle));
const current = computed(() => {
  let p = root.value;
  for (const id of path.value) {
    const n = p?.behavior.nodes.find((n) => n.id === id);
    p = props.bundle.packages.find(
      (l) =>
        n?.component &&
        `${l.package.id}@${l.package.version}:${l.digest}` ===
          referenceKey(n.component),
    )?.package;
  }
  return p;
});
const contracts = computed(() =>
  props.bundle.packages.flatMap((p) => p.package.dataContracts || []),
);
const node = computed(() =>
  current.value?.behavior.nodes.find((n) => n.id === selected.value),
);
const primitive = computed(() =>
  props.primitives.find((p) => p.op === node.value?.op),
);
function inspect(id: string) {
  const n = current.value?.behavior.nodes.find((n) => n.id === id);
  if (n?.component) {
    path.value.push(id);
    selected.value = "";
  } else selected.value = id;
}
</script>
<template>
  <section class="wf-inspect">
    <div class="wf-actions">
      <strong>Exact pure dependency</strong
      ><button
        class="button secondary"
        @click="
          downloadWorkflow(bundle, `${bundle.root.packageId}.bundle.json`)
        "
      >
        Export reusable pure bundle
      </button>
    </div>
    <div class="wf-breadcrumb">
      <button
        class="button quiet"
        @click="
          path = [];
          selected = '';
        "
      >
        {{ root?.name || bundle.root.packageId }}</button
      ><button
        v-for="(part, i) in path"
        :key="i"
        class="button quiet"
        @click="
          path = path.slice(0, i + 1);
          selected = '';
        "
      >
        / {{ part }}
      </button>
    </div>
    <template v-if="current"
      ><p class="wf-help">
        {{ current.id }} @ {{ current.version }} · locked in this workflow.
        Select a primitive, or double-click a component to descend.
      </p>
      <BehaviorGraph
        :behavior="current.behavior"
        :dependencies="bundle.packages"
        :primitives="primitives"
        :selected-id="selected"
        :readonly="true"
        :contracts="contracts"
        @select="selected = $event"
        @inspect="inspect"
      />
      <div v-if="node" class="wf-panel">
        <h4>
          {{ node.id }} ·
          {{ primitive?.name || node.component?.packageId || node.op }}
        </h4>
        <p class="wf-help">{{ primitive?.description }}</p>
        <button
          v-if="node.component"
          class="button secondary"
          @click="inspect(node.id)"
        >
          Inspect nested component
        </button>
        <div class="wf-grid">
          <section>
            <h4>Inputs</h4>
            <p
              v-for="p in nodeInputs(
                node,
                bundle.packages,
                primitives,
                contracts,
                current.behavior,
              )"
              :key="p.name"
              class="wf-help"
            >
              {{ p.name }} · {{ typeLabel(p) }}<br /><code>{{
                node.inputs[p.name]?.literal
                  ? valueText(node.inputs[p.name]?.literal)
                  : JSON.stringify(node.inputs[p.name])
              }}</code>
            </p>
          </section>
          <section>
            <h4>Outputs</h4>
            <p
              v-for="p in nodeOutputs(
                node,
                current.behavior,
                bundle.packages,
                primitives,
                contracts,
              )"
              :key="p.port"
              class="wf-help"
            >
              {{ p.port }} · {{ typeLabel(p) }}
            </p>
          </section>
        </div>
      </div></template
    >
    <p v-else class="wf-warning">
      The selected exact dependency cannot be resolved.
    </p>
  </section>
</template>
<style scoped>
.wf-inspect {
  min-width: 0;
}
.wf-breadcrumb {
  display: flex;
  gap: 3px;
  flex-wrap: wrap;
}
.wf-inspect code {
  overflow-wrap: anywhere;
}
</style>
