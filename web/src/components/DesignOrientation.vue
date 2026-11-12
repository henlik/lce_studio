<script setup lang="ts">
import { computed, ref } from "vue";
import { ChevronDown, Layers3 } from "lucide-vue-next";

const props = defineProps<{ workspace: string }>();
const open = ref(false);
const flow = ref<"top-down" | "bottom-up">("top-down");
const contexts: Record<string, { kind: string; detail: string }> = {
  plan: { kind: "System requirements", detail: "Define the boundary, assumptions and measurable outcomes." },
  design: { kind: "Application / system model", detail: "Assemble a processing topology for timed simulation." },
  deployment: { kind: "Deployment view", detail: "Place application instances on computing resources." },
  results: { kind: "Experiment evidence", detail: "Inspect the behavior of an exact recorded design." },
  components: { kind: "Reusable pure component", detail: "Compose primitive operations or exact versions of other components." },
  data: { kind: "Reusable data contract", detail: "Define information shared by components at any level." },
  state: { kind: "Stateful component model", detail: "Give an owner state and commands backed by pure decisions." },
  messages: { kind: "Component collaboration model", detail: "Connect independent owners through typed message routes." },
  workflows: { kind: "Reusable workflow component", detail: "Connect tasks, exact decisions and owned commands, then follow their execution." },
};
const context = computed(() => contexts[props.workspace] || contexts.design);
const levels = [
  { name: "Primitive", boundary: "One operation defined by the engine", example: "Equality, addition, field access" },
  { name: "Component", boundary: "A reusable contract with an internal design", example: "Access rule, reservation owner" },
  { name: "Application", boundary: "An independently runnable application", example: "Pharmacy or school management" },
  { name: "System", boundary: "Applications working toward a shared purpose", example: "A hospital information system" },
  { name: "System of systems", boundary: "Independent systems collaborating", example: "A national data exchange" },
];
</script>

<template>
  <section class="design-orientation" aria-label="Current design context">
    <div class="orientation-line">
      <Layers3 :size="16" aria-hidden="true" />
      <div class="orientation-context">
        <strong>{{ context.kind }}</strong>
        <span>{{ context.detail }}</span>
      </div>
      <button
        class="orientation-toggle"
        :aria-expanded="open"
        aria-controls="design-hierarchy-guide"
        @click="open = !open"
      >
        Design levels &amp; flow
        <ChevronDown :size="15" :class="{ rotated: open }" aria-hidden="true" />
      </button>
    </div>
    <div v-if="open" id="design-hierarchy-guide" class="orientation-guide">
      <h2>Choose the boundary you are designing</h2>
      <p>
        A component can contain components at several depths. An application or
        system can also become a reusable building block when it exposes a
        contract. The levels describe responsibility, not a fixed number of nests.
      </p>
      <div class="orientation-table-wrap">
        <table>
          <thead><tr><th>Design level</th><th>Boundary</th><th>Example</th></tr></thead>
          <tbody>
            <tr v-for="level in levels" :key="level.name">
              <th scope="row">{{ level.name }}</th><td>{{ level.boundary }}</td><td>{{ level.example }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="orientation-note">
        Combining primitives produces a composite component. Creating a new
        engine primitive requires implementing its semantics; drawing a box does
        not create one. Data, State, Messages and Deployment are design views,
        not additional levels.
      </p>
      <div class="orientation-flow" role="group" aria-label="Design flow guidance">
        <button :aria-pressed="flow === 'top-down'" @click="flow = 'top-down'">Top-down flow</button>
        <button :aria-pressed="flow === 'bottom-up'" @click="flow = 'bottom-up'">Bottom-up flow</button>
      </div>
      <ol v-if="flow === 'top-down'" class="orientation-steps">
        <li><strong>Define the outside.</strong> Describe purpose, inputs, outputs, ownership and acceptance criteria.</li>
        <li><strong>Decompose the inside.</strong> Identify smaller responsibilities and their contracts; reuse existing definitions where they fit.</li>
        <li><strong>Verify and assemble.</strong> Test each part, then test the composed behavior against the original boundary.</li>
      </ol>
      <ol v-else class="orientation-steps">
        <li><strong>Start with a small behavior.</strong> Select primitives and data types, or an existing tested component.</li>
        <li><strong>Expose a reusable contract.</strong> Compose the parts, define public ports and verify independent expectations.</li>
        <li><strong>Use it at a larger boundary.</strong> Pin its exact version inside another component or supported owner model.</li>
      </ol>
      <p class="orientation-status">
        Workflows assemble explicit human or adapter tasks, pure decisions and
        owned commands behind a reusable contract. Their exact dependencies can
        be inspected down to primitive operations. Data, state, message and timed
        deployment experiments retain separate artifacts; a common system tree,
        generated applications and live service adapters remain future work.
      </p>
    </div>
  </section>
</template>

<style scoped>
.design-orientation { margin: 0 28px 16px; border: 1px solid var(--line); border-radius: 8px; background: #f8fafc; }
.orientation-line { display: flex; align-items: center; gap: 12px; padding: 11px 14px; color: #587184; }
.orientation-context { display: flex; align-items: baseline; gap: 12px; flex: 1; min-width: 0; flex-wrap: wrap; }
.orientation-context strong { color: #294659; font-size: 12px; }
.orientation-context span { font-size: 12px; line-height: 1.5; }
.orientation-toggle { display: inline-flex; align-items: center; justify-content: center; gap: 8px; padding: 7px 9px; border-radius: 5px; background: #eaf2f5; color: #355b68; font-size: 12px; flex-shrink: 0; }
.orientation-toggle:hover { background: #dfecee; }
.orientation-toggle svg { transition: transform .16s; }
.orientation-toggle .rotated { transform: rotate(180deg); }
.orientation-guide { padding: 18px; border-top: 1px solid var(--line); }
.orientation-guide h2 { font-size: 16px; margin-bottom: 8px; }
.orientation-guide p { line-height: 1.7; max-width: 100ch; color: #50677a; }
.orientation-table-wrap { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; min-width: 520px; font-size: 12px; }
th, td { padding: 11px 12px; text-align: left; border-bottom: 1px solid var(--line); line-height: 1.5; }
thead th { color: #657a8b; background: #edf3f6; }
tbody th { color: #2c5260; width: 18%; }
.orientation-note { margin: 16px 0; }
.orientation-flow { display: flex; gap: 6px; }
.orientation-flow button { border-radius: 5px; padding: 9px 14px; color: #516b7c; background: #edf2f5; }
.orientation-flow button[aria-pressed="true"] { color: #096f64; background: #dcefe9; }
.orientation-steps { padding-left: 20px; color: #4d6576; line-height: 1.8; }
.orientation-steps li { padding: 4px 0; }
.orientation-status { margin: 16px 0 0; padding-top: 14px; border-top: 1px solid var(--line); font-size: 12px; }
@media (max-width: 700px) {
  .design-orientation { margin: 0 14px 12px; }
  .orientation-line { flex-wrap: wrap; align-items: flex-start; }
  .orientation-context { flex-basis: calc(100% - 30px); gap: 4px; }
  .orientation-toggle { margin-left: 28px; }
  .orientation-guide { padding: 14px; }
}
</style>
