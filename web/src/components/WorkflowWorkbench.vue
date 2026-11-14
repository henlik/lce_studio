<script setup lang="ts">
import { useWorkspaceDraft } from "../composables/useWorkspaceDraft";
import { useActionFeedback } from "../composables/useNotifications";
import DraftStatus from "./DraftStatus.vue";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { api, apiRawJson, APIError } from "../api";
import {
  cloneBehavior,
  lockKey,
  referenceKey,
  type CatalogRecord,
  type InstalledComposite,
  type ComponentBundle,
  type PrimitiveSpec,
  type TypedValue,
} from "../behaviorTypes";
import { dataKey, type DataContractRecord } from "../dataTypes";
import {
  stateKey,
  type InstalledStateMachine,
  type StateMachineLock,
} from "../stateTypes";
import type { ValidationIssue } from "../types";
import {
  workflowKey,
  workflowStamp,
  workflowContracts,
  assertWorkflowNumbers,
  downloadWorkflow,
  type WorkflowDefinition,
  type WorkflowScenario,
  type WorkflowLock,
  type InstalledWorkflow,
  type WorkflowExample,
  type WorkflowResult,
  type WorkflowRun,
  type WorkflowRunSummary,
  type WorkflowEvent,
} from "../workflowTypes";
import WorkflowDefinitionEditor from "./WorkflowDefinitionEditor.vue";
import WorkflowScenarioEditor from "./WorkflowScenarioEditor.vue";
import WorkflowLive from "./WorkflowLive.vue";
import WorkflowEvidence from "./WorkflowEvidence.vue";
function blankWorkflow(): WorkflowDefinition {
  return {
    schemaVersion: 1,
    id: `local.workflow_${crypto.randomUUID().slice(0, 8)}`,
    version: "1.0.0",
    name: "Untitled workflow",
    description: "",
    author: "Local author",
    license: "UNLICENSED",
    dataContracts: [],
    decisions: [],
    machines: [],
    owners: [],
    inputs: [],
    outputs: [],
    variables: [],
    entry: "",
    nodes: [],
    signals: [],
  };
}
function blankScenario(): WorkflowScenario {
  return {
    schemaVersion: 1,
    name: "New workflow experiment",
    inputs: {},
    events: [],
  };
}
const workflow = ref(blankWorkflow()),
  scenario = ref(blankScenario()),
  examples = ref<WorkflowExample[]>([]),
  catalog = ref<InstalledWorkflow[]>([]),
  components = ref<InstalledComposite[]>([]),
  machines = ref<InstalledStateMachine[]>([]),
  dataCatalog = ref<DataContractRecord[]>([]),
  primitives = ref<PrimitiveSpec[]>([]),
  history = ref<WorkflowRunSummary[]>([]);
const tab = ref<
    "definition" | "interactive" | "scenario" | "evidence" | "history"
  >("definition"),
  busy = ref(""),
  error = ref(""),
  notice = ref(""),
  issues = ref<ValidationIssue[]>([]),
  exampleChoice = ref("0"),
  catalogChoice = ref("");
const definitionStamp = computed(() => workflowStamp(workflow.value)),
  scenarioStamp = computed(() => workflowStamp(scenario.value)),
  workspaceStamp = computed(
    () => `${definitionStamp.value}|${scenarioStamp.value}`,
  ),
  contracts = computed(() => workflowContracts(workflow.value));
const savedDefinition = ref(definitionStamp.value),
  savedScenario = ref(scenarioStamp.value),
  installedStamp = ref("");
const record = ref<WorkflowRun | null>(null),
  recordStamp = ref("");
const liveForm = ref<any>(null);
const live = ref<(WorkflowExample & { result: WorkflowResult }) | null>(null),
  liveInputs = ref<Record<string, TypedValue>>({}),
  liveGeneration = ref(0),
  livePending = ref(false),
  savedLiveStamp = ref(""),
  savedLiveInputs = ref(workflowStamp(liveInputs.value));
const showJSON = ref(false),
  jsonKind = ref<"workflow" | "scenario">("workflow"),
  jsonText = ref(""),
  jsonBaseline = ref(""),
  jsonSourceStamp = ref(""),
  pendingJSON = computed(() => jsonText.value !== jsonBaseline.value);
const dirty = computed(
    () =>
      definitionStamp.value !== savedDefinition.value ||
      scenarioStamp.value !== savedScenario.value ||
      workflowStamp(liveInputs.value) !== savedLiveInputs.value ||
      pendingJSON.value,
  ),
  liveDirty = computed(
    () =>
      livePending.value ||
      (!!live.value &&
        workflowStamp(live.value.scenario) !== savedLiveStamp.value),
  ),
  stale = computed(
    () =>
      !!record.value &&
      (recordStamp.value !== workspaceStamp.value || pendingJSON.value),
  ),
  liveStale = computed(
    () =>
      !!live.value &&
      (workflowStamp(live.value.workflow.workflow) !== definitionStamp.value ||
        pendingJSON.value),
  );
const definitionEditor = ref<InstanceType<
  typeof WorkflowDefinitionEditor
> | null>(null);
function focusIssue(issue: ValidationIssue) {
  tab.value = issue.path.startsWith("scenario") ? "scenario" : "definition";
  definitionEditor.value?.focusIssue(issue);
}
const file = ref<HTMLInputElement | null>(null),
  fileKind = ref<"workflow" | "scenario" | "decision" | "machine">("workflow");
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
function safeCopy<T>(v: T, exact = false): T {
  assertWorkflowNumbers(v, "$", exact);
  return cloneBehavior(v);
}
function requireApplied() {
  if (pendingJSON.value)
    throw new Error(
      "Apply or reset pending Workflow JSON before validating, running, installing or exporting the draft.",
    );
}
function replaceAllowed() {
  return (
    !dirty.value ||
    confirm(
      "Replace the current workflow and independent scenario? Export or record them first to retain these edits. The separate interactive experiment remains available.",
    )
  );
}
function scenarioAllowed() {
  return (
    (savedScenario.value === scenarioStamp.value && !pendingJSON.value) ||
    confirm(
      "Replace the current scenario and discard its pending JSON after successful import? Export or record it first to retain these edits.",
    )
  );
}
function clearJSON() {
  showJSON.value = false;
  jsonText.value = "";
  jsonBaseline.value = "";
  jsonSourceStamp.value = "";
}
function adopt(lock: WorkflowLock, s?: WorkflowScenario, installed = false) {
  const w = safeCopy(lock.workflow, true),
    next = s ? safeCopy(s) : blankScenario();
  workflow.value = w;
  scenario.value = next;
  liveInputs.value = safeCopy(next.inputs);
  savedLiveInputs.value = workflowStamp(liveInputs.value);
  savedDefinition.value = definitionStamp.value;
  savedScenario.value = scenarioStamp.value;
  installedStamp.value = installed ? definitionStamp.value : "";
  clearJSON();
}
async function prepare(ignorePending = false) {
  if (!ignorePending) requireApplied();
  const snapshot = safeCopy(workflow.value);
  const lock = await api<WorkflowLock>("/workflow/prepare", "POST", snapshot);
  assertWorkflowNumbers(lock, "$", true);
  return lock;
}
async function initialize() {
  await work("load", async () => {
    const results = await Promise.allSettled([
      api<WorkflowExample[]>("/workflow/examples"),
      api<InstalledWorkflow[]>("/workflow/packages"),
      api<WorkflowRunSummary[]>("/workflow/runs"),
      api<CatalogRecord[]>("/components"),
      api<InstalledStateMachine[]>("/state/machines"),
      api<DataContractRecord[]>("/data/contracts"),
      api<PrimitiveSpec[]>("/components/structured-primitives"),
    ]);
    if (results[0].status === "fulfilled") examples.value = results[0].value;
    else fail(results[0].reason);
    if (results[1].status === "fulfilled") catalog.value = results[1].value;
    else fail(results[1].reason);
    if (results[2].status === "fulfilled") history.value = results[2].value;
    else fail(results[2].reason);
    if (results[3].status === "fulfilled")
      components.value = results[3].value.filter((p): p is InstalledComposite =>
        [2, 3].includes(p.package.schemaVersion),
      );
    else fail(results[3].reason);
    if (results[4].status === "fulfilled") machines.value = results[4].value;
    else fail(results[4].reason);
    if (results[5].status === "fulfilled") dataCatalog.value = results[5].value;
    else fail(results[5].reason);
    if (results[6].status === "fulfilled") primitives.value = results[6].value;
    else fail(results[6].reason);
  });
}
function newDraft() {
  if (busy.value || !replaceAllowed()) return;
  workflow.value = blankWorkflow();
  scenario.value = blankScenario();
  liveInputs.value = {};
  savedLiveInputs.value = workflowStamp(liveInputs.value);
  savedDefinition.value = definitionStamp.value;
  savedScenario.value = scenarioStamp.value;
  installedStamp.value = "";
  clearJSON();
  tab.value = "definition";
  error.value = "";
  issues.value = [];
  notice.value =
    "Empty reusable workflow. Add its interface, variables and first task, or explicitly open a reference example.";
}
function openExample() {
  const e = examples.value[Number(exampleChoice.value)];
  if (busy.value || !e || !replaceAllowed()) return;
  try {
    adopt(e.workflow, e.scenario);
    tab.value = "definition";
    notice.value =
      "Reference workflow and handwritten scenario loaded. Inspect its steps or open Interactive to start a fresh synthetic session.";
  } catch (e) {
    fail(e);
  }
}
function openPackage() {
  const p = catalog.value.find((p) => workflowKey(p) === catalogChoice.value);
  if (busy.value || !p || !replaceAllowed()) return;
  try {
    adopt(p, undefined, true);
    tab.value = "definition";
    notice.value =
      "Exact reusable workflow loaded with an empty independent scenario.";
  } catch (e) {
    fail(e);
  }
}
async function validate() {
  await work("validate", async () => {
    await prepare();
    notice.value =
      "Workflow routes, typed task boundaries, dependency locks and owned command mappings are valid.";
  });
}
async function install() {
  await work("install", async () => {
    const stamp = definitionStamp.value,
      lock = await prepare();
    await api("/workflow/packages", "POST", lock);
    savedDefinition.value = stamp;
    installedStamp.value = stamp;
    catalog.value = await api<InstalledWorkflow[]>("/workflow/packages");
    notice.value =
      "Exact workflow installed. Its independent scenario remains a separate artifact.";
  });
}
async function exportPackage() {
  await work("export", async () => {
    const stamp = definitionStamp.value,
      lock = await prepare();
    downloadWorkflow(
      lock,
      `${lock.workflow.id}-${lock.workflow.version}.workflow.json`,
    );
    savedDefinition.value = stamp;
    notice.value =
      "Portable exact workflow exported with its pure, state and data dependencies.";
  });
}
async function exportScenario() {
  await work("export", async () => {
    requireApplied();
    const stamp = scenarioStamp.value,
      s = safeCopy(scenario.value),
      lock = await prepare();
    const checked = await api<{
      scenario: WorkflowScenario;
      result: WorkflowResult;
    }>("/workflow/check", "POST", { workflow: lock, scenario: s });
    downloadWorkflow(checked.scenario, `${lock.workflow.id}.scenario.json`);
    savedScenario.value = stamp;
    notice.value =
      "Canonical independent scenario exported. Keep its exact workflow package alongside it.";
  });
}
async function run() {
  await work("run", async () => {
    requireApplied();
    const stamp = workspaceStamp.value,
      s = safeCopy(scenario.value),
      lock = await prepare();
    record.value = await api<WorkflowRun>("/workflow/runs", "POST", {
      workflow: lock,
      scenario: s,
    });
    recordStamp.value = stamp;
    savedDefinition.value = definitionStamp.value;
    savedScenario.value = scenarioStamp.value;
    tab.value = "evidence";
    history.value = await api<WorkflowRunSummary[]>("/workflow/runs");
    notice.value = record.value.result.verified
      ? record.value.result.passed
        ? "Independent assertions passed. Complete evidence saved."
        : "Independent assertions failed. Complete evidence saved for inspection."
      : "Execution saved without independent assertions; its result is unverified.";
  });
}
async function openRun(id: string) {
  await work("history", async () => {
    const r = await api<WorkflowRun>(
      `/workflow/runs/${encodeURIComponent(id)}`,
    );
    record.value = r;
    recordStamp.value = `${workflowStamp(r.workflow.workflow)}|${workflowStamp(r.scenario)}`;
    tab.value = "evidence";
    notice.value =
      "Viewing immutable evidence; your current draft has not been replaced.";
  });
}
function loadSnapshot() {
  if (!record.value || busy.value || !replaceAllowed()) return;
  try {
    adopt(record.value.workflow, record.value.scenario);
    recordStamp.value = workspaceStamp.value;
    tab.value = "scenario";
    notice.value =
      "Recorded package and scenario loaded for reproducible execution.";
  } catch (e) {
    fail(e);
  }
}
async function startLive() {
  if (
    busy.value ||
    (liveDirty.value &&
      !confirm(
        "Restart the interactive experiment? Save its evidence or copy its events to a scenario before replacing it.",
      ))
  )
    return;
  await work("interactive", async () => {
    requireApplied();
    const inputs = safeCopy(liveInputs.value),
      lock = await prepare();
    const s: WorkflowScenario = {
      schemaVersion: 1,
      name: `Interactive ${lock.workflow.name}`,
      inputs,
      events: [],
    };
    const checked = await api<{
      scenario: WorkflowScenario;
      result: WorkflowResult;
    }>("/workflow/check", "POST", { workflow: lock, scenario: s });
    live.value = {
      workflow: lock,
      scenario: checked.scenario,
      result: checked.result,
    };
    liveGeneration.value++;
    savedLiveInputs.value = workflowStamp(liveInputs.value);
    savedLiveStamp.value = "";
    notice.value =
      "Started a finite synthetic experiment. The engine routed automatically to the next task or outcome.";
  });
}
async function continueLive(event: WorkflowEvent) {
  const current = live.value;
  if (!current) return;
  await work("continue", async () => {
    const e = safeCopy(event),
      snapshot = safeCopy(current, true);
    snapshot.scenario.events.push(e);
    const checked = await api<{
      scenario: WorkflowScenario;
      result: WorkflowResult;
    }>("/workflow/check", "POST", {
      workflow: snapshot.workflow,
      scenario: snapshot.scenario,
    });
    live.value = {
      workflow: snapshot.workflow,
      scenario: checked.scenario,
      result: checked.result,
    };
    notice.value = `Event processed. Workflow is ${checked.result.status}${checked.result.outcome ? `: ${checked.result.outcome}` : ""}.`;
  });
}
async function recordLive() {
  const current = live.value;
  if (!current) return;
  await work("record", async () => {
    const snapshot = safeCopy(current, true);
    record.value = await api<WorkflowRun>("/workflow/runs", "POST", {
      workflow: snapshot.workflow,
      scenario: snapshot.scenario,
    });
    recordStamp.value = `${workflowStamp(snapshot.workflow.workflow)}|${workflowStamp(snapshot.scenario)}`;
    savedLiveStamp.value = workflowStamp(snapshot.scenario);
    history.value = await api<WorkflowRunSummary[]>("/workflow/runs");
    notice.value =
      "Interactive trace saved as immutable unverified evidence. Continue here or inspect it in Evidence.";
  });
}
function copyLive() {
  if (!live.value || busy.value || !scenarioAllowed()) return;
  try {
    const s = safeCopy(live.value.scenario);
    if (workflowStamp(live.value.workflow.workflow) !== definitionStamp.value)
      throw new Error(
        "This interactive session uses an earlier workflow. Save its evidence and explicitly load that snapshot before copying its scenario.",
      );
    scenario.value = s;
    clearJSON();
    tab.value = "scenario";
    notice.value =
      "Interactive inputs and events copied. Add independent expectations yourself before verifying.";
  } catch (e) {
    fail(e);
  }
}
function embedDecision(bundle: ComponentBundle) {
  assertWorkflowNumbers(bundle, "$", true);
  const key = referenceKey(bundle.root);
  if (workflow.value.decisions.some((b) => referenceKey(b.root) === key)) {
    notice.value = "This exact pure decision is already embedded.";
    return;
  }
  workflow.value.decisions.push(safeCopy(bundle, true));
  notice.value = "Exact pure bundle embedded. Select it on a decision step.";
}
async function loadDecision(key: string) {
  const p = components.value.find((p) => lockKey(p) === key);
  if (!p) return;
  await work("dependency", async () => {
    embedDecision(
      await api<ComponentBundle>(
        `/components/bundle?id=${encodeURIComponent(p.package.id)}&version=${encodeURIComponent(p.package.version)}&digest=${encodeURIComponent(p.digest)}`,
      ),
    );
  });
}
function embedMachine(lock: StateMachineLock) {
  assertWorkflowNumbers(lock, "$", true);
  if (workflow.value.machines.some((m) => stateKey(m) === stateKey(lock))) {
    notice.value = "This exact machine is already embedded.";
    return;
  }
  workflow.value.machines.push(
    safeCopy({ machine: lock.machine, digest: lock.digest }, true),
  );
  notice.value =
    "Exact state machine embedded. Add a named owner, then a command step.";
}
function loadMachine(key: string) {
  if (busy.value) return;
  try {
    const m = machines.value.find((m) => stateKey(m) === key);
    if (m) embedMachine(m);
  } catch (e) {
    fail(e);
  }
}
function embedData(key: string) {
  if (busy.value) return;
  try {
    const d = dataCatalog.value.find((d) => dataKey(d) === key);
    if (!d) return;
    if (contracts.value.some((l) => dataKey(l) === key)) {
      notice.value =
        "This exact contract is already available from a dependency.";
      return;
    }
    workflow.value.dataContracts.push(
      safeCopy({ contract: d.contract, digest: d.digest }, true),
    );
  } catch (e) {
    fail(e);
  }
}
function requestImport(kind: typeof fileKind.value) {
  if (busy.value) return;
  fileKind.value = kind;
  file.value?.click();
}
async function importFile(event: Event) {
  const input = event.target as HTMLInputElement,
    selected = input.files?.[0];
  if (!selected) return;
  const kind = fileKind.value;
  if (
    busy.value ||
    (kind === "workflow" && !replaceAllowed()) ||
    (kind === "scenario" && !scenarioAllowed())
  ) {
    input.value = "";
    return;
  }
  await work("import", async () => {
    if (selected.size > 2 * 1024 * 1024)
      throw new Error("JSON imports must be at most 2 MiB.");
    const raw = await selected.text();
    if (kind === "workflow") {
      const checked = await apiRawJson<{
        valid: boolean;
        issues: ValidationIssue[];
        workflow?: WorkflowLock;
      }>("/workflow/validate-lock", raw);
      if (!checked.valid || !checked.workflow)
        throw new APIError("Workflow package rejected.", checked.issues);
      adopt(checked.workflow);
      tab.value = "definition";
    } else if (kind === "scenario") {
      const lock = await prepare(true),
        checked = await apiRawJson<{
          scenario: WorkflowScenario;
          result: WorkflowResult;
        }>(
          "/workflow/check",
          `{"workflow":${JSON.stringify(lock)},"scenario":${raw}}`,
        );
      assertWorkflowNumbers(checked.scenario);
      scenario.value = checked.scenario;
      clearJSON();
      tab.value = "scenario";
    } else if (kind === "decision") {
      const checked = await apiRawJson<{
        valid: boolean;
        issues: ValidationIssue[];
        bundle?: ComponentBundle;
      }>("/components/validate-bundle", raw);
      if (!checked.valid || !checked.bundle)
        throw new APIError("Pure dependency rejected.", checked.issues);
      embedDecision(checked.bundle);
    } else {
      const checked = await apiRawJson<{
        valid: boolean;
        issues: ValidationIssue[];
        machine?: StateMachineLock;
      }>("/state/validate-lock", raw);
      if (!checked.valid || !checked.machine)
        throw new APIError("State dependency rejected.", checked.issues);
      embedMachine(checked.machine);
    }
    notice.value =
      "Canonical artifact imported after server validation of the original JSON.";
  });
  input.value = "";
}
function jsonSource(kind: typeof jsonKind.value) {
  return kind === "workflow" ? definitionStamp.value : scenarioStamp.value;
}
function populateJSON() {
  const value = jsonKind.value === "workflow" ? workflow.value : scenario.value;
  assertWorkflowNumbers(value);
  jsonText.value = JSON.stringify(value, null, 2);
  jsonBaseline.value = jsonText.value;
  jsonSourceStamp.value = jsonSource(jsonKind.value);
}
function openJSON(kind: typeof jsonKind.value) {
  if (busy.value) return;
  try {
    assertWorkflowNumbers(
      kind === "workflow" ? workflow.value : scenario.value,
    );
    const changed = kind !== jsonKind.value;
    if (
      changed &&
      pendingJSON.value &&
      !confirm("Discard pending Workflow JSON and switch editors?")
    )
      return;
    const wasVisible = showJSON.value;
    if (changed || !pendingJSON.value) {
      jsonKind.value = kind;
      populateJSON();
    }
    showJSON.value = changed || !wasVisible;
  } catch (e) {
    fail(e);
  }
}
function resetJSON() {
  if (
    busy.value ||
    (pendingJSON.value && !confirm("Discard unapplied Workflow JSON edits?"))
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
    (jsonSourceStamp.value !== jsonSource(jsonKind.value) &&
      !confirm(
        "The graphical draft changed after this JSON was opened. Apply the buffer over its newer content?",
      ))
  )
    return;
  await work("json", async () => {
    if (jsonKind.value === "workflow") {
      const lock = await apiRawJson<WorkflowLock>(
        "/workflow/prepare",
        jsonText.value,
      );
      assertWorkflowNumbers(lock, "$", true);
      workflow.value = lock.workflow;
    } else {
      const lock = await prepare(true),
        checked = await apiRawJson<{
          scenario: WorkflowScenario;
          result: WorkflowResult;
        }>(
          "/workflow/check",
          `{"workflow":${JSON.stringify(lock)},"scenario":${jsonText.value}}`,
        );
      assertWorkflowNumbers(checked.scenario);
      scenario.value = checked.scenario;
    }
    populateJSON();
    notice.value =
      "Canonical JSON applied. Export, install or record the changed artifact to retain it.";
  });
}
const autosave = useWorkspaceDraft("workflows", {
  workflow,
  scenario,
  tab,
  jsonKind,
  jsonText,
  jsonBaseline,
  jsonSourceStamp,
  showJSON,
  savedDefinition,
  savedScenario,
  installedStamp,
  record,
  recordStamp,
  live,
  liveForm,
  liveInputs,
  liveGeneration,
  savedLiveStamp,
  savedLiveInputs,
});
useActionFeedback("Workflows", busy, notice, error);
function beforeUnload(e: BeforeUnloadEvent) {
  if (autosave.unsaved()) {
    e.preventDefault();
    e.returnValue = "";
  }
}
onMounted(async () => {
  await autosave.ready;
  void initialize();
  window.addEventListener("beforeunload", beforeUnload);
});
onUnmounted(() => window.removeEventListener("beforeunload", beforeUnload));
</script>
<template>
  <section
    class="workflow-workbench"
    :inert="autosave.status.value === 'loading'"
  >
    <DraftStatus :draft="autosave" />
    <header class="wf-header">
      <span class="section-eyebrow"
        >REUSABLE WORKFLOW · HUMAN & ADAPTER BOUNDARIES</span
      >
      <h2>Design top-down. Execute one explicit response at a time.</h2>
      <p>
        Compose a reusable workflow from tasks, exact pure decisions and owned
        state commands. Follow its automatic route, interrupt it, and verify
        independently specified outcomes.
      </p>
    </header>
    <div class="wf-library">
      <label class="field"
        >Reference experiment<select
          v-model="exampleChoice"
          aria-label="Workflow example"
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
        Open workflow example</button
      ><label class="field"
        >Installed workflows<select
          v-model="catalogChoice"
          aria-label="Installed workflow"
          :disabled="!!busy"
        >
          <option value="">Choose exact version</option>
          <option
            v-for="p in catalog"
            :key="workflowKey(p)"
            :value="workflowKey(p)"
          >
            {{ p.workflow.name }} @ {{ p.workflow.version }}
          </option>
        </select></label
      ><button
        class="button secondary"
        :disabled="!!busy || !catalogChoice"
        @click="openPackage"
      >
        Open workflow package</button
      ><button class="button quiet" :disabled="!!busy" @click="initialize">
        Refresh workflow catalog
      </button>
    </div>
    <div class="wf-actions wf-toolbar">
      <strong
        >{{ workflow.name
        }}<small>{{ workflow.id }} @ {{ workflow.version }}</small></strong
      ><span>{{
        pendingJSON
          ? "Unapplied JSON edits"
          : installedStamp === definitionStamp
            ? "Exact workflow installed"
            : "Local workflow draft"
      }}</span
      ><button class="button secondary" :disabled="!!busy" @click="newDraft">
        New workflow</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="requestImport('workflow')"
      >
        Import workflow</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="exportPackage"
      >
        Export workflow</button
      ><button class="button secondary" :disabled="!!busy" @click="validate">
        Validate workflow</button
      ><button class="button secondary" :disabled="!!busy" @click="install">
        Install workflow</button
      ><button class="button primary" :disabled="!!busy" @click="run">
        Run independent scenario
      </button>
    </div>
    <input
      ref="file"
      type="file"
      class="hidden"
      accept=".json,application/json"
      aria-label="Import Workflow JSON"
      @change="importFile"
    />
    <div v-if="error" class="wf-warning" role="alert">
      <strong>{{ error }}</strong>
      <ul>
        <li v-for="(issue, i) in issues" :key="i">
          <button class="issue-link" @click="focusIssue(issue)">
            <code>{{ issue.path }}</code> {{ issue.message }} → Fix
          </button>
        </li>
      </ul>
    </div>
    <p v-if="notice" class="wf-notice" role="status">{{ notice }}</p>
    <p v-if="busy" class="wf-notice" role="status">
      Working with the workflow engine…
    </p>
    <nav class="wf-tabs">
      <button
        v-for="item in [
          ['definition', 'Definition & hierarchy'],
          ['interactive', 'Interactive tasks'],
          ['scenario', 'Scenario & expectations'],
          ['evidence', 'Execution evidence'],
          ['history', 'Run history'],
        ] as const"
        :key="item[0]"
        :class="{ active: tab === item[0] }"
        @click="tab = item[0]"
      >
        {{ item[1] }}
      </button>
    </nav>
    <WorkflowDefinitionEditor
      ref="definitionEditor"
      :issues="issues"
      v-show="tab === 'definition'"
      :workflow="workflow"
      :contracts="contracts"
      :components="components"
      :machines="machines"
      :data-catalog="dataCatalog"
      :primitives="primitives"
      :disabled="!!busy"
      @arranged="
        notice =
          'Flow arrangement applied to the draft. Autosave will retain the positions.'
      "
      @load-decision="loadDecision"
      @load-machine="loadMachine"
      @embed-data="embedData"
      @import-dependency="requestImport"
    /><WorkflowLive
      v-show="tab === 'interactive'"
      :workflow="workflow"
      :inputs="liveInputs"
      :session="live"
      :form="liveForm"
      @form-change="liveForm = $event"
      :generation="liveGeneration"
      :stale="liveStale"
      :disabled="!!busy"
      @start="startLive"
      @event="continueLive"
      @record="recordLive"
      @copy-scenario="copyLive"
      @pending-change="livePending = $event"
    />
    <div v-show="tab === 'scenario'">
      <div class="wf-actions">
        <button
          class="button secondary"
          :disabled="!!busy"
          @click="requestImport('scenario')"
        >
          Import workflow scenario</button
        ><button
          class="button secondary"
          :disabled="!!busy"
          @click="exportScenario"
        >
          Export workflow scenario
        </button>
      </div>
      <WorkflowScenarioEditor
        :workflow="workflow"
        :scenario="scenario"
        :contracts="contracts"
        :tasks="record && !stale ? record.result.tasks : undefined"
        :disabled="!!busy"
        @error="fail"
      />
    </div>
    <div v-show="tab === 'evidence'">
      <WorkflowEvidence
        v-if="record"
        :record="record"
        :stale="stale"
        :disabled="!!busy"
        @load-snapshot="loadSnapshot"
      />
      <p v-else class="wf-panel wf-help">
        Run an independent scenario or save interactive evidence to inspect task
        lifecycles, automatic decisions, owned state and exact primitive traces.
      </p>
    </div>
    <section v-show="tab === 'history'" class="wf-panel">
      <h3>Immutable workflow history</h3>
      <p class="wf-help">
        Opening evidence preserves your current workflow, scenario and
        interactive experiment. Loading its recorded snapshot is a separate
        explicit action.
      </p>
      <div class="wf-history">
        <button
          v-for="r in history"
          :key="r.id"
          :disabled="!!busy"
          @click="openRun(r.id)"
        >
          <strong>{{ r.workflowId }} @ {{ r.workflowVersion }}</strong
          ><span
            >{{ r.verified ? (r.passed ? "PASS" : "FAIL") : "UNVERIFIED" }} ·
            {{ r.status }} · {{ r.outcome || `${r.taskCount} tasks` }} ·
            {{ r.eventCount }} events ·
            {{ new Date(r.createdAt).toLocaleString() }}</span
          >
        </button>
      </div>
      <p v-if="!history.length" class="wf-help">
        No saved workflow experiments yet.
      </p>
    </section>
    <div class="wf-json">
      <div class="wf-actions">
        <button
          class="button quiet"
          :disabled="!!busy"
          @click="openJSON('workflow')"
        >
          Workflow definition JSON</button
        ><button
          class="button quiet"
          :disabled="!!busy"
          @click="openJSON('scenario')"
        >
          Workflow scenario JSON</button
        ><span v-if="pendingJSON"
          >Unapplied {{ jsonKind }} buffer retained</span
        >
      </div>
      <div v-show="showJSON">
        <p class="wf-help">
          Advanced {{ jsonKind }} source is validated by the Go server before
          the browser adopts its canonical response. Pending text is preserved
          when you switch tabs. Exact dependency locks containing negative-zero
          coordinates cannot be preserved by browser JSON and are rejected
          visibly.
        </p>
        <textarea
          v-model="jsonText"
          class="wf-json-editor"
          :aria-label="`Workflow ${jsonKind} JSON source`"
          :disabled="!!busy"
          spellcheck="false"
        />
        <div class="wf-actions">
          <button class="button primary" :disabled="!!busy" @click="applyJSON">
            Apply workflow JSON</button
          ><button
            class="button secondary"
            :disabled="!!busy"
            @click="resetJSON"
          >
            Reset workflow JSON</button
          ><button class="button quiet" @click="showJSON = false">
            Hide JSON (keep buffer)
          </button>
        </div>
      </div>
    </div>
    <p class="wf-help wf-boundary">
      Finite synthetic workflow execution, with exact reusable dependencies and
      explicit task responses. No real adapters, external authorization
      enforcement, clinical decision service, production workflow scheduler or
      continuum deployment coupling is implied.
    </p>
  </section>
</template>
<style>
.workflow-workbench {
  padding: 24px 30px;
  min-width: 0;
  background: #f5f8fa;
}
.workflow-workbench .wf-header h2 {
  font-size: 24px;
  color: #355269;
  margin: 8px 0;
}
.workflow-workbench .wf-header p,
.workflow-workbench .wf-help {
  font-size: 12px;
  line-height: 1.8;
  color: #627d8e;
}
.workflow-workbench .wf-panel,
.workflow-workbench .wf-library {
  background: white;
  border: 1px solid #dce6ed;
  border-radius: 9px;
  padding: 18px;
  margin: 16px 0;
  min-width: 0;
}
.workflow-workbench .wf-library {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: end;
}
.workflow-workbench .wf-library .field {
  flex: 1;
  min-width: 200px;
  margin: 0;
}
.workflow-workbench .button {
  font-size: 12px;
  padding: 9px 12px;
}
.workflow-workbench .wf-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin: 14px 0;
}
.workflow-workbench .wf-actions > strong {
  font-size: 14px;
  color: #3d5e73;
  margin-right: auto;
}
.workflow-workbench .wf-actions > strong small {
  display: block;
  font-size: 11px;
  color: #7a919e;
  margin-top: 5px;
}
.workflow-workbench .wf-actions > span {
  font-size: 11px;
  color: #568475;
}
.workflow-workbench .wf-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 14px;
}
.workflow-workbench .wf-grid > * {
  min-width: 0;
}
.workflow-workbench .wf-card {
  border: 1px solid #e0e9ed;
  background: #fbfdfd;
  border-radius: 7px;
  padding: 15px;
  margin: 12px 0;
  min-width: 0;
}
.workflow-workbench h3 {
  font-size: 16px;
  color: #42667c;
  margin: 0 0 14px;
}
.workflow-workbench h4 {
  font-size: 13px;
  color: #52778b;
  margin: 16px 0 10px;
}
.workflow-workbench summary {
  cursor: pointer;
  font-size: 13px;
  color: #526f83;
  line-height: 1.6;
}
.workflow-workbench details[open] > summary {
  margin-bottom: 15px;
}
.workflow-workbench .wf-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  border-bottom: 1px solid #dce7ed;
  padding-bottom: 10px;
  margin: 18px 0;
}
.workflow-workbench .wf-tabs button {
  padding: 10px 13px;
  background: transparent;
  color: #627e90;
  border-radius: 5px;
  font-size: 12px;
}
.workflow-workbench .wf-tabs button.active {
  background: #e6f4ee;
  color: #23846e;
}
.workflow-workbench .wf-warning {
  background: #fff6e8;
  border: 1px solid #eed9b8;
  color: #93613a;
  padding: 12px 15px;
  border-radius: 7px;
  font-size: 12px;
  line-height: 1.8;
  overflow-wrap: anywhere;
}
.workflow-workbench .wf-notice {
  background: #edf8f2;
  border: 1px solid #d1e7dc;
  color: #3e866d;
  padding: 12px 15px;
  border-radius: 7px;
  font-size: 12px;
  line-height: 1.8;
}
.workflow-workbench .wf-history {
  display: grid;
  gap: 8px;
  max-height: 500px;
  overflow: auto;
}
.workflow-workbench .wf-history button {
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
.workflow-workbench .wf-history span {
  color: #738b98;
}
.workflow-workbench .wf-json-editor {
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
.workflow-workbench .wf-json {
  margin-top: 22px;
}
.workflow-workbench .wf-boundary {
  margin-top: 24px;
}
.workflow-workbench fieldset {
  padding: 0;
  margin: 0;
  border: 0;
  min-width: 0;
}
.workflow-workbench fieldset:disabled {
  opacity: 0.8;
}
.workflow-workbench code {
  overflow-wrap: anywhere;
  font-size: 11px;
}
.workflow-workbench pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 420px;
  overflow: auto;
}
.workflow-workbench .field small {
  font-size: 10px;
  line-height: 1.5;
  color: #7893a2;
}
.workflow-workbench .wf-values input,
.workflow-workbench .wf-values select {
  max-width: 100%;
}
.workflow-workbench .state-help {
  font-size: 11px;
  color: #738d9c;
  line-height: 1.7;
}
.workflow-workbench .state-warning {
  color: #97683e;
  font-size: 11px;
  line-height: 1.7;
}
.workflow-workbench .state-port-row {
  grid-template-columns: minmax(120px, 1fr) minmax(180px, 2fr) auto;
}
.workflow-workbench .state-port-row .button {
  margin-bottom: 16px;
}
@media (max-width: 1100px) {
  .workflow-workbench {
    padding: 20px;
  }
  .workflow-workbench .wf-toolbar > strong {
    width: 100%;
  }
  .workflow-workbench .wf-history button {
    flex-direction: column;
  }
}
@media (max-width: 650px) {
  .workflow-workbench {
    padding: 12px;
  }
  .workflow-workbench .wf-grid {
    grid-template-columns: 1fr;
  }
  .workflow-workbench .wf-library .field {
    min-width: 100%;
  }
  .workflow-workbench .state-port-row {
    display: block;
  }
}
</style>
