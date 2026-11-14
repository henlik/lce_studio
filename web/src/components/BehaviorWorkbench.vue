<script setup lang="ts">
import { useWorkspaceDraft } from "../composables/useWorkspaceDraft";
import { useActionFeedback } from "../composables/useNotifications";
import DraftStatus from "./DraftStatus.vue";
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
  bindingShape,
  componentDataContracts,
  typeKey,
  typeLabel,
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
import { focusField } from "../validationFocus";
import ComponentPalette from "./ComponentPalette.vue";
import { nodeColors, nodeColor } from "../graphPresentation";
import TypedValueEditor from "./TypedValueEditor.vue";
import {
  dataKey,
  recordRef,
  resolveRecord,
  dataRefKey,
  type DataContractRecord,
  type DataTypeRef,
} from "../dataTypes";

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
      inputs: [],
      outputs: [],
      nodes: [],
      bindings: {},
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
const dataCatalog = ref<DataContractRecord[]>([]);
const selectedDataContract = ref("");
const allDataContracts = computed(() =>
  componentDataContracts(
    draft.value,
    dependencyClosure(draft.value, dependencies.value),
  ),
);
const recordChoices = computed(() =>
  allDataContracts.value.flatMap((l) =>
    l.contract.records.map((r) => ({
      key: dataRefKey(recordRef(l, r.name)),
      ref: recordRef(l, r.name),
      label: `${r.name} · ${l.contract.id}@${l.contract.version} · ${l.digest.slice(0, 8)}`,
    })),
  ),
);
const portChoices = computed(() => [
  ...scalarTypes.map((type) => ({
    key: typeKey({ type }),
    type,
    recordRef: undefined as DataTypeRef | undefined,
    label: type as string,
  })),
  ...(viewed.value.schemaVersion === 3
    ? recordChoices.value.flatMap((r) =>
        (["record", "collection"] as const).map((type) => ({
          key: typeKey({ type, recordRef: r.ref }),
          type,
          recordRef: r.ref,
          label: `${type} <${r.label}>`,
        })),
      )
    : []),
]);
const availablePrimitives = computed(() =>
  primitives.value.filter(
    (p) =>
      draft.value.schemaVersion === 3 ||
      ![
        "record_make",
        "record_get",
        "collection_count",
        "collection_at",
        "collection_append",
      ].includes(p.op),
  ),
);
function upgradeStructured() {
  if (readonly.value || busy.value) return;
  draft.value.schemaVersion = 3;
  notice.value =
    "Schema 3 enabled. Embed an exact data contract to use its record and collection types.";
}
function embedDataContract() {
  const l = dataCatalog.value.find(
    (l) => dataKey(l) === selectedDataContract.value,
  );
  if (!l || busy.value || readonly.value) return;
  const old = (draft.value.dataContracts || []).find(
    (x) =>
      x.contract.id === l.contract.id &&
      x.contract.version === l.contract.version,
  );
  if (old && old.digest !== l.digest) {
    error.value =
      "This ID/version is already pinned to different data content.";
    return;
  }
  if (old) return;
  if ((draft.value.dataContracts?.length || 0) >= 8) {
    error.value = "A component may embed at most eight data contracts.";
    return;
  }
  draft.value.schemaVersion = 3;
  (draft.value.dataContracts ||= []).push(
    cloneBehavior({ contract: l.contract, digest: l.digest }),
  );
  notice.value =
    "Exact data contract embedded in the component. Select its types in Typed interface.";
}
function setPortType(port: BehaviorPort, event: Event) {
  const selected = portChoices.value.find(
    (p) => p.key === (event.target as HTMLSelectElement).value,
  );
  if (!selected) return;
  port.type = selected.type;
  if (selected.recordRef) port.recordRef = cloneBehavior(selected.recordRef);
  else delete port.recordRef;
}
function setNodeRecord(event: Event) {
  if (!selectedNode.value) return;
  const choice = recordChoices.value.find(
    (r) => r.key === (event.target as HTMLSelectElement).value,
  );
  if (!choice) return;
  selectedNode.value.recordRef = cloneBehavior(choice.ref);
  const fields =
    resolveRecord(choice.ref, allDataContracts.value)?.fields || [];
  selectedNode.value.inputs = Object.fromEntries(
    fields.map((f) => [
      f.name,
      selectedNode.value!.inputs[f.name] || { literal: typedDefault(f.type) },
    ]),
  );
}
const getFieldChoices = computed(() => {
  const node = selectedNode.value;
  if (!node || node.op !== "record_get") return [];
  const source = bindingShape(
    node.inputs.value,
    behavior.value,
    dependencies.value,
    primitives.value,
    allDataContracts.value,
  );
  return resolveRecord(source?.recordRef, allDataContracts.value)?.fields || [];
});
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
const inputValues = ref<Record<string, TypedValue>>({});
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
    ? nodeInputs(
        selectedNode.value,
        dependencies.value,
        primitives.value,
        allDataContracts.value,
        behavior.value,
      )
    : [],
);
const sources = computed(() =>
  (behavior.value.nodes || []).flatMap((n) =>
    nodeOutputs(
      n,
      behavior.value,
      dependencies.value,
      primitives.value,
      allDataContracts.value,
    ),
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
          typedDefault(p.type, p.recordRef, allDataContracts.value),
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
  installed.value = values.filter((p): p is InstalledComposite =>
    [2, 3].includes(p.package.schemaVersion),
  );
}
async function initialize() {
  await work("load", async () => {
    const values = await Promise.allSettled([
      api<PrimitiveSpec[]>("/components/structured-primitives"),
      Promise.all([
        api<ComponentBundle[]>("/components/composite-examples"),
        api<ComponentBundle[]>("/components/structured-examples"),
      ]).then((values) => values.flat()),
      refreshCatalog(),
      api<EvaluationSummary[]>("/components/evaluations"),
      api<DataContractRecord[]>("/data/contracts"),
    ]);
    if (values[0].status === "fulfilled") primitives.value = values[0].value;
    else fail(values[0].reason);
    if (values[1].status === "fulfilled") {
      examples.value = values[1].value;
    } else fail(values[1].reason);
    if (values[2].status === "rejected") fail(values[2].reason);
    if (values[4].status === "fulfilled") dataCatalog.value = values[4].value;
    else fail(values[4].reason);
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
    "Empty component draft. Add an input or output, then connect primitives or reusable components. Examples open only when you choose Open example.";
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
const inspectorHidden = ref(false),
  paletteHidden = ref(false);
let pendingPosition: { x: number; y: number } | undefined;
async function addFromPalette(
  kind: string,
  key: string,
  position?: { x: number; y: number },
) {
  if (busy.value || readonly.value || draft.value.behavior.nodes.length >= 32)
    return;
  pendingPosition = position;
  try {
    if (kind === "primitive") {
      selectedPrimitive.value = key;
      addPrimitive();
    } else if (kind === "nested") {
      selectedNested.value = key;
      await addNested();
    } else if (kind === "example") {
      const bundle = examples.value.find((b) => referenceKey(b.root) === key);
      if (!bundle) return;
      mergeDependencies(bundle.packages);
      if (rootLock(bundle).package.schemaVersion === 3)
        draft.value.schemaVersion = 3;
      selectedNested.value = key;
      await addNested();
    }
  } catch (e) {
    fail(e);
  } finally {
    pendingPosition = undefined;
  }
}
function selectCanvasNode(id: string) {
  selectedNodeId.value = id;
  inspectorHidden.value = false;
}
function reorderNode(delta: number) {
  const nodes = draft.value.behavior.nodes,
    index = nodes.findIndex((n) => n.id === selectedNodeId.value),
    target = index + delta;
  if (
    readonly.value ||
    busy.value ||
    index < 0 ||
    target < 0 ||
    target >= nodes.length
  )
    return;
  [nodes[index].position, nodes[target].position] = [
    nodes[target].position,
    nodes[index].position,
  ];
  const [node] = nodes.splice(index, 1);
  nodes.splice(target, 0, node);
  notice.value =
    "Node order and placement updated. Typed bindings still determine execution.";
}
function autoArrange() {
  const nodes = draft.value.behavior.nodes,
    levels = new Map<string, number>();
  for (let pass = 0; pass < nodes.length; pass++)
    for (const n of nodes) {
      const deps = Object.values(n.inputs)
        .filter((b) => b.node)
        .map((b) => b.node!);
      if (deps.every((id) => levels.has(id)))
        levels.set(
          n.id,
          Math.min(
            nodes.length,
            Math.max(-1, ...deps.map((id) => levels.get(id)!)) + 1,
          ),
        );
    }
  const rows = new Map<number, number>();
  for (const n of nodes) {
    const level = levels.get(n.id) || 0,
      row = rows.get(level) || 0;
    n.position = { x: level * 340, y: 30 + row * 240 };
    rows.set(level, row + 1);
  }
  notice.value = "Canvas arranged by dependencies. Connections are unchanged.";
}
function issueNodeIndex(path: string) {
  const pkg = path.match(/packages\[(\d+)\]/);
  if (pkg && Number(pkg[1]) !== 0) return -1;
  return Number(path.match(/behavior\.nodes\[(\d+)\]/)?.[1] ?? -1);
}
const invalidNodeIds = computed(() =>
  issues.value
    .map((i) => behavior.value.nodes[issueNodeIndex(i.path)]?.id)
    .filter((id): id is string => !!id),
);
function nodeIssue(port?: string) {
  const index = behavior.value.nodes.findIndex(
    (n) => n.id === selectedNodeId.value,
  );
  return issues.value
    .filter(
      (i) =>
        issueNodeIndex(i.path) === index &&
        (!port || i.path.includes(`.inputs.${port}`)),
    )
    .map((i) => i.message)
    .join(" ");
}
function focusIssue(issue: ValidationIssue) {
  const index = issueNodeIndex(issue.path);
  if (index >= 0) {
    tab.value = "compose";
    inspected.value = [];
    selectCanvasNode(draft.value.behavior.nodes[index]?.id || "");
  } else if (issue.path.includes("behavior.bindings")) {
    tab.value = "compose";
    inspectorHidden.value = false;
  } else if (
    issue.path.includes("behavior.inputs") ||
    issue.path.includes("behavior.outputs")
  )
    tab.value = "interface";
  else if (issue.path.includes("tests")) tab.value = "evaluate";
  else metadataOpen.value = true;
  focusField(
    document.querySelector(".behavior-workbench"),
    index >= 0
      ? ".node-inspector .invalid select, .node-inspector input"
      : issue.path.includes("behavior.bindings")
        ? ".composition-outputs select"
        : issue.path.includes("behavior.inputs") ||
            issue.path.includes("behavior.outputs")
          ? ".interface-grid input"
          : ".definition-metadata input",
  );
}
function insertNode(node: Omit<BehaviorNode, "position">) {
  const nodes = (draft.value.behavior.nodes ||= []);
  if (nodes.length >= 32) {
    notice.value = "A definition supports up to 32 internal nodes.";
    return;
  }
  nodes.push({
    ...node,
    position: pendingPosition || {
      x: (nodes.length % 3) * 340,
      y: Math.floor(nodes.length / 3) * 235 + 30,
    },
  });
  pendingPosition = undefined;
  inspectorHidden.value = false;
  notice.value = `Added ${node.id}. Configure its required inputs in the right inspector.`;
  selectedNodeId.value = node.id;
  tab.value = "compose";
  inspected.value = [];
}
function addPrimitive() {
  const spec = availablePrimitives.value.find(
    (p) => p.op === selectedPrimitive.value,
  );
  if (!spec || readonly.value) return;
  const metadata =
    spec.op === "record_make" && recordChoices.value[0]
      ? { recordRef: cloneBehavior(recordChoices.value[0].ref) }
      : spec.op === "record_get"
        ? { field: "" }
        : {};
  const node: BehaviorNode = {
    id: freshNodeId(spec.op),
    op: spec.op,
    inputs: {},
    position: { x: 0, y: 0 },
    ...metadata,
  };
  insertNode(node);
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
  if (entry.package.schemaVersion === 3 && draft.value.schemaVersion !== 3) {
    error.value =
      "Enable structured data on this component before adding a schema 3 component.";
    return;
  }
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
      inputs: {},
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
              typedDefault(p.type, p.recordRef, allDataContracts.value),
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
const autosave = useWorkspaceDraft("components", {
  draft,
  dependencies,
  tab,
  inputValues,
  selectedNodeId,
  inspected,
  selectedCase,
  advancedJSON,
  advancedBaseline,
  advancedSourceStamp,
  showAdvanced,
  metadataOpen,
  inspectorHidden,
  paletteHidden,
  record,
  recordStamp,
  testResult,
  testedStamp,
  baseline,
  installedStamp,
});
useActionFeedback("Components", busy, notice, error);
function beforeUnload(event: BeforeUnloadEvent) {
  if (autosave.unsaved()) {
    event.preventDefault();
    event.returnValue = "";
  }
}
watch(draftStamp, () => {
  issues.value = [];
});
onMounted(async () => {
  await autosave.ready;
  initialize();
  window.addEventListener("beforeunload", beforeUnload);
});
onUnmounted(() => window.removeEventListener("beforeunload", beforeUnload));
</script>

<template>
  <section
    class="behavior-workbench"
    :inert="autosave.status.value === 'loading'"
  >
    <DraftStatus :draft="autosave" />
    <div class="workbench-intro">
      <div>
        <span class="section-eyebrow">TYPED COMPONENT WORKBENCH</span>
        <h2>Build behavior from reusable parts</h2>
        <p>
          Define scalar or structured inputs and outputs, compose pure
          operations, and nest exact component versions. Evaluate the resulting
          behavior with real values and inspect every step.
        </p>
      </div>
      <span class="workbench-version">DESIGN · EVALUATE</span>
    </div>
    <details class="library-disclosure">
      <summary>Open an installed definition or example</summary>
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
    </details>
    <details class="workbench-data-contracts">
      <summary>Structured data & contracts</summary>
      <div class="workbench-data-heading">
        <strong
          >Data contracts · component schema {{ draft.schemaVersion }}</strong
        ><button
          v-if="draft.schemaVersion === 2"
          class="button secondary"
          :disabled="!!busy || readonly"
          @click="upgradeStructured"
        >
          Enable structured data
        </button>
      </div>
      <p class="workbench-help">
        Record and collection ports use exact embedded type identities. Install
        a data contract in Data, then refresh and embed it here. Scalar
        definitions remain schema 2 until explicitly upgraded.
      </p>
      <div v-if="draft.schemaVersion === 3" class="workbench-data-picker">
        <label class="field"
          >Installed data contract<select
            v-model="selectedDataContract"
            aria-label="Component data contract"
            :disabled="!!busy || readonly"
          >
            <option value="">Choose an installed exact version</option>
            <option
              v-for="lock in dataCatalog"
              :key="dataKey(lock)"
              :value="dataKey(lock)"
            >
              {{ lock.contract.id }} @ {{ lock.contract.version }}
            </option>
          </select></label
        ><button
          class="button secondary"
          :disabled="!!busy || readonly || !selectedDataContract"
          @click="embedDataContract"
        >
          Embed data contract
        </button>
      </div>
      <div
        v-for="(lock, index) in draft.dataContracts"
        :key="dataKey(lock)"
        class="workbench-data-lock"
      >
        <span
          >{{ lock.contract.name }} · {{ lock.contract.id }} @
          {{ lock.contract.version }}<small>{{ lock.digest }}</small></span
        ><button
          class="button quiet"
          :disabled="!!busy || readonly"
          @click="draft.dataContracts?.splice(index, 1)"
        >
          Remove embedded contract
        </button>
      </div>
    </details>
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
            : "Drafts autosave on this browser and recover after reload. Export a bundle to share it, or install an exact version in this studio."
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
          <button class="issue-link" @click="focusIssue(issue)">
            <code>{{ issue.path }}</code> {{ issue.message }} → Fix
          </button>
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
          ><label class="field"
            >Design level<select
              v-model="draft.designLevel"
              aria-label="Component design level"
            >
              <option :value="undefined">Component</option>
              <option value="system">System · pure composition</option>
            </select></label
          >
          <label class="field metadata-description"
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
    <section
      v-if="
        !readonly &&
        (tab === 'compose' || tab === 'interface') &&
        !behavior.inputs.length &&
        !behavior.outputs.length &&
        !behavior.nodes.length
      "
      class="workbench-panel"
    >
      <p class="workbench-help">
        This component is empty. Start with its typed interface, then add
        primitives or reusable components in Composition.
      </p>
      <div class="workbench-actions">
        <button
          class="button secondary"
          :disabled="!!busy"
          @click="
            addPort('inputs');
            tab = 'interface';
          "
        >
          Add first input
        </button>
        <button
          class="button secondary"
          :disabled="!!busy"
          @click="
            addPort('outputs');
            tab = 'interface';
          "
        >
          Add first output
        </button>
      </div>
    </section>
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
        <div class="workbench-actions canvas-layout-actions">
          <button
            class="button secondary"
            @click="inspectorHidden = !inspectorHidden"
          >
            {{ inspectorHidden ? "Show" : "Hide" }} inspector</button
          ><button
            class="button secondary"
            @click="paletteHidden = !paletteHidden"
          >
            {{ paletteHidden ? "Show" : "Hide" }} library</button
          ><button
            class="button secondary"
            :disabled="readonly || !!busy"
            @click="autoArrange"
          >
            Auto-arrange</button
          ><small
            >Blue input pins · amber output pins · purple dashed feedback</small
          >
        </div>
        <div
          class="component-studio"
          :class="{
            'inspector-hidden': inspectorHidden,
            'palette-hidden': paletteHidden,
          }"
        >
          <ComponentPalette
            v-show="!paletteHidden"
            :primitives="availablePrimitives"
            :components="nestedChoices"
            :examples="examples"
            :disabled="readonly || !!busy"
            :full="behavior.nodes.length >= 32"
            @add="addFromPalette"
            @import="fileInput?.click()"
            @export="exportBundle"
            @publish="installDraft"
          />
          <div class="component-canvas">
            <BehaviorGraph
              :behavior="behavior"
              :dependencies="dependencies"
              :primitives="primitives"
              :contracts="allDataContracts"
              :selected-id="selectedNodeId"
              :readonly="readonly || !!busy"
              :invalid-ids="invalidNodeIds"
              @drop-part="
                (kind, key, position) => addFromPalette(kind, key, position)
              "
              @select="selectCanvasNode"
              @inspect="inspectNode"
              @move="
                (id, position) => {
                  const n = draft.behavior.nodes.find((node) => node.id === id);
                  if (n && !readonly && !busy) n.position = position;
                }
              "
            />
          </div>
          <aside
            v-show="!inspectorHidden"
            class="component-inspector"
            aria-label="Component inspector"
          >
            <section class="workbench-panel node-inspector">
              <div class="workbench-panel-heading">
                <h3>{{ selectedNode ? selectedNode.id : "Node inspector" }}</h3>
                <span>{{
                  readonly ? "Pinned / read-only" : "Editable bindings"
                }}</span>
              </div>
              <template v-if="selectedNode"
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
                  <p v-if="nodeIssue()" class="workbench-error">
                    {{ nodeIssue() }}
                  </p>
                  <div class="node-color-controls">
                    <span>Node color</span
                    ><button
                      v-for="color in nodeColors"
                      :key="color"
                      type="button"
                      :style="{ background: color }"
                      :aria-label="`Set node color ${color}`"
                      :aria-pressed="
                        nodeColor(selectedNode, dependencies) === color
                      "
                      @click="selectedNode.color = color"
                    ></button
                    ><button
                      class="button quiet"
                      @click="delete selectedNode.color"
                    >
                      Reset
                    </button>
                  </div>
                  <div class="workbench-actions">
                    <button
                      class="button secondary"
                      type="button"
                      :disabled="behavior.nodes[0]?.id === selectedNode.id"
                      @click="reorderNode(-1)"
                    >
                      Move earlier</button
                    ><button
                      class="button secondary"
                      type="button"
                      :disabled="behavior.nodes.at(-1)?.id === selectedNode.id"
                      @click="reorderNode(1)"
                    >
                      Move later
                    </button>
                  </div>
                  <h4>Required inputs · {{ selectedInputs.length }}</h4>
                  <label v-if="selectedNode.op === 'record_make'" class="field"
                    >Constructed record type<select
                      :value="dataRefKey(selectedNode.recordRef)"
                      aria-label="Constructed record type"
                      @change="setNodeRecord"
                    >
                      <option value="">Select an exact record</option>
                      <option
                        v-if="
                          selectedNode.recordRef &&
                          !recordChoices.some(
                            (r) =>
                              r.key === dataRefKey(selectedNode!.recordRef),
                          )
                        "
                        :value="dataRefKey(selectedNode.recordRef)"
                      >
                        Unresolved: {{ selectedNode.recordRef.record }} @
                        {{ selectedNode.recordRef.version }}
                      </option>
                      <option
                        v-for="r in recordChoices"
                        :key="r.key"
                        :value="r.key"
                      >
                        {{ r.label }}
                      </option>
                    </select></label
                  >
                  <label v-if="selectedNode.op === 'record_get'" class="field"
                    >Field to extract<select
                      v-model="selectedNode.field"
                      aria-label="Record field to extract"
                    >
                      <option value="">Bind a record and select a field</option>
                      <option
                        v-if="
                          selectedNode.field &&
                          !getFieldChoices.some(
                            (f) => f.name === selectedNode!.field,
                          )
                        "
                        :value="selectedNode.field"
                      >
                        Unresolved: {{ selectedNode.field }}
                      </option>
                      <option
                        v-for="f in getFieldChoices"
                        :key="f.name"
                        :value="f.name"
                      >
                        {{ f.name }} · {{ f.type }}
                      </option>
                    </select></label
                  >
                  <BehaviorBindingEditor
                    v-for="port in selectedInputs"
                    :key="selectedNode.id + ':' + port.name"
                    :model-value="selectedNode.inputs?.[port.name]"
                    :expected-type="port.type"
                    :record-ref="port.recordRef"
                    :contracts="allDataContracts"
                    :structured="viewed.schemaVersion === 3"
                    :label="`${selectedNode.id}.${port.name}`"
                    :issue="nodeIssue(port.name)"
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
                  <Trash2 :size="13" /> Remove node
                </button></template
              >
              <p v-else class="workbench-empty">
                Select a node on the canvas to configure its required inputs
                here. A component may also bind inputs or literals directly to
                public outputs.
              </p>
            </section>
            <section class="workbench-panel composition-outputs">
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
                :record-ref="port.recordRef"
                :contracts="allDataContracts"
                :structured="viewed.schemaVersion === 3"
                :label="`Output ${port.name}`"
                :issue="
                  issues
                    .filter((i) =>
                      i.path.includes(`behavior.bindings.${port.name}`),
                    )
                    .map((i) => i.message)
                    .join(' ')
                "
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
          </aside>
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
                >Port type<select
                  :value="typeKey(port)"
                  @change="setPortType(port, $event)"
                  :aria-label="`${direction} port ${index + 1} type`"
                >
                  <option
                    v-if="
                      !portChoices.some(
                        (choice) => choice.key === typeKey(port),
                      )
                    "
                    :value="typeKey(port)"
                  >
                    Unresolved: {{ typeLabel(port) }}
                  </option>
                  <option
                    v-for="choice in portChoices"
                    :key="choice.key"
                    :value="choice.key"
                  >
                    {{ choice.label }}
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
                :record-ref="port.recordRef"
                :contracts="allDataContracts"
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
                    :record-ref="port.recordRef"
                    :contracts="allDataContracts"
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
                    :record-ref="port.recordRef"
                    :contracts="allDataContracts"
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
.workbench-data-contracts {
  background: #fff;
  border: 1px solid #dce7eb;
  padding: 16px;
  border-radius: 8px;
  margin: 16px 0;
}
.workbench-data-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
  color: #486c7f;
  margin-bottom: 12px;
}
.workbench-data-picker {
  display: flex;
  gap: 12px;
  align-items: end;
}
.workbench-data-picker .field {
  flex: 1;
  margin: 0;
}
.workbench-data-lock {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: #f4faf7;
  border: 1px solid #d7e8df;
  padding: 12px;
  border-radius: 6px;
  margin-top: 10px;
  font-size: 12px;
  color: #537c6e;
}
.workbench-data-lock small {
  display: block;
  font-size: 10px;
  overflow-wrap: anywhere;
  margin-top: 5px;
}
.workbench-data-lock > span {
  min-width: 0;
}
.behavior-workbench {
  padding: 22px 30px 28px;
  background: #f5f8fa;
  min-width: 0;
  container: component-workbench / inline-size;
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
  overflow-wrap: anywhere;
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

/* Size against the actual workspace, including the navigation rail. */
.component-studio {
  display: grid;
  grid-template-areas: "library canvas inspector";
  grid-template-columns: 250px minmax(0, 1fr) 280px;
  gap: 12px;
  align-items: start;
  margin-top: 12px;
}
.component-studio > * {
  min-width: 0;
  max-width: 100%;
}
.component-studio > .component-palette {
  grid-area: library;
}
.component-inspector {
  grid-area: inspector;
  max-height: 78vh;
  overflow: auto;
  display: grid;
  gap: 12px;
}
.component-canvas {
  grid-area: canvas;
  min-width: 0;
  position: sticky;
  top: 12px;
}
.component-studio.inspector-hidden {
  grid-template-areas: "library canvas";
  grid-template-columns: 250px minmax(0, 1fr);
}
.component-studio.palette-hidden {
  grid-template-areas: "canvas inspector";
  grid-template-columns: minmax(0, 1fr) 280px;
}
.component-studio.inspector-hidden.palette-hidden {
  grid-template-areas: "canvas";
  grid-template-columns: minmax(0, 1fr);
}
.component-inspector .workbench-panel {
  margin: 0;
  padding: 14px;
}
/* Keep the expanded graph out of its sticky parent's stacking context. */
.component-canvas:has(.canvas-focus--expanded) {
  position: static;
}
.component-inspector .node-identity-fields {
  grid-template-columns: 1fr 64px 64px;
}
.component-inspector .node-identity-fields .field:first-child {
  grid-column: 1 / -1;
}
.node-color-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  margin: 12px 0;
}
.node-color-controls > button:not(.button) {
  width: 21px;
  height: 21px;
  border: 2px solid white;
  border-radius: 50%;
  outline: 1px solid #d0dce5;
  cursor: pointer;
}
.node-color-controls > button[aria-pressed="true"] {
  outline: 2px solid #263f56;
}
.issue-link {
  text-align: left;
  border: 0;
  background: transparent;
  color: inherit;
  text-decoration: underline;
  cursor: pointer;
  line-height: 1.6;
}
.canvas-layout-actions small {
  font-size: 11px;
  color: #61788a;
}
.component-inspector h4 {
  font-size: 12px;
  margin: 16px 0 8px;
}
@container component-workbench (max-width: 960px) {
  .component-studio {
    grid-template-areas: "library canvas" "library inspector";
    grid-template-columns: 220px minmax(0, 1fr);
  }
  .component-studio.inspector-hidden {
    grid-template-areas: "library canvas";
    grid-template-columns: 220px minmax(0, 1fr);
  }
  .component-studio.palette-hidden {
    grid-template-areas: "canvas" "inspector";
    grid-template-columns: minmax(0, 1fr);
  }
  .component-studio.inspector-hidden.palette-hidden {
    grid-template-areas: "canvas";
  }
  .component-canvas {
    position: static;
  }
}
@container component-workbench (max-width: 620px) {
  .component-studio,
  .component-studio.inspector-hidden,
  .component-studio.palette-hidden,
  .component-studio.inspector-hidden.palette-hidden {
    grid-template-columns: minmax(0, 1fr);
  }
  .component-studio {
    grid-template-areas: "canvas" "library" "inspector";
  }
  .component-studio.inspector-hidden {
    grid-template-areas: "canvas" "library";
  }
  .component-studio.palette-hidden {
    grid-template-areas: "canvas" "inspector";
  }
  .component-studio.inspector-hidden.palette-hidden {
    grid-template-areas: "canvas";
  }
  .component-studio > .component-palette {
    max-height: 360px;
  }
  .component-inspector {
    max-height: none;
    overflow: visible;
  }
}
.library-disclosure > summary,
.workbench-data-contracts > summary {
  font-size: 12px;
  cursor: pointer;
  color: #526f83;
  padding: 8px 0;
}
.workbench-data-contracts {
  padding: 8px 14px;
}
.library-disclosure {
  margin-bottom: 10px;
}
</style>
