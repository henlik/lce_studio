<script setup lang="ts">
import { watch as watchInspection } from "vue";
const inspectionProps = defineProps<{ inspectionRequest?: {nonce: number; body: Record<string, any>} }>();
import StudioRibbon from "./StudioRibbon.vue";
import { useWorkspaceDraft } from "../composables/useWorkspaceDraft";
import { useActionFeedback } from "../composables/useNotifications";
import DraftStatus from "./DraftStatus.vue";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { api, apiRawJson, APIError } from "../api";
import {
  cloneBehavior,
  stableBehavior,
  typeKey,
  lockKey,
  type CatalogRecord,
  type InstalledComposite,
  type ComponentBundle,
} from "../behaviorTypes";
import { dataKey, type DataContractRecord } from "../dataTypes";
import {
  stateKey,
  stateDataContracts,
  decisionRoot,
  type StateMachine,
  type StateMachineLock,
  type StateScenario,
  type StateExample,
  type InstalledStateMachine,
  type StateRun,
  type StateRunSummary,
  type StateCommand,
  type StateResult,
} from "../stateTypes";
import type { ValidationIssue } from "../types";
import StateDefinitionEditor from "./StateDefinitionEditor.vue";
import StateScenarioEditor from "./StateScenarioEditor.vue";
import StateEvidence from "./StateEvidence.vue";
function blankMachine(): StateMachine {
  return {
    schemaVersion: 1,
    id: `local.machine_${crypto.randomUUID().slice(0, 8)}`,
    version: "1.0.0",
    name: "Untitled owned state machine",
    description: "",
    author: "Local author",
    license: "UNLICENSED",
    dataContracts: [],
    state: [],
    effects: [],
    commands: [],
  };
}
function blankScenario(): StateScenario {
  return {
    schemaVersion: 1,
    name: "New state experiment",
    instances: [],
    steps: [],
    expected: [],
  };
}
const machine = ref<StateMachine>(blankMachine()),
  scenario = ref<StateScenario>(blankScenario()),
  examples = ref<StateExample[]>([]),
  catalog = ref<InstalledStateMachine[]>([]),
  components = ref<InstalledComposite[]>([]),
  dataCatalog = ref<DataContractRecord[]>([]),
  history = ref<StateRunSummary[]>([]);
const tab = ref<"definition" | "scenario" | "evidence" | "history">(
    "definition",
  ),
  busy = ref(""),
  error = ref(""),
  notice = ref(""),
  issues = ref<ValidationIssue[]>([]),
  selectedExample = ref("0"),
  selectedMachine = ref("");
const machineStamp = computed(() => stableBehavior(machine.value)),
  scenarioStamp = computed(() => stableBehavior(scenario.value)),
  workspaceStamp = computed(
    () => `${machineStamp.value}|${scenarioStamp.value}`,
  ),
  contracts = computed(() => stateDataContracts(machine.value));
const savedMachine = ref(machineStamp.value),
  savedScenario = ref(scenarioStamp.value),
  installedStamp = ref("");
const record = ref<StateRun | null>(null),
  recordStamp = ref("");
const showJSON = ref(false),
  jsonKind = ref<"machine" | "scenario">("machine"),
  jsonText = ref(""),
  jsonBaseline = ref(""),
  jsonSourceStamp = ref(""),
  pendingJSON = computed(() => jsonText.value !== jsonBaseline.value);
const dirty = computed(
    () =>
      savedMachine.value !== machineStamp.value ||
      savedScenario.value !== scenarioStamp.value ||
      pendingJSON.value,
  ),
  stale = computed(
    () =>
      !!record.value &&
      (recordStamp.value !== workspaceStamp.value || pendingJSON.value),
  );
const definitionEditor = ref<InstanceType<typeof StateDefinitionEditor> | null>(
  null,
);
function focusIssue(issue: ValidationIssue) {
  tab.value = issue.path.startsWith("scenario") ? "scenario" : "definition";
  definitionEditor.value?.focusIssue(issue);
}
const file = ref<HTMLInputElement | null>(null),
  fileKind = ref<"machine" | "scenario" | "handler">("machine"),
  handlerIndex = ref(-1);
function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e);
  issues.value = e instanceof APIError ? e.issues : [];
}
async function work(name: string, fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = name;
  error.value = "";
  notice.value = "";
  issues.value = [];
  try {
    await fn();
  } catch (e) {
    fail(e);
  } finally {
    busy.value = "";
  }
}
function replaceAllowed() {
  return (
    !dirty.value ||
    confirm(
      "Replace the current state machine and scenario? Export them or keep their recorded snapshot before discarding these edits.",
    )
  );
}
function adopt(
  lock: StateMachineLock,
  value?: StateScenario,
  installed = false,
) {
  machine.value = cloneBehavior(lock.machine);
  scenario.value = value ? cloneBehavior(value) : blankScenario();
  savedMachine.value = machineStamp.value;
  savedScenario.value = scenarioStamp.value;
  installedStamp.value = installed ? machineStamp.value : "";
  jsonText.value = "";
  jsonBaseline.value = "";
  showJSON.value = false;
}
function requireApplied() {
  if (pendingJSON.value)
    throw new Error(
      "Apply or reset pending State JSON before validating, running, installing or exporting.",
    );
}
function assertScenarioNumbers() {
  scenario.value.steps.forEach((step, i) => {
    if (
      step.kind === "command" &&
      (!Number.isSafeInteger(step.expectedRevision) ||
        step.expectedRevision! < 0)
    )
      throw new Error(
        `Step ${i + 1} expected revision must be a nonnegative safe integer.`,
      );
  });
  scenario.value.expected.forEach((e) => {
    for (const key of ["revision", "receipts", "effects"] as const)
      if (!Number.isSafeInteger(e[key]) || e[key] < 0)
        throw new Error(
          `Expected ${e.instanceId} ${key} must be a nonnegative safe integer.`,
        );
  });
}
async function prepare(snapshot = cloneBehavior(machine.value)) {
  requireApplied();
  return api<StateMachineLock>("/state/prepare", "POST", snapshot);
}
async function initialize() {
  await work("load", async () => {
    const results = await Promise.allSettled([
      api<StateExample[]>("/state/examples"),
      api<InstalledStateMachine[]>("/state/machines"),
      api<StateRunSummary[]>("/state/runs"),
      api<CatalogRecord[]>("/components"),
      api<DataContractRecord[]>("/data/contracts"),
    ]);
    if (results[0].status === "fulfilled") {
      examples.value = results[0].value;
    } else fail(results[0].reason);
    if (results[1].status === "fulfilled") catalog.value = results[1].value;
    else fail(results[1].reason);
    if (results[2].status === "fulfilled") history.value = results[2].value;
    else fail(results[2].reason);
    if (results[3].status === "fulfilled")
      components.value = results[3].value.filter((p): p is InstalledComposite =>
        [2, 3].includes(p.package.schemaVersion),
      );
    else fail(results[3].reason);
    if (results[4].status === "fulfilled") dataCatalog.value = results[4].value;
    else fail(results[4].reason);
  });
}
function newDraft() {
  if (busy.value || !replaceAllowed()) return;
  machine.value = blankMachine();
  scenario.value = blankScenario();
  savedMachine.value = "";
  savedScenario.value = "";
  installedStamp.value = "";
  jsonText.value = "";
  jsonBaseline.value = "";
  showJSON.value = false;
  tab.value = "definition";
  error.value = "";
  issues.value = [];
  notice.value =
    "Empty state machine and scenario. Add a state slot and a command, choose its pure handler, then add scenario instances, steps and independent expectations.";
}
function openExample() {
  const example = examples.value[Number(selectedExample.value)];
  if (busy.value || !example || !replaceAllowed()) return;
  adopt(example.machine, example.scenario);
  tab.value = "scenario";
  notice.value =
    "Reference machine and handwritten scenario expectations loaded. Run it, then inspect current state and historical replies.";
}
function openMachine() {
  const lock = catalog.value.find((l) => stateKey(l) === selectedMachine.value);
  if (busy.value || !lock || !replaceAllowed()) return;
  adopt(lock, undefined, true);
  tab.value = "scenario";
  notice.value =
    "Installed machine opened with an empty scenario. Add an instance, ordered steps and independent expectations before running.";
}
async function validateMachine() {
  await work("validate", async () => {
    await prepare();
    notice.value =
      "Owned state, handler locks, nominal bindings and effect declarations are valid.";
  });
}
async function install() {
  await work("install", async () => {
    const stamp = machineStamp.value,
      lock = await prepare();
    await api("/state/machines", "POST", lock);
    installedStamp.value = stamp;
    savedMachine.value = stamp;
    catalog.value = await api<InstalledStateMachine[]>("/state/machines");
    notice.value =
      "Exact machine installed. Scenario edits remain separate; export or record them to retain the experiment.";
  });
}
function download(value: unknown, name: string) {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(value, null, 2)], { type: "application/json" }),
  );
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  anchor.click();
  URL.revokeObjectURL(url);
}
async function exportMachine() {
  await work("export", async () => {
    const stamp = machineStamp.value,
      lock = await prepare();
    download(lock, `${lock.machine.id}-${lock.machine.version}.state.json`);
    savedMachine.value = stamp;
    notice.value =
      "Exact machine exported with every pure handler and data lock. Export the scenario separately to reproduce its experiment.";
  });
}
async function exportScenario() {
  await work("export", async () => {
    assertScenarioNumbers();
    const stamp = scenarioStamp.value,
      scenarioSnapshot = cloneBehavior(scenario.value),
      lock = await prepare();
    const checked = await api<{ scenario: StateScenario; result: StateResult }>(
      "/state/check",
      "POST",
      { machine: lock, scenario: scenarioSnapshot },
    );
    download(checked.scenario, `${lock.machine.id}.scenario.json`);
    savedScenario.value = stamp;
    notice.value =
      "Canonical scenario exported. Keep its exact machine alongside it. This check did not store a run.";
  });
}
async function run() {
  await work("run", async () => {
    assertScenarioNumbers();
    const stamp = workspaceStamp.value,
      snapshot = cloneBehavior(scenario.value),
      lock = await prepare();
    record.value = await api<StateRun>("/state/runs", "POST", {
      machine: lock,
      scenario: snapshot,
    });
    recordStamp.value = stamp;
    tab.value = "evidence";
    history.value = await api<StateRunSummary[]>("/state/runs");
    notice.value = record.value.result.verified
      ? record.value.result.passed
        ? "Independent final expectations passed. Complete evidence saved."
        : "Independent final expectations failed. Complete evidence saved for inspection."
      : "Scenario completed without independent expectations. The saved result is unverified.";
  });
}
async function openRun(id: string) {
  await work("history", async () => {
    record.value = await api<StateRun>(`/state/runs/${encodeURIComponent(id)}`);
    recordStamp.value = `${stableBehavior(record.value.machine.machine)}|${stableBehavior(record.value.scenario)}`;
    tab.value = "evidence";
    notice.value =
      "Viewing immutable recorded evidence without replacing the current state draft.";
  });
}
function loadSnapshot() {
  if (!record.value || busy.value || !replaceAllowed()) return;
  adopt(record.value.machine, record.value.scenario);
  recordStamp.value = workspaceStamp.value;
  tab.value = "scenario";
  notice.value =
    "Saved machine and scenario loaded. Rerunning starts each instance from its declared initialization.";
}
function embedData(key: string) {
  if (busy.value) return;
  const lock = dataCatalog.value.find((l) => dataKey(l) === key);
  if (!lock) return;
  const old = contracts.value.find(
    (l) =>
      l.contract.id === lock.contract.id &&
      l.contract.version === lock.contract.version,
  );
  if (old) {
    if (old.digest !== lock.digest)
      error.value =
        "This data ID/version is already locked to different content.";
    else
      notice.value =
        "This exact data contract is already available from the machine or a handler.";
    return;
  }
  if (machine.value.dataContracts.length >= 8) {
    error.value = "A machine may embed at most eight local data contracts.";
    return;
  }
  machine.value.dataContracts.push(
    cloneBehavior({ contract: lock.contract, digest: lock.digest }),
  );
}
function attachHandler(command: StateCommand, bundle: ComponentBundle) {
  const wasEmpty = !command.decision.packages.length;
  command.decision = cloneBehavior(bundle);
  const root = decisionRoot(command);
  if (!root)
    throw new Error("The canonical handler bundle has no root definition.");
  if (wasEmpty) {
    command.inputs = cloneBehavior(root.behavior.inputs);
    command.outputs = cloneBehavior(root.behavior.outputs);
    command.bindings = Object.fromEntries(
      root.behavior.inputs.map((p) => [p.name, { input: p.name }]),
    );
    command.results = Object.fromEntries(
      root.behavior.outputs.map((p) => [p.name, p.name]),
    );
    command.acceptedOutput =
      root.behavior.outputs.find((p) => p.type === "boolean")?.name || "";
  }
  notice.value = wasEmpty
    ? "Exact handler attached. Caller ports mirror its public boundary; choose owned-state bindings, the acceptance output, writes and effects."
    : "Exact handler replaced. Existing public ports, mappings and values were retained for explicit correction.";
}
async function loadHandler(index: number, key: string) {
  const entry = components.value.find((p) => lockKey(p) === key),
    command = machine.value.commands[index];
  if (!entry || !command || busy.value) return;
  if (
    command.decision.packages.length &&
    !confirm(
      "Replace this command’s locked pure handler? Existing mappings are retained and may need correction.",
    )
  )
    return;
  await work("handler", async () => {
    const ref = entry.package,
      bundle = await api<ComponentBundle>(
        `/components/bundle?id=${encodeURIComponent(ref.id)}&version=${encodeURIComponent(ref.version)}&digest=${encodeURIComponent(entry.digest)}`,
      );
    attachHandler(command, bundle);
  });
}
function requestImport(kind: "machine" | "scenario" | "handler", index = -1) {
  if (busy.value) return;
  fileKind.value = kind;
  handlerIndex.value = index;
  file.value?.click();
}
async function importFile(event: Event) {
  const input = event.target as HTMLInputElement,
    selected = input.files?.[0];
  if (!selected) return;
  if (busy.value) {
    input.value = "";
    return;
  }
  const kind = fileKind.value,
    command = machine.value.commands[handlerIndex.value];
  if (
    (kind === "machine" && !replaceAllowed()) ||
    (kind === "scenario" &&
      (savedScenario.value !== scenarioStamp.value || pendingJSON.value) &&
      !confirm(
        "Replace the current scenario? Export or record it first to retain these edits.",
      )) ||
    (kind === "handler" &&
      command?.decision.packages.length &&
      !confirm(
        "Replace this command’s exact handler? Existing mappings are retained for correction.",
      ))
  ) {
    input.value = "";
    return;
  }
  await work("import", async () => {
    if (selected.size > 2 * 1024 * 1024)
      throw new Error("JSON imports must be at most 2 MiB.");
    const raw = await selected.text();
    if (kind === "machine") {
      const checked = await apiRawJson<{
        valid: boolean;
        issues: ValidationIssue[];
        machine?: StateMachineLock;
      }>("/state/validate-lock", raw);
      if (!checked.valid || !checked.machine)
        throw new APIError("State machine import rejected.", checked.issues);
      adopt(checked.machine);
      tab.value = "definition";
      notice.value =
        "Machine imported from the canonical validated server response.";
    } else if (kind === "scenario") {
      const lock = await prepare();
      const checked = await apiRawJson<{
        scenario: StateScenario;
        result: StateResult;
      }>(
        "/state/check",
        `{"machine":${JSON.stringify(lock)},"scenario":${raw}}`,
      );
      scenario.value = checked.scenario;
      tab.value = "scenario";
      notice.value =
        "Scenario validated and checked from original JSON before adopting canonical values. No run was stored.";
    } else {
      if (!command)
        throw new Error("Select a command before importing its handler.");
      const checked = await apiRawJson<{
        valid: boolean;
        issues: ValidationIssue[];
        bundle?: ComponentBundle;
      }>("/components/validate-bundle", raw);
      if (!checked.valid || !checked.bundle)
        throw new APIError("Pure handler bundle rejected.", checked.issues);
      attachHandler(command, checked.bundle);
    }
  });
  input.value = "";
}
function populateJSON() {
  if (jsonKind.value === "scenario") assertScenarioNumbers();
  jsonText.value = JSON.stringify(
    jsonKind.value === "machine" ? machine.value : scenario.value,
    null,
    2,
  );
  jsonBaseline.value = jsonText.value;
  jsonSourceStamp.value = workspaceStamp.value;
}
function openJSON(kind: "machine" | "scenario") {
  if (busy.value) return;
  if (
    pendingJSON.value &&
    jsonKind.value !== kind &&
    !confirm("Discard unapplied State JSON and open another editor?")
  )
    return;
  try {
    if (!pendingJSON.value || jsonKind.value !== kind) {
      jsonKind.value = kind;
      populateJSON();
    }
    showJSON.value = true;
  } catch (e) {
    fail(e);
  }
}
function resetJSON() {
  if (
    busy.value ||
    (pendingJSON.value && !confirm("Discard unapplied State JSON edits?"))
  )
    return;
  try {
    populateJSON();
  } catch (e) {
    fail(e);
  }
}
async function applyJSON() {
  if (
    busy.value ||
    (jsonSourceStamp.value !== workspaceStamp.value &&
      !confirm(
        "The graphical state machine or scenario changed after this JSON was opened. Apply the JSON over the newer content?",
      ))
  )
    return;
  await work("json", async () => {
    if (jsonKind.value === "machine") {
      const lock = await apiRawJson<StateMachineLock>(
        "/state/prepare",
        jsonText.value,
      );
      machine.value = lock.machine;
    } else {
      const lock = await api<StateMachineLock>(
        "/state/prepare",
        "POST",
        cloneBehavior(machine.value),
      );
      const checked = await apiRawJson<{
        scenario: StateScenario;
        result: StateResult;
      }>(
        "/state/check",
        `{"machine":${JSON.stringify(lock)},"scenario":${jsonText.value}}`,
      );
      scenario.value = checked.scenario;
    }
    populateJSON();
    notice.value =
      "Canonical JSON applied. Export, install or record the changed artifact to retain it.";
  });
}
const autosave = useWorkspaceDraft("state", {
  machine,
  scenario,
  tab,
  jsonKind,
  jsonText,
  jsonBaseline,
  jsonSourceStamp,
  showJSON,
  savedMachine,
  savedScenario,
  installedStamp,
  record,
  recordStamp,
});
useActionFeedback("State", busy, notice, error);
function beforeUnload(e: BeforeUnloadEvent) {
  if (autosave.unsaved()) {
    e.preventDefault();
    e.returnValue = "";
  }
}
onMounted(async () => {
  await autosave.ready;
  initialize();
  window.addEventListener("beforeunload", beforeUnload);
});
onUnmounted(() => window.removeEventListener("beforeunload", beforeUnload));
watchInspection(() => inspectionProps.inspectionRequest, async request => {
  if (!request) return;
  await autosave.ready;
  if (busy.value || !replaceAllowed()) return;
  try { const e = JSON.parse(JSON.stringify(request.body)); adopt(e.machine,e.scenario); tab.value = 'definition'; notice.value = 'Inspection exercise opened. The existing draft is retained in recovery checkpoints.'; }
  catch (e) { error.value = (e as Error).message; }
}, { immediate:true });
</script>
<template>
  <section class="state-workbench" :inert="autosave.status.value === 'loading'">
    <StudioRibbon workspace="state" :title="machine.name + ' · v' + machine.version" :active="tab" :structure='["definition"]' :behavior='["scenario"]' :execution='["evidence", "history"]' @select="tab = $event as typeof tab">
      <template #status><small>{{ pendingJSON ? "Unapplied JSON edits" : installedStamp === machineStamp ? "Installed exact version" : "Local draft" }}</small><DraftStatus :draft="autosave" /></template>
      <template #file><div class="state-actions"><button class="button secondary" :disabled="!!busy" @click="newDraft">
        New state machine</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="requestImport('machine')"
      >
        Import state machine</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="exportMachine"
      >
        Export state machine</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="validateMachine"
      >
        Validate state machine</button
      ><button class="button secondary" :disabled="!!busy" @click="install">
        Install state machine</button
      ><button class="button primary" :disabled="!!busy" @click="run">
        Run state scenario
      </button>
    </div>
    <div class="state-library">
      <label class="field"
        >Reference experiment<select
          v-model="selectedExample"
          aria-label="State example"
          :disabled="!!busy"
        >
          <option v-for="(e, i) in examples" :key="i" :value="String(i)">
            {{ e.scenario.name }}
          </option>
        </select></label
      ><button
        class="button secondary"
        :disabled="!!busy || !examples.length"
        @click="openExample"
      >
        Open state example</button
      ><label class="field"
        >Installed machines<select
          v-model="selectedMachine"
          aria-label="Installed state machine"
          :disabled="!!busy"
        >
          <option value="">Select an exact version</option>
          <option v-for="l in catalog" :key="stateKey(l)" :value="stateKey(l)">
            {{ l.machine.id }} @ {{ l.machine.version }}
          </option>
        </select></label
      ><button
        class="button secondary"
        :disabled="!!busy || !selectedMachine"
        @click="openMachine"
      >
        Open state machine</button
      ><button class="button quiet" :disabled="!!busy" @click="initialize">
        Refresh state workspace
      </button>
    </div>
</template>
      <template #help>
    <header>
      <span class="section-eyebrow">OWNED STATE & ATOMIC TRANSITIONS</span>
      <h2>Test what survives a command, retry or restart</h2>
      <p>
        Define owned state, pin a pure decision handler, then run ordered
        scenarios with independent expectations. Inspect current state,
        historical receipts and committed effect intentions.
      </p>
    </header>
    <div class="state-guide">
      <span><b>1</b> Define ownership & handlers</span
      ><span><b>2</b> Arrange commands & failures</span
      ><span><b>3</b> Compare independent expectations</span>
    </div>
</template>
          <nav class="state-tabs">
      <button
        v-show="['definition'].includes(tab)" :class="{ active: tab === 'definition' }"
        @click="tab = 'definition'"
      >
        Ownership & handlers</button
      ><button
        v-show="['scenario'].includes(tab)" :class="{ active: tab === 'scenario' }"
        @click="tab = 'scenario'"
      >
        Scenario & expectations</button
      ><button
        v-show="['evidence', 'history'].includes(tab)" :class="{ active: tab === 'evidence' }"
        @click="tab = 'evidence'"
      >
        State evidence</button
      ><button v-show="['evidence', 'history'].includes(tab)" :class="{ active: tab === 'history' }" @click="tab = 'history'">
        State run history
      </button>
    </nav>
      <template #commands><div class="state-actions">
        <button class="button secondary" :disabled="!!busy" @click="validateMachine">Validate</button>
        <button class="button primary" :disabled="!!busy" @click="run">Run scenario</button>
      </div></template>
    </StudioRibbon>
    <input
      ref="file"
      class="hidden"
      type="file"
      accept=".json,application/json"
      aria-label="Import State JSON"
      @change="importFile"
    />
    <div v-if="error" class="state-error" role="alert">
      <strong>{{ error }}</strong>
      <ul>
        <li v-for="(issue, i) in issues" :key="i">
          <button class="issue-link" @click="focusIssue(issue)">
            <code>{{ issue.path }}</code> {{ issue.message }} → Fix
          </button>
        </li>
      </ul>
    </div>
    <p v-if="notice" class="state-notice" role="status">{{ notice }}</p>
    <p v-if="busy" class="state-notice" role="status">
      Working with the state engine…
    </p>

    <section
      v-if="
        tab === 'definition' &&
        !machine.state.length &&
        !machine.commands.length
      "
      class="state-panel"
    >
      <p class="state-help">
        This machine is empty. Use Add state slot to declare what it owns, then
        Add command and choose a pure handler for its behavior.
      </p>
    </section>
    <StateDefinitionEditor
      ref="definitionEditor"
      :issues="issues"
      v-show="tab === 'definition'"
      :machine="machine"
      :contracts="contracts"
      :catalog="components"
      :data-catalog="dataCatalog"
      :disabled="!!busy"
      @load-handler="loadHandler"
      @import-handler="(index) => requestImport('handler', index)"
      @embed-data="embedData"
    />
    <div v-show="tab === 'scenario'">
      <p v-if="!scenario.instances.length" class="state-panel state-help">
        This scenario is empty. Use Add state instance below, then add the
        commands or restarts you want to test and your expected outcomes.
      </p>
      <div class="state-actions">
        <button
          class="button secondary"
          :disabled="!!busy"
          @click="requestImport('scenario')"
        >
          Import state scenario</button
        ><button
          class="button secondary"
          :disabled="!!busy"
          @click="exportScenario"
        >
          Export state scenario
        </button>
      </div>
      <StateScenarioEditor
        :machine="machine"
        :scenario="scenario"
        :contracts="contracts"
        :disabled="!!busy"
      />
    </div>
    <div v-if="tab === 'evidence'">
      <StateEvidence
        v-if="record"
        :record="record"
        :stale="stale"
        :disabled="!!busy"
        @load-snapshot="loadSnapshot"
      />
      <p v-else class="state-panel state-help">
        Run the scenario to record complete state, receipt, effect and pure
        handler evidence.
      </p>
    </div>
    <section v-if="tab === 'history'" class="state-panel">
      <h3>Immutable state run history</h3>
      <p class="state-help">
        Opening a record leaves your current draft unchanged. Explicitly load
        its snapshot to reproduce the experiment.
      </p>
      <div class="state-history">
        <button
          v-for="r in history"
          :key="r.id"
          :disabled="!!busy"
          @click="openRun(r.id)"
        >
          <strong>{{ r.machineId }} @ {{ r.machineVersion }}</strong
          ><span
            >{{ r.verified ? (r.passed ? "PASS" : "FAIL") : "UNVERIFIED" }} ·
            {{ r.stepCount }} steps ·
            {{ new Date(r.createdAt).toLocaleString() }}</span
          >
        </button>
      </div>
      <p v-if="!history.length" class="state-help">
        No recorded state scenarios yet.
      </p>
    </section>
    <div class="state-json">
      <button
        class="button quiet"
        :disabled="!!busy"
        @click="openJSON('machine')"
      >
        State machine JSON</button
      ><button
        class="button quiet"
        :disabled="!!busy"
        @click="openJSON('scenario')"
      >
        State scenario JSON</button
      ><template v-if="showJSON"
        ><p class="state-help">
          Original {{ jsonKind }} JSON goes to the Go validator before the
          browser adopts canonical values. Pending text remains when hidden.
        </p>
        <p v-if="jsonSourceStamp !== workspaceStamp" class="state-warning">
          The graphical machine or scenario changed after this JSON was opened.
          Applying it will ask before replacing newer content.
        </p>
        <textarea
          v-model="jsonText"
          aria-label="Advanced State JSON"
          class="state-json-editor"
          :disabled="!!busy"
          spellcheck="false"
        />
        <div class="state-actions">
          <button
            class="button secondary"
            :disabled="!!busy"
            @click="applyJSON"
          >
            Validate & apply State JSON</button
          ><button class="button quiet" :disabled="!!busy" @click="resetJSON">
            Reset State JSON</button
          ><button class="button quiet" @click="showJSON = false">
            Hide State JSON
          </button>
        </div></template
      >
    </div>
    <p class="state-help state-boundary">
      Durability here means surviving a simulated restart within one scenario.
      PostgreSQL retains definitions and experiment evidence; it does not run
      these modeled business transactions. Effect intentions have no delivery,
      acknowledgement, deployment or resource-timing semantics.
    </p>
  </section>
</template>
<style>
.state-workbench {
  padding: 24px 30px;
  background: #f5f8fa;
  min-width: 0;
}
.state-workbench header h2 {
  font-size: 24px;
  color: #355269;
  margin: 8px 0;
}
.state-workbench header p,
.state-help {
  font-size: 12px;
  line-height: 1.8;
  color: #627d8e;
}
.state-guide {
  display: flex;
  gap: 18px;
  flex-wrap: wrap;
  margin: 18px 0;
  color: #5a7d8f;
  font-size: 12px;
}
.state-guide span {
  display: flex;
  align-items: center;
  gap: 8px;
}
.state-guide b {
  display: grid;
  place-items: center;
  background: #e3f2ea;
  color: #2b8c74;
  border-radius: 50%;
  width: 24px;
  height: 24px;
}
.state-library,
.state-panel {
  background: #fff;
  border: 1px solid #dce6ed;
  border-radius: 9px;
  padding: 18px;
  margin: 16px 0;
}
.state-library {
  display: flex;
  align-items: end;
  gap: 12px;
  flex-wrap: wrap;
}
.state-library .field {
  min-width: 180px;
  flex: 1;
  margin: 0;
}
.state-workbench .button {
  font-size: 12px;
  padding: 9px 12px;
}
.state-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
  margin: 14px 0;
}
.state-actions > .field {
  margin: 0;
  min-width: 220px;
  flex: 1;
}
.state-actions > strong {
  font-size: 14px;
  color: #3d5e73;
  margin-right: auto;
}
.state-actions > strong small {
  display: block;
  font-size: 12px;
  color: #77909e;
  margin-top: 5px;
}
.state-actions > span {
  font-size: 12px;
  color: #4f8c75;
}
.state-panel-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.state-panel-heading h3,
.state-panel-heading h4 {
  margin: 0;
}
.state-panel-heading > .state-actions {
  margin: 0;
}
.state-panel-heading > span {
  font-size: 12px;
  color: #78909d;
}
.state-workbench h3 {
  font-size: 16px;
  color: #42667c;
  margin: 0 0 14px;
}
.state-workbench h4 {
  font-size: 13px;
  color: #52778b;
  margin: 16px 0 10px;
}
.state-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 14px;
}
.state-grid .field {
  min-width: 0;
}
.state-card {
  border: 1px solid #e0e9ed;
  background: #fbfdfd;
  border-radius: 7px;
  padding: 16px;
  margin: 12px 0;
  min-width: 0;
}
.state-card > .button {
  margin-top: 12px;
}
.state-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  border-bottom: 1px solid #dce7ed;
  padding-bottom: 10px;
  margin: 18px 0;
}
.state-tabs button {
  padding: 10px 13px;
  color: #627e90;
  background: transparent;
  border-radius: 5px;
  font-size: 12px;
}
.state-tabs button.active {
  background: #e6f4ee;
  color: #23846e;
}
.state-workbench .state-panel > summary,
.state-subsection > summary {
  cursor: pointer;
  color: #526f83;
  font-size: 13px;
}
.state-subsection {
  margin: 15px 0;
  border-top: 1px solid #e1e9ed;
  padding-top: 13px;
}
.state-lock {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
}
.state-lock code {
  font-size: 11px;
  color: #5e7e90;
  overflow-wrap: anywhere;
}
.state-check {
  display: flex;
  gap: 9px;
  align-items: center;
  font-size: 12px;
  color: #52778b;
  margin: 12px 0;
}
.state-check input {
  width: 15px;
  height: 15px;
  min-height: 0;
}
.state-error,
.state-warning {
  background: #fff6e8;
  border: 1px solid #eed9b8;
  color: #a36b42;
  padding: 12px 15px;
  border-radius: 7px;
  font-size: 12px;
  line-height: 1.8;
  overflow-wrap: anywhere;
}
.state-notice {
  background: #edf8f2;
  border: 1px solid #d1e7dc;
  color: #3e866d;
  padding: 12px 15px;
  border-radius: 7px;
  font-size: 12px;
  line-height: 1.8;
}
.state-history {
  display: grid;
  gap: 8px;
  max-height: 450px;
  overflow: auto;
}
.state-history button {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  background: #fafdfc;
  border: 1px solid #dfebee;
  padding: 14px;
  border-radius: 6px;
  text-align: left;
  color: #4f7286;
  font-size: 12px;
}
.state-history span {
  color: #738b98;
}
.state-json {
  margin-top: 20px;
}
.state-json-editor {
  display: block;
  width: 100%;
  height: 380px;
  border: 1px solid #d9e4eb;
  border-radius: 7px;
  padding: 16px;
  font-family: monospace;
  font-size: 12px;
  line-height: 1.6;
  color: #456779;
  background: #fcfefe;
}
.state-boundary {
  margin-top: 22px;
}
.state-workbench fieldset {
  margin: 0;
  padding: 0;
  border: 0;
  min-width: 0;
}
.state-workbench fieldset:disabled {
  opacity: 0.8;
}
@media (max-width: 1100px) {
  .state-workbench {
    padding: 20px;
  }
  .state-actions > strong {
    width: 100%;
  }
  .state-panel-heading {
    flex-wrap: wrap;
  }
  .state-history button {
    flex-direction: column;
  }
}
@media (max-width: 650px) {
  .state-workbench {
    padding: 12px;
  }
  .state-grid {
    grid-template-columns: 1fr;
  }
  .state-library .field {
    min-width: 100%;
  }
  .state-actions > .field {
    min-width: 100%;
  }
}
</style>
