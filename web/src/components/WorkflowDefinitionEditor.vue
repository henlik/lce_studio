<script setup lang="ts">
import { nodeColors } from "../graphPresentation";
import { focusField } from "../validationFocus";
import type { ValidationIssue } from "../types";
import { computed, ref, watch } from "vue";
import {
  typedDefault,
  referenceKey,
  typeLabel,
  lockKey,
  type InstalledComposite,
  type PrimitiveSpec,
  type ValueShape,
  type ValueType,
} from "../behaviorTypes";
import {
  dataKey,
  type DataContractLock,
  type DataContractRecord,
} from "../dataTypes";
import {
  freshStateName,
  stateUIKey,
  stateKey,
  type InstalledStateMachine,
} from "../stateTypes";
import {
  bundleRoot,
  ownerMachine,
  workflowDecision,
  workflowCommand,
  downloadWorkflow,
  type WorkflowDefinition,
  type WorkflowNode,
  type WorkflowVariable,
} from "../workflowTypes";
import StatePortEditor from "./StatePortEditor.vue";
import StateTypeSelect from "./StateTypeSelect.vue";
import TypedValueEditor from "./TypedValueEditor.vue";
import DeliveryPositionEditor from "./DeliveryPositionEditor.vue";
import WorkflowCanvas from "./WorkflowCanvas.vue";
import WorkflowBindingsEditor from "./WorkflowBindingsEditor.vue";
import WorkflowAssignEditor from "./WorkflowAssignEditor.vue";
import WorkflowInspect from "./WorkflowInspect.vue";
const props = defineProps<{
  workflow: WorkflowDefinition;
  contracts: DataContractLock[];
  components: InstalledComposite[];
  machines: InstalledStateMachine[];
  dataCatalog: DataContractRecord[];
  primitives: PrimitiveSpec[];
  disabled?: boolean;
  issues?: ValidationIssue[];
}>();
const emit = defineEmits<{
  "load-decision": [key: string];
  "load-machine": [key: string];
  "embed-data": [key: string];
  "import-dependency": [kind: "decision" | "machine"];
}>();
const selected = ref(""),
  treeOpen = ref(true),
  inspectorOpen = ref(true),
  decisionChoice = ref(""),
  machineChoice = ref(""),
  dataChoice = ref(""),
  inspectBundle = ref<number | null>(null),
  inspectCommand = ref<{ owner: string; command: string } | null>(null);
watch(
  () => props.workflow,
  () => {
    selected.value = "";
    inspectBundle.value = null;
    inspectCommand.value = null;
  },
);
const booleans = computed(() =>
  props.workflow.variables.filter((v) => v.type === "boolean"),
);
const inspection = computed(() =>
  inspectBundle.value !== null
    ? props.workflow.decisions[inspectBundle.value]
    : inspectCommand.value
      ? ownerMachine(props.workflow, inspectCommand.value.owner)?.commands.find(
          (c) => c.name === inspectCommand.value!.command,
        )?.decision
      : undefined,
);
const editorRoot = ref<HTMLElement | null>(null);
const issueNode = (path: string) =>
  Number(path.match(/(?:^|\.)nodes\[(\d+)\]/)?.[1] ?? -1);
const invalidIds = computed(() =>
  (props.issues || [])
    .map((i) => props.workflow.nodes[issueNode(i.path)]?.id)
    .filter((id): id is string => !!id),
);
function focusIssue(issue: ValidationIssue) {
  const index = issueNode(issue.path);
  inspectorOpen.value = true;
  selected.value = index >= 0 ? props.workflow.nodes[index]?.id || "" : "";
  focusField(
    editorRoot.value,
    index >= 0
      ? `[aria-label="Step ${selected.value} name"]`
      : '[aria-label="Workflow name"]',
  );
}
defineExpose({ focusIssue });
function reorderNode(node: WorkflowNode, delta: number) {
  const nodes = props.workflow.nodes,
    i = nodes.indexOf(node),
    j = i + delta;
  if (props.disabled || j < 0 || j >= nodes.length) return;
  [nodes[i].position, nodes[j].position] = [
    nodes[j].position,
    nodes[i].position,
  ];
  nodes.splice(i, 1);
  nodes.splice(j, 0, node);
}
function addNode(kind: WorkflowNode["kind"]) {
  const id = freshStateName(
      kind,
      props.workflow.nodes.map((n) => n.id),
    ),
    i = props.workflow.nodes.length;
  const n: WorkflowNode = {
    id,
    name: kind === "task" ? "New human task" : `New ${kind}`,
    kind,
    position: { x: (i % 3) * 280, y: Math.floor(i / 3) * 180 },
  };
  if (kind === "task")
    n.task = {
      kind: "human",
      title: "New human task",
      instructions: "Describe the work and the explicit response required.",
      request: {},
      response: [],
      assign: {},
    };
  if (kind === "decision")
    n.decision = {
      component: { packageId: "", version: "", digest: "" },
      inputs: {},
      assign: {},
    };
  if (kind === "command")
    n.command = {
      owner: "",
      command: "",
      inputs: {},
      assign: {},
      acceptedVariable: "",
      errorNext: "",
    };
  if (kind === "end") n.end = { outcome: "completed", outputs: {} };
  props.workflow.nodes.push(n);
  if (!props.workflow.entry) props.workflow.entry = id;
  selected.value = id;
  inspectorOpen.value = true;
}
function renameNode(node: WorkflowNode, event: Event) {
  node.id = (event.target as HTMLInputElement).value;
  selected.value = node.id;
}
function move(id: string, position: { x: number; y: number }) {
  const n = props.workflow.nodes.find((n) => n.id === id);
  if (n && !props.disabled) n.position = position;
}
function setType(v: WorkflowVariable, shape: ValueShape) {
  v.type = shape.type as ValueType;
  if (shape.recordRef) v.recordRef = { ...shape.recordRef };
  else delete v.recordRef;
}
function addVariable() {
  props.workflow.variables.push({
    name: freshStateName(
      "value",
      props.workflow.variables.map((v) => v.name),
    ),
    type: "string",
    initial: typedDefault("string"),
  });
}
function chooseDecision(n: WorkflowNode, e: Event) {
  const b = props.workflow.decisions.find(
    (b) => referenceKey(b.root) === (e.target as HTMLSelectElement).value,
  );
  if (n.decision && b) n.decision.component = { ...b.root };
}
function route(n: WorkflowNode, e: Event) {
  if ((e.target as HTMLSelectElement).value === "branch") {
    delete n.next;
    n.branch = { variable: booleans.value[0]?.name || "", then: "", else: "" };
  } else {
    delete n.branch;
    n.next = "";
  }
}
function addOwner() {
  const m = props.workflow.machines[0];
  props.workflow.owners.push({
    id: freshStateName(
      "owner",
      props.workflow.owners.map((o) => o.id),
    ),
    name: "New state owner",
    machine: m
      ? {
          machineId: m.machine.id,
          version: m.machine.version,
          digest: m.digest,
        }
      : { machineId: "", version: "", digest: "" },
    initial: {},
  });
}
function ownerRef(i: number, e: Event) {
  const m = props.workflow.machines.find(
    (m) => stateKey(m) === (e.target as HTMLSelectElement).value,
  );
  if (m)
    props.workflow.owners[i].machine = {
      machineId: m.machine.id,
      version: m.machine.version,
      digest: m.digest,
    };
}
function addSignal() {
  props.workflow.signals.push({
    name: freshStateName(
      "interrupt",
      props.workflow.signals.map((s) => s.name),
    ),
    inputs: [],
    set: {},
    target: "",
  });
}
function inspectNode(n: WorkflowNode) {
  if (n.decision) {
    inspectCommand.value = null;
    inspectBundle.value = props.workflow.decisions.findIndex(
      (b) => referenceKey(b.root) === referenceKey(n.decision!.component),
    );
  } else if (n.command) {
    inspectBundle.value = null;
    inspectCommand.value = {
      owner: n.command.owner,
      command: n.command.command,
    };
  }
}
</script>
<template>
  <div ref="editorRoot">
    <div class="wf-actions">
      <button
        class="button secondary"
        :aria-expanded="treeOpen"
        @click="treeOpen = !treeOpen"
      >
        {{ treeOpen ? "Hide" : "Show" }} hierarchy</button
      ><button
        class="button secondary"
        :aria-expanded="inspectorOpen"
        @click="inspectorOpen = !inspectorOpen"
      >
        {{ inspectorOpen ? "Hide" : "Show" }} inspector</button
      ><button
        v-for="kind in ['task', 'decision', 'command', 'end'] as const"
        :key="kind"
        class="button secondary"
        :disabled="disabled"
        @click="addNode(kind)"
      >
        Add {{ kind }}
      </button>
    </div>
    <div
      class="wf-designer"
      :class="{ 'wf-no-tree': !treeOpen, 'wf-no-inspector': !inspectorOpen }"
    >
      <aside v-show="treeOpen" class="wf-tree">
        <button :class="{ active: !selected }" @click="selected = ''">
          {{ workflow.name || "Workflow root"
          }}<small>ROOT · {{ workflow.nodes.length }} steps</small></button
        ><button
          v-for="n in workflow.nodes"
          :key="stateUIKey(n)"
          :class="{ active: selected === n.id }"
          @click="
            selected = n.id;
            inspectorOpen = true;
          "
        >
          <span>{{ n.name || n.id }}</span><small
            >{{ n.task?.kind || n.kind }} · {{ n.id }}</small
          >
        </button>
      </aside>
      <WorkflowCanvas
        :workflow="workflow"
        :selected="selected"
        :invalid-ids="invalidIds"
        :disabled="disabled"
        @select="selected = $event"
        @move="move"
      />
      <aside v-show="inspectorOpen" class="wf-inspector">
        <section v-show="!selected">
          <h3>Workflow root</h3>
          <p class="wf-help">
            A reusable boundary that orchestrates explicit tasks, pure decisions
            and owned commands. Start empty, or open a reference example.
          </p>
          <fieldset :disabled="disabled">
            <label class="field"
              >Workflow name<input
                v-model="workflow.name"
                aria-label="Workflow name" /></label
            ><label class="field"
              >Namespaced ID<input
                v-model="workflow.id"
                aria-label="Workflow ID" /></label
            ><label class="field"
              >Version<input
                v-model="workflow.version"
                aria-label="Workflow version" /></label
            ><label class="field"
              >Entry step<select
                v-model="workflow.entry"
                aria-label="Workflow entry"
              >
                <option value="">Choose entry</option>
                <option
                  v-if="
                    workflow.entry &&
                    !workflow.nodes.some((n) => n.id === workflow.entry)
                  "
                  :value="workflow.entry"
                >
                  Unresolved: {{ workflow.entry }}
                </option>
                <option v-for="n in workflow.nodes" :key="n.id" :value="n.id">
                  {{ n.name }} · {{ n.id }}
                </option>
              </select></label
            >
            <details>
              <summary>Metadata</summary>
              <label class="field"
                >Description<textarea v-model="workflow.description" /></label
              ><label class="field"
                >Author<input v-model="workflow.author" /></label
              ><label class="field"
                >License<input v-model="workflow.license"
              /></label>
            </details>
          </fieldset>
          <p class="wf-help">
            Root interface, variables, dependencies and interrupts are below the
            canvas.
          </p>
        </section>
        <section
          v-for="n in workflow.nodes"
          v-show="selected === n.id"
          :key="stateUIKey(n)"
        >
          <h3>{{ n.kind }} step</h3>
          <p
            v-for="issue in (issues || []).filter(
              (i) => issueNode(i.path) === workflow.nodes.indexOf(n),
            )"
            :key="issue.path"
            class="wf-warning"
          >
            {{ issue.message }}
          </p>
          <div class="wf-actions">
            <button
              class="button secondary"
              :disabled="disabled || workflow.nodes[0] === n"
              @click="reorderNode(n, -1)"
            >
              Move earlier</button
            ><button
              class="button secondary"
              :disabled="disabled || workflow.nodes.at(-1) === n"
              @click="reorderNode(n, 1)"
            >
              Move later
            </button>
          </div>
          <label class="field"
            >Node color<select v-model="n.color" :disabled="disabled">
              <option :value="undefined">Default for step type</option>
              <option v-for="color in nodeColors" :key="color" :value="color">
                {{ color }}
              </option>
            </select></label
          >
          <fieldset :disabled="disabled">
            <label class="field"
              >Step name<input
                v-model="n.name"
                :aria-label="`Step ${n.id} name`" /></label
            ><label class="field"
              >Step ID<input
                :value="n.id"
                @change="renameNode(n, $event)"
                :aria-label="`Step ${n.id} ID`"
              /><small
                >Renaming retains references for explicit correction.</small
              ></label
            >
            <details>
              <summary>Position</summary>
              <DeliveryPositionEditor
                v-model="n.position.x"
                :identity="stateUIKey(n)"
                :label="`${n.id} X`"
                :disabled="disabled"
              /><DeliveryPositionEditor
                v-model="n.position.y"
                :identity="stateUIKey(n)"
                :label="`${n.id} Y`"
                :disabled="disabled"
              />
            </details>
            <template v-if="n.task"
              ><label class="field"
                >Task boundary<select v-model="n.task.kind">
                  <option value="human">Human response</option>
                  <option value="adapter">Synthetic adapter response</option>
                </select></label
              ><label class="field"
                >Task title<input v-model="n.task.title" /></label
              ><label class="field"
                >Instructions<textarea v-model="n.task.instructions" />
              </label>
              <details>
                <summary>Request sent to task</summary>
                <WorkflowBindingsEditor
                  :bindings="n.task.request"
                  :free="true"
                  :inputs="workflow.inputs"
                  :variables="workflow.variables"
                  :contracts="contracts"
                  label="Task request"
                  :disabled="disabled"
                />
              </details>
              <details open>
                <summary>Typed response contract</summary>
                <StatePortEditor
                  :ports="n.task.response"
                  :contracts="contracts"
                  label="response field"
                  :disabled="disabled"
                />
              </details>
              <details>
                <summary>Assign response to variables</summary>
                <WorkflowAssignEditor
                  :assign="n.task.assign"
                  :variables="workflow.variables"
                  :outputs="n.task.response"
                  :disabled="disabled"
                /></details
            ></template>
            <template v-if="n.decision"
              ><label class="field"
                >Exact pure decision<select
                  :value="referenceKey(n.decision.component)"
                  @change="chooseDecision(n, $event)"
                >
                  <option value="">Choose embedded decision</option>
                  <option
                    v-if="!workflowDecision(workflow, n)"
                    :value="referenceKey(n.decision.component)"
                  >
                    Unresolved decision
                  </option>
                  <option
                    v-for="b in workflow.decisions"
                    :key="referenceKey(b.root)"
                    :value="referenceKey(b.root)"
                  >
                    {{ bundleRoot(b)?.name || b.root.packageId }} @
                    {{ b.root.version }}
                  </option>
                </select></label
              ><button
                class="button secondary"
                :disabled="!workflowDecision(workflow, n)"
                @click="inspectNode(n)"
              >
                Inspect exact decision & primitives</button
              ><WorkflowBindingsEditor
                :bindings="n.decision.inputs"
                :ports="
                  bundleRoot(workflowDecision(workflow, n))?.behavior.inputs ||
                  []
                "
                :inputs="workflow.inputs"
                :variables="workflow.variables"
                :contracts="contracts"
                label="Decision input"
                :disabled="disabled" />
              <details>
                <summary>Assign decision outputs</summary>
                <WorkflowAssignEditor
                  :assign="n.decision.assign"
                  :variables="workflow.variables"
                  :outputs="
                    bundleRoot(workflowDecision(workflow, n))?.behavior
                      .outputs || []
                  "
                  :disabled="disabled"
                /></details
            ></template>
            <template v-if="n.command"
              ><label class="field"
                >State owner<select v-model="n.command.owner">
                  <option value="">Choose owner</option>
                  <option
                    v-if="
                      n.command.owner &&
                      !workflow.owners.some((o) => o.id === n.command!.owner)
                    "
                    :value="n.command.owner"
                  >
                    Unresolved: {{ n.command.owner }}
                  </option>
                  <option
                    v-for="o in workflow.owners"
                    :key="o.id"
                    :value="o.id"
                  >
                    {{ o.name }} · {{ o.id }}
                  </option>
                </select></label
              ><label class="field"
                >Command<select v-model="n.command.command">
                  <option value="">Choose command</option>
                  <option
                    v-if="n.command.command && !workflowCommand(workflow, n)"
                    :value="n.command.command"
                  >
                    Unresolved: {{ n.command.command }}
                  </option>
                  <option
                    v-for="c in ownerMachine(workflow, n.command.owner)
                      ?.commands || []"
                    :key="c.name"
                    :value="c.name"
                  >
                    {{ c.name }}
                  </option>
                </select></label
              ><button
                class="button secondary"
                :disabled="!workflowCommand(workflow, n)"
                @click="inspectNode(n)"
              >
                Inspect exact command decision</button
              ><WorkflowBindingsEditor
                :bindings="n.command.inputs"
                :ports="workflowCommand(workflow, n)?.inputs || []"
                :inputs="workflow.inputs"
                :variables="workflow.variables"
                :contracts="contracts"
                label="Command input"
                :disabled="disabled" /><label class="field"
                >Record business acceptance in<select
                  v-model="n.command.acceptedVariable"
                >
                  <option value="">Choose Boolean variable</option>
                  <option v-for="v in booleans" :key="v.name" :value="v.name">
                    {{ v.name }}
                  </option>
                </select></label
              ><label class="field"
                >Command error route<select v-model="n.command.errorNext">
                  <option value="">Choose failure step</option>
                  <option v-for="p in workflow.nodes" :key="p.id" :value="p.id">
                    {{ p.name }} · {{ p.id }}
                  </option>
                </select></label
              >
              <details>
                <summary>Assign command outputs</summary>
                <WorkflowAssignEditor
                  :assign="n.command.assign"
                  :variables="workflow.variables"
                  :outputs="workflowCommand(workflow, n)?.outputs || []"
                  :disabled="disabled"
                /></details
            ></template>
            <template v-if="n.end"
              ><label class="field"
                >Terminal outcome<input v-model="n.end.outcome" /></label
              ><WorkflowBindingsEditor
                :bindings="n.end.outputs"
                :ports="workflow.outputs"
                :inputs="workflow.inputs"
                :variables="workflow.variables"
                :contracts="contracts"
                label="Root output"
                :disabled="disabled"
            /></template>
            <template v-else
              ><h4>Automatic route after this step</h4>
              <label class="field"
                >Routing<select
                  :value="n.branch ? 'branch' : 'next'"
                  @change="route(n, $event)"
                >
                  <option value="next">Next step</option>
                  <option value="branch">Boolean branch</option>
                </select></label
              ><label v-if="!n.branch" class="field"
                >Next step<select v-model="n.next">
                  <option value="">Choose target</option>
                  <option
                    v-if="
                      n.next && !workflow.nodes.some((p) => p.id === n.next)
                    "
                    :value="n.next"
                  >
                    Unresolved: {{ n.next }}
                  </option>
                  <option v-for="p in workflow.nodes" :key="p.id" :value="p.id">
                    {{ p.name }} · {{ p.id }}
                  </option>
                </select></label
              ><template v-else
                ><label class="field"
                  >Branch variable<select v-model="n.branch.variable">
                    <option value="">Choose Boolean variable</option>
                    <option v-for="v in booleans" :key="v.name" :value="v.name">
                      {{ v.name }}
                    </option>
                  </select></label
                ><label
                  v-for="side in ['then', 'else'] as const"
                  :key="side"
                  class="field"
                  >{{ side === "then" ? "True" : "False" }} target<select
                    v-model="n.branch[side]"
                  >
                    <option value="">Choose target</option>
                    <option
                      v-for="p in workflow.nodes"
                      :key="p.id"
                      :value="p.id"
                    >
                      {{ p.name }} · {{ p.id }}
                    </option>
                  </select></label
                ></template
              ></template
            >
          </fieldset>
          <button
            class="button quiet"
            :disabled="disabled"
            @click="
              workflow.nodes.splice(workflow.nodes.indexOf(n), 1);
              selected = '';
            "
          >
            Remove step (retain references)
          </button>
        </section>
        <p
          v-if="selected && !workflow.nodes.some((n) => n.id === selected)"
          class="wf-warning"
        >
          Selection changed after a step rename. Choose its new name from the
          hierarchy.
        </p>
      </aside>
    </div>
    <section v-if="inspection" class="wf-panel">
      <div class="wf-actions">
        <strong>Root → step → exact pure component → primitive</strong
        ><button
          class="button quiet"
          @click="
            inspectBundle = null;
            inspectCommand = null;
          "
        >
          Close dependency inspection
        </button>
      </div>
      <WorkflowInspect :bundle="inspection" :primitives="primitives" />
    </section>
    <details class="wf-panel">
      <summary>Public inputs & outputs</summary>
      <div class="wf-grid">
        <section>
          <h4>Inputs required to start</h4>
          <StatePortEditor
            :ports="workflow.inputs"
            :contracts="contracts"
            label="workflow input"
            :disabled="disabled"
          />
        </section>
        <section>
          <h4>Outputs at completion</h4>
          <StatePortEditor
            :ports="workflow.outputs"
            :contracts="contracts"
            label="workflow output"
            :disabled="disabled"
          />
        </section>
      </div>
    </details>
    <details class="wf-panel">
      <summary>
        Owned workflow variables · {{ workflow.variables.length }}
      </summary>
      <p class="wf-help">
        Initial typed values are reset for each finite experiment. Changing a
        type keeps its prior value for explicit correction.
      </p>
      <div
        v-for="(v, i) in workflow.variables"
        :key="stateUIKey(v)"
        class="wf-card"
      >
        <div class="wf-grid">
          <label class="field"
            >Variable name<input v-model="v.name" :disabled="disabled" /></label
          ><label class="field"
            >Type<StateTypeSelect
              :model-value="v"
              :contracts="contracts"
              :label="`Variable ${v.name} type`"
              :disabled="disabled"
              @update:model-value="setType(v, $event)"
          /></label>
        </div>
        <TypedValueEditor
          :model-value="v.initial"
          :type="v.type"
          :record-ref="v.recordRef"
          :contracts="contracts"
          :label="`${v.name} initial value`"
          :disabled="disabled"
          @update:model-value="v.initial = $event || ({ type: v.type } as any)"
        /><button
          class="button quiet"
          :disabled="disabled"
          @click="workflow.variables.splice(i, 1)"
        >
          Remove variable {{ v.name }}
        </button>
      </div>
      <button
        class="button secondary"
        :disabled="disabled"
        @click="addVariable"
      >
        Add workflow variable
      </button>
    </details>
    <details class="wf-panel">
      <summary>
        Exact reusable dependencies · {{ workflow.decisions.length }} decisions
        · {{ workflow.machines.length }} machines
      </summary>
      <div class="wf-grid">
        <section>
          <h4>Pure decisions</h4>
          <div class="wf-actions">
            <select
              v-model="decisionChoice"
              aria-label="Workflow decision catalog"
              :disabled="disabled"
            >
              <option value="">Select installed pure component</option>
              <option
                v-for="p in components"
                :key="lockKey(p)"
                :value="lockKey(p)"
              >
                {{ p.package.name }} @ {{ p.package.version }}
              </option></select
            ><button
              class="button secondary"
              :disabled="disabled || !decisionChoice"
              @click="emit('load-decision', decisionChoice)"
            >
              Embed decision</button
            ><button
              class="button quiet"
              :disabled="disabled"
              @click="emit('import-dependency', 'decision')"
            >
              Import pure bundle
            </button>
          </div>
          <div
            v-for="(b, i) in workflow.decisions"
            :key="referenceKey(b.root)"
            class="wf-card"
          >
            <strong>{{
              bundleRoot(b)?.name || b.root.packageId
            }}</strong><code>{{ referenceKey(b.root) }}</code>
            <div class="wf-actions">
              <button
                class="button secondary"
                @click="
                  inspectBundle = i;
                  inspectCommand = null;
                "
              >
                Inspect primitives</button
              ><button
                class="button quiet"
                :disabled="disabled"
                @click="workflow.decisions.splice(i, 1)"
              >
                Remove lock
              </button>
            </div>
          </div>
        </section>
        <section>
          <h4>State machines</h4>
          <div class="wf-actions">
            <select
              v-model="machineChoice"
              aria-label="Workflow machine catalog"
              :disabled="disabled"
            >
              <option value="">Select installed machine</option>
              <option
                v-for="m in machines"
                :key="stateKey(m)"
                :value="stateKey(m)"
              >
                {{ m.machine.name }} @ {{ m.machine.version }}
              </option></select
            ><button
              class="button secondary"
              :disabled="disabled || !machineChoice"
              @click="emit('load-machine', machineChoice)"
            >
              Embed machine</button
            ><button
              class="button quiet"
              :disabled="disabled"
              @click="emit('import-dependency', 'machine')"
            >
              Import machine lock
            </button>
          </div>
          <details
            v-for="(m, i) in workflow.machines"
            :key="stateKey(m)"
            class="wf-card"
          >
            <summary>{{ m.machine.name }} @ {{ m.machine.version }}</summary>
            <code>{{ stateKey(m) }}</code>
            <p v-for="s in m.machine.state" :key="s.name" class="wf-help">
              {{ s.name }} · {{ typeLabel(s) }} · {{ s.durability }}
            </p>
            <p v-for="c in m.machine.commands" :key="c.name" class="wf-help">
              Command {{ c.name }} · {{ c.inputs.length }} inputs · pure
              {{ c.decision.root.packageId }}
            </p>
            <button
              class="button secondary"
              @click="downloadWorkflow(m, `${m.machine.id}.state.json`)"
            >
              Export reusable state machine</button
            ><button
              class="button quiet"
              :disabled="disabled"
              @click="workflow.machines.splice(i, 1)"
            >
              Remove lock
            </button>
          </details>
        </section>
      </div>
      <h4>Additional data contracts</h4>
      <div class="wf-actions">
        <select
          v-model="dataChoice"
          :disabled="disabled"
          aria-label="Workflow data contract"
        >
          <option value="">Choose exact data contract</option>
          <option
            v-for="d in dataCatalog"
            :key="dataKey(d)"
            :value="dataKey(d)"
          >
            {{ d.contract.name }} @ {{ d.contract.version }}
          </option></select
        ><button
          class="button secondary"
          :disabled="disabled || !dataChoice"
          @click="emit('embed-data', dataChoice)"
        >
          Embed data contract
        </button>
      </div>
      <p v-for="d in contracts" :key="dataKey(d)" class="wf-help">
        {{ dataKey(d) }}
      </p>
    </details>
    <details class="wf-panel">
      <summary>State owners · {{ workflow.owners.length }}</summary>
      <div
        v-for="(o, i) in workflow.owners"
        :key="stateUIKey(o)"
        class="wf-card"
      >
        <div class="wf-grid">
          <label class="field"
            >Owner ID<input v-model="o.id" :disabled="disabled" /></label
          ><label class="field"
            >Owner name<input v-model="o.name" :disabled="disabled" /></label
          ><label class="field"
            >Exact machine<select
              :value="`${o.machine.machineId}@${o.machine.version}:${o.machine.digest}`"
              :disabled="disabled"
              @change="ownerRef(i, $event)"
            >
              <option value="">Choose embedded machine</option>
              <option
                v-for="m in workflow.machines"
                :key="stateKey(m)"
                :value="stateKey(m)"
              >
                {{ m.machine.name }} @ {{ m.machine.version }}
              </option>
            </select></label
          >
        </div>
        <details>
          <summary>
            Initial state overrides (otherwise machine defaults)
          </summary>
          <WorkflowBindingsEditor
            :bindings="o.initial"
            :ports="ownerMachine(workflow, o.id)?.state || []"
            :inputs="workflow.inputs"
            :variables="[]"
            :contracts="contracts"
            label="Owner initial"
            :disabled="disabled"
          />
        </details>
        <button
          class="button quiet"
          :disabled="disabled"
          @click="workflow.owners.splice(i, 1)"
        >
          Remove owner {{ o.id }}
        </button>
      </div>
      <button class="button secondary" :disabled="disabled" @click="addOwner">
        Add state owner
      </button>
    </details>
    <details class="wf-panel">
      <summary>Interrupt signals · {{ workflow.signals.length }}</summary>
      <p class="wf-help">
        A signal cancels the active task and follows its declared route. It is
        an explicit synthetic input, not an automatic clinical assessment.
      </p>
      <div
        v-for="(s, i) in workflow.signals"
        :key="stateUIKey(s)"
        class="wf-card"
      >
        <div class="wf-grid">
          <label class="field"
            >Signal name<input v-model="s.name" :disabled="disabled" /></label
          ><label class="field"
            >Guard Boolean variable<select
              v-model="s.when"
              :disabled="disabled"
            >
              <option value="">Always allowed while active</option>
              <option v-for="v in booleans" :key="v.name" :value="v.name">
                {{ v.name }}
              </option>
            </select></label
          ><label class="field"
            >Interrupt target<select v-model="s.target" :disabled="disabled">
              <option value="">Choose target</option>
              <option v-for="n in workflow.nodes" :key="n.id" :value="n.id">
                {{ n.name }} · {{ n.id }}
              </option>
            </select></label
          >
        </div>
        <StatePortEditor
          :ports="s.inputs"
          :contracts="contracts"
          label="signal input"
          :disabled="disabled"
        />
        <details>
          <summary>Variables set by this signal</summary>
          <WorkflowBindingsEditor
            :bindings="s.set"
            :ports="workflow.variables"
            :inputs="s.inputs"
            :variables="workflow.variables"
            :contracts="contracts"
            label="Signal assignment"
            :disabled="disabled"
          />
        </details>
        <button
          class="button quiet"
          :disabled="disabled"
          @click="workflow.signals.splice(i, 1)"
        >
          Remove signal {{ s.name }}
        </button>
      </div>
      <button class="button secondary" :disabled="disabled" @click="addSignal">
        Add interrupt signal
      </button>
    </details>
  </div>
</template>
<style scoped>
.wf-designer {
  display: grid;
  grid-template-columns: 190px minmax(260px, 1fr) 315px;
  gap: 12px;
  align-items: start;
}
.wf-designer.wf-no-tree {
  grid-template-columns: minmax(260px, 1fr) 315px;
}
.wf-designer.wf-no-inspector {
  grid-template-columns: 190px minmax(260px, 1fr);
}
.wf-designer.wf-no-tree.wf-no-inspector {
  grid-template-columns: minmax(0, 1fr);
}
.wf-tree,
.wf-inspector {
  background: #fff;
  border: 1px solid #dce6ec;
  border-radius: 8px;
  padding: 12px;
  max-height: 660px;
  overflow: auto;
  min-width: 0;
}
.wf-tree button {
  display: block;
  text-align: left;
  width: 100%;
  padding: 12px 9px;
  border-radius: 6px;
  color: #4b6d80;
  font-size: 12px;
  background: transparent;
  overflow-wrap: anywhere;
}
.wf-tree button.active {
  background: #e7f4ef;
  color: #238875;
}
.wf-tree small {
  display: block;
  color: #7c94a2;
  margin-top: 5px;
  font-size: 10px;
}
.wf-inspector .field {
  margin-bottom: 12px;
}
.wf-inspector details {
  margin: 14px 0;
  border-top: 1px solid #e1eaef;
  padding-top: 12px;
}
.wf-inspector summary {
  font-size: 12px;
  color: #517286;
  cursor: pointer;
}
.wf-inspector :deep(.state-port-row) {
  display: block;
}
.wf-inspector code {
  overflow-wrap: anywhere;
}
.wf-card > code {
  display: block;
  overflow-wrap: anywhere;
  font-size: 10px;
  margin-top: 9px;
}
.wf-actions select {
  max-width: 100%;
  flex: 1;
  min-width: 150px;
}
@media (max-width: 1350px) {
  .wf-designer {
    grid-template-columns: 160px minmax(230px, 1fr);
  }
  .wf-inspector {
    grid-column: 1/-1;
    max-height: 650px;
  }
  .wf-designer.wf-no-tree {
    grid-template-columns: minmax(0, 1fr);
  }
  .wf-designer.wf-no-inspector {
    grid-template-columns: 160px minmax(230px, 1fr);
  }
}
@media (max-width: 700px) {
  .wf-designer,
  .wf-designer.wf-no-inspector {
    grid-template-columns: minmax(0, 1fr);
  }
  .wf-tree {
    max-height: 200px;
  }
}
</style>
