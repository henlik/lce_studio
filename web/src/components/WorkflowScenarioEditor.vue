<script setup lang="ts">
import { computed, ref } from "vue";
import {
  typedDefault,
  cloneBehavior,
  type BehaviorPort,
  type TypedValue,
} from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import { freshStateName, stateUIKey } from "../stateTypes";
import {
  assertWorkflowNumbers,
  ownerMachine,
  type WorkflowDefinition,
  type WorkflowScenario,
  type WorkflowEvent,
  type WorkflowTaskResult,
} from "../workflowTypes";
import WorkflowValuesEditor from "./WorkflowValuesEditor.vue";
import StateNumberEditor from "./StateNumberEditor.vue";
const props = defineProps<{
  workflow: WorkflowDefinition;
  scenario: WorkflowScenario;
  contracts: DataContractLock[];
  tasks?: WorkflowTaskResult[];
  disabled?: boolean;
}>();
const emit = defineEmits<{ error: [message: string] }>();
const selected = ref<number | null>(null),
  schemaByEvent = ref<Record<number, string>>({});
const taskNodes = computed(() => props.workflow.nodes.filter((n) => n.task));
function schema(e: WorkflowEvent) {
  const remembered = schemaByEvent.value[stateUIKey(e)];
  return (
    taskNodes.value.find((n) => n.id === remembered) ||
    taskNodes.value.find(
      (n) => n.id === props.tasks?.find((t) => t.id === e.taskId)?.nodeId,
    ) ||
    taskNodes.value.find(
      (n) =>
        n.task!.response.length === Object.keys(e.values).length &&
        n.task!.response.every((p) => Object.hasOwn(e.values, p.name)),
    ) ||
    taskNodes.value[0]
  );
}
function ports(e: WorkflowEvent): BehaviorPort[] {
  return e.kind === "signal"
    ? props.workflow.signals.find((s) => s.name === e.signal)?.inputs || []
    : schema(e)?.task?.response || [];
}
function defaults(ports: BehaviorPort[]) {
  return Object.fromEntries(
    ports.map((p) => [
      p.name,
      typedDefault(p.type, p.recordRef, props.contracts),
    ]),
  );
}
function add(kind: WorkflowEvent["kind"]) {
  const first = taskNodes.value[0],
    signal = props.workflow.signals[0];
  const e: WorkflowEvent = {
    id: freshStateName(
      "event",
      props.scenario.events.map((e) => e.id),
    ),
    kind,
    values: defaults(
      kind === "signal" ? signal?.inputs || [] : first?.task?.response || [],
    ),
  };
  if (kind === "response")
    e.taskId = `task${props.scenario.events.filter((e) => e.kind === "response").length + 1}`;
  else e.signal = signal?.name || "";
  props.scenario.events.push(e);
  selected.value = stateUIKey(e);
}
function move(i: number, delta: number) {
  const e = props.scenario.events.splice(i, 1)[0];
  props.scenario.events.splice(i + delta, 0, e);
}
function duplicate(e: WorkflowEvent, i: number) {
  try {
    assertWorkflowNumbers(e);
    const copy = JSON.parse(JSON.stringify(e)) as WorkflowEvent;
    copy.id = freshStateName(
      "event",
      props.scenario.events.map((e) => e.id),
    );
    props.scenario.events.splice(i + 1, 0, copy);
    schemaByEvent.value[stateUIKey(copy)] = schema(e)?.id || "";
    selected.value = stateUIKey(copy);
  } catch (error) {
    emit("error", error instanceof Error ? error.message : String(error));
  }
}
function enableExpected(e: Event) {
  if ((e.target as HTMLInputElement).checked)
    props.scenario.expected = {
      status: "waiting",
      outcome: "",
      activeTaskNode: "",
      variables: {},
      owners: [],
      tasks: 0,
      completedTasks: 0,
      cancelledTasks: 0,
      eventReceipts: 0,
    };
  else delete props.scenario.expected;
}
function addOwnerExpected() {
  const o = props.workflow.owners.find(
    (o) => !props.scenario.expected?.owners.some((e) => e.instanceId === o.id),
  );
  if (!o) return;
  try {
    const state: Record<string, TypedValue> = {};
    for (const slot of ownerMachine(props.workflow, o.id)?.state || []) {
      const binding = o.initial[slot.name];
      const initial = binding?.input
        ? props.scenario.inputs[binding.input]
        : binding?.variable
          ? props.workflow.variables.find((v) => v.name === binding.variable)
              ?.initial
          : binding?.literal || slot.initial;
      state[slot.name] = initial || {
        type: slot.type,
        ...(slot.recordRef ? { recordRef: slot.recordRef } : {}),
      };
    }
    assertWorkflowNumbers(state);
    props.scenario.expected?.owners.push({
      instanceId: o.id,
      state: cloneBehavior(state),
      revision: 0,
      receipts: 0,
      effects: 0,
    });
  } catch (e) {
    emit("error", e instanceof Error ? e.message : String(e));
  }
}
</script>
<template>
  <div>
    <section class="wf-panel">
      <h3>Independent finite experiment</h3>
      <p class="wf-help">
        Define inputs, ordered responses and expected outcomes before running.
        Task IDs are allocated as task1, task2… each time a task activates. A
        signal may cancel a task; retries keep the same event ID and payload.
        Responses to completed or cancelled tasks remain visible in evidence.
      </p>
      <label class="field"
        >Scenario name<input
          v-model="scenario.name"
          :disabled="disabled"
          aria-label="Workflow scenario name" /></label
      ><WorkflowValuesEditor
        :ports="workflow.inputs"
        :values="scenario.inputs"
        :contracts="contracts"
        label="Scenario input"
        :disabled="disabled"
      />
    </section>
    <section class="wf-panel">
      <div class="wf-actions">
        <h3>Ordered synthetic events · {{ scenario.events.length }}</h3>
        <button
          class="button secondary"
          :disabled="disabled || !taskNodes.length"
          @click="add('response')"
        >
          Add task response</button
        ><button
          class="button secondary"
          :disabled="disabled || !workflow.signals.length"
          @click="add('signal')"
        >
          Add signal event
        </button>
      </div>
      <p v-if="!scenario.events.length" class="wf-help">
        Zero events is a valid interactive start: the engine runs automatically
        to the first task.
      </p>
      <article
        v-for="(e, i) in scenario.events"
        :key="stateUIKey(e)"
        class="wf-event"
      >
        <header>
          <button
            class="wf-event-title"
            :aria-expanded="selected === stateUIKey(e)"
            @click="
              selected = selected === stateUIKey(e) ? null : stateUIKey(e)
            "
          >
            <strong>{{ i + 1 }} · {{ e.id }}</strong
            ><span
              >{{ e.kind }} ·
              {{ e.taskId || e.signal || "Choose target" }}</span
            >
          </button>
          <div class="wf-actions">
            <button
              class="button quiet"
              :disabled="disabled || i === 0"
              :aria-label="`Move event ${i + 1} up`"
              @click="move(i, -1)"
            >
              ↑</button
            ><button
              class="button quiet"
              :disabled="disabled || i === scenario.events.length - 1"
              :aria-label="`Move event ${i + 1} down`"
              @click="move(i, 1)"
            >
              ↓</button
            ><button
              class="button quiet"
              :disabled="disabled"
              @click="duplicate(e, i)"
            >
              Duplicate</button
            ><button
              class="button quiet"
              :disabled="disabled"
              @click="scenario.events.splice(i, 1)"
            >
              Remove
            </button>
          </div>
        </header>
        <div v-show="selected === stateUIKey(e)" class="wf-event-body">
          <div class="wf-grid">
            <label class="field"
              >Event ID<input
                v-model="e.id"
                :disabled="disabled"
                :aria-label="`Event ${i + 1} ID`" /></label
            ><label v-if="e.kind === 'response'" class="field"
              >Target task ID<input
                v-model="e.taskId"
                :disabled="disabled"
                :aria-label="`Event ${i + 1} task ID`" /></label
            ><label v-else class="field"
              >Signal<select v-model="e.signal" :disabled="disabled">
                <option value="">Choose signal</option>
                <option
                  v-if="
                    e.signal &&
                    !workflow.signals.some((s) => s.name === e.signal)
                  "
                  :value="e.signal"
                >
                  Unresolved: {{ e.signal }}
                </option>
                <option
                  v-for="s in workflow.signals"
                  :key="s.name"
                  :value="s.name"
                >
                  {{ s.name }}
                </option>
              </select></label
            ><label v-if="e.kind === 'response'" class="field"
              >Response form from step<select
                :value="schema(e)?.id || ''"
                :disabled="disabled"
                @change="
                  schemaByEvent[stateUIKey(e)] = (
                    $event.target as HTMLSelectElement
                  ).value
                "
              >
                <option v-for="n in taskNodes" :key="n.id" :value="n.id">
                  {{ n.name }} · {{ n.id }}
                </option></select
              ><small
                >Editor aid only; execution uses target task ID.</small
              ></label
            >
          </div>
          <WorkflowValuesEditor
            :ports="ports(e)"
            :values="e.values"
            :contracts="contracts"
            :label="`Event ${i + 1} value`"
            :disabled="disabled"
          />
        </div>
      </article>
    </section>
    <section class="wf-panel">
      <label class="wf-check"
        ><input
          type="checkbox"
          :checked="!!scenario.expected"
          :disabled="disabled"
          @change="enableExpected"
        />Verify independently specified final expectations</label
      >
      <p class="wf-help">
        Expectations are never copied from execution. Without them, the saved
        experiment is marked unverified. Workflow variable assertions can select
        specific fields. Owner assertions require every state slot; their
        starting values are only an authoring baseline and must be edited to the
        independently expected final state.
      </p>
      <div v-if="scenario.expected">
        <div class="wf-grid">
          <label class="field"
            >Expected status<select
              v-model="scenario.expected.status"
              :disabled="disabled"
              aria-label="Expected workflow status"
            >
              <option value="waiting">waiting</option>
              <option value="completed">completed</option>
              <option value="failed">failed</option>
            </select></label
          ><label class="field"
            >Expected outcome<input
              v-model="scenario.expected.outcome"
              :disabled="disabled"
              aria-label="Expected workflow outcome" /></label
          ><label class="field"
            >Expected active task node<select
              v-model="scenario.expected.activeTaskNode"
              :disabled="disabled"
            >
              <option value="">No active task</option>
              <option v-for="n in taskNodes" :key="n.id" :value="n.id">
                {{ n.name }} · {{ n.id }}
              </option>
            </select></label
          >
        </div>
        <div class="wf-grid">
          <StateNumberEditor
            v-for="key in [
              'tasks',
              'completedTasks',
              'cancelledTasks',
              'eventReceipts',
            ] as const"
            :key="key"
            v-model="scenario.expected[key]"
            :label="`Expected ${key}`"
            :disabled="disabled"
          />
        </div>
        <h4>Workflow variable expectations</h4>
        <WorkflowValuesEditor
          :ports="workflow.variables"
          :values="scenario.expected.variables"
          :contracts="contracts"
          label="Expected variable"
          :partial="true"
          :disabled="disabled"
        />
        <h4>State owner expectations</h4>
        <article
          v-for="(o, i) in scenario.expected.owners"
          :key="stateUIKey(o)"
          class="wf-card"
        >
          <label class="field"
            >Owner<select v-model="o.instanceId" :disabled="disabled">
              <option
                v-if="
                  !workflow.owners.some((owner) => owner.id === o.instanceId)
                "
                :value="o.instanceId"
              >
                Unresolved: {{ o.instanceId }}
              </option>
              <option
                v-for="owner in workflow.owners"
                :key="owner.id"
                :value="owner.id"
              >
                {{ owner.id }}
              </option>
            </select></label
          >
          <div class="wf-grid">
            <StateNumberEditor
              v-for="key in ['revision', 'receipts', 'effects'] as const"
              :key="key"
              v-model="o[key]"
              :label="`Expected ${o.instanceId} ${key}`"
              :disabled="disabled"
            />
          </div>
          <WorkflowValuesEditor
            :ports="ownerMachine(workflow, o.instanceId)?.state || []"
            :values="o.state"
            :contracts="contracts"
            label="Expected owner state"
            :disabled="disabled"
          /><button
            class="button quiet"
            :disabled="disabled"
            @click="scenario.expected.owners.splice(i, 1)"
          >
            Remove owner assertion
          </button>
        </article>
        <button
          class="button secondary"
          :disabled="
            disabled ||
            scenario.expected.owners.length >= workflow.owners.length
          "
          @click="addOwnerExpected"
        >
          Add owner expectation
        </button>
      </div>
    </section>
  </div>
</template>
<style scoped>
.wf-event {
  border: 1px solid #dce6eb;
  border-radius: 7px;
  margin: 10px 0;
  background: #fbfdfd;
}
.wf-event header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  flex-wrap: wrap;
}
.wf-event-title {
  text-align: left;
  background: none;
  color: #456b7e;
  flex: 1;
  padding: 7px 0;
}
.wf-event-title strong,
.wf-event-title span {
  display: block;
}
.wf-event-title span {
  font-size: 11px;
  margin-top: 5px;
  color: #7c94a2;
}
.wf-event-body {
  padding: 15px;
  border-top: 1px solid #e1e9ed;
}
.wf-event .wf-actions {
  margin: 0;
}
.wf-check {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 13px;
  color: #45697c;
}
.wf-check input {
  width: 16px;
  height: 16px;
}
</style>
