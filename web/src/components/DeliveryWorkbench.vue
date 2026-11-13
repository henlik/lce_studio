<script setup lang="ts">
import { useWorkspaceDraft } from "../composables/useWorkspaceDraft";
import { useActionFeedback } from "../composables/useNotifications";
import DraftStatus from "./DraftStatus.vue";
import { focusField } from "../validationFocus";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { api, apiRawJson, APIError } from "../api";
import { cloneBehavior } from "../behaviorTypes";
import {
  stateKey,
  type StateMachineLock,
  type InstalledStateMachine,
} from "../stateTypes";
import {
  deliveryKey,
  deliveryStamp,
  deliveryDataContracts,
  assertDeliveryNumbers,
  assertBrowserExactNumbers,
  type DeliveryModel,
  type DeliveryModelLock,
  type DeliveryScenario,
  type DeliveryExample,
  type InstalledDeliveryModel,
  type DeliveryRun,
  type DeliveryRunSummary,
  type DeliveryResult,
} from "../deliveryTypes";
import type { ValidationIssue } from "../types";
import DeliveryDefinitionEditor from "./DeliveryDefinitionEditor.vue";
import DeliveryScenarioEditor from "./DeliveryScenarioEditor.vue";
import DeliveryEvidence from "./DeliveryEvidence.vue";
function blankModel(): DeliveryModel {
  return {
    schemaVersion: 1,
    id: `local.messages_${crypto.randomUUID().slice(0, 8)}`,
    version: "1.0.0",
    name: "Untitled message model",
    description: "",
    author: "Local author",
    license: "UNLICENSED",
    machines: [],
    instances: [],
    routes: [],
  };
}
function blankScenario(model: DeliveryModel): DeliveryScenario {
  return {
    schemaVersion: 1,
    name: "New message experiment",
    initial: model.instances.map((i) => ({ instanceId: i.id, initial: {} })),
    steps: [],
    expected: [],
    expectedMessages: [],
  };
}
const model = ref<DeliveryModel>(blankModel()),
  scenario = ref<DeliveryScenario>(blankScenario(model.value)),
  examples = ref<DeliveryExample[]>([]),
  catalog = ref<InstalledDeliveryModel[]>([]),
  machines = ref<InstalledStateMachine[]>([]),
  history = ref<DeliveryRunSummary[]>([]);
const tab = ref<"model" | "scenario" | "evidence" | "history">("model"),
  busy = ref(""),
  error = ref(""),
  notice = ref(""),
  issues = ref<ValidationIssue[]>([]),
  selectedExample = ref("0"),
  selectedModel = ref("");
const modelStamp = computed(() => deliveryStamp(model.value)),
  scenarioStamp = computed(() => deliveryStamp(scenario.value)),
  workspaceStamp = computed(() => `${modelStamp.value}|${scenarioStamp.value}`),
  contracts = computed(() => deliveryDataContracts(model.value));
const savedModel = ref(modelStamp.value),
  savedScenario = ref(scenarioStamp.value),
  installedStamp = ref(""),
  record = ref<DeliveryRun | null>(null),
  recordStamp = ref("");
const showJSON = ref(false),
  jsonKind = ref<"model" | "scenario">("model"),
  jsonText = ref(""),
  jsonBaseline = ref(""),
  jsonSourceStamp = ref(""),
  pendingJSON = computed(() => jsonText.value !== jsonBaseline.value),
  dirty = computed(
    () =>
      modelStamp.value !== savedModel.value ||
      scenarioStamp.value !== savedScenario.value ||
      pendingJSON.value,
  ),
  stale = computed(
    () =>
      !!record.value &&
      (recordStamp.value !== workspaceStamp.value || pendingJSON.value),
  );
const file = ref<HTMLInputElement | null>(null),
  fileKind = ref<"model" | "scenario" | "machine">("model");
const definitionEditor = ref<InstanceType<
  typeof DeliveryDefinitionEditor
> | null>(null);
function focusIssue(issue: ValidationIssue) {
  if (issue.path.startsWith("scenario")) {
    tab.value = "scenario";
    focusField(
      document.querySelector(".delivery-workbench"),
      '[aria-label="Message scenario name"]',
    );
  } else {
    tab.value = "model";
    definitionEditor.value?.focusIssue(issue);
  }
}
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
      "Replace this message model and scenario? Export them or retain a recorded snapshot before discarding current edits.",
    )
  );
}
function adopt(
  lock: DeliveryModelLock,
  value?: DeliveryScenario,
  installed = false,
) {
  assertBrowserExactNumbers(lock);
  model.value = cloneBehavior(lock.model);
  scenario.value = value ? cloneBehavior(value) : blankScenario(lock.model);
  savedModel.value = modelStamp.value;
  savedScenario.value = scenarioStamp.value;
  installedStamp.value = installed ? modelStamp.value : "";
  jsonText.value = "";
  jsonBaseline.value = "";
  showJSON.value = false;
}
function requireApplied() {
  if (pendingJSON.value)
    throw new Error(
      "Apply or reset pending Messages JSON before validating, running, installing or exporting.",
    );
}
async function prepare() {
  requireApplied();
  assertDeliveryNumbers(model.value);
  return api<DeliveryModelLock>(
    "/delivery/prepare",
    "POST",
    cloneBehavior(model.value),
  );
}
async function initialize() {
  await work("load", async () => {
    const results = await Promise.allSettled([
      api<DeliveryExample[]>("/delivery/examples"),
      api<InstalledDeliveryModel[]>("/delivery/models"),
      api<DeliveryRunSummary[]>("/delivery/runs"),
      api<InstalledStateMachine[]>("/state/machines"),
    ]);
    if (results[0].status === "fulfilled") {
      examples.value = results[0].value;
    } else fail(results[0].reason);
    if (results[1].status === "fulfilled") catalog.value = results[1].value;
    else fail(results[1].reason);
    if (results[2].status === "fulfilled") history.value = results[2].value;
    else fail(results[2].reason);
    if (results[3].status === "fulfilled") machines.value = results[3].value;
    else fail(results[3].reason);
  });
}
function newDraft() {
  if (busy.value || !replaceAllowed()) return;
  model.value = blankModel();
  scenario.value = blankScenario(model.value);
  savedModel.value = "";
  savedScenario.value = "";
  installedStamp.value = "";
  jsonText.value = "";
  jsonBaseline.value = "";
  showJSON.value = false;
  tab.value = "model";
  error.value = "";
  issues.value = [];
  notice.value =
    "Empty message model and scenario. Embed exact state machines, add instances and routes, then define explicit dispatch/delivery steps and independent expectations.";
}
function openExample() {
  const example = examples.value[Number(selectedExample.value)];
  if (busy.value || !example || !replaceAllowed()) return;
  try {
    adopt(example.model, example.scenario);
  } catch (e) {
    fail(e);
    return;
  }
  tab.value = "scenario";
  notice.value =
    "Reference owners, typed routes and handwritten expectations loaded. Run to inspect transport and business outcomes separately.";
}
function openModel() {
  const lock = catalog.value.find(
    (l) => deliveryKey(l) === selectedModel.value,
  );
  if (busy.value || !lock || !replaceAllowed()) return;
  try {
    adopt(lock, undefined, true);
  } catch (e) {
    fail(e);
    return;
  }
  tab.value = "scenario";
  notice.value =
    "Exact model loaded with a fresh initialized scenario. Add explicit steps before running.";
}
function instanceAdded(id: string) {
  if (!scenario.value.initial.some((i) => i.instanceId === id))
    scenario.value.initial.push({ instanceId: id, initial: {} });
}
function addMachine(lock: StateMachineLock) {
  assertBrowserExactNumbers(lock);
  const existing = model.value.machines.find(
    (l) =>
      l.machine.id === lock.machine.id &&
      l.machine.version === lock.machine.version,
  );
  if (existing) {
    if (existing.digest !== lock.digest)
      throw new Error(
        "This machine ID/version is already pinned to different content.",
      );
    notice.value = "This exact owner machine is already embedded.";
    return;
  }
  if (model.value.machines.length >= 4)
    throw new Error(
      "A message model may embed at most four exact machine locks.",
    );
  model.value.machines.push(
    cloneBehavior({ machine: lock.machine, digest: lock.digest }),
  );
  notice.value =
    "Exact machine embedded. Add an instance or select this lock on an existing instance.";
}
function embedMachine(key: string) {
  if (busy.value) return;
  const lock = machines.value.find((l) => stateKey(l) === key);
  if (!lock) return;
  try {
    addMachine(lock);
  } catch (e) {
    fail(e);
  }
}
async function validateModel() {
  await work("validate", async () => {
    await prepare();
    notice.value =
      "Exact machine locks, instances, nominal route mappings and model bounds are valid.";
  });
}
async function install() {
  await work("install", async () => {
    const stamp = modelStamp.value,
      lock = await prepare();
    await api("/delivery/models", "POST", lock);
    savedModel.value = stamp;
    installedStamp.value = stamp;
    catalog.value = await api<InstalledDeliveryModel[]>("/delivery/models");
    notice.value =
      "Exact message model installed. Scenario edits autosave separately on this browser. Export or record them for a portable checkpoint.";
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
async function exportModel() {
  await work("export", async () => {
    const stamp = modelStamp.value,
      lock = await prepare();
    download(lock, `${lock.model.id}-${lock.model.version}.messages.json`);
    savedModel.value = stamp;
    notice.value =
      "Exact model exported with all state machine and transitive locks. Keep its scenario alongside it.";
  });
}
async function exportScenario() {
  await work("export", async () => {
    requireApplied();
    assertDeliveryNumbers(model.value, scenario.value);
    const stamp = scenarioStamp.value,
      snapshot = cloneBehavior(scenario.value),
      lock = await prepare(),
      checked = await api<{
        scenario: DeliveryScenario;
        result: DeliveryResult;
      }>("/delivery/check", "POST", { model: lock, scenario: snapshot });
    download(checked.scenario, `${lock.model.id}.message-scenario.json`);
    savedScenario.value = stamp;
    notice.value =
      "Canonical scenario exported after checking it against this exact model. No run was stored.";
  });
}
async function run() {
  await work("run", async () => {
    requireApplied();
    assertDeliveryNumbers(model.value, scenario.value);
    const stamp = workspaceStamp.value,
      snapshot = cloneBehavior(scenario.value),
      lock = await prepare();
    record.value = await api<DeliveryRun>("/delivery/runs", "POST", {
      model: lock,
      scenario: snapshot,
    });
    recordStamp.value = stamp;
    tab.value = "evidence";
    history.value = await api<DeliveryRunSummary[]>("/delivery/runs");
    notice.value = record.value.result.verified
      ? record.value.result.passed
        ? "Independent owner/message expectations passed. Complete evidence saved."
        : "Independent expectations failed. Complete evidence saved for inspection."
      : "Scenario completed without independent expectations. Evidence is saved as unverified.";
  });
}
async function openRun(id: string) {
  await work("history", async () => {
    record.value = await api<DeliveryRun>(
      `/delivery/runs/${encodeURIComponent(id)}`,
    );
    recordStamp.value = `${deliveryStamp(record.value.model.model)}|${deliveryStamp(record.value.scenario)}`;
    tab.value = "evidence";
    notice.value =
      "Viewing saved evidence without replacing the current model or scenario.";
  });
}
function loadSnapshot() {
  if (!record.value || busy.value || !replaceAllowed()) return;
  try {
    adopt(record.value.model, record.value.scenario);
  } catch (e) {
    fail(e);
    return;
  }
  recordStamp.value = workspaceStamp.value;
  tab.value = "scenario";
  notice.value =
    "Recorded model and scenario loaded. Rerunning initializes owners and transport from the declared experiment.";
}
function requestImport(kind: "model" | "scenario" | "machine") {
  if (busy.value) return;
  fileKind.value = kind;
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
  const kind = fileKind.value;
  const discardPendingAfterImport = kind === "scenario" && pendingJSON.value;
  if (
    (kind === "model" && !replaceAllowed()) ||
    (kind === "scenario" &&
      (savedScenario.value !== scenarioStamp.value || pendingJSON.value) &&
      !confirm(
        discardPendingAfterImport
          ? "Replace this message scenario and discard pending Messages JSON after the import succeeds? Export or record existing edits first to retain them. A rejected import keeps the current draft and JSON."
          : "Replace this message scenario? Export or record it first to retain edits.",
      ))
  ) {
    input.value = "";
    return;
  }
  await work("import", async () => {
    if (selected.size > 2 * 1024 * 1024)
      throw new Error("JSON imports must be at most 2 MiB.");
    const raw = await selected.text();
    if (kind === "model") {
      const checked = await apiRawJson<{
        valid: boolean;
        issues: ValidationIssue[];
        model?: DeliveryModelLock;
      }>("/delivery/validate-lock", raw);
      if (!checked.valid || !checked.model)
        throw new APIError("Message model import rejected.", checked.issues);
      adopt(checked.model);
      tab.value = "model";
      notice.value =
        "Exact model imported from the canonical validated server response.";
    } else if (kind === "scenario") {
      // Replacement was confirmed. Keep the existing buffer until both canonical checks succeed.
      assertDeliveryNumbers(model.value);
      const lock = await api<DeliveryModelLock>(
          "/delivery/prepare",
          "POST",
          cloneBehavior(model.value),
        ),
        checked = await apiRawJson<{
          scenario: DeliveryScenario;
          result: DeliveryResult;
        }>(
          "/delivery/check",
          `{"model":${JSON.stringify(lock)},"scenario":${raw}}`,
        );
      scenario.value = checked.scenario;
      if (discardPendingAfterImport) {
        jsonText.value = "";
        jsonBaseline.value = "";
        jsonSourceStamp.value = workspaceStamp.value;
        showJSON.value = false;
      }
      tab.value = "scenario";
      notice.value =
        "Scenario checked from original JSON before canonical values were adopted. No run was stored.";
    } else {
      const checked = await apiRawJson<{
        valid: boolean;
        issues: ValidationIssue[];
        machine?: StateMachineLock;
      }>("/state/validate-lock", raw);
      if (!checked.valid || !checked.machine)
        throw new APIError("Owner machine import rejected.", checked.issues);
      addMachine(checked.machine);
    }
  });
  input.value = "";
}
function populateJSON() {
  assertDeliveryNumbers(
    model.value,
    jsonKind.value === "scenario" ? scenario.value : undefined,
  );
  jsonText.value = JSON.stringify(
    jsonKind.value === "model" ? model.value : scenario.value,
    null,
    2,
  );
  jsonBaseline.value = jsonText.value;
  jsonSourceStamp.value = workspaceStamp.value;
}
function openJSON(kind: "model" | "scenario") {
  if (busy.value) return;
  try {
    if (!pendingJSON.value || jsonKind.value !== kind) {
      // Keep both the old buffer and its kind if the target draft cannot be serialized.
      assertDeliveryNumbers(
        model.value,
        kind === "scenario" ? scenario.value : undefined,
      );
      if (
        pendingJSON.value &&
        jsonKind.value !== kind &&
        !confirm("Discard pending Messages JSON and open another editor?")
      )
        return;
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
    (pendingJSON.value && !confirm("Discard unapplied Messages JSON edits?"))
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
        "The graphical model or scenario changed after this JSON opened. Apply the JSON over newer content?",
      ))
  )
    return;
  await work("json", async () => {
    if (jsonKind.value === "model") {
      const lock = await apiRawJson<DeliveryModelLock>(
        "/delivery/prepare",
        jsonText.value,
      );
      assertBrowserExactNumbers(lock);
      model.value = lock.model;
    } else {
      assertDeliveryNumbers(model.value);
      const lock = await api<DeliveryModelLock>(
          "/delivery/prepare",
          "POST",
          cloneBehavior(model.value),
        ),
        checked = await apiRawJson<{
          scenario: DeliveryScenario;
          result: DeliveryResult;
        }>(
          "/delivery/check",
          `{"model":${JSON.stringify(lock)},"scenario":${jsonText.value}}`,
        );
      scenario.value = checked.scenario;
    }
    populateJSON();
    notice.value =
      "Canonical JSON applied. Install, export or record the changed artifact to retain it.";
  });
}
const autosave = useWorkspaceDraft("messages", {
  model,
  scenario,
  tab,
  jsonKind,
  jsonText,
  jsonBaseline,
  jsonSourceStamp,
  showJSON,
  savedModel,
  savedScenario,
  installedStamp,
  record,
  recordStamp,
});
useActionFeedback("Messages", busy, notice, error);
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
</script>
<template>
  <section
    class="state-workbench delivery-workbench"
    :inert="autosave.status.value === 'loading'"
  >
    <DraftStatus :draft="autosave" />
    <header>
      <span class="section-eyebrow">TYPED MESSAGE DELIVERY</span>
      <h2>Follow an intention from one owner to another</h2>
      <p>
        Connect exact state machines through typed routes. Explicitly dispatch,
        deliver, retry and restart, then inspect acknowledgement separately from
        business acceptance.
      </p>
    </header>
    <div class="state-guide">
      <span><b>1</b> Pin owners & map routes</span
      ><span><b>2</b> Declare every transport step</span
      ><span><b>3</b> Verify state & message outcomes</span>
    </div>
    <div class="state-library">
      <label class="field"
        >Reference experiment<select
          v-model="selectedExample"
          aria-label="Message example"
          :disabled="!!busy"
        >
          <option v-for="(example, i) in examples" :key="i" :value="String(i)">
            {{ example.scenario.name }}
          </option>
        </select></label
      ><button
        class="button secondary"
        :disabled="!!busy || !examples.length"
        @click="openExample"
      >
        Open message example</button
      ><label class="field"
        >Installed message models<select
          v-model="selectedModel"
          aria-label="Installed message model"
          :disabled="!!busy"
        >
          <option value="">Select an exact version</option>
          <option
            v-for="lock in catalog"
            :key="deliveryKey(lock)"
            :value="deliveryKey(lock)"
          >
            {{ lock.model.id }} @ {{ lock.model.version }}
          </option>
        </select></label
      ><button
        class="button secondary"
        :disabled="!!busy || !selectedModel"
        @click="openModel"
      >
        Open message model</button
      ><button class="button quiet" :disabled="!!busy" @click="initialize">
        Refresh Messages
      </button>
    </div>
    <div class="state-actions">
      <strong
        >{{ model.name
        }}<small>{{ model.id }} @ {{ model.version }}</small></strong
      ><span>{{
        pendingJSON
          ? "Unapplied JSON edits"
          : installedStamp === modelStamp
            ? "Exact model installed"
            : "Local message draft"
      }}</span
      ><button class="button secondary" :disabled="!!busy" @click="newDraft">
        New message model</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="requestImport('model')"
      >
        Import message model</button
      ><button class="button secondary" :disabled="!!busy" @click="exportModel">
        Export message model</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="validateModel"
      >
        Validate message model</button
      ><button class="button secondary" :disabled="!!busy" @click="install">
        Install message model</button
      ><button class="button primary" :disabled="!!busy" @click="run">
        Run message scenario
      </button>
    </div>
    <input
      ref="file"
      class="hidden"
      type="file"
      accept=".json,application/json"
      aria-label="Import Messages JSON"
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
      Working with the message engine…
    </p>
    <nav class="state-tabs">
      <button :class="{ active: tab === 'model' }" @click="tab = 'model'">
        Owners & routes</button
      ><button
        :class="{ active: tab === 'scenario' }"
        @click="tab = 'scenario'"
      >
        Delivery scenario</button
      ><button
        :class="{ active: tab === 'evidence' }"
        @click="tab = 'evidence'"
      >
        Message evidence</button
      ><button :class="{ active: tab === 'history' }" @click="tab = 'history'">
        Message run history
      </button>
    </nav>
    <section
      v-if="tab === 'model' && !model.instances.length"
      class="state-panel"
    >
      <p class="state-help">
        This message model is empty. Import or embed an exact state machine
        below, then use Add message instance and connect its typed routes.
      </p>
    </section>
    <DeliveryDefinitionEditor
      ref="definitionEditor"
      v-show="tab === 'model'"
      :model="model"
      :machines="machines"
      :disabled="!!busy"
      @embed-machine="embedMachine"
      @import-machine="requestImport('machine')"
      @instance-added="instanceAdded"
    />
    <div v-show="tab === 'scenario'">
      <p v-if="!model.instances.length" class="state-panel state-help">
        Add an owner in Owners &amp; routes first. Then choose each command,
        dispatch, delivery or restart explicitly in this scenario.
      </p>
      <div class="state-actions">
        <button
          class="button secondary"
          :disabled="!!busy"
          @click="requestImport('scenario')"
        >
          Import message scenario</button
        ><button
          class="button secondary"
          :disabled="!!busy"
          @click="exportScenario"
        >
          Export message scenario
        </button>
      </div>
      <DeliveryScenarioEditor
        :model="model"
        :scenario="scenario"
        :contracts="contracts"
        :disabled="!!busy"
      />
    </div>
    <div v-if="tab === 'evidence'">
      <DeliveryEvidence
        v-if="record"
        :record="record"
        :stale="stale"
        :disabled="!!busy"
        @load-snapshot="loadSnapshot"
      />
      <p v-else class="state-panel state-help">
        Run a scenario to record frozen envelopes, acknowledgements, owned state
        and step evidence.
      </p>
    </div>
    <section v-if="tab === 'history'" class="state-panel">
      <h3>Immutable message run history</h3>
      <p class="state-help">
        Opening evidence does not replace your draft. Load its recorded snapshot
        explicitly to reproduce the experiment.
      </p>
      <div class="state-history">
        <button
          v-for="run in history"
          :key="run.id"
          :disabled="!!busy"
          @click="openRun(run.id)"
        >
          <strong>{{ run.modelId }} @ {{ run.modelVersion }}</strong
          ><span
            >{{
              run.verified ? (run.passed ? "PASS" : "FAIL") : "UNVERIFIED"
            }}
            · {{ run.stepCount }} steps · {{ run.messageCount }} messages ·
            {{ new Date(run.createdAt).toLocaleString() }}</span
          >
        </button>
      </div>
      <p v-if="!history.length" class="state-help">
        No recorded message scenarios yet.
      </p>
    </section>
    <div class="state-json">
      <button
        class="button quiet"
        :disabled="!!busy"
        @click="openJSON('model')"
      >
        Message model JSON</button
      ><button
        class="button quiet"
        :disabled="!!busy"
        @click="openJSON('scenario')"
      >
        Message scenario JSON</button
      ><template v-if="showJSON"
        ><p class="state-help">
          Original {{ jsonKind }} JSON is checked by Go before the browser
          adopts canonical values. Pending text remains when hidden.
        </p>
        <p v-if="jsonSourceStamp !== workspaceStamp" class="state-warning">
          The graphical model or scenario changed after this JSON opened.
          Applying it requires confirmation before replacing newer content.
        </p>
        <textarea
          v-model="jsonText"
          aria-label="Advanced Messages JSON"
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
            Validate & apply Messages JSON</button
          ><button class="button quiet" :disabled="!!busy" @click="resetJSON">
            Reset Messages JSON</button
          ><button class="button quiet" @click="showJSON = false">
            Hide Messages JSON
          </button>
        </div></template
      >
    </div>
    <p class="state-help state-boundary">
      This experiment assumes messages and acknowledgements survive modeled
      owner restarts. It has no real broker, automatic delivery, hidden retry,
      clock, external I/O or resource timing. PostgreSQL stores definitions and
      evidence, not live business transactions.
    </p>
  </section>
</template>
<style>
.delivery-workbench .state-card > summary {
  cursor: pointer;
  font-size: 13px;
  line-height: 1.8;
  color: #53788c;
}
.delivery-workbench .state-card[open] > summary {
  margin-bottom: 16px;
}
.delivery-workbench .state-help {
  overflow-wrap: anywhere;
}
.delivery-workbench .state-grid label small {
  font-weight: 400;
  line-height: 1.6;
  color: #78909e;
}
</style>
