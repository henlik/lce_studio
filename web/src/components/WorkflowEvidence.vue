<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { downloadWorkflow, type WorkflowRun } from "../workflowTypes";
import StateValueMap from "./StateValueMap.vue";
const props = defineProps<{
  record: WorkflowRun;
  stale: boolean;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "load-snapshot": [] }>();
const selected = ref(0),
  trace = ref(0);
watch(
  () => props.record.id,
  () => {
    selected.value = 0;
    trace.value = 0;
  },
);
watch(selected, () => (trace.value = 0));
const step = computed(() => props.record.result.steps[selected.value]),
  evaluation = computed(
    () => step.value?.decision || step.value?.state?.decision,
  ),
  event = computed(() => evaluation.value?.events[trace.value]);
</script>
<template>
  <div>
    <section class="wf-panel">
      <div class="wf-actions">
        <strong
          >{{
            record.result.verified
              ? record.result.passed
                ? "Independent workflow expectations passed"
                : "Independent workflow expectations failed"
              : "Unverified workflow experiment"
          }}<small
            >{{ record.result.status }} ·
            {{ record.result.outcome || record.result.currentNode }}</small
          ></strong
        ><button
          class="button secondary"
          :disabled="disabled"
          @click="emit('load-snapshot')"
        >
          Load recorded workflow snapshot</button
        ><button
          class="button secondary"
          @click="
            downloadWorkflow(
              record,
              `${record.workflow.workflow.id}-${record.id}.evidence.json`,
            )
          "
        >
          Export immutable evidence
        </button>
      </div>
      <p v-if="stale" class="wf-warning">
        This is earlier evidence. The current workflow, scenario or pending JSON
        differs; edits have not changed this recorded result.
      </p>
      <p v-if="!record.result.verified" class="wf-warning">
        No independent expectations were supplied. Reaching a terminal outcome
        is not a verification verdict.
      </p>
      <p class="wf-help">
        {{ new Date(record.createdAt).toLocaleString() }} · engine
        {{ record.result.engineVersion }} · {{ record.result.operations }} pure
        operations · {{ record.result.invocations }} component invocations
      </p>
      <ul v-if="record.result.issues.length" class="wf-warning">
        <li v-for="(issue, i) in record.result.issues" :key="i">
          <code>{{ issue.path }}</code> {{ issue.message }}
        </li>
      </ul>
      <details>
        <summary>Immutable identities and submitted expectations</summary>
        <code>Workflow SHA-256 {{ record.result.workflowHash }}</code
        ><br /><code>Scenario SHA-256 {{ record.result.scenarioHash }}</code>
        <pre>{{
          JSON.stringify(record.scenario.expected || null, null, 2)
        }}</pre>
      </details>
    </section>
    <section class="wf-panel">
      <h3>Final workflow state</h3>
      <div class="wf-grid">
        <section>
          <h4>Outputs</h4>
          <StateValueMap :values="record.result.outputs" />
        </section>
        <section>
          <h4>Variables</h4>
          <StateValueMap :values="record.result.variables" />
        </section>
      </div>
    </section>
    <section class="wf-panel">
      <h3>Task lifecycle · {{ record.result.tasks.length }}</h3>
      <p class="wf-help">
        A task can be pending, completed or cancelled. Adapter completion
        records a supplied synthetic response, not external delivery. Human
        acceptance is distinct from a request being created.
      </p>
      <div class="wf-task-list">
        <details v-for="t in record.result.tasks" :key="t.id" class="wf-card">
          <summary>
            <strong>{{ t.id }} · {{ t.title }}</strong
            ><span>{{ t.kind }} · {{ t.status }}</span>
          </summary>
          <p class="wf-help">
            Step {{ t.nodeId }} · completed by event
            {{ t.completedBy || "none" }}
          </p>
          <h4>Request</h4>
          <StateValueMap :values="t.request" />
          <h4>Response</h4>
          <StateValueMap :values="t.values" />
        </details>
      </div>
    </section>
    <section class="wf-panel">
      <h3>Automatic execution trace</h3>
      <div class="wf-trace-layout">
        <div class="wf-trace-list">
          <button
            v-for="(s, i) in record.result.steps"
            :key="i"
            :class="{ active: selected === i }"
            @click="selected = i"
          >
            <strong>{{ i + 1 }} · {{ s.nodeId }}</strong
            ><span
              >{{ s.kind }} · {{ s.status }} → {{ s.next || "pause/end" }}</span
            >
          </button>
        </div>
        <article v-if="step" class="wf-card">
          <h4>{{ step.nodeId }} · {{ step.status }}</h4>
          <p class="wf-help">{{ step.message }}</p>
          <StateValueMap :values="step.assignments" /><template
            v-if="step.state"
            ><h4>Owned command result</h4>
            <p class="wf-help">
              {{ step.state.status }} · revision
              {{ step.state.beforeRevision }} → {{ step.state.afterRevision }} ·
              {{
                step.state.receipt
                  ? step.state.receipt.accepted
                    ? "business accepted"
                    : "business denied"
                  : "no receipt"
              }}
            </p>
            <StateValueMap :values="step.state.after" /></template
          ><template v-if="evaluation"
            ><h4>Pure component / primitive evidence</h4>
            <p class="wf-help">
              {{ evaluation.operations }} operations ·
              {{ evaluation.invocations }} invocations
            </p>
            <label class="field"
              >Trace event<select v-model="trace">
                <option v-for="(e, i) in evaluation.events" :key="i" :value="i">
                  {{ e.sequence }} · {{ e.kind }} · {{ e.path }}
                  {{ e.op || "" }}
                </option>
              </select></label
            ><template v-if="event"
              ><p class="wf-help">
                {{ event.path }} · {{ event.op || event.component?.packageId }}
              </p>
              <div class="wf-grid">
                <section>
                  <h4>Actual inputs</h4>
                  <StateValueMap :values="event.inputs || {}" />
                </section>
                <section>
                  <h4>Actual outputs</h4>
                  <StateValueMap :values="event.outputs || {}" />
                </section></div></template
          ></template>
        </article>
      </div>
    </section>
    <section class="wf-panel">
      <h3>Event processing and receipts</h3>
      <article v-for="(e, i) in record.result.events" :key="i" class="wf-card">
        <strong>{{ i + 1 }} · {{ e.id }} · {{ e.status }}</strong>
        <p class="wf-help">
          {{ e.message }} · {{ e.beforeTaskId || "none" }} →
          {{ e.afterTaskId || "none" }}
        </p>
        <details v-if="e.receipt">
          <summary>Historical event receipt</summary>
          <pre>{{ JSON.stringify(e.receipt, null, 2) }}</pre>
        </details>
      </article>
      <p v-if="!record.result.events.length" class="wf-help">
        No external event was supplied; automatic initialization stopped at its
        first task or outcome.
      </p>
    </section>
    <section class="wf-panel">
      <h3>Owned state, receipts and simulated effects</h3>
      <details
        v-for="owner in record.result.owners"
        :key="owner.id"
        class="wf-card"
      >
        <summary>
          {{ owner.id }} · revision {{ owner.revision }} ·
          {{ owner.receipts.length }} receipts ·
          {{ owner.effects.length }} effect intentions
        </summary>
        <StateValueMap :values="owner.state" />
        <h4>Historical command receipts</h4>
        <pre>{{ JSON.stringify(owner.receipts, null, 2) }}</pre>
        <h4>Committed effect intentions</h4>
        <pre>{{ JSON.stringify(owner.effects, null, 2) }}</pre>
      </details>
      <p v-if="!record.result.owners.length" class="wf-help">
        This workflow has no state machine owners.
      </p>
    </section>
  </div>
</template>
<style scoped>
.wf-task-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(270px, 1fr));
  gap: 10px;
}
.wf-task-list .wf-card {
  margin: 0;
}
.wf-task-list summary span {
  display: block;
  font-size: 11px;
  margin-top: 7px;
  color: #76929e;
}
.wf-trace-layout {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: 16px;
}
.wf-trace-list {
  max-height: 560px;
  overflow: auto;
}
.wf-trace-list button {
  display: block;
  padding: 11px;
  text-align: left;
  width: 100%;
  background: #f5f9fb;
  color: #496c7e;
  border-radius: 5px;
  margin-bottom: 6px;
}
.wf-trace-list button.active {
  background: #e3f3ec;
  color: #277d6b;
}
.wf-trace-list span {
  display: block;
  font-size: 10px;
  margin-top: 6px;
}
.wf-trace-layout > .wf-card {
  margin: 0;
}
pre {
  max-height: 360px;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font-size: 11px;
}
@media (max-width: 800px) {
  .wf-trace-layout {
    grid-template-columns: 1fr;
  }
  .wf-trace-list {
    max-height: 230px;
  }
}
</style>
