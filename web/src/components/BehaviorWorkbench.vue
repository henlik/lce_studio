<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import {
  ArrowDownToLine,
  ArrowLeft,
  ArrowRight,
  ArrowUpFromLine,
  Box,
  Braces,
  Check,
  CircleAlert,
  FileCheck2,
  History,
  Layers3,
  LoaderCircle,
  Play,
  Plus,
  RefreshCw,
  Save,
  Trash2,
} from "lucide-vue-next";
import { api, apiRawJson, APIError } from "../api";
import type { Validation, ValidationIssue } from "../types";
import {
  bindingType,
  cloneBehavior,
  dependencyClosure,
  lockKey,
  nestedPackage,
  nodeInputs,
  nodeOutputs,
  referenceKey,
  scalarTypes,
  stableBehavior,
  typedDefault,
  type BehaviorBinding,
  type BehaviorCase,
  type BehaviorNode,
  type BehaviorPort,
  type BehaviorTestResult,
  type CatalogRecord,
  type ComponentBundle,
  type CompositeLock,
  type CompositePackage,
  type EvaluationRecord,
  type EvaluationSummary,
  type InstalledComposite,
  type PrimitiveSpec,
  type ScalarType,
  type TypedValue,
} from "../behaviorTypes";
import BehaviorBindingEditor from "./BehaviorBindingEditor.vue";
import BehaviorEvidence from "./BehaviorEvidence.vue";
import BehaviorGraph from "./BehaviorGraph.vue";
import TypedValueEditor from "./TypedValueEditor.vue";

function newDefinition(): CompositePackage {
  return {
    schemaVersion: 2,
    id: `local.component_${crypto.randomUUID().slice(0, 8)}`,
    version: "1.0.0",
    name: "Untitled component",
    description: "",
    author: "Local author",
    license: "UNLICENSED",
    kernel: "composite",
    capabilities: ["design", "evaluate"],
    config: {
      processingMs: { default: 0, min: 0, max: 0 },
      capacity: { default: 0, min: 0, max: 0 },
      memoryMb: { default: 0, min: 0, max: 0 },
      deduplicate: false,
    },
    behavior: {
      inputs: [{ name: "value", type: "string" }],
      outputs: [{ name: "result", type: "string" }],
      nodes: [],
      bindings: { result: { input: "value" } },
    },
    tests: [],
  };
}
const draft = ref<CompositePackage>(newDefinition());
const dependencies = ref<CompositeLock[]>([]);
const primitives = ref<PrimitiveSpec[]>([]);
const examples = ref<ComponentBundle[]>([]);
const installed = ref<InstalledComposite[]>([]);
const history = ref<EvaluationSummary[]>([]);
const tab = ref<"compose" | "interface" | "evaluate" | "history">("compose");
const busy = ref("");
const error = ref("");
const issues = ref<ValidationIssue[]>([]);
const notice = ref("");
const selectedNodeId = ref("");
const inspected = ref<string[]>([]);
const selectedPrimitive = ref("identity");
const selectedNested = ref("");
const selectedLibrary = ref("");
const selectedExample = ref("0");
const selectedCase = ref(0);
const inputValues = ref<Record<string, TypedValue>>({
  value: { type: "string", string: "Hello" },
});
const record = ref<EvaluationRecord | null>(null);
const recordStamp = ref("");
const testResult = ref<BehaviorTestResult | null>(null);
const testedStamp = ref("");
const baseline = ref("");
const installedStamp = ref("");
const fileInput = ref<HTMLInputElement | null>(null);
const advancedJSON = ref("");
const advancedBaseline = ref("");
const advancedSourceStamp = ref("");
const pendingAdvanced = computed(
  () => advancedJSON.value !== advancedBaseline.value,
);
const showAdvanced = ref(false);
const metadataOpen = ref(false);
let initialLoaded = false;

function definitionStamp(p: CompositePackage, deps: CompositeLock[]) {
  const comparable = (value: CompositePackage) => ({
    ...value,
    tests: value.tests || [],
  });
  return stableBehavior({
    package: comparable(p),
    dependencies: dependencyClosure(p, deps).map((d) => ({
      ...d,
      package: comparable(d.package),
    })),
  });
}
const draftStamp = computed(() =>
  definitionStamp(draft.value, dependencies.value),
);
baseline.value = draftStamp.value;
const dirty = computed(
  () => draftStamp.value !== baseline.value || pendingAdvanced.value,
);
const isInstalled = computed(
  () => !pendingAdvanced.value && installedStamp.value === draftStamp.value,
);
const viewedLock = computed(() =>
  dependencies.value.find((p) => lockKey(p) === inspected.value.at(-1)),
);
const viewed = computed(() => viewedLock.value?.package || draft.value);
const readonly = computed(() => !!viewedLock.value);
const behavior = computed(() => viewed.value.behavior);
const selectedNode = computed(() =>
  behavior.value.nodes?.find((n) => n.id === selectedNodeId.value),
);
const selectedInputs = computed(() =>
  selectedNode.value
    ? nodeInputs(selectedNode.value, dependencies.value, primitives.value)
    : [],
);
const sources = computed(() =>
  (behavior.value.nodes || []).flatMap((n) =>
    nodeOutputs(n, behavior.value, dependencies.value, primitives.value),
  ),
);
const nodeSources = computed(() =>
  sources.value.filter((p) => p.node !== selectedNodeId.value),
);
const selectedDescription = computed(() =>
  selectedNode.value?.op
    ? primitives.value.find((p) => p.op === selectedNode.value?.op)?.description
    : selectedNode.value
      ? nestedPackage(selectedNode.value, dependencies.value)?.description
      : "",
);
const activeCase = computed(() => draft.value.tests?.[selectedCase.value]);
const evidenceStale = computed(
  () =>
    !!record.value &&
    (pendingAdvanced.value ||
      recordStamp.value !==
        `${draftStamp.value}|${stableBehavior(inputValues.value)}`),
);
const testsStale = computed(
  () =>
    !!testResult.value &&
    (pendingAdvanced.value || testedStamp.value !== draftStamp.value),
);
const usedDependencies = computed(() =>
  dependencyClosure(draft.value, dependencies.value),
);
const currentReference = computed(
  () => `${draft.value.id} @ ${draft.value.version}`,
);

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
function canReplace() {
  return (
    !dirty.value ||
    confirm(
      "Replace your edited component draft? Export or install it first to keep the changes.",
    )
  );
}
function rootLock(bundle: ComponentBundle) {
  const root = bundle.packages.find(
    (p) => lockKey(p) === referenceKey(bundle.root),
  );
  if (!root) throw new Error("This bundle has no matching root definition.");
  return root;
}
function resetInputs() {
  const first = draft.value.tests?.[0];
  inputValues.value = first
    ? cloneBehavior(first.inputs || {})
    : Object.fromEntries(
        (draft.value.behavior.inputs || []).map((p) => [
          p.name,
          typedDefault(p.type),
        ]),
      );
}
function adopt(bundle: ComponentBundle, catalog = false) {
  const root = rootLock(bundle);
  draft.value = cloneBehavior(root.package);
  dependencies.value = cloneBehavior(
    bundle.packages.filter((p) => lockKey(p) !== lockKey(root)),
  );
  inspected.value = [];
  selectedNodeId.value = draft.value.behavior.nodes?.[0]?.id || "";
  selectedCase.value = 0;
  baseline.value = draftStamp.value;
  installedStamp.value = catalog ? draftStamp.value : "";
  resetInputs();
  showAdvanced.value = false;
  advancedJSON.value = "";
  advancedBaseline.value = "";
  advancedSourceStamp.value = "";
  metadataOpen.value = false;
}
async function refreshCatalog() {
  const values = await api<CatalogRecord[]>("/components");
  installed.value = values.filter(
    (p): p is InstalledComposite => p.package.schemaVersion === 2,
  );
}
async function initialize() {
  await work("load", async () => {
    const values = await Promise.allSettled([
      api<PrimitiveSpec[]>("/components/primitives"),
      api<ComponentBundle[]>("/components/composite-examples"),
      refreshCatalog(),
      api<EvaluationSummary[]>("/components/evaluations"),
    ]);
    if (values[0].status === "fulfilled") primitives.value = values[0].value;
    else fail(values[0].reason);
    if (values[1].status === "fulfilled") {
      examples.value = values[1].value;
      if (!initialLoaded && examples.value[0] && !dirty.value) {
        adopt(examples.value[0]);
        initialLoaded = true;
      }
    } else fail(values[1].reason);
    if (values[2].status === "rejected") fail(values[2].reason);
    if (values[3].status === "fulfilled") history.value = values[3].value;
    else fail(values[3].reason);
  });
}
function captureDraft() {
  return {
    package: cloneBehavior(draft.value),
    dependencies: cloneBehavior(usedDependencies.value),
    stamp: draftStamp.value,
  };
}
async function prepare(snapshot = captureDraft()) {
  if (pendingAdvanced.value)
    throw new Error(
      "Apply or reset the pending advanced JSON edits before validating, evaluating, installing or exporting the definition.",
    );
  return api<ComponentBundle>("/components/prepare", "POST", {
    package: snapshot.package,
    dependencies: snapshot.dependencies,
  });
}
function createDraft() {
  if (busy.value || !canReplace()) return;
  draft.value = newDefinition();
  dependencies.value = [];
  inspected.value = [];
  selectedNodeId.value = "";
  selectedCase.value = 0;
  installedStamp.value = "";
  baseline.value = "";
  advancedJSON.value = "";
  advancedBaseline.value = "";
  advancedSourceStamp.value = "";
  showAdvanced.value = false;
  resetInputs();
  tab.value = "interface";
  metadataOpen.value = true;
  error.value = "";
  issues.value = [];
  notice.value =
    "New local draft. Its initial output passes the string input through unchanged.";
}
function loadExample() {
  const bundle = examples.value[Number(selectedExample.value)];
  if (busy.value || !bundle || !canReplace()) return;
  adopt(bundle);
  tab.value = "compose";
  error.value = "";
  issues.value = [];
  notice.value =
    "Portable example loaded. Its nested definitions are embedded; installation is not needed to evaluate it.";
}
async function fetchBundle(ref: {
  packageId: string;
  version: string;
  digest: string;
}) {
  return api<ComponentBundle>(
    `/components/bundle?id=${encodeURIComponent(ref.packageId)}&version=${encodeURIComponent(ref.version)}&digest=${encodeURIComponent(ref.digest)}`,
  );
}
async function loadInstalled() {
  const selected = installed.value.find(
    (p) => lockKey(p) === selectedLibrary.value,
  );
  if (!selected || !canReplace()) return;
  await work("open", async () => {
    adopt(
      await fetchBundle({
        packageId: selected.package.id,
        version: selected.package.version,
        digest: selected.digest,
      }),
      true,
    );
    tab.value = "compose";
    notice.value =
      "Installed definition opened as an editable draft. Use a new version to install changed content.";
  });
}
async function validateDraft() {
  await work("validate", async () => {
    const bundle = await prepare();
    notice.value = `Valid typed composition · ${bundle.packages.length} exact definitions. Ready to evaluate or install.`;
  });
}
async function installDraft() {
  await work("install", async () => {
    const snapshot = captureDraft(),
      bundle = await prepare(snapshot);
    await api("/components/bundles", "POST", bundle);
    installedStamp.value = snapshot.stamp;
    baseline.value = snapshot.stamp;
    await refreshCatalog();
    notice.value =
      "Exact bundle installed. Existing versions remain immutable; edited definitions need a new version.";
  });
}
async function exportBundle() {
  await work("export", async () => {
    const snapshot = captureDraft(),
      bundle = await prepare(snapshot);
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" }),
    );
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `${bundle.root.packageId}-${bundle.root.version}.bundle.json`;
    anchor.click();
    URL.revokeObjectURL(url);
    baseline.value = snapshot.stamp;
    notice.value =
      "Portable bundle exported with its complete dependency closure and exact digests.";
  });
}
async function importBundle(event: Event) {
  const input = event.target as HTMLInputElement,
    file = input.files?.[0];
  if (!file || !canReplace()) {
    input.value = "";
    return;
  }
  await work("import", async () => {
    if (!file.name.toLowerCase().endsWith(".json"))
      throw new Error("Choose a component bundle with the .json extension.");
    if (file.size > 2 * 1024 * 1024)
      throw new Error("Component bundles must be smaller than 2 MiB.");
    const raw = await file.text();
    const validation = await apiRawJson<
      Validation & { bundle?: ComponentBundle }
    >("/components/validate-bundle", raw);
    if (!validation.valid)
      throw new APIError(
        "Bundle import rejected. Fix the reported definitions or locks.",
        validation.issues,
      );
    if (!validation.bundle)
      throw new Error(
        "The server did not return the canonical validated bundle. Update the component API before importing this file.",
      );
    adopt(validation.bundle);
    tab.value = "compose";
    notice.value =
      "Bundle imported and validated from its original JSON. No catalog lookup was needed.";
  });
  input.value = "";
}
function setBinding(
  target: Record<string, BehaviorBinding>,
  name: string,
  value?: BehaviorBinding,
) {
  if (value) target[name] = value;
  else delete target[name];
}
function valueMap(
  target: Record<string, TypedValue>,
  name: string,
  value?: TypedValue,
) {
  if (value) target[name] = value;
  else delete target[name];
}
function freshNodeId(prefix: string) {
  let i = 1;
  while (
    (draft.value.behavior.nodes || []).some((n) => n.id === `${prefix}${i}`)
  )
    i++;
  return `${prefix}${i}`;
}
function insertNode(node: Omit<BehaviorNode, "position">) {
  const nodes = (draft.value.behavior.nodes ||= []);
  if (nodes.length >= 32) {
    notice.value = "A definition supports up to 32 internal nodes.";
    return;
  }
  nodes.push({
    ...node,
    position: {
      x: (nodes.length % 3) * 340,
      y: Math.floor(nodes.length / 3) * 235 + 30,
    },
  });
  selectedNodeId.value = node.id;
  tab.value = "compose";
  inspected.value = [];
}
function addPrimitive() {
  const spec = primitives.value.find((p) => p.op === selectedPrimitive.value);
  if (!spec || readonly.value) return;
  insertNode({
    id: freshNodeId(spec.op),
    op: spec.op,
    inputs: Object.fromEntries(
      spec.inputs.map((p) => [
        p.name,
        { literal: typedDefault(p.type === "T" ? "integer" : p.type) },
      ]),
    ),
  });
}
function mergeDependencies(incoming: CompositeLock[]) {
  for (const lock of incoming) {
    if (
      lock.package.id === draft.value.id &&
      lock.package.version === draft.value.version
    )
      throw new Error(
        "A component cannot nest its own ID and version. Select another component or change the root identity.",
      );
    const existing = dependencies.value.find(
      (p) =>
        p.package.id === lock.package.id &&
        p.package.version === lock.package.version,
    );
    if (existing && existing.digest !== lock.digest)
      throw new Error(
        `Conflicting dependency ${lock.package.id} @ ${lock.package.version}. Existing pinned content was kept.`,
      );
  }
  const additions = incoming.filter(
    (p) => !dependencies.value.some((d) => lockKey(d) === lockKey(p)),
  );
  if (dependencies.value.length + additions.length > 49)
    throw new Error(
      "A bundle supports at most 50 definitions including its root.",
    );
  dependencies.value.push(...cloneBehavior(additions));
}
async function addNested() {
  const entry =
    installed.value.find((p) => lockKey(p) === selectedNested.value) ||
    dependencies.value.find((p) => lockKey(p) === selectedNested.value);
  if (!entry || readonly.value) return;
  await work("dependency", async () => {
    const alreadyEmbedded = dependencies.value.find(
      (p) => lockKey(p) === lockKey(entry),
    );
    const bundle = alreadyEmbedded
      ? {
          root: {
            packageId: entry.package.id,
            version: entry.package.version,
            digest: entry.digest,
          },
          packages: [
            alreadyEmbedded,
            ...dependencyClosure(alreadyEmbedded.package, dependencies.value),
          ],
        }
      : await fetchBundle({
          packageId: entry.package.id,
          version: entry.package.version,
          digest: entry.digest,
        });
    mergeDependencies(bundle.packages);
    const p = rootLock({ ...bundle, schemaVersion: 1 }).package;
    insertNode({
      id: freshNodeId("call"),
      component: cloneBehavior(bundle.root),
      inputs: Object.fromEntries(
        (p.behavior.inputs || []).map((port) => [
          port.name,
          { literal: typedDefault(port.type) },
        ]),
      ),
    });
    notice.value =
      "Nested instance added with its exact dependency closure. Bind its inputs and connect a result to a public output.";
  });
}
const nestedChoices = computed(() => {
  const choices = new Map<string, CompositeLock>();
  dependencies.value.forEach((p) => choices.set(lockKey(p), p));
  installed.value.forEach((p) => {
    if (!choices.has(lockKey(p))) choices.set(lockKey(p), p);
  });
  return [...choices.values()].filter(
    (p) =>
      !(
        p.package.id === draft.value.id &&
        p.package.version === draft.value.version
      ),
  );
});
function inspectNode(id: string) {
  const node = behavior.value.nodes?.find((n) => n.id === id);
  if (!node?.component) return;
  if (
    !dependencies.value.some(
      (p) => lockKey(p) === referenceKey(node.component!),
    )
  ) {
    error.value =
      "This nested definition is missing from the embedded dependencies.";
    return;
  }
  inspected.value.push(referenceKey(node.component));
  selectedNodeId.value = "";
}
function openNestedDraft() {
  const lock = viewedLock.value;
  if (busy.value || !lock || !canReplace()) return;
  adopt({
    schemaVersion: 1,
    root: {
      packageId: lock.package.id,
      version: lock.package.version,
      digest: lock.digest,
    },
    packages: [lock, ...dependencyClosure(lock.package, dependencies.value)],
  });
  metadataOpen.value = true;
  notice.value =
    "Pinned definition copied to the root draft. Change its version before installing modified content.";
}
function removeNode() {
  if (!selectedNode.value || readonly.value) return;
  const id = selectedNode.value.id;
  draft.value.behavior.nodes = (draft.value.behavior.nodes || []).filter(
    (n) => n.id !== id,
  );
  const maps = [
    draft.value.behavior.bindings,
    ...draft.value.behavior.nodes.map((n) => n.inputs),
  ];
  maps.forEach((map) =>
    Object.keys(map || {}).forEach((key) => {
      if (map[key].node === id) delete map[key];
    }),
  );
  selectedNodeId.value = "";
}
function renameNode(event: Event) {
  if (!selectedNode.value || readonly.value || busy.value) return;
  const old = selectedNode.value.id,
    name = (event.target as HTMLInputElement).value;
  if (
    !/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(name) ||
    (behavior.value.nodes || []).some(
      (n) => n !== selectedNode.value && n.id === name,
    )
  ) {
    (event.target as HTMLInputElement).value = old;
    error.value =
      "Node IDs must be unique, begin with a letter and contain at most 64 letters, digits or underscores. The original ID was kept.";
    return;
  }
  selectedNode.value.id = name;
  [
    behavior.value.bindings,
    ...(behavior.value.nodes || []).map((n) => n.inputs),
  ].forEach((map) =>
    Object.values(map || {}).forEach((b) => {
      if (b.node === old) b.node = name;
    }),
  );
  selectedNodeId.value = name;
}
function addPort(direction: "inputs" | "outputs") {
  const ports = (draft.value.behavior[direction] ||= []);
  if (ports.length >= 16) return;
  const prefix = direction === "inputs" ? "input" : "output";
  let i = 1;
  while (ports.some((p) => p.name === `${prefix}${i}`)) i++;
  ports.push({ name: `${prefix}${i}`, type: "boolean" });
  if (direction === "inputs")
    inputValues.value[`${prefix}${i}`] = typedDefault("boolean");
  else
    (draft.value.behavior.bindings ||= {})[`${prefix}${i}`] = {
      literal: typedDefault("boolean"),
    };
}
function renamePort(
  direction: "inputs" | "outputs",
  index: number,
  event: Event,
) {
  if (busy.value || readonly.value) return;
  const port = draft.value.behavior[direction][index],
    old = port.name,
    name = (event.target as HTMLInputElement).value;
  if (
    !/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(name) ||
    draft.value.behavior[direction].some(
      (p, i) => i !== index && p.name === name,
    )
  ) {
    (event.target as HTMLInputElement).value = old;
    error.value =
      "Port names must be unique within their direction, begin with a letter and contain at most 64 letters, digits or underscores. Existing bindings and values were kept.";
    return;
  }
  port.name = name;
  if (old === name) return;
  if (direction === "inputs") {
    [
      draft.value.behavior.bindings,
      ...(draft.value.behavior.nodes || []).map((n) => n.inputs),
    ].forEach((map) =>
      Object.values(map || {}).forEach((b) => {
        if (b.input === old) b.input = name;
      }),
    );
    if (inputValues.value[old]) {
      inputValues.value[name] = inputValues.value[old];
      delete inputValues.value[old];
    }
  } else if (draft.value.behavior.bindings?.[old]) {
    draft.value.behavior.bindings[name] = draft.value.behavior.bindings[old];
    delete draft.value.behavior.bindings[old];
  }
  (draft.value.tests || []).forEach((test) => {
    const map = direction === "inputs" ? test.inputs : test.expected;
    if (map?.[old]) {
      map[name] = map[old];
      delete map[old];
    }
  });
}
function removePort(direction: "inputs" | "outputs", index: number) {
  const [port] = draft.value.behavior[direction].splice(index, 1);
  if (direction === "inputs") {
    delete inputValues.value[port.name];
    [
      draft.value.behavior.bindings,
      ...(draft.value.behavior.nodes || []).map((n) => n.inputs),
    ].forEach((map) =>
      Object.keys(map || {}).forEach((key) => {
        if (map[key].input === port.name) delete map[key];
      }),
    );
  } else delete draft.value.behavior.bindings?.[port.name];
  (draft.value.tests || []).forEach((test) => {
    const map = direction === "inputs" ? test.inputs : test.expected;
    if (map) delete map[port.name];
  });
}
function addCase(capture = false) {
  if (busy.value || (capture && (!record.value || evidenceStale.value))) return;
  const tests = (draft.value.tests ||= []);
  if (tests.length >= 32) return;
  let i = tests.length + 1;
  while (tests.some((c) => c.name === `Case ${i}`)) i++;
  tests.push({
    name: `Case ${i}`,
    inputs: cloneBehavior(inputValues.value),
    expected:
      capture && record.value
        ? cloneBehavior(record.value.result.outputs)
        : Object.fromEntries(
            (draft.value.behavior.outputs || []).map((p) => [
              p.name,
              typedDefault(p.type),
            ]),
          ),
  });
  selectedCase.value = tests.length - 1;
  tab.value = "evaluate";
}
async function evaluate() {
  await work("evaluate", async () => {
    const snapshot = captureDraft(),
      inputs = cloneBehavior(inputValues.value),
      stamp = `${snapshot.stamp}|${stableBehavior(inputs)}`,
      bundle = await prepare(snapshot);
    record.value = await api<EvaluationRecord>("/components/evaluate", "POST", {
      bundle,
      inputs,
    });
    recordStamp.value = stamp;
    history.value = await api<EvaluationSummary[]>("/components/evaluations");
    tab.value = "evaluate";
    notice.value =
      "Evaluation completed and its exact definition, inputs and trace were saved.";
  });
}
async function runTests() {
  await work("test", async () => {
    const snapshot = captureDraft(),
      bundle = await prepare(snapshot);
    testResult.value = await api<BehaviorTestResult>(
      "/components/test",
      "POST",
      { bundle },
    );
    testedStamp.value = snapshot.stamp;
    tab.value = "evaluate";
    notice.value = testResult.value.cases.length
      ? "Definition tests completed."
      : "No tests are defined. Add cases before claiming verification.";
  });
}
async function openEvaluation(id: string) {
  await work("history", async () => {
    const result = await api<EvaluationRecord>(
      `/components/evaluations/${encodeURIComponent(id)}`,
    );
    record.value = result;
    const root = rootLock(result.bundle);
    recordStamp.value = `${definitionStamp(
      root.package,
      result.bundle.packages.filter((p) => lockKey(p) !== lockKey(root)),
    )}|${stableBehavior(result.inputs || {})}`;
    notice.value =
      "Viewing immutable saved evidence. The current draft has not been replaced.";
  });
}
function loadRecordedSnapshot() {
  if (busy.value || !record.value || !canReplace()) return;
  adopt(record.value.bundle);
  inputValues.value = cloneBehavior(record.value.inputs || {});
  recordStamp.value = `${draftStamp.value}|${stableBehavior(inputValues.value)}`;
  tab.value = "evaluate";
  notice.value = "Recorded definition and inputs loaded as a local draft.";
}
function editAdvanced() {
  if (!pendingAdvanced.value) populateAdvanced();
  showAdvanced.value = true;
}
function populateAdvanced() {
  advancedJSON.value = JSON.stringify(
    { package: draft.value, dependencies: usedDependencies.value },
    null,
    2,
  );
  advancedBaseline.value = advancedJSON.value;
  advancedSourceStamp.value = draftStamp.value;
}
function resetAdvanced() {
  if (
    busy.value ||
    (pendingAdvanced.value &&
      !confirm(
        "Discard the unapplied JSON edits and reload the current graphical draft?",
      ))
  )
    return;
  populateAdvanced();
}
async function applyAdvanced() {
  if (
    busy.value ||
    (advancedSourceStamp.value !== draftStamp.value &&
      !confirm(
        "The graphical draft changed after this JSON buffer was opened. Replace it with the JSON definition?",
      ))
  )
    return;
  await work("prepare", async () => {
    const previousBaseline = baseline.value,
      bundle = await apiRawJson<ComponentBundle>(
        "/components/prepare",
        advancedJSON.value,
      );
    adopt(bundle);
    baseline.value = previousBaseline;
    notice.value =
      "Advanced definition prepared and validated from its original JSON. Export or install it to keep these edits.";
  });
}
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault();
    event.returnValue = "";
  }
}
watch(draftStamp, () => {
  issues.value = [];
});
onMounted(() => {
  initialize();
  window.addEventListener("beforeunload", beforeUnload);
});
onUnmounted(() => window.removeEventListener("beforeunload", beforeUnload));
</script>

<template>
  <section class="behavior-workbench">
    <div class="workbench-intro">
      <div>
        <span class="section-eyebrow">TYPED COMPONENT WORKBENCH</span>
        <h2>Build behavior from reusable parts</h2>
        <p>
          Define scalar inputs and outputs, compose pure operations, and nest
          exact component versions. Evaluate the resulting behavior with real
          values and inspect every step.
        </p>
      </div>
      <span class="workbench-version">DESIGN · EVALUATE</span>
    </div>
    <div class="workbench-library">
      <div>
        <label class="field"
          >Bundled composition<select
            v-model="selectedExample"
            aria-label="Bundled component example"
            :disabled="!!busy"
          >
            <option
              v-for="(example, index) in examples"
              :key="referenceKey(example.root)"
              :value="String(index)"
            >
              {{ rootLock(example).package.name }}
            </option>
          </select></label
        ><button
          class="button secondary"
          @click="loadExample"
          :disabled="!!busy || !examples.length"
        >
          Open example
        </button>
      </div>
      <div>
        <label class="field"
          >Installed definitions<select
            v-model="selectedLibrary"
            aria-label="Installed component definition"
            :disabled="!!busy"
          >
            <option value="">Select an exact version</option>
            <option
              v-for="entry in installed"
              :key="lockKey(entry)"
              :value="lockKey(entry)"
            >
              {{ entry.package.id }} @ {{ entry.package.version }}
            </option>
          </select></label
        ><button
          class="button secondary"
          @click="loadInstalled"
          :disabled="!!busy || !selectedLibrary"
        >
          Open definition</button
        ><button
          class="icon-button"
          aria-label="Refresh component workbench"
          @click="initialize"
          :disabled="!!busy"
        >
          <RefreshCw :size="15" />
        </button>
      </div>
    </div>
    <div class="workbench-draft-bar">
      <div>
        <Box :size="18" /><span
          ><strong>{{ draft.name }}</strong
          ><code>{{ currentReference }}</code></span
        ><small :class="{ installed: isInstalled }">{{
          pendingAdvanced
            ? "Unapplied JSON edits"
            : isInstalled
              ? "Installed exact version"
              : "Local draft"
        }}</small>
      </div>
      <div class="workbench-actions">
        <button
          class="button secondary"
          @click="createDraft"
          :disabled="!!busy"
        >
          <Plus :size="14" /> New definition</button
        ><button
          class="button secondary"
          @click="fileInput?.click()"
          :disabled="!!busy"
        >
          <ArrowUpFromLine :size="14" /> Import bundle</button
        ><button
          class="button secondary"
          @click="exportBundle"
          :disabled="!!busy"
        >
          <ArrowDownToLine :size="14" /> Export bundle</button
        ><button
          class="button secondary"
          @click="validateDraft"
          :disabled="!!busy"
        >
          <FileCheck2 :size="14" /> Validate composition</button
        ><button
          class="button primary"
          @click="installDraft"
          :disabled="!!busy"
        >
          <Save :size="14" /> Install bundle
        </button>
      </div>
      <input
        ref="fileInput"
        class="hidden"
        type="file"
        accept=".json,application/json"
        aria-label="Import component bundle JSON"
        @change="importBundle"
      />
    </div>
    <p class="workbench-draft-note">
      {{
        pendingAdvanced
          ? "Advanced JSON edits are retained but not yet applied. Apply or reset them before validating, evaluating, installing or exporting."
          : isInstalled
            ? "This draft matches an immutable catalog version. Change the version before installing edited content."
            : "Draft edits live in this browser session. Export a complete bundle or install it to keep the definition."
      }}
      Component evaluation is separate from application resource simulation.
    </p>
    <div v-if="busy" class="workbench-progress" role="status">
      <LoaderCircle :size="14" class="spin" />{{
        busy === "evaluate"
          ? "Evaluating and recording evidence…"
          : busy === "test"
            ? "Running definition tests…"
            : "Working with the Go component engine…"
      }}
    </div>
    <div v-if="error" class="workbench-error" role="alert">
      <strong>{{ error }}</strong>
      <ul v-if="issues.length">
        <li v-for="issue in issues" :key="issue.path + issue.message">
          <code>{{ issue.path }}</code> {{ issue.message }}
        </li>
      </ul>
    </div>
    <p v-if="notice" class="workbench-notice" role="status">
      <Check :size="14" />{{ notice }}
    </p>
    <details
      class="workbench-metadata"
      :open="metadataOpen"
      @toggle="metadataOpen = ($event.target as HTMLDetailsElement).open"
    >
      <summary>
        Definition identity & description <span>{{ currentReference }}</span>
      </summary>
      <fieldset :disabled="!!busy">
        <div class="metadata-grid">
          <label class="field"
            >Namespaced ID<input
              v-model="draft.id"
              aria-label="Component definition ID"
              maxlength="120" /></label
          ><label class="field"
            >Exact version<input
              v-model="draft.version"
              aria-label="Component definition version"
              maxlength="32"
              placeholder="1.0.0" /></label
          ><label class="field"
            >Display name<input
              v-model="draft.name"
              aria-label="Component definition name"
              maxlength="200" /></label
          ><label class="field"
            >Author<input
              v-model="draft.author"
              aria-label="Component author"
              maxlength="200" /></label
          ><label class="field"
            >License<input
              v-model="draft.license"
              aria-label="Component license"
              maxlength="200" /></label
          ><label class="field metadata-description"
            >Description<textarea
              v-model="draft.description"
              aria-label="Component description"
              rows="2"
              maxlength="4000"
            />
          </label>
        </div>
      </fieldset>
    </details>
    <nav class="workbench-tabs" aria-label="Component workbench views">
      <button :class="{ active: tab === 'compose' }" @click="tab = 'compose'">
        <Layers3 :size="15" /> Composition</button
      ><button
        :class="{ active: tab === 'interface' }"
        @click="tab = 'interface'"
      >
        <Braces :size="15" /> Typed interface</button
      ><button
        :class="{ active: tab === 'evaluate' }"
        @click="tab = 'evaluate'"
      >
        <Play :size="15" /> Evaluate & test</button
      ><button :class="{ active: tab === 'history' }" @click="tab = 'history'">
        <History :size="15" /> Evaluation history
        <span>{{ history.length }}</span>
      </button>
    </nav>
    <template v-if="tab === 'compose' || tab === 'interface'">
      <div v-if="readonly" class="nested-breadcrumb">
        <button
          class="button quiet"
          @click="
            inspected.pop();
            selectedNodeId = '';
          "
        >
          <ArrowLeft :size="14" /> Back one level</button
        ><span>Pinned definition · {{ viewed.id }} @ {{ viewed.version }}</span
        ><button
          class="button secondary"
          @click="openNestedDraft"
          :disabled="!!busy"
        >
          Open as root draft <ArrowRight :size="14" />
        </button>
      </div>
      <template v-if="tab === 'compose'">
        <BehaviorGraph
          :behavior="behavior"
          :dependencies="dependencies"
          :primitives="primitives"
          :selected-id="selectedNodeId"
          :readonly="readonly || !!busy"
          @select="selectedNodeId = $event"
          @inspect="inspectNode"
          @move="
            (id, position) => {
              const n = draft.behavior.nodes.find((node) => node.id === id);
              if (n && !readonly && !busy) n.position = position;
            }
          "
        />
        <div v-if="!readonly" class="composition-add">
          <div>
            <label class="field"
              >Pure primitive<select
                v-model="selectedPrimitive"
                aria-label="Primitive operation"
                :disabled="!!busy"
              >
                <option
                  v-for="primitive in primitives"
                  :key="primitive.op"
                  :value="primitive.op"
                >
                  {{ primitive.name }} · {{ primitive.op }}
                </option>
              </select></label
            ><button
              class="button secondary"
              @click="addPrimitive"
              :disabled="
                !!busy ||
                !primitives.length ||
                (draft.behavior.nodes?.length || 0) >= 32
              "
            >
              <Plus :size="14" /> Add operation
            </button>
          </div>
          <div>
            <label class="field"
              >Pinned nested component<select
                v-model="selectedNested"
                aria-label="Nested component definition"
                :disabled="!!busy"
              >
                <option value="">Select embedded or installed version</option>
                <option
                  v-for="entry in nestedChoices"
                  :key="lockKey(entry)"
                  :value="lockKey(entry)"
                >
                  {{ entry.package.name }} · {{ entry.package.version }}
                </option>
              </select></label
            ><button
              class="button secondary"
              @click="addNested"
              :disabled="
                !!busy ||
                !selectedNested ||
                (draft.behavior.nodes?.length || 0) >= 32
              "
            >
              <Plus :size="14" /> Add nested instance
            </button>
          </div>
        </div>
        <div class="composition-inspectors">
          <section class="workbench-panel">
            <div class="workbench-panel-heading">
              <h3>Internal node</h3>
              <span>{{
                readonly ? "Pinned / read-only" : "Editable bindings"
              }}</span>
            </div>
            <label class="field"
              >Selected node<select
                v-model="selectedNodeId"
                aria-label="Selected behavior node"
              >
                <option value="">Select a node</option>
                <option
                  v-for="node in behavior.nodes"
                  :key="node.id"
                  :value="node.id"
                >
                  {{ node.id }} · {{ node.op || node.component?.packageId }}
                </option>
              </select></label
            ><template v-if="selectedNode"
              ><fieldset :disabled="readonly || !!busy">
                <div class="node-identity-fields">
                  <label class="field"
                    >Local node ID<input
                      :value="selectedNode.id"
                      aria-label="Behavior node ID"
                      maxlength="64"
                      @change="renameNode" /></label
                  ><label class="field"
                    >X<input
                      v-model.number="selectedNode.position.x"
                      type="number"
                      step="1"
                      aria-label="Behavior node X" /></label
                  ><label class="field"
                    >Y<input
                      v-model.number="selectedNode.position.y"
                      type="number"
                      step="1"
                      aria-label="Behavior node Y"
                  /></label>
                </div>
                <p class="workbench-help">{{ selectedDescription }}</p>
                <BehaviorBindingEditor
                  v-for="port in selectedInputs"
                  :key="selectedNode.id + ':' + port.name"
                  :model-value="selectedNode.inputs?.[port.name]"
                  :expected-type="port.type"
                  :label="`${selectedNode.id}.${port.name}`"
                  :inputs="behavior.inputs || []"
                  :sources="nodeSources"
                  :disabled="readonly || !!busy"
                  @update:model-value="
                    (value) =>
                      setBinding(
                        (selectedNode!.inputs ||= {}),
                        port.name,
                        value,
                      )
                  "
                />
              </fieldset>
              <div v-if="selectedNode.component" class="nested-identity">
                <code
                  >{{ selectedNode.component.packageId }} @
                  {{ selectedNode.component.version }}</code
                ><small>SHA-256 {{ selectedNode.component.digest }}</small
                ><button
                  class="button secondary"
                  @click="inspectNode(selectedNode.id)"
                >
                  Inspect nested definition <ArrowRight :size="14" />
                </button>
              </div>
              <button
                v-if="!readonly"
                class="button danger"
                @click="removeNode"
                :disabled="!!busy"
              >
                <Trash2 :size="13" /> Remove internal node
              </button></template
            >
            <p v-else class="workbench-empty">
              Select an internal node to inspect its typed sources. A component
              may also bind inputs or literals directly to public outputs.
            </p>
          </section>
          <section class="workbench-panel">
            <div class="workbench-panel-heading">
              <h3>Public output bindings</h3>
              <span>{{ behavior.outputs?.length || 0 }} outputs</span>
            </div>
            <p class="workbench-help">
              Each output needs one source of exactly its declared type. Every
              internal node must contribute to an output.
            </p>
            <BehaviorBindingEditor
              v-for="port in behavior.outputs"
              :key="port.name"
              :model-value="behavior.bindings?.[port.name]"
              :expected-type="port.type"
              :label="`Output ${port.name}`"
              :inputs="behavior.inputs || []"
              :sources="sources"
              :disabled="readonly || !!busy"
              @update:model-value="
                (value) =>
                  setBinding((behavior.bindings ||= {}), port.name, value)
              "
            />
            <details class="dependency-list">
              <summary>
                {{ usedDependencies.length }} pinned dependency definitions
              </summary>
              <button
                v-for="lock in usedDependencies"
                :key="lockKey(lock)"
                @click="
                  inspected.push(lockKey(lock));
                  selectedNodeId = '';
                "
              >
                <Layers3 :size="13" /><span
                  >{{ lock.package.id }} @ {{ lock.package.version
                  }}<small>{{ lock.digest.slice(0, 16) }}</small></span
                ><ArrowRight :size="13" />
              </button>
            </details>
          </section>
        </div>
      </template>
      <div v-else class="interface-grid">
        <section
          v-for="direction in ['inputs', 'outputs'] as const"
          :key="direction"
          class="workbench-panel"
        >
          <div class="workbench-panel-heading">
            <h3>
              {{ direction === "inputs" ? "Public inputs" : "Public outputs" }}
            </h3>
            <button
              v-if="!readonly"
              class="button quiet"
              @click="addPort(direction)"
              :disabled="!!busy || (behavior[direction]?.length || 0) >= 16"
            >
              <Plus :size="13" /> Add
              {{ direction === "inputs" ? "input" : "output" }}
            </button>
          </div>
          <p class="workbench-help">
            {{
              direction === "inputs"
                ? "Named values supplied by a caller. No implicit coercion, truthiness or defaults."
                : "Named values returned to a caller. Bind each one in Composition."
            }}
          </p>
          <fieldset :disabled="readonly || !!busy">
            <div
              v-for="(port, index) in behavior[direction]"
              :key="direction + index"
              class="interface-port"
            >
              <label class="field"
                >Port name<input
                  :value="port.name"
                  :aria-label="`${direction} port ${index + 1} name`"
                  maxlength="64"
                  @change="renamePort(direction, index, $event)" /></label
              ><label class="field"
                >Scalar type<select
                  v-model="port.type"
                  :aria-label="`${direction} port ${index + 1} type`"
                >
                  <option v-for="type in scalarTypes" :key="type">
                    {{ type }}
                  </option>
                </select></label
              ><button
                class="icon-button"
                :aria-label="`Remove ${direction} port ${port.name}`"
                @click="removePort(direction, index)"
                :disabled="
                  readonly ||
                  !!busy ||
                  (direction === 'outputs' &&
                    (behavior.outputs?.length || 0) <= 1)
                "
              >
                <Trash2 :size="14" />
              </button>
            </div>
          </fieldset>
          <p v-if="!behavior[direction]?.length" class="workbench-empty">
            No {{ direction }} declared.
          </p>
        </section>
        <p class="workbench-help interface-note">
          Names use letters, digits and underscores, starting with a letter.
          Changing a port type can invalidate its bindings and tests; validation
          reports each mismatch.
        </p>
      </div>
      <div v-if="!readonly" class="workbench-advanced">
        <button
          class="button quiet"
          @click="showAdvanced ? (showAdvanced = false) : editAdvanced()"
        >
          <Braces :size="14" />{{
            showAdvanced ? "Hide advanced JSON" : "Advanced definition JSON"
          }}</button
        ><template v-if="showAdvanced"
          ><p class="workbench-help">
            Edits are checked from the original JSON text by the Go engine
            before replacing the draft. This view includes the root package and
            reachable dependency locks.
          </p>
          <p v-if="pendingAdvanced" class="workbench-help">
            Unapplied JSON edits are preserved when hidden and protected when
            replacing or closing this draft.
          </p>
          <textarea
            v-model="advancedJSON"
            class="advanced-json"
            aria-label="Advanced component definition JSON"
            spellcheck="false"
            :disabled="!!busy"
          /><button
            class="button secondary"
            @click="applyAdvanced"
            :disabled="!!busy"
          >
            Validate & apply JSON</button
          ><button
            class="button quiet"
            @click="resetAdvanced"
            :disabled="!!busy"
          >
            Reset JSON from current draft
          </button></template
        >
      </div>
    </template>
    <template v-if="tab === 'evaluate'">
      <div class="evaluation-editors">
        <section class="workbench-panel">
          <div class="workbench-panel-heading">
            <h3>Evaluation inputs</h3>
            <button
              class="button quiet"
              @click="resetInputs"
              :disabled="!!busy"
            >
              Reset inputs
            </button>
          </div>
          <p class="workbench-help">
            Values are sent to the Go evaluator. Missing or invalid values cause
            errors; they never mean an automatic permit or deny.
          </p>
          <fieldset :disabled="!!busy">
            <div class="evaluation-input-grid">
              <TypedValueEditor
                v-for="port in draft.behavior.inputs"
                :key="port.name"
                :model-value="inputValues[port.name]"
                :type="port.type"
                :label="`Input ${port.name}`"
                @update:model-value="
                  (value) => valueMap(inputValues, port.name, value)
                "
              />
            </div>
          </fieldset>
          <p v-if="!draft.behavior.inputs?.length" class="workbench-help">
            This definition takes no external inputs.
          </p>
          <div class="evaluate-buttons">
            <button class="button primary" @click="evaluate" :disabled="!!busy">
              <Play :size="14" /> Evaluate & record</button
            ><button
              class="button secondary"
              @click="runTests"
              :disabled="!!busy"
            >
              <FileCheck2 :size="14" /> Run definition tests
            </button>
          </div>
        </section>
        <section class="workbench-panel">
          <div class="workbench-panel-heading">
            <h3>Definition test cases</h3>
            <button
              class="button quiet"
              @click="addCase()"
              :disabled="!!busy || (draft.tests?.length || 0) >= 32"
            >
              <Plus :size="13" /> Add test case
            </button>
          </div>
          <p class="workbench-help">
            Tests are part of the versioned definition. Each case supplies every
            input and expects every public output.
          </p>
          <label class="field"
            >Selected test<select
              v-model.number="selectedCase"
              aria-label="Selected component test"
              :disabled="!!busy"
            >
              <option
                v-for="(test, index) in draft.tests"
                :key="index"
                :value="index"
              >
                {{ test.name }}
              </option>
            </select></label
          ><template v-if="activeCase"
            ><fieldset :disabled="!!busy">
              <label class="field"
                >Test name<input
                  v-model="activeCase.name"
                  aria-label="Component test name"
                  maxlength="100"
              /></label>
              <div class="test-value-columns">
                <div>
                  <strong>Inputs</strong
                  ><TypedValueEditor
                    v-for="port in draft.behavior.inputs"
                    :key="port.name"
                    :model-value="activeCase.inputs?.[port.name]"
                    :type="port.type"
                    :label="`Test input ${port.name}`"
                    @update:model-value="
                      (value) =>
                        valueMap((activeCase!.inputs ||= {}), port.name, value)
                    "
                  />
                </div>
                <div>
                  <strong>Expected outputs</strong
                  ><TypedValueEditor
                    v-for="port in draft.behavior.outputs"
                    :key="port.name"
                    :model-value="activeCase.expected?.[port.name]"
                    :type="port.type"
                    :label="`Expected ${port.name}`"
                    @update:model-value="
                      (value) =>
                        valueMap(
                          (activeCase!.expected ||= {}),
                          port.name,
                          value,
                        )
                    "
                  />
                </div>
              </div>
            </fieldset>
            <button
              class="button danger"
              @click="
                draft.tests?.splice(selectedCase, 1);
                selectedCase = 0;
              "
              :disabled="!!busy"
            >
              <Trash2 :size="13" /> Remove test case
            </button></template
          >
          <p v-else class="workbench-empty">
            No cases defined. Add a case or capture the outputs from a matching
            successful evaluation.
          </p>
        </section>
      </div>
      <BehaviorEvidence
        :busy="!!busy"
        :record="record"
        :stale="evidenceStale"
        :test-result="testResult"
        :tests-stale="testsStale"
        @load-snapshot="loadRecordedSnapshot"
        @capture-test="addCase(true)"
      />
    </template>
    <template v-if="tab === 'history'"
      ><section class="workbench-panel">
        <div class="workbench-panel-heading">
          <h3>Immutable evaluation history</h3>
          <span>Newest {{ history.length }} evaluations</span>
        </div>
        <p class="workbench-help">
          Successful evaluations retain the exact bundle, typed inputs, outputs
          and trace. Opening evidence leaves the current draft unchanged.
        </p>
        <div class="evaluation-history">
          <button
            v-for="entry in history"
            :key="entry.id"
            :class="{ selected: record?.id === entry.id }"
            @click="openEvaluation(entry.id)"
            :disabled="!!busy"
          >
            <History :size="16" /><span
              ><strong
                >{{ entry.componentId }} @ {{ entry.componentVersion }}</strong
              ><small
                >{{ new Date(entry.createdAt).toLocaleString() }} ·
                {{ entry.operations }} operations ·
                {{ entry.invocations }} invocations</small
              ></span
            ><ArrowRight :size="15" />
          </button>
        </div>
        <p v-if="!history.length" class="workbench-empty">
          Run an evaluation to create persistent evidence.
        </p>
      </section>
      <BehaviorEvidence
        :busy="!!busy"
        :record="record"
        :stale="evidenceStale"
        :test-result="null"
        :tests-stale="false"
        @load-snapshot="loadRecordedSnapshot"
        @capture-test="addCase(true)"
    /></template>
    <p class="workbench-boundary">
      <CircleAlert :size="15" /><span
        >Pure value computation only: no state, side effects, parallel workflow
        engine, simulated hardware timing or deployment. The access-policy
        examples demonstrate composition; they do not authenticate users or
        enforce production authorization.</span
      >
    </p>
  </section>
</template>

<style scoped>
.behavior-workbench {
  padding: 22px 30px 28px;
  background: #f5f8fa;
  min-width: 0;
}
.workbench-intro {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  margin-bottom: 22px;
}
.workbench-intro h2 {
  font-size: 23px;
  color: #304c62;
  letter-spacing: -0.5px;
  margin: 8px 0;
}
.workbench-intro p {
  color: #657e8f;
  font-size: 12px;
  line-height: 1.8;
  max-width: 730px;
  margin: 0;
}
.workbench-version {
  color: #659c8c;
  font-size: 12px;
  letter-spacing: 0.8px;
  border: 1px solid #cfe5dc;
  background: #eef8f2;
  border-radius: 5px;
  padding: 8px;
  white-space: nowrap;
}
.workbench-library {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  padding: 16px;
  border: 1px solid #dfe8ee;
  background: #fff;
  border-radius: 9px;
  margin-bottom: 16px;
}
.workbench-library > div,
.composition-add > div {
  display: flex;
  align-items: end;
  gap: 8px;
  min-width: 0;
}
.workbench-library .field,
.composition-add .field {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: 12px;
}
.workbench-library select,
.composition-add select {
  font-size: 12px;
}
.workbench-library .button {
  padding: 10px 11px;
  font-size: 12px;
  white-space: nowrap;
}
.workbench-draft-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 14px;
}
.workbench-draft-bar > div:first-child {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #6a8b96;
  min-width: 0;
}
.workbench-draft-bar strong,
.workbench-draft-bar code {
  display: block;
}
.workbench-draft-bar strong {
  color: #405e73;
  font-size: 13px;
}
.workbench-draft-bar code {
  font-size: 12px;
  margin-top: 5px;
  color: #657e8f;
}
.workbench-draft-bar small {
  font-size: 12px;
  color: #ab8c59;
  background: #fbf5e7;
  border-radius: 4px;
  padding: 5px 7px;
  white-space: nowrap;
}
.workbench-draft-bar small.installed {
  color: #5b9a80;
  background: #edf7f0;
}
.workbench-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.workbench-actions .button {
  font-size: 12px;
  padding: 8px 10px;
}
.workbench-draft-note {
  font-size: 12px;
  line-height: 1.7;
  color: #657e8f;
  margin: 12px 0 17px;
}
.workbench-error {
  padding: 13px 15px;
  border: 1px solid #edcabe;
  background: #fff7f3;
  color: #ae755d;
  border-radius: 7px;
  font-size: 12px;
  line-height: 1.8;
  margin: 13px 0;
  overflow-wrap: anywhere;
}
.workbench-error ul {
  padding-left: 18px;
  margin-bottom: 0;
}
.workbench-notice,
.workbench-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  border-radius: 6px;
  color: #5e9982;
  background: #eef9f2;
  padding: 11px 13px;
  font-size: 12px;
  line-height: 1.7;
}
.workbench-progress {
  background: #edf5fa;
  color: #7995a9;
}
.workbench-metadata {
  background: #fff;
  border: 1px solid #e0e8ee;
  border-radius: 7px;
}
.workbench-metadata summary {
  cursor: pointer;
  color: #6b8496;
  font-size: 12px;
  padding: 13px 15px;
}
.workbench-metadata summary span {
  color: #657e8f;
  font-size: 12px;
  margin-left: 15px;
}
.metadata-grid {
  display: grid;
  grid-template-columns: 2fr 1fr 2fr;
  gap: 13px;
  padding: 0 15px 15px;
}
.metadata-grid .field {
  margin: 0;
  font-size: 12px;
}
.metadata-description {
  grid-column: span 2;
}
.metadata-grid textarea {
  resize: vertical;
  min-height: 52px;
}
.workbench-tabs {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 7px;
  border-bottom: 1px solid #dfe7ed;
  margin: 20px 0 16px;
  padding-bottom: 12px;
}
.workbench-tabs button {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #657e8f;
  background: transparent;
  padding: 9px 12px;
  font-size: 12px;
  border-radius: 5px;
}
.workbench-tabs button.active {
  color: #3f927e;
  background: #e9f5ef;
}
.workbench-tabs span {
  color: #657e8f;
  font-size: 12px;
}
.nested-breadcrumb {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
  background: #edf6f3;
  border: 1px solid #cfe4da;
  padding: 10px;
  border-radius: 6px;
  margin-bottom: 14px;
}
.nested-breadcrumb > span {
  font-size: 12px;
  color: #699483;
  overflow-wrap: anywhere;
}
.nested-breadcrumb .button {
  padding: 7px 10px;
  font-size: 12px;
}
.composition-add {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
  margin: 15px 0;
  padding: 14px;
  background: #fff;
  border: 1px solid #e1e9ee;
  border-radius: 7px;
}
.composition-add .button {
  padding: 10px;
  font-size: 12px;
  white-space: nowrap;
}
.composition-inspectors,
.interface-grid,
.evaluation-editors {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 17px;
}
.workbench-panel {
  padding: 19px;
  border: 1px solid #e1e9ef;
  border-radius: 8px;
  background: #fff;
  min-width: 0;
}
.workbench-panel-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 9px;
  margin-bottom: 13px;
}
.workbench-panel-heading h3 {
  font-size: 14px;
  color: #48667c;
  margin: 0;
}
.workbench-panel-heading > span {
  font-size: 12px;
  color: #657e8f;
}
.workbench-panel-heading .button {
  font-size: 12px;
  padding: 5px;
}
.workbench-panel > .field {
  font-size: 12px;
}
.workbench-help {
  font-size: 12px;
  color: #657e8f;
  line-height: 1.8;
  margin: 0 0 13px;
}
.workbench-empty {
  font-size: 12px;
  color: #657e8f;
  line-height: 1.9;
  padding: 20px 5px;
}
.node-identity-fields {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr;
  gap: 9px;
}
.node-identity-fields .field {
  font-size: 12px;
}
.node-identity-fields input {
  font-size: 12px;
}
.workbench-panel .button.danger {
  padding: 8px 10px;
  font-size: 12px;
  margin-top: 8px;
}
.nested-identity {
  background: #f4f9f7;
  border: 1px solid #deebe4;
  padding: 12px;
  border-radius: 5px;
  margin: 12px 0;
}
.nested-identity code,
.nested-identity small {
  display: block;
  overflow-wrap: anywhere;
  font-size: 12px;
  color: #71988a;
}
.nested-identity small {
  font-size: 12px;
  margin: 7px 0;
  color: #657e8f;
}
.nested-identity .button {
  font-size: 12px;
  margin-top: 5px;
  padding: 7px 10px;
}
.dependency-list {
  font-size: 12px;
  color: #657e8f;
  margin-top: 16px;
}
.dependency-list summary {
  cursor: pointer;
  padding: 10px 0;
}
.dependency-list button {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px;
  background: #f6faf8;
  border: 1px solid #e3eee8;
  border-radius: 5px;
  margin-top: 6px;
  width: 100%;
  text-align: left;
  color: #6a9485;
  font-size: 12px;
}
.dependency-list span {
  flex: 1;
  overflow-wrap: anywhere;
}
.dependency-list small {
  display: block;
  font-size: 12px;
  color: #657e8f;
  margin-top: 4px;
}
.interface-port {
  display: grid;
  grid-template-columns: 1.5fr 1fr 25px;
  gap: 9px;
  align-items: end;
  margin-bottom: 12px;
}
.interface-port .field {
  margin: 0;
  font-size: 12px;
}
.interface-port .icon-button {
  margin-bottom: 3px;
}
.interface-note {
  grid-column: 1 / -1;
}
.workbench-advanced {
  margin-top: 16px;
}
.workbench-advanced > .button {
  font-size: 12px;
  padding: 9px 11px;
}
.advanced-json {
  display: block;
  width: 100%;
  height: 330px;
  resize: vertical;
  font-family: monospace;
  font-size: 12px;
  line-height: 1.7;
  background: #fbfdfd;
  color: #5d7888;
  border: 1px solid #dbe6ec;
  border-radius: 7px;
  padding: 16px;
  margin: 10px 0;
}
.evaluation-input-grid {
  display: grid;
  gap: 13px;
}
.evaluate-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 19px;
}
.evaluate-buttons .button {
  font-size: 12px;
  padding: 9px 12px;
}
.test-value-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}
.test-value-columns > div {
  display: grid;
  gap: 11px;
  align-content: start;
}
.test-value-columns > div > strong {
  font-size: 12px;
  color: #657e8f;
  margin-bottom: 4px;
}
.evaluation-history {
  display: grid;
  gap: 7px;
  max-height: 340px;
  overflow: auto;
}
.evaluation-history button {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px;
  border: 1px solid #e3ebf0;
  border-radius: 6px;
  background: #fbfdfd;
  text-align: left;
  color: #657e8f;
}
.evaluation-history button.selected {
  border-color: #657e8f;
  background: #f3fbf6;
}
.evaluation-history span {
  flex: 1;
  min-width: 0;
}
.evaluation-history strong {
  display: block;
  font-size: 12px;
  color: #617f91;
  overflow-wrap: anywhere;
}
.evaluation-history small {
  display: block;
  font-size: 12px;
  color: #657e8f;
  margin-top: 7px;
}
.workbench-boundary {
  display: flex;
  gap: 9px;
  font-size: 12px;
  line-height: 1.8;
  color: #657e8f;
  margin: 22px 0 0;
}
.workbench-boundary svg {
  flex-shrink: 0;
  margin-top: 2px;
}
fieldset {
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}
fieldset:disabled {
  opacity: 0.8;
}
@media (max-width: 1180px) {
  .behavior-workbench {
    padding: 20px;
  }
  .workbench-library,
  .composition-add {
    grid-template-columns: 1fr;
  }
  .workbench-version {
    display: none;
  }
  .workbench-draft-bar {
    align-items: flex-start;
  }
  .workbench-actions .button {
    font-size: 12px;
  }
  .composition-inspectors,
  .evaluation-editors {
    grid-template-columns: 1fr;
  }
  .metadata-grid {
    grid-template-columns: 1fr 1fr;
  }
  .metadata-description {
    grid-column: span 2;
  }
}
@media (max-width: 700px) {
  .behavior-workbench {
    padding: 14px;
  }
  .workbench-intro h2 {
    font-size: 20px;
  }
  .workbench-library {
    padding: 12px;
    gap: 13px;
  }
  .workbench-library > div,
  .composition-add > div {
    flex-wrap: wrap;
  }
  .workbench-library .field,
  .composition-add .field {
    flex-basis: 100%;
  }
  .workbench-actions {
    gap: 5px;
  }
  .workbench-tabs {
    gap: 4px;
  }
  .workbench-tabs button {
    font-size: 12px;
    padding: 8px;
  }
  .workbench-metadata summary span {
    display: block;
    margin: 7px 0 0;
  }
  .metadata-grid,
  .interface-grid {
    grid-template-columns: 1fr;
  }
  .metadata-description {
    grid-column: auto;
  }
  .nested-breadcrumb {
    flex-wrap: wrap;
  }
  .nested-breadcrumb > span {
    order: -1;
    width: 100%;
  }
  .workbench-panel {
    padding: 15px;
  }
  .test-value-columns {
    grid-template-columns: 1fr;
  }
  .workbench-draft-bar > div:first-child {
    flex-wrap: wrap;
  }
}
.behavior-workbench :deep(.field) {
  font-size: 12px;
}
.behavior-workbench :deep(input),
.behavior-workbench :deep(select),
.behavior-workbench :deep(textarea) {
  font-size: 12px;
  color: #415d70;
}
</style>
