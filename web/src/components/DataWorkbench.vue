<script setup lang="ts">
import { watch as watchInspection } from "vue";
const inspectionProps = defineProps<{ inspectionRequest?: {nonce: number; body: Record<string, any>} }>();
import WorkspaceModes from "./WorkspaceModes.vue";
import { useWorkspaceDraft } from "../composables/useWorkspaceDraft";
import { useActionFeedback } from "../composables/useNotifications";
import DraftStatus from "./DraftStatus.vue";
import { focusField } from "../validationFocus";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { api, apiRawJson, APIError } from "../api";
import type { ValidationIssue } from "../types";
import {
  cloneBehavior,
  scalarTypes,
  stableBehavior,
  typedDefault,
} from "../behaviorTypes";
import {
  dataKey,
  splitFields,
  type DataContract,
  type DataContractLock,
  type DataContractRecord,
  type DataFixture,
  type DataExample,
  type DataValidationRecord,
  type DataValidationSummary,
  type DataComparison,
} from "../dataTypes";
import type { DataSelection } from "../dataCanvas";
import DataGraph from "./DataGraph.vue";
import DataFixtureEditor from "./DataFixtureEditor.vue";
function blank(): DataContract {
  return {
    schemaVersion: 1,
    id: `local.data_${crypto.randomUUID().slice(0, 8)}`,
    version: "1.0.0",
    name: "Untitled data contract",
    description: "",
    author: "Local author",
    license: "UNLICENSED",
    records: [],
    entities: [],
    relationships: [],
  };
}
const draft = ref<DataContract>(blank()),
  fixture = ref<DataFixture>({ name: "New fixture", rows: {} }),
  examples = ref<DataExample[]>([]),
  catalog = ref<DataContractRecord[]>([]),
  history = ref<DataValidationSummary[]>([]);
const tab = ref<"model" | "relationships" | "fixture" | "history" | "compare">(
    "model",
  ),
  busy = ref(""),
  error = ref(""),
  notice = ref(""),
  issues = ref<ValidationIssue[]>([]),
  selectedExample = ref("0"),
  selectedCatalog = ref(""),
  selectedEntity = ref(""),
  selectedRecord = ref("");
const canvasSelection = ref<DataSelection | null>(null);
const recordPositions = ref<Record<string, {x:number;y:number}>>({});
const canvasMode = ref<"properties" | "fields">("properties");
const connectionSource = ref<number | null>(null);
const baselineContract = ref(""),
  baselineFixture = ref(""),
  installedStamp = ref(""),
  record = ref<DataValidationRecord | null>(null),
  recordStamp = ref(""),
  comparison = ref<DataComparison | null>(null),
  comparisonStamp = ref(""),
  comparedLabels = ref(""),
  oldKey = ref("");
const file = ref<HTMLInputElement | null>(null),
  fileKind = ref<"contract" | "fixture">("contract"),
  jsonKind = ref<"contract" | "fixture">("contract"),
  jsonText = ref(""),
  jsonBaseline = ref(""),
  jsonSourceStamp = ref(""),
  showJSON = ref(false);
const draftStamp = computed(() => stableBehavior(draft.value)),
  fixtureStamp = computed(() => stableBehavior(fixture.value)),
  workspaceStamp = computed(() => `${draftStamp.value}|${fixtureStamp.value}`),
  pendingJSON = computed(() => jsonText.value !== jsonBaseline.value),
  dirty = computed(
    () =>
      draftStamp.value !== baselineContract.value ||
      fixtureStamp.value !== baselineFixture.value ||
      pendingJSON.value,
  ),
  stale = computed(
    () =>
      !!record.value &&
      (recordStamp.value !== workspaceStamp.value || pendingJSON.value),
  );
const currentRecord = computed(
  () =>
    draft.value.records.find((r) => r.name === selectedRecord.value) ||
    draft.value.records[0],
);
const currentEntity = computed(
  () =>
    draft.value.entities.find((e) => e.name === selectedEntity.value) ||
    draft.value.entities[0],
);
const canvasRecord = computed(() => canvasSelection.value?.kind === "entity"
  ? draft.value.records.find(r => r.name === currentEntity.value?.record) : currentRecord.value);
const canvasIssues = computed(() => {
 const selection = canvasSelection.value;
 if (!selection) return [];
 const prefix = `${selection.kind === 'entity' ? 'entities' : selection.kind === 'record' ? 'records' : 'relationships'}[${selection.index}]`;
 const recordIndex = draft.value.records.findIndex(r => r === canvasRecord.value);
 return issues.value.filter(i => i.path.startsWith(prefix) || (canvasMode.value === 'fields' && i.path.startsWith('records[' + recordIndex + ']')));
});
function selectCanvas(selection: DataSelection) {
 canvasSelection.value = selection; canvasMode.value = 'properties';
 if (selection.kind === 'entity') { selectedEntity.value = draft.value.entities[selection.index]?.name || ''; selectedRecord.value = currentEntity.value?.record || ''; }
 if (selection.kind === 'record') selectedRecord.value = draft.value.records[selection.index]?.name || '';
}
function closeCanvasEditor() { canvasSelection.value = null; canvasMode.value = 'properties'; }
function removeCanvasRecord() {
 const item=canvasRecord.value; if(!item || busy.value) return;
 draft.value.records = draft.value.records.filter(r => r !== item);
 delete recordPositions.value[item.name]; closeCanvasEditor();
}
function removeCanvasEntity() {
 if(busy.value) return;
 draft.value.entities = draft.value.entities.filter(e => e !== currentEntity.value);
 closeCanvasEditor(); connectionSource.value=null;
}
function removeCanvasRelationship(index:number) {
 if(busy.value) return;
 draft.value.relationships.splice(index,1);closeCanvasEditor();
}
function connectEntities(from:number,to:number) {
 if(busy.value) return;
 addRelationship(from,to); connectionSource.value=null;
}
baselineContract.value = draftStamp.value;
baselineFixture.value = fixtureStamp.value;
function focusIssue(issue: ValidationIssue) {
  const entity = issue.path.match(/entities\[(\d+)\]/),
    record = issue.path.match(/records\[(\d+)\]/),
    relationship = issue.path.match(/relationships\[(\d+)\]/);
  tab.value = relationship
    ? "relationships"
    : issue.path.includes("fixture")
      ? "fixture"
      : "model";
  if (entity) selectCanvas({kind:'entity',index:Number(entity[1])});
  if (record) selectCanvas({kind:'record',index:Number(record[1])});
  if (relationship) selectCanvas({kind:'relationship',index:Number(relationship[1])});
  const field = issue.path.match(/fields\[(\d+)\]/);
  focusField(
    document.querySelector(".data-workbench"),
    entity
      ? (issue.path.includes('primaryKey') ? '[aria-label="Entity primary key"]' : '[aria-label="Entity name"]')
      : record
        ? (field ? `[aria-label="Record field ${Number(field[1])+1} ${issue.path.endsWith('.type') ? 'type' : 'name'}"]` : '[aria-label="Record type name"]')
        : relationship
          ? `[aria-label="Relationship ${Number(relationship[1]) + 1} name"]`
          : '[aria-label="Data contract ID"]',
  );
}
function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e);
  issues.value = e instanceof APIError ? e.issues : [];
}
async function work(name: string, fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = name;
  error.value = "";
  issues.value = [];
  notice.value = "";
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
      "Replace this data draft and fixture? Export them or install the contract first to retain your work.",
    )
  );
}
function adopt(lock: DataContractLock, data?: DataFixture, installed = false) {
  closeCanvasEditor(); connectionSource.value=null; recordPositions.value={};
  draft.value = cloneBehavior(lock.contract);
  fixture.value = data
    ? cloneBehavior(data)
    : { name: "New fixture", rows: {} };
  selectedRecord.value = draft.value.records[0]?.name || "";
  selectedEntity.value = draft.value.entities[0]?.name || "";
  jsonText.value = "";
  jsonBaseline.value = "";
  showJSON.value = false;
  baselineContract.value = draftStamp.value;
  baselineFixture.value = fixtureStamp.value;
  installedStamp.value = installed ? draftStamp.value : "";
}
function requireApplied() {
  if (pendingJSON.value)
    throw new Error(
      "Apply or reset the pending JSON edits before validating, installing or exporting.",
    );
}
async function prepare(snapshot = cloneBehavior(draft.value)) {
  requireApplied();
  return api<DataContractLock>("/data/prepare", "POST", snapshot);
}
async function initialize() {
  await work("load", async () => {
    const results = await Promise.allSettled([
      api<DataExample[]>("/data/examples"),
      api<DataContractRecord[]>("/data/contracts"),
      api<DataValidationSummary[]>("/data/validations"),
    ]);
    if (results[0].status === "fulfilled") {
      examples.value = results[0].value;
    } else fail(results[0].reason);
    if (results[1].status === "fulfilled") catalog.value = results[1].value;
    else fail(results[1].reason);
    if (results[2].status === "fulfilled") history.value = results[2].value;
    else fail(results[2].reason);
  });
}
function newDraft() {
  if (busy.value || !replaceAllowed()) return;
  closeCanvasEditor(); connectionSource.value=null; recordPositions.value={};
  draft.value = blank();
  fixture.value = { name: "New fixture", rows: {} };
  selectedRecord.value = "";
  selectedEntity.value = "";
  baselineContract.value = "";
  baselineFixture.value = "";
  installedStamp.value = "";
  jsonText.value = "";
  jsonBaseline.value = "";
  showJSON.value = false;
  tab.value = "model";
  error.value = "";
  issues.value = [];
  notice.value =
    "Empty data contract. Add a record type, define its fields, then add entities and relationships as needed.";
}
function openExample() {
  const e = examples.value[Number(selectedExample.value)];
  if (busy.value || !e || !replaceAllowed()) return;
  adopt(e.contract, e.fixture);
  tab.value = "model";
  notice.value =
    "Portable example loaded. Its fixture is a complete snapshot for this exact contract.";
}
function openCatalog() {
  const e = catalog.value.find((c) => dataKey(c) === selectedCatalog.value);
  if (!e || busy.value || !replaceAllowed()) return;
  adopt(e, undefined, true);
  tab.value = "model";
}
async function validateDefinition() {
  await work("validate", async () => {
    await prepare();
    notice.value =
      "Data contract is valid. Record types, entity keys and relationships are consistent.";
  });
}
async function install() {
  await work("install", async () => {
    const snapshot = cloneBehavior(draft.value),
      stamp = draftStamp.value,
      lock = await prepare(snapshot);
    await api("/data/contracts", "POST", lock);
    installedStamp.value = stamp;
    baselineContract.value = stamp;
    catalog.value = await api<DataContractRecord[]>("/data/contracts");
    notice.value =
      "Exact contract installed. Fixture edits are separate; export them to retain them.";
  });
}
function download(value: unknown, name: string) {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(value, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}
async function exportContract() {
  await work("export", async () => {
    const stamp = draftStamp.value,
      lock = await prepare();
    download(lock, `${lock.contract.id}-${lock.contract.version}.data.json`);
    baselineContract.value = stamp;
    notice.value =
      "Exact data contract exported. Export the fixture separately if needed.";
  });
}
async function exportFixture() {
  await work("export", async () => {
    const stamp = fixtureStamp.value,
      lock = await prepare(),
      snapshot = cloneBehavior(fixture.value);
    await api("/data/check", "POST", { contract: lock, fixture: snapshot });
    download(snapshot, `${draft.value.id}.fixture.json`);
    baselineFixture.value = stamp;
    notice.value = "Fixture exported. Keep its exact contract alongside it.";
  });
}
async function importFile(e: Event) {
  const input = e.target as HTMLInputElement,
    f = input.files?.[0];
  if (!f) return;
  if (busy.value || !replaceAllowed()) {
    input.value = "";
    return;
  }
  await work("import", async () => {
    if (f.size > 2 * 1024 * 1024)
      throw new Error("Import must be at most 2 MiB.");
    const raw = await f.text();
    if (fileKind.value === "contract") {
      const result = await apiRawJson<{
        valid: boolean;
        issues: ValidationIssue[];
        lock?: DataContractLock;
      }>("/data/validate-lock", raw);
      if (!result.valid || !result.lock)
        throw new APIError("Data contract import rejected.", result.issues);
      adopt(result.lock);
      tab.value = "model";
    } else {
      const lock = await prepare();
      const result = await apiRawJson<{ fixture: DataFixture }>(
        "/data/check",
        `{"contract":${JSON.stringify(lock)},"fixture":${raw}}`,
      );
      fixture.value = result.fixture;
      tab.value = "fixture";
    }
    notice.value =
      "Import checked from original JSON and loaded from the canonical server response.";
  });
  input.value = "";
}
function requestImport(kind: "contract" | "fixture") {
  fileKind.value = kind;
  file.value?.click();
}
function openJSON(kind: "contract" | "fixture") {
  if (busy.value) return;
  if (
    pendingJSON.value &&
    jsonKind.value !== kind &&
    !confirm("Discard pending JSON text and open another editor?")
  )
    return;
  if (!pendingJSON.value || jsonKind.value !== kind) {
    jsonKind.value = kind;
    populateJSON();
  }
  showJSON.value = true;
}
function populateJSON() {
  jsonText.value = JSON.stringify(
    jsonKind.value === "contract" ? draft.value : fixture.value,
    null,
    2,
  );
  jsonBaseline.value = jsonText.value;
  jsonSourceStamp.value = workspaceStamp.value;
}
function resetJSON() {
  if (
    busy.value ||
    (pendingJSON.value && !confirm("Discard unapplied JSON edits?"))
  )
    return;
  populateJSON();
}
async function applyJSON() {
  if (
    busy.value ||
    (jsonSourceStamp.value !== workspaceStamp.value &&
      !confirm(
        "The graphical data draft or fixture changed after this JSON editor was opened. Apply this JSON over the current content?",
      ))
  )
    return;
  await work("json", async () => {
    if (jsonKind.value === "contract") {
      const lock = await apiRawJson<DataContractLock>(
        "/data/prepare",
        jsonText.value,
      );
      draft.value = lock.contract;
    } else {
      const lock = await api<DataContractLock>(
        "/data/prepare",
        "POST",
        cloneBehavior(draft.value),
      );
      const result = await apiRawJson<{ fixture: DataFixture }>(
        "/data/check",
        `{"contract":${JSON.stringify(lock)},"fixture":${jsonText.value}}`,
      );
      fixture.value = result.fixture;
    }
    populateJSON();
    notice.value =
      "Validated JSON applied. Export the changed contract or fixture to retain it.";
  });
}
function fresh(prefix: string, used: string[]) {
  let i = 1;
  while (used.includes(prefix + i)) i++;
  return prefix + i;
}
function addRecord() {
  if (draft.value.records.length >= 32) return;
  const name = fresh(
    "Record",
    draft.value.records.map((r) => r.name),
  );
  draft.value.records.push({ name, fields: [{ name: "id", type: "string" }] });
  selectedRecord.value = name; selectCanvas({kind:'record',index:draft.value.records.length-1});
}
function addEntity(recordName?: string) {
  if (recordName) selectedRecord.value=recordName;
  if (draft.value.entities.length >= 32 || !currentRecord.value) return;
  const name = fresh(
    "Entity",
    draft.value.entities.map((e) => e.name),
  );
  draft.value.entities.push({
    name,
    record: currentRecord.value.name,
    primaryKey: [currentRecord.value.fields[0]?.name || "id"],
    uniqueKeys: [],
    position: {
      x: 40 + (draft.value.entities.length % 3) * 390,
      y: 40 + Math.floor(draft.value.entities.length / 3) * 300,
    },
  });
  selectedEntity.value = name; selectCanvas({kind:'entity',index:draft.value.entities.length-1});
}
function addField() {
  if (!canvasRecord.value || canvasRecord.value.fields.length >= 16) return;
  canvasRecord.value.fields.push({
    name: fresh(
      "field",
      canvasRecord.value.fields.map((f) => f.name),
    ),
    type: "string",
  });
}
function addRelationship(from=0, to=1) {
  if (draft.value.relationships.length >= 64 || !draft.value.entities.length)
    return;
  const a = draft.value.entities[from] || draft.value.entities[0],
    b = draft.value.entities[to] || a;
  draft.value.relationships.push({
    name: fresh(
      "Reference",
      draft.value.relationships.map((r) => r.name),
    ),
    fromEntity: a.name,
    fromFields: [...a.primaryKey],
    toEntity: b.name,
    toFields: [...b.primaryKey],
    minSources: 0,
    maxSources: 0,
  });
  selectCanvas({kind:'relationship',index:draft.value.relationships.length-1});
}
function changeName(kind: "record" | "entity", old: string, e: Event) {
  const value = (e.target as HTMLInputElement).value,
    items = kind === "record" ? draft.value.records : draft.value.entities;
  if (
    !/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(value) ||
    items.some((x) => x.name === value && x.name !== old)
  ) {
    (e.target as HTMLInputElement).value = old;
    error.value = "Names must be unique identifiers. Existing data was kept.";
    return;
  }
  const item = items.find((x) => x.name === old);
  if (!item) return;
  item.name = value;
  if (kind === "record") {
    draft.value.entities.forEach((x) => {
      if (x.record === old) x.record = value;
    });
    if(recordPositions.value[old]) { recordPositions.value[value]=recordPositions.value[old]; delete recordPositions.value[old]; }
    selectedRecord.value = value;
  } else {
    draft.value.relationships.forEach((r) => {
      if (r.fromEntity === old) r.fromEntity = value;
      if (r.toEntity === old) r.toEntity = value;
    });
    if (fixture.value.rows && Object.hasOwn(fixture.value.rows, old)) {
      fixture.value.rows[value] = fixture.value.rows[old];
      delete fixture.value.rows[old];
    }
    selectedEntity.value = value;
  }
}
async function validateFixture() {
  await work("check", async () => {
    const snapshot = cloneBehavior(draft.value),
      data = cloneBehavior(fixture.value),
      stamp = workspaceStamp.value,
      lock = await prepare(snapshot);
    record.value = await api<DataValidationRecord>("/data/validate", "POST", {
      contract: lock,
      fixture: data,
    });
    recordStamp.value = stamp;
    history.value = await api<DataValidationSummary[]>("/data/validations");
    notice.value = record.value.result.valid
      ? "Fixture satisfies this exact contract. Evidence saved."
      : "Fixture has constraint violations. The invalid verdict and snapshot were saved for inspection.";
  });
}
async function openEvidence(id: string) {
  await work("history", async () => {
    record.value = await api<DataValidationRecord>(
      `/data/validations/${encodeURIComponent(id)}`,
    );
    recordStamp.value = `${stableBehavior(record.value.contract.contract)}|${stableBehavior(record.value.fixture)}`;
    notice.value =
      "Viewing saved evidence without replacing the current draft.";
  });
}
function loadSnapshot() {
  if (!record.value || busy.value || !replaceAllowed()) return;
  adopt(record.value.contract, record.value.fixture);
  recordStamp.value = workspaceStamp.value;
  tab.value = "fixture";
}
async function compare() {
  await work("compare", async () => {
    const old = catalog.value.find((x) => dataKey(x) === oldKey.value);
    if (!old) throw new Error("Select an installed old contract.");
    const stamp = `${oldKey.value}|${draftStamp.value}`,
      newLock = await prepare();
    comparison.value = await api<DataComparison>("/data/compare", "POST", {
      old: { contract: old.contract, digest: old.digest },
      new: newLock,
    });
    comparisonStamp.value = stamp;
    comparedLabels.value = `${old.contract.id} @ ${old.contract.version} → ${newLock.contract.id} @ ${newLock.contract.version}`;
  });
}
const autosave = useWorkspaceDraft("data", {
  draft,
  fixture,
  tab,
  selectedEntity,
  selectedRecord,
  canvasSelection,
  recordPositions,
  jsonKind,
  jsonText,
  jsonBaseline,
  jsonSourceStamp,
  showJSON,
  baselineContract,
  baselineFixture,
  installedStamp,
  record,
  recordStamp,
});
useActionFeedback("Data", busy, notice, error);
function unload(e: BeforeUnloadEvent) {
  if (autosave.unsaved()) {
    e.preventDefault();
    e.returnValue = "";
  }
}
onMounted(async () => {
  await autosave.ready;
  initialize();
  window.addEventListener("beforeunload", unload);
});
onUnmounted(() => window.removeEventListener("beforeunload", unload));
watchInspection(() => inspectionProps.inspectionRequest, async request => {
  if (!request) return;
  await autosave.ready;
  if (busy.value || !replaceAllowed()) return;
  try { const e = JSON.parse(JSON.stringify(request.body)); adopt(e.contract,e.fixture); tab.value = 'model'; notice.value = 'Inspection exercise opened. The existing draft is retained in recovery checkpoints.'; }
  catch (e) { error.value = (e as Error).message; }
}, { immediate:true });
</script>
<template>
  <section class="data-workbench" :inert="autosave.status.value === 'loading'">
    <DraftStatus :draft="autosave" />
    <header>
      <span class="section-eyebrow">REUSABLE DATA CONTRACTS</span>
      <h2>Design data and its relationships</h2>
      <p>
        Record types define required scalar fields. Entities add identity and
        uniqueness; explicit foreign keys connect complete datasets. The same
        locked types can be used in Components.
      </p>
    </header>
    <div class="data-library">
      <label class="field"
        >Reference dataset<select
          v-model="selectedExample"
          aria-label="Data example"
          :disabled="!!busy"
        >
          <option v-for="(e, i) in examples" :key="i" :value="String(i)">
            {{ e.fixture.name }}
          </option>
        </select></label
      ><button
        class="button secondary"
        :disabled="!!busy || !examples.length"
        @click="openExample"
      >
        Open data example</button
      ><label class="field"
        >Installed contracts<select
          v-model="selectedCatalog"
          aria-label="Installed data contract"
          :disabled="!!busy"
        >
          <option value="">Choose exact version</option>
          <option v-for="l in catalog" :key="dataKey(l)" :value="dataKey(l)">
            {{ l.contract.id }} @ {{ l.contract.version }}
          </option>
        </select></label
      ><button
        class="button secondary"
        :disabled="!!busy || !selectedCatalog"
        @click="openCatalog"
      >
        Open contract</button
      ><button class="button quiet" :disabled="!!busy" @click="initialize">
        Refresh
      </button>
    </div>
    <div class="data-actions">
      <strong
        >{{ draft.name }}
        <small>{{ draft.id }} @ {{ draft.version }}</small></strong
      ><span>{{
        pendingJSON
          ? "Unapplied JSON edits"
          : installedStamp === draftStamp
            ? "Installed exact contract"
            : "Local draft"
      }}</span
      ><button class="button secondary" :disabled="!!busy" @click="newDraft">
        New data contract</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="requestImport('contract')"
      >
        Import data contract</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="exportContract"
      >
        Export data contract</button
      ><button
        class="button secondary"
        :disabled="!!busy"
        @click="validateDefinition"
      >
        Validate data model</button
      ><button class="button primary" :disabled="!!busy" @click="install">
        Install data contract</button
      ><input
        ref="file"
        class="hidden"
        type="file"
        accept=".json,application/json"
        aria-label="Import data JSON"
        @change="importFile"
      />
    </div>
    <p class="help">
      Drafts and fixtures autosave on this browser. Installing retains the exact
      definition; export fixtures separately. Changing installed content
      requires a new version.
    </p>
    <div v-if="error" class="data-error" role="alert">
      <strong>{{ error }}</strong>
      <ul>
        <li v-for="i in issues" :key="i.path + i.message">
          <button class="issue-link" @click="focusIssue(i)">
            <code>{{ i.path }}</code> {{ i.message }} → Fix
          </button>
        </li>
      </ul>
    </div>
    <p v-if="notice" class="data-notice" role="status">{{ notice }}</p>
    <p v-if="busy" class="data-notice" role="status">
      Working with the data engine…
    </p>
    <details class="data-panel">
      <summary>Contract identity and description</summary>
      <fieldset :disabled="!!busy">
        <div class="data-grid">
          <label class="field"
            >Namespaced ID<input
              v-model="draft.id"
              aria-label="Data contract ID" /></label
          ><label class="field"
            >Exact version<input
              v-model="draft.version"
              aria-label="Data contract version" /></label
          ><label class="field"
            >Display name<input
              v-model="draft.name"
              aria-label="Data contract name" /></label
          ><label class="field"
            >Author<input
              v-model="draft.author"
              aria-label="Data contract author" /></label
          ><label class="field"
            >License<input
              v-model="draft.license"
              aria-label="Data contract license" /></label
          ><label class="field"
            >Description<textarea
              v-model="draft.description"
              aria-label="Data contract description"
            />
          </label>
        </div>
      </fieldset>
    </details>
    <WorkspaceModes :active="tab" :structure='["model", "relationships"]' :behavior='["compare"]' :execution='["fixture", "history"]' @select='tab = $event as "model" | "relationships" | "compare" | "fixture" | "history"' />
    <nav class="data-tabs">
      <button v-show="['model', 'relationships'].includes(tab)" class="active" @click="tab = 'model'">
        Data canvas</button
      ><button v-show="['fixture', 'history'].includes(tab)" :class="{ active: tab === 'fixture' }" @click="tab = 'fixture'">
        Fixture validation</button
      ><button v-show="['fixture', 'history'].includes(tab)" :class="{ active: tab === 'history' }" @click="tab = 'history'">
        Data validation history</button
      ><button v-show="['compare'].includes(tab)" :class="{ active: tab === 'compare' }" @click="tab = 'compare'">
        Compatibility
      </button>
    </nav>
    <section v-if="tab === 'model' && !draft.records.length" class="data-panel">
      <p class="help">
        This contract is empty. Add the first record type and its fields, then
        create entities and relationships for the data you want to model.
      </p>
      <button class="button secondary" :disabled="!!busy" @click="addRecord">
        Add first record type
      </button>
    </section>
    <DataGraph v-if="tab === 'model' || tab === 'relationships'"
      :contract="draft" :selection="canvasSelection" :disabled="!!busy"
      :record-positions="recordPositions" :connection-source="connectionSource" :issues="issues"
      @select="selectCanvas" @close="closeCanvasEditor" @add-record="addRecord" @add-entity="addEntity()"
      @add-relationship="addRelationship()" @connect="connectEntities" @cancel-connect="connectionSource=null"
      @validate="validateDefinition"
      @move="(name,position) => { const e=draft.entities.find(e => e.name===name); if(e && !busy) e.position=position; }"
      @move-record="(name,position) => { if(!busy) recordPositions[name]=position; }"
    >
      <template #editor>
        <p v-if="error" class="data-error" role="alert">{{ error }}</p>
        <div class="canvas-context-actions" v-if="canvasSelection?.kind === 'entity'">
          <button class="button secondary" :aria-pressed="canvasMode === 'properties'" @click="canvasMode='properties'">Entity &amp; keys</button>
          <button class="button secondary" :aria-pressed="canvasMode === 'fields'" @click="canvasMode='fields'">Record fields</button>
          <button class="button secondary" :disabled="!!busy || draft.relationships.length >= 64" @click="connectionSource=canvasSelection.index; closeCanvasEditor()">Connect to…</button>
        </div>
        <div v-if="canvasSelection?.kind === 'record'" class="canvas-context-actions">
          <button class="button secondary" :disabled="!!busy || draft.entities.length >= 32" @click="addEntity(canvasRecord?.name)">Create entity from this record</button>
        </div>
        <p v-if="canvasRecord && (canvasSelection?.kind === 'record' || canvasMode === 'fields')" class="help">Record {{ canvasRecord.name }} · shared by {{ draft.entities.filter(e => e.record === canvasRecord?.name).length }} entities. All fields are required; edits affect every entity using this type.</p>
        <ul v-if="canvasIssues.length" class="issue-list"><li v-for="issue in canvasIssues" :key="issue.path"><button class="issue-link" @click="focusIssue(issue)">{{ issue.path }} · {{ issue.message }}</button></li></ul>
        <section v-if="canvasSelection?.kind === 'record' || canvasMode === 'fields'">
          <fieldset v-if="canvasRecord && (canvasSelection?.kind === 'record' || canvasMode === 'fields')" :disabled="!!busy">
            <label class="field"
              >Record name<input
                :value="canvasRecord.name"
                aria-label="Record type name"
                @change="changeName('record', canvasRecord.name, $event)"
            /></label>
            <div
              v-for="(f, i) in canvasRecord.fields"
              :key="i"
              class="field-row"
            >
              <label class="field"
                >Field<input
                  v-model="f.name"
                  :aria-label="`Record field ${i + 1} name`" /></label
              ><label class="field"
                >Type<select
                  v-model="f.type"
                  :aria-label="`Record field ${i + 1} type`"
                >
                  <option v-for="s in scalarTypes" :key="s">{{ s }}</option>
                </select></label
              ><button
                class="button quiet"
                :disabled="canvasRecord.fields.length <= 1"
                @click="canvasRecord.fields.splice(i, 1)"
              >
                Remove field {{ i + 1 }}
              </button>
            </div>
            <button
              class="button secondary"
              :disabled="canvasRecord.fields.length >= 16"
              @click="addField"
            >
              Add record field</button
            ><button
              class="button danger"
              :disabled="draft.records.length <= 1"
              @click="removeCanvasRecord"
            >
              Remove record type
            </button>
          </fieldset>
          <p class="help">
            All fields are required. Renaming or changing fields may invalidate
            keys, relationships and existing fixture rows; validate to inspect
            the affected paths.
          </p>

        </section>
        <section v-if="canvasSelection?.kind === 'entity' && canvasMode === 'properties'">
          <fieldset v-if="currentEntity" :disabled="!!busy">
            <label class="field"
              >Entity name<input
                :value="currentEntity.name"
                aria-label="Entity name"
                @change="
                  changeName('entity', currentEntity.name, $event)
                " /></label
            ><label class="field"
              >Record type<select
                v-model="currentEntity.record"
                aria-label="Entity record type"
              >
                <option v-for="r in draft.records" :key="r.name">
                  {{ r.name }}
                </option>
              </select></label
            ><label class="field"
              >Primary key fields, in order<input
                :value="currentEntity.primaryKey.join(', ')"
                aria-label="Entity primary key"
                @change="
                  currentEntity.primaryKey = splitFields(
                    ($event.target as HTMLInputElement).value,
                  )
                "
            /></label>
            <div
              v-for="(key, i) in currentEntity.uniqueKeys"
              :key="i"
              class="key-row"
            >
              <label class="field"
                >Unique key {{ i + 1
                }}<input
                  :value="key.join(', ')"
                  :aria-label="`Entity unique key ${i + 1}`"
                  @change="
                    currentEntity.uniqueKeys[i] = splitFields(
                      ($event.target as HTMLInputElement).value,
                    )
                  " /></label
              ><button
                class="button quiet"
                @click="currentEntity.uniqueKeys.splice(i, 1)"
              >
                Remove key
              </button>
            </div>
            <button
              class="button secondary"
              :disabled="currentEntity.uniqueKeys.length >= 8"
              @click="currentEntity.uniqueKeys.push([])"
            >
              Add unique key
            </button>
            <div class="data-grid">
              <label class="field"
                >X<input
                  v-model.number="currentEntity.position.x"
                  aria-label="Entity X"
                  type="number" /></label
              ><label class="field"
                >Y<input
                  v-model.number="currentEntity.position.y"
                  aria-label="Entity Y"
                  type="number"
              /></label>
            </div>
            <button
              class="button danger"
              @click="removeCanvasEntity"
            >
              Remove entity
            </button>
          </fieldset>
          <p class="help">
            Keys contain one to four declared fields, separated by commas. A
            unique key constrains the complete ordered tuple. Removing an entity
            leaves its references and fixture rows visible for correction.
          </p>

        </section>
        <section v-if="canvasSelection?.kind === 'relationship'">
        <fieldset :disabled="!!busy">
          <article
            v-for="{r,i} in draft.relationships.map((r,i) => ({r,i})).filter(x => x.i === canvasSelection?.index)"
            :key="i"
            class="relationship"
          >
            <div class="data-grid">
              <label class="field"
                >Name<input
                  v-model="r.name"
                  :aria-label="`Relationship ${i + 1} name`" /></label
              ><label class="field"
                >Source entity<select
                  v-model="r.fromEntity"
                  :aria-label="`Relationship ${i + 1} source`"
                >
                  <option v-for="e in draft.entities" :key="e.name">
                    {{ e.name }}
                  </option>
                </select></label
              ><label class="field"
                >Source fields<input
                  :value="r.fromFields.join(', ')"
                  :aria-label="`Relationship ${i + 1} source fields`"
                  @change="
                    r.fromFields = splitFields(
                      ($event.target as HTMLInputElement).value,
                    )
                  " /></label
              ><label class="field"
                >Target entity<select
                  v-model="r.toEntity"
                  :aria-label="`Relationship ${i + 1} target`"
                >
                  <option v-for="e in draft.entities" :key="e.name">
                    {{ e.name }}
                  </option>
                </select></label
              ><label class="field"
                >Target key fields<input
                  :value="r.toFields.join(', ')"
                  :aria-label="`Relationship ${i + 1} target fields`"
                  @change="
                    r.toFields = splitFields(
                      ($event.target as HTMLInputElement).value,
                    )
                  " /></label
              ><label class="field"
                >Minimum source rows per target<select
                  v-model.number="r.minSources"
                  :aria-label="`Relationship ${i + 1} minimum`"
                >
                  <option :value="0">0 · optional participation</option>
                  <option :value="1">1 · at least one source</option>
                </select></label
              ><label class="field"
                >Maximum source rows per target<select
                  v-model.number="r.maxSources"
                  :aria-label="`Relationship ${i + 1} maximum`"
                >
                  <option :value="0">Many</option>
                  <option :value="1">1 · one-to-one</option>
                </select></label
              >
            </div>
            <button
              class="button danger"
              @click="removeCanvasRelationship(i)"
            >
              Remove relationship {{ i + 1 }}
            </button>
          </article>
        </fieldset>
        <p class="help">
          Each source tuple must reference one declared primary or unique target
          key with matching ordered field types. Model many-to-many with a join
          entity and two foreign keys. Cycles do not imply cascade behavior.
        </p>

        </section>
      </template>
    </DataGraph>
    <section v-if="tab === 'fixture'" class="data-panel">
      <DataFixtureEditor
        :contract="draft"
        :fixture="fixture"
        :disabled="!!busy"
      />
      <div class="data-actions">
        <button
          class="button primary"
          :disabled="!!busy"
          @click="validateFixture"
        >
          Validate & record fixture</button
        ><button
          class="button secondary"
          :disabled="!!busy"
          @click="requestImport('fixture')"
        >
          Import fixture</button
        ><button
          class="button secondary"
          :disabled="!!busy"
          @click="exportFixture"
        >
          Export fixture
        </button>
      </div>
    </section>
    <section v-if="tab === 'history'" class="data-panel">
      <h3>Immutable fixture evidence</h3>
      <p class="help">
        The newest {{ history.length }} validations include valid and invalid
        verdicts. Opening evidence leaves the current draft unchanged.
      </p>
      <div class="data-history">
        <button
          v-for="h in history"
          :key="h.id"
          :disabled="!!busy"
          @click="openEvidence(h.id)"
        >
          <strong>{{ h.contractId }} @ {{ h.contractVersion }}</strong
          ><span
            >{{ h.valid ? "VALID" : "INVALID" }} · {{ h.rowCount }} rows ·
            {{ new Date(h.createdAt).toLocaleString() }}</span
          >
        </button>
      </div>
    </section>
    <section
      v-if="(tab === 'fixture' || tab === 'history') && record"
      class="data-panel evidence"
    >
      <div class="panel-heading">
        <h3>
          {{ record.result.valid ? "Fixture valid" : "Constraint violations" }}
        </h3>
        <span>{{
          stale ? "Earlier draft or fixture" : "Matches current draft & fixture"
        }}</span>
      </div>
      <p class="help">
        {{ record.contract.contract.id }} @
        {{ record.contract.contract.version }} · engine
        {{ record.result.engineVersion }} ·
        {{ record.result.entityCount }} entities ·
        {{ record.result.rowCount }} rows
      </p>
      <code>{{ record.contract.digest }}</code>
      <p v-if="stale" class="data-warning">
        This verdict belongs to the saved contract and fixture. Current edits
        have not changed the evidence.
      </p>
      <ul class="issue-list">
        <li v-for="(i, index) in record.result.issues" :key="index">
          <code>{{ i.path }}</code> {{ i.message }}
        </li>
      </ul>
      <p v-if="record.result.issuesTruncated" class="data-warning">
        More violations exist. The first 200 issue details are shown.
      </p>
      <button class="button secondary" :disabled="!!busy" @click="loadSnapshot">
        Load recorded data snapshot
      </button>
      <details>
        <summary>Recorded fixture and identity</summary>
        <pre>{{
          JSON.stringify(
            {
              id: record.id,
              createdAt: record.createdAt,
              fixture: record.fixture,
            },
            null,
            2,
          )
        }}</pre>
      </details>
    </section>
    <section v-if="tab === 'compare'" class="data-panel">
      <h3>Directional compatibility</h3>
      <p class="help">
        Would valid old values and complete datasets remain accepted by the new
        draft? This conservative check does not perform migration or make exact
        component type locks interchangeable.
      </p>
      <label class="field"
        >Old installed contract<select
          v-model="oldKey"
          aria-label="Old data contract"
          :disabled="!!busy"
        >
          <option value="">Select a contract with the same ID</option>
          <option v-for="l in catalog" :key="dataKey(l)" :value="dataKey(l)">
            {{ l.contract.id }} @ {{ l.contract.version }}
          </option>
        </select></label
      >
      <p class="help">
        New: current draft {{ draft.id }} @ {{ draft.version }}
      </p>
      <button
        class="button primary"
        :disabled="!!busy || !oldKey"
        @click="compare"
      >
        Compare old to current draft
      </button>
      <div v-if="comparison" class="comparison-result">
        <h3>
          {{
            comparison.compatible
              ? "Compatible in this direction"
              : "Breaking changes found"
          }}
        </h3>
        <p
          v-if="comparisonStamp !== `${oldKey}|${draftStamp}` || pendingJSON"
          class="data-warning"
        >
          The selected old contract or current draft changed after this
          comparison.
        </p>
        <p class="help">
          Compared: {{ comparedLabels }} · Direction: {{ comparison.direction }}
        </p>
        <ul class="issue-list">
          <li v-for="(c, i) in comparison.changes" :key="i">
            <strong>{{ c.kind }}</strong> <code>{{ c.path }}</code>
            {{ c.message }}
          </li>
        </ul>
      </div>
    </section>
    <div class="data-json">
      <button
        class="button quiet"
        :disabled="!!busy"
        @click="openJSON('contract')"
      >
        Contract JSON</button
      ><button
        class="button quiet"
        :disabled="!!busy"
        @click="openJSON('fixture')"
      >
        Fixture JSON</button
      ><template v-if="showJSON"
        ><p class="help">
          {{ jsonKind === "contract" ? "Contract" : "Fixture" }} JSON is
          validated from its original text before the browser adopts canonical
          values. Pending text is retained when hidden.
        </p>
        <p v-if="jsonSourceStamp !== workspaceStamp" class="data-warning">
          Graphical changes were made after this JSON was opened. Applying it
          will ask before replacing current content.
        </p>
        <textarea
          v-model="jsonText"
          aria-label="Advanced data JSON"
          class="json-editor"
          :disabled="!!busy"
          spellcheck="false"
        />
        <div class="data-actions">
          <button
            class="button secondary"
            :disabled="!!busy"
            @click="applyJSON"
          >
            Validate & apply data JSON</button
          ><button class="button quiet" :disabled="!!busy" @click="resetJSON">
            Reset data JSON</button
          ><button class="button quiet" @click="showJSON = false">
            Hide data JSON
          </button>
        </div></template
      >
    </div>
    <p class="help boundary">
      This checks a bounded complete snapshot. It does not create database
      tables, enforce live transactions or infer tenancy, access policy,
      migrations or deployment.
    </p>
  </section>
</template>
<style scoped>
.canvas-context-actions { display:flex; flex-wrap:wrap; gap:6px; margin-bottom:12px; }
.canvas-context-actions button[aria-pressed=true] { background:#e6f4ee; border-color:#23846e; }
.data-workbench :deep(.canvas-editor) .field-row { grid-template-columns:minmax(0,1fr) minmax(85px,.65fr); }
.data-workbench :deep(.canvas-editor) .field-row .button { grid-column:1/-1; justify-self:end; margin:0 0 8px; }
.data-workbench :deep(.canvas-editor) .data-grid { grid-template-columns:1fr; }
.data-workbench :deep(.canvas-editor) fieldset { min-width:0; }
.data-workbench {
  padding: 24px 30px;
  background: #f5f8fa;
  min-width: 0;
}
.data-workbench header h2 {
  font-size: 24px;
  color: #355269;
  margin: 8px 0;
}
.data-workbench header p,
.help {
  font-size: 12px;
  line-height: 1.8;
  color: #627d8e;
}
.data-library,
.data-panel {
  background: #fff;
  border: 1px solid #dce6ed;
  border-radius: 9px;
  padding: 18px;
  margin: 16px 0;
}
.data-library {
  display: flex;
  gap: 12px;
  align-items: end;
  flex-wrap: wrap;
}
.data-library .field {
  flex: 1;
  min-width: 200px;
  margin: 0;
}
.data-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin: 16px 0;
}
.data-actions > strong {
  color: #3d5c71;
  margin-right: auto;
  font-size: 14px;
}
.data-actions small {
  display: block;
  color: #6c8493;
  font-size: 12px;
  margin-top: 5px;
}
.data-actions > span {
  font-size: 12px;
  color: #488573;
}
.data-workbench .button {
  font-size: 12px;
  padding: 9px 12px;
}
.data-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 13px;
}
.data-grid .field {
  margin: 8px 0;
}
.data-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
}
.data-columns .data-panel {
  margin-top: 0;
  min-width: 0;
}
.data-panel h3 {
  color: #416178;
  font-size: 16px;
  margin: 0 0 14px;
}
.panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 15px;
}
.panel-heading h3 {
  margin: 0;
}
.panel-heading > span {
  font-size: 12px;
  color: #738a99;
}
.data-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  border-bottom: 1px solid #dce7ed;
  margin: 18px 0;
  padding-bottom: 10px;
}
.data-tabs button {
  padding: 10px 13px;
  color: #627e90;
  background: transparent;
  border-radius: 5px;
  font-size: 12px;
}
.data-tabs button.active {
  background: #e6f4ee;
  color: #23846e;
}
.field-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 10px;
  align-items: end;
}
.field-row .button {
  margin-bottom: 13px;
}
.key-row {
  display: flex;
  gap: 10px;
  align-items: center;
}
.key-row .field {
  flex: 1;
}
.relationship {
  border: 1px solid #e0e9ed;
  padding: 16px;
  border-radius: 7px;
  margin: 14px 0;
  background: #fbfdfd;
}
.data-error,
.data-warning {
  padding: 13px 16px;
  color: #a36b42;
  background: #fff6e8;
  border: 1px solid #eed9b8;
  border-radius: 7px;
  font-size: 12px;
  line-height: 1.8;
}
.data-notice {
  padding: 13px 16px;
  background: #edf8f2;
  color: #3e866d;
  border: 1px solid #d1e7dc;
  border-radius: 7px;
  font-size: 12px;
  line-height: 1.8;
}
.data-history {
  display: grid;
  gap: 8px;
  max-height: 350px;
  overflow: auto;
}
.data-history button {
  display: flex;
  justify-content: space-between;
  gap: 15px;
  text-align: left;
  border: 1px solid #dfebee;
  background: #fafdfc;
  padding: 14px;
  border-radius: 6px;
  color: #4f7286;
  font-size: 12px;
}
.data-history span {
  color: #738b98;
}
.issue-list {
  font-size: 12px;
  color: #607d8d;
  line-height: 1.9;
  padding-left: 20px;
}
.issue-list code,
.evidence > code {
  font-size: 11px;
  overflow-wrap: anywhere;
}
.data-json {
  margin-top: 20px;
}
.json-editor {
  display: block;
  width: 100%;
  height: 360px;
  font-family: monospace;
  font-size: 12px;
  line-height: 1.6;
  padding: 16px;
  border: 1px solid #d9e4eb;
  border-radius: 7px;
  background: #fcfefe;
  color: #456779;
}
.data-panel summary {
  cursor: pointer;
  color: #526e80;
  font-size: 13px;
}
.data-panel pre {
  max-height: 400px;
  overflow: auto;
  font-size: 11px;
  color: #527183;
  background: #f7fafb;
  padding: 16px;
}
.data-panel details {
  margin-top: 16px;
}
.boundary {
  margin-top: 20px;
}
fieldset {
  padding: 0;
  margin: 0;
  border: 0;
  min-width: 0;
}
fieldset:disabled {
  opacity: 0.75;
}
@media (max-width: 1100px) {
  .data-columns {
    grid-template-columns: 1fr;
  }
  .data-workbench {
    padding: 20px;
  }
  .data-actions > strong {
    width: 100%;
  }
  .data-history button {
    flex-direction: column;
  }
}
@media (max-width: 650px) {
  .data-workbench {
    padding: 12px;
  }
  .field-row {
    grid-template-columns: 1fr 1fr;
  }
  .field-row .button {
    grid-column: 1/-1;
  }
  .data-library .field {
    min-width: 100%;
  }
  .data-grid {
    grid-template-columns: 1fr;
  }
  .panel-heading {
    flex-wrap: wrap;
  }
}
</style>
