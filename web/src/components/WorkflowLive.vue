<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { typedDefault, type TypedValue } from "../behaviorTypes";
import {
  workflowContracts,
  workflowStamp,
  type WorkflowDefinition,
  type WorkflowExample,
  type WorkflowResult,
  type WorkflowEvent,
} from "../workflowTypes";
import { freshStateName } from "../stateTypes";
import WorkflowValuesEditor from "./WorkflowValuesEditor.vue";
import StateValueMap from "./StateValueMap.vue";
import WorkflowJourney from "./WorkflowJourney.vue";
const props = defineProps<{
  workflow: WorkflowDefinition;
  inputs: Record<string, TypedValue>;
  session: (WorkflowExample & { result: WorkflowResult }) | null;
  generation: number;
  stale: boolean;
  disabled?: boolean;
  form?: {
    context: string;
    response: Record<string, TypedValue>;
    responseBaseline: string;
    signalDrafts: Record<
      string,
      { values: Record<string, TypedValue>; baseline: string }
    >;
    signalName: string;
  } | null;
}>();
const emit = defineEmits<{
  start: [];
  "form-change": [form: NonNullable<typeof props.form>];
  event: [event: WorkflowEvent];
  record: [];
  "copy-scenario": [];
  "pending-change": [pending: boolean];
}>();
const definition = computed(
    () => props.session?.workflow.workflow || props.workflow,
  ),
  contracts = computed(() => workflowContracts(definition.value)),
  task = computed(() => props.session?.result.task),
  response = ref<Record<string, TypedValue>>({}),
  responseBaseline = ref(""),
  signalDrafts = ref<
    Record<string, { values: Record<string, TypedValue>; baseline: string }>
  >({}),
  signalName = ref("");
const formContext = computed(
  () =>
    `${props.generation}:${props.session?.workflow.digest || ""}:${task.value?.id || ""}`,
);
const signal = computed(() =>
  definition.value.signals.find((s) => s.name === signalName.value),
);
watch(
  () => [props.generation, task.value?.id] as const,
  () => {
    response.value = Object.fromEntries(
      (task.value?.response || []).map((p) => [
        p.name,
        typedDefault(p.type, p.recordRef, contracts.value),
      ]),
    );
    responseBaseline.value = workflowStamp(response.value);
    if (props.form?.context === formContext.value) {
      response.value = props.form.response;
      responseBaseline.value = props.form.responseBaseline;
    }
  },
  { immediate: true },
);
let signalGeneration = -1;
watch(
  () => [props.generation, definition.value.signals] as const,
  () => {
    if (signalGeneration !== props.generation) {
      signalGeneration = props.generation;
      signalDrafts.value = {};
      signalName.value = "";
      if (props.form?.context === formContext.value) {
        signalDrafts.value = props.form.signalDrafts;
        signalName.value = props.form.signalName;
      }
    }
    for (const signal of definition.value.signals) {
      if (signalDrafts.value[signal.name]) continue;
      const values = Object.fromEntries(
        signal.inputs.map((p) => [
          p.name,
          typedDefault(p.type, p.recordRef, contracts.value),
        ]),
      );
      signalDrafts.value[signal.name] = {
        values,
        baseline: workflowStamp(values),
      };
    }
  },
  { immediate: true, deep: true },
);
watch(
  () => props.session?.scenario.events.at(-1),
  (event) => {
    if (event?.kind === "signal" && event.signal) {
      const draft = signalDrafts.value[event.signal];
      if (draft && workflowStamp(event.values) === workflowStamp(draft.values))
        draft.baseline = workflowStamp(draft.values);
    }
  },
);
watch(
  () =>
    workflowStamp(response.value) !== responseBaseline.value ||
    Object.values(signalDrafts.value).some(
      (d) => workflowStamp(d.values) !== d.baseline,
    ),
  (pending) => emit("pending-change", pending),
  { immediate: true },
);
watch(
  [response, responseBaseline, signalDrafts, signalName],
  () =>
    emit("form-change", {
      context: formContext.value,
      response: response.value,
      responseBaseline: responseBaseline.value,
      signalDrafts: signalDrafts.value,
      signalName: signalName.value,
    }),
  { deep: true, flush: "post" },
);
function sendResponse() {
  if (!task.value) return;
  emit("event", {
    id: freshStateName(
      "response",
      props.session?.scenario.events.map((e) => e.id) || [],
    ),
    kind: "response",
    taskId: task.value.id,
    values: response.value,
  });
}
function sendSignal() {
  if (!signal.value) return;
  emit("event", {
    id: freshStateName(
      "signal",
      props.session?.scenario.events.map((e) => e.id) || [],
    ),
    kind: "signal",
    signal: signal.value.name,
    values: signalDrafts.value[signal.value.name]?.values || {},
  });
}
const lastEvent = computed(() => props.session?.scenario.events.at(-1));
</script>
<template>
  <div>
    <section class="wf-panel">
      <div class="wf-actions">
        <h3>Interactive synthetic experiment</h3>
        <button
          class="button primary"
          :disabled="disabled"
          @click="emit('start')"
        >
          {{
            session
              ? "Restart from current workflow"
              : "Start interactive workflow"
          }}
        </button>
      </div>
      <p class="wf-help">
        The Go engine automatically executes routes, exact pure decisions and
        owned commands until it needs a human or adapter response. Each Continue
        replays a bounded, immutable experiment. These forms do not contact
        external systems, provide clinical care, or create deployed production
        sessions.
      </p>
      <details :open="!session">
        <summary>Root inputs for a new interactive experiment</summary>
        <WorkflowValuesEditor
          :ports="workflow.inputs"
          :values="inputs"
          :contracts="workflowContracts(workflow)"
          label="Interactive start"
          :disabled="disabled"
        />
      </details>
    </section>
    <template v-if="session"
      ><p v-if="stale" class="wf-warning">
        The current workflow draft differs from this interactive experiment.
        Continuation uses its original exact workflow. Restart explicitly to use
        your newer draft.
      </p>
      <WorkflowJourney
        :workflow="session.workflow.workflow"
        :result="session.result"
        :scenario="session.scenario"
      />
      <section class="wf-panel">
        <div class="wf-actions">
          <strong
            >{{ session.result.status.toUpperCase()
            }}<small>{{
              session.result.outcome || session.result.currentNode
            }}</small></strong
          ><span
            >{{ session.result.tasks.length }} tasks ·
            {{ session.scenario.events.length }} submitted events</span
          ><button
            class="button secondary"
            :disabled="disabled"
            @click="emit('record')"
          >
            Save interactive evidence</button
          ><button
            class="button secondary"
            :disabled="disabled"
            @click="emit('copy-scenario')"
          >
            Copy events to scenario draft
          </button>
        </div>
        <article v-if="task" class="wf-live-task">
          <div class="wf-task-tag">
            {{
              task.kind === "human"
                ? "Human task"
                : "Synthetic adapter boundary"
            }}
            · {{ task.id }} · {{ task.nodeId }}
          </div>
          <h3>{{ task.title }}</h3>
          <p class="wf-instructions">{{ task.instructions }}</p>
          <details>
            <summary>Request values</summary>
            <StateValueMap :values="task.request" />
          </details>
          <p v-if="task.kind === 'adapter'" class="wf-warning">
            Supply an explicit synthetic success, unavailable or failure outcome
            using this response contract. No real adapter was invoked. For
            example, a failed lookup must be entered as a failed lookup, not a
            successful zero-match result.
          </p>
          <WorkflowValuesEditor
            :key="`${generation}:${task.id}`"
            :ports="task.response"
            :values="response"
            :contracts="contracts"
            :label="`Task ${task.id} response`"
            :disabled="disabled"
          /><button
            class="button primary"
            :disabled="disabled"
            @click="sendResponse"
          >
            Submit response & continue
          </button>
        </article>
        <div v-else class="wf-card">
          <h3>
            {{
              session.result.status === "completed"
                ? "Workflow reached its terminal outcome"
                : "Workflow stopped"
            }}
          </h3>
          <StateValueMap :values="session.result.outputs" />
          <p class="wf-help">
            This interactive result has no independent assertions. Record it for
            inspection, then author separate expectations in Scenario.
          </p>
        </div>
        <details
          v-if="
            session.result.status === 'waiting' && definition.signals.length
          "
          class="wf-signal"
          open
        >
          <summary>Interrupt / emergency signal</summary>
          <p class="wf-help">
            Submitting a permitted signal cancels the pending task and follows
            the configured interrupt route. Required provenance is entered
            explicitly below.
          </p>
          <label class="field"
            >Signal action<select
              v-model="signalName"
              aria-label="Interactive workflow signal"
              :disabled="disabled"
            >
              <option value="">Choose an interrupt</option>
              <option
                v-for="s in definition.signals"
                :key="s.name"
                :value="s.name"
              >
                {{ s.name }}
              </option>
            </select></label
          >
          <section
            v-for="signal in definition.signals"
            v-show="signalName === signal.name"
            :key="signal.name"
          >
            <p class="wf-help">
              Guard: {{ signal.when || "none" }} · target: {{ signal.target }}
            </p>
            <WorkflowValuesEditor
              :ports="signal.inputs"
              :values="signalDrafts[signal.name]?.values || {}"
              :contracts="contracts"
              :label="`Signal ${signal.name}`"
              :disabled="disabled"
            /><button
              class="button danger"
              :disabled="disabled"
              @click="sendSignal"
            >
              Send {{ signal.name }}
            </button>
          </section>
        </details>
        <details v-if="lastEvent">
          <summary>Retry the last exact event</summary>
          <p class="wf-help">
            Append the same event ID and payload to test receipt replay. A retry
            is not a new business action.
          </p>
          <button
            class="button secondary"
            :disabled="disabled"
            @click="emit('event', lastEvent)"
          >
            Replay last event unchanged
          </button>
        </details>
      </section>
      <details class="wf-panel">
        <summary>Current workflow variables and event receipts</summary>
        <StateValueMap :values="session.result.variables" />
        <article
          v-for="(e, i) in session.result.events"
          :key="i"
          class="wf-card"
        >
          <strong>{{ e.id }} · {{ e.status }}</strong>
          <p class="wf-help">
            {{ e.message }} · {{ e.beforeTaskId || "start" }} →
            {{ e.afterTaskId || session.result.outcome }}
          </p>
        </article>
      </details></template
    >
  </div>
</template>
<style scoped>
.wf-live-task {
  border: 1px solid #b8dacc;
  background: #f5fcf8;
  border-radius: 9px;
  padding: 22px;
  max-width: 860px;
  margin: 18px auto;
}
.wf-task-tag {
  font-size: 11px;
  text-transform: uppercase;
  color: #318673;
  font-weight: 600;
  margin-bottom: 10px;
}
.wf-instructions {
  font-size: 14px;
  line-height: 1.75;
  color: #476b7b;
  white-space: pre-line;
}
.wf-live-task > .button {
  margin-top: 18px;
}
.wf-live-task details {
  margin: 16px 0;
}
.wf-signal {
  margin-top: 20px;
  border: 1px solid #ecd8ba;
  background: #fffaf2;
  padding: 18px;
  border-radius: 8px;
}
.wf-signal > summary {
  color: #956841;
}
.wf-signal .button {
  margin-top: 16px;
}
</style>
