<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type {
  WorkflowDefinition,
  WorkflowResult,
  WorkflowScenario,
} from "../workflowTypes";
import { workflowTokenVisits } from "../workflowPresentation";
import WorkflowStages from "./WorkflowStages.vue";
import { stagePath, stageBoundary } from "../workflowStages";
import WorkflowCanvas from "./WorkflowCanvas.vue";
const props = defineProps<{
  workflow: WorkflowDefinition;
  result: WorkflowResult;
  scenario?: WorkflowScenario;
  index?: number;
}>();
const emit = defineEmits<{ "update:index": [index: number] }>();
const localIndex = ref(0),
  followLatest = ref(true);
const visits = computed(() =>
  workflowTokenVisits(props.workflow, props.result, props.scenario),
);
const cursor = computed(() =>
  Math.min(
    Math.max(0, props.index ?? localIndex.value),
    Math.max(0, visits.value.length - 1),
  ),
);
const visit = computed(() => visits.value[cursor.value]);
const stage = ref("");
const followToken = ref(true);
const fullMap = ref(false);
watch([visit, followToken], () => { if (followToken.value) stage.value = stagePath(props.workflow.nodes.find(n => n.id === visit.value?.nodeId)?.stage); }, { immediate: true });
const name = (id: string) =>
  props.workflow.nodes.find((n) => n.id === id)?.name || id;
const visitedIds = computed(() =>
  visits.value.slice(0, cursor.value).map((v) => v.nodeId),
);
const travelledRoutes = computed(() =>
  visits.value
    .slice(0, cursor.value)
    .flatMap((v) => (v.route ? [v.route] : [])),
);
function go(index: number) {
  localIndex.value = Math.max(0, Math.min(visits.value.length - 1, index));
  followLatest.value = localIndex.value === visits.value.length - 1;
  emit("update:index", localIndex.value);
}
watch(
  () => props.result,
  () => {
    if (props.index === undefined && followLatest.value)
      localIndex.value = Math.max(0, visits.value.length - 1);
  },
  { immediate: true },
);
function inspectNode(id: string) {
  if (!visits.value.some(v => v.nodeId === id)) { followToken.value = false; stage.value = stagePath(props.workflow.nodes.find(n => n.id === id)?.stage); fullMap.value = !stage.value; return; }
  const index = visits.value.findIndex(
    (v, i) => i >= cursor.value && v.nodeId === id,
  );
  const fallback = visits.value.map((v) => v.nodeId).lastIndexOf(id);
  if (index >= 0 || fallback >= 0) go(index >= 0 ? index : fallback);
}
</script>
<template>
  <section class="token-journey" aria-label="Workflow token journey">
    <header class="journey-heading">
      <div>
        <h3>Token journey</h3>
        <p>Follow the recorded visits in order. Loops appear once per visit.</p>
      </div>
      <div class="current-token">
        <span>{{
          result.status === "waiting"
            ? "Waiting at"
            : result.status === "completed"
              ? "Finished at"
              : "Stopped at"
        }}</span><strong>{{ name(result.currentNode) }}</strong>
      </div>
    </header>
    <template v-if="visit">
      <div class="journey-controls">
        <button :disabled="cursor === 0" @click="go(cursor - 1)">
          ← Previous
        </button>
        <label
          >Recorded visit<select
            :value="cursor"
            aria-label="Token journey visit"
            @change="go(Number(($event.target as HTMLSelectElement).value))"
          >
            <option v-for="v in visits" :key="v.index" :value="v.index">
              {{ v.index + 1 }}. {{ name(v.nodeId) }} · {{ v.status }}
            </option>
          </select></label
        >
        <button :disabled="cursor >= visits.length - 1" @click="go(cursor + 1)">
          Next →
        </button>
        <button @click="go(visits.length - 1)">Latest position</button>
      </div>
      <div class="journey-step" aria-live="polite">
        <span class="visit-number">{{ cursor + 1 }} / {{ visits.length }}</span>
        <div>
          <strong>{{ name(visit.nodeId) }}</strong>
          <p>
            {{ visit.status
            }}<template v-if="visit.route">
              · {{ visit.route.label }} → {{ name(visit.route.to) }}</template
            >
          </p>
          <small v-if="visit.message">{{ visit.message }}</small>
        </div>
      </div>
      <div class="journey-controls"><label><input type="checkbox" v-model="followToken" /> Follow token</label><label><input type="checkbox" v-model="fullMap" /> Full map</label></div>
      <WorkflowStages :workflow="workflow" :path="fullMap ? '' : stage" :visits="visits" :cursor="cursor" @update:path="stage = $event; followToken = false; fullMap = !$event" @select="inspectNode" />
      <WorkflowCanvas
        :workflow="workflow"
        :visible-ids="!fullMap && stage ? stageBoundary(workflow,stage).ids : undefined"
        :selected="visit.nodeId"
        :active-node="visit.nodeId"
        :active-route="visit.route"
        :visited-ids="visitedIds"
        :travelled-routes="travelledRoutes"
        disabled
        @select="inspectNode"
      />
      <p class="journey-note">
        Blue marks this recorded visit and its departure route; green marks
        earlier travel. Stepping through this view does not execute the workflow
        or submit a response.
      </p>
    </template>
    <p v-else class="journey-note">No execution visits were recorded.</p>
  </section>
</template>
<style scoped>
.token-journey {
  min-width: 0;
  margin: 16px 0;
  border: 1px solid #d5e4eb;
  background: #fff;
  border-radius: 10px;
  overflow: hidden;
}
.journey-heading {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: space-between;
  padding: 16px;
  background: #f3f8fb;
}
h3 {
  margin: 0;
  font-size: 15px;
  color: #294f65;
}
.journey-heading p,
.journey-note {
  font-size: 12px;
  color: #647f8e;
  line-height: 1.6;
}
.journey-heading p {
  margin: 5px 0 0;
}
.current-token {
  border-left: 3px solid #137fca;
  padding: 3px 12px;
  max-width: 100%;
  overflow-wrap: anywhere;
}
.current-token span,
.current-token strong {
  display: block;
  font-size: 12px;
}
.current-token span {
  color: #647f8e;
  margin-bottom: 5px;
}
.journey-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
  padding: 12px 16px;
}
.journey-controls label {
  flex: 1 1 240px;
  min-width: 0;
  color: #617c8e;
  font-size: 11px;
}
.journey-controls select {
  display: block;
  width: 100%;
  min-width: 0;
  margin-top: 5px;
  padding: 8px;
  border: 1px solid #cedfe7;
  border-radius: 6px;
  background: white;
  color: #345b70;
}
.journey-controls button {
  padding: 9px 12px;
  color: #315b70;
  background: #f5f9fb;
  border: 1px solid #d6e4eb;
  border-radius: 6px;
  font-size: 12px;
}
.journey-controls button:disabled {
  opacity: 0.4;
  cursor: default;
}
.journey-step {
  display: flex;
  align-items: start;
  gap: 12px;
  margin: 0 16px 14px;
  padding: 12px;
  border: 1px solid #bcd8ec;
  border-radius: 8px;
  background: #eef7fd;
  color: #246185;
  overflow-wrap: anywhere;
  font-size: 13px;
}
.visit-number {
  white-space: nowrap;
  padding: 4px 7px;
  background: #137fca;
  color: white;
  border-radius: 5px;
  font-size: 11px;
}
.journey-step p {
  margin: 6px 0;
  line-height: 1.6;
  font-size: 12px;
}
.journey-step small {
  font-size: 11px;
  color: #627d8b;
}
.journey-note {
  margin: 12px 16px;
}
.token-journey :deep(.wf-canvas) {
  border-radius: 0;
  border-inline: 0;
}
@media (max-width: 600px) {
  .journey-controls label {
    flex-basis: 100%;
    order: -1;
  }
  .journey-step {
    margin-inline: 10px;
  }
}
</style>
