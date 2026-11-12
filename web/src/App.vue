<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import {
  VueFlow,
  MarkerType,
  type Connection,
  type NodeDragEvent,
  type NodeMouseEvent,
  type EdgeMouseEvent,
} from "@vue-flow/core";
import { Background } from "@vue-flow/background";
import { Controls } from "@vue-flow/controls";
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  ArrowRight,
  Box,
  Check,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  CircleDot,
  Database,
  FileCheck2,
  FlaskConical,
  FolderOpen,
  GitBranch,
  Layers3,
  LoaderCircle,
  Monitor,
  Pause,
  Play,
  Plus,
  RotateCcw,
  Save,
  Server,
  Settings2,
  ShieldCheck,
  SkipForward,
  Trash2,
  X,
  XCircle,
  PanelLeftClose,
} from "lucide-vue-next";
import StudioNode from "./components/StudioNode.vue";
import DeploymentEditor from "./components/DeploymentEditor.vue";
import InfrastructureResults from "./components/InfrastructureResults.vue";
import { api, APIError } from "./api";
import type {
  ComponentNode,
  Link,
  NodeKind,
  Project,
  Requirement,
  Run,
  Validation,
} from "./types";

const project = ref<Project | null>(null);
const projects = ref<Project[]>([]);
const runs = ref<Run[]>([]);
const currentRun = ref<Run | null>(null);
const view = ref<"design" | "plan" | "deployment" | "results">("design");
const inspector = ref<"properties" | "scenario">("scenario");
const selectedNodeId = ref("");
const selectedEdgeId = ref("");
const busy = ref("");
const error = ref("");
const notice = ref("");
const connected = ref(false);
const validation = ref<Validation | null>(null);
const saved = ref("");
const showProjects = ref(false);
const showRunHistory = ref(false);
const showMobilePalette = ref(false);
const importInput = ref<HTMLInputElement | null>(null);
const replayIndex = ref(-1);
const playing = ref(false);
const replaySpeed = ref(120);
let replayTimer: ReturnType<typeof setInterval> | undefined;
let noticeTimer: ReturnType<typeof setTimeout> | undefined;
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value));
const stableJSON = (value: unknown): string =>
  JSON.stringify(value, (_key, v) =>
    v && typeof v === "object" && !Array.isArray(v)
      ? Object.fromEntries(
          Object.entries(v).sort(([a], [b]) => a.localeCompare(b)),
        )
      : v,
  );
const uid = (prefix: string) => `${prefix}-${crypto.randomUUID().slice(0, 8)}`;
const dirty = computed(
  () => !!project.value && stableJSON(project.value) !== saved.value,
);
const selectedNode = computed(() =>
  project.value?.nodes.find((n) => n.id === selectedNodeId.value),
);
const selectedEdge = computed(() =>
  project.value?.edges.find((e) => e.id === selectedEdgeId.value),
);
const runStale = computed(
  () =>
    !!currentRun.value &&
    !!project.value &&
    stableJSON(currentRun.value.project) !== stableJSON(project.value),
);
const currentEvent = computed(
  () => currentRun.value?.events?.[replayIndex.value],
);
const passCount = computed(
  () => currentRun.value?.verdicts.filter((v) => v.passed).length || 0,
);
const flowNodes = computed(
  () =>
    project.value?.nodes.map((n) => ({
      id: n.id,
      type: n.type,
      position: n.position,
      data: {
        node: n,
        active: !runStale.value && currentEvent.value?.nodeId === n.id,
      },
      selected: n.id === selectedNodeId.value,
    })) || [],
);
const flowEdges = computed(
  () =>
    project.value?.edges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      type: "smoothstep",
      label: `${e.latencyMs} ms`,
      markerEnd: MarkerType.ArrowClosed,
      selected: e.id === selectedEdgeId.value,
      style: {
        stroke: e.id === selectedEdgeId.value ? "#0d9488" : "#718399",
        strokeWidth: e.id === selectedEdgeId.value ? 2.5 : 1.6,
      },
      labelStyle: { fill: "#4b6075", fontSize: 11 },
      labelBgStyle: { fill: "#f2f6fa", fillOpacity: 0.95 },
      labelBgPadding: [6, 3] as [number, number],
    })) || [],
);
const visibleEvents = computed(() => {
  const events = currentRun.value?.events || [];
  const at = replayIndex.value < 0 ? 0 : Math.max(0, replayIndex.value - 6);
  return events.slice(at, at + 18);
});
const ruleLabels: Record<string, string> = {
  no_duplicates: "No duplicate writes",
  all_complete: "All requests complete",
  latency_p95: "P95 latency limit",
};
const selectedLabel = computed(
  () =>
    selectedNode.value?.name || selectedEdge.value?.id || "Model properties",
);
const numberFormat = (n: number | undefined) =>
  new Intl.NumberFormat("en", { maximumFractionDigits: 1 }).format(n || 0);
const runTime = (s: string) =>
  new Date(s).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
const clearSelection = () => {
  selectedNodeId.value = "";
  selectedEdgeId.value = "";
};
function notify(text: string) {
  notice.value = text;
  clearTimeout(noticeTimer);
  noticeTimer = setTimeout(() => (notice.value = ""), 4000);
}
function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e);
  if (e instanceof APIError && e.issues.length)
    validation.value = { valid: false, issues: e.issues };
}
async function action(name: string, work: () => Promise<void>) {
  if (busy.value) return;
  busy.value = name;
  error.value = "";
  try {
    await work();
  } catch (e) {
    fail(e);
  } finally {
    busy.value = "";
  }
}
function adopt(p: Project, isSaved = true) {
  project.value = clone(p);
  saved.value = isSaved ? stableJSON(project.value) : "";
  clearSelection();
  validation.value = null;
  currentRun.value = null;
  pause();
  replayIndex.value = -1;
}
async function refreshRuns() {
  if (project.value?.id)
    runs.value = await api<Run[]>(
      `/runs?projectId=${encodeURIComponent(project.value.id)}`,
    );
  else runs.value = [];
}
async function connect() {
  await action("connect", async () => {
    await api("/health");
    connected.value = true;
    projects.value = await api<Project[]>("/projects");
    if (!project.value) {
      const p = await api<Project>("/example");
      adopt(p, false);
    }
    await refreshRuns();
  });
}
async function loadProject(p: Project) {
  if (dirty.value && !confirm("Discard unsaved changes and open this project?"))
    return;
  await action("open", async () => {
    adopt(await api<Project>(`/projects/${encodeURIComponent(p.id)}`));
    showProjects.value = false;
    await refreshRuns();
    notify("Project opened");
  });
}
async function saveProject() {
  if (!project.value) return;
  await action("save", async () => {
    const snapshot = clone(project.value!);
    const p = await api<Project>("/projects", "POST", snapshot);
    project.value!.id = p.id;
    saved.value = stableJSON(p);
    projects.value = await api<Project[]>("/projects");
    notify("Project saved");
  });
}
async function newProject() {
  if (
    dirty.value &&
    !confirm("Discard unsaved changes and create a new project?")
  )
    return;
  await action("new", async () => {
    const p = await api<Project>("/example");
    p.id = uid("project");
    p.name = "Untitled system";
    p.description = "";
    adopt(p, false);
    runs.value = [];
    view.value = "design";
    showProjects.value = false;
    notify("New project created; save to keep it");
  });
}
async function saveCopy() {
  if (!project.value) return;
  await action("copy", async () => {
    const copy = clone(project.value!);
    copy.id = uid("project");
    copy.name = `${copy.name} — copy`.slice(0, 120);
    const p = await api<Project>("/projects", "POST", copy);
    adopt(p);
    projects.value = await api<Project[]>("/projects");
    runs.value = [];
    showProjects.value = false;
    notify("Project copy saved");
  });
}
async function resetExample() {
  if (dirty.value && !confirm("Discard unsaved changes and open the example?"))
    return;
  await action("example", async () => {
    adopt(await api<Project>("/example"), false);
    await refreshRuns();
    showProjects.value = false;
    notify("Example loaded");
  });
}
async function loadContinuumExample() {
  if (
    dirty.value &&
    !confirm("Discard unsaved changes and open the continuum example?")
  )
    return;
  await action("example", async () => {
    adopt(await api<Project>("/example?template=continuum"), false);
    await refreshRuns();
    showProjects.value = false;
    view.value = "deployment";
    notify(
      "Continuum example loaded. Compare placements, network links, and power outages.",
    );
  });
}
async function validateModel() {
  if (!project.value) return;
  await action("validate", async () => {
    validation.value = await api<Validation>(
      "/validate",
      "POST",
      project.value,
    );
    if (validation.value.valid) notify("Model is ready to simulate");
  });
}
async function simulate() {
  if (!project.value) return;
  await action("run", async () => {
    pause();
    const result = await api<Run>("/runs", "POST", {
      project: clone(project.value!),
    });
    currentRun.value = result;
    replayIndex.value = -1;
    validation.value = { valid: true, issues: [] };
    await refreshRuns();
    view.value = "results";
    notify(
      `Run complete · ${result.verdicts.filter((v) => v.passed).length}/${result.verdicts.length} checks passed`,
    );
  });
}
async function openRun(run: Run) {
  await action("history", async () => {
    currentRun.value = await api<Run>(`/runs/${encodeURIComponent(run.id)}`);
    pause();
    replayIndex.value = -1;
    view.value = "results";
    showRunHistory.value = false;
  });
}
function openRunModel() {
  if (!currentRun.value) return;
  if (
    dirty.value &&
    !confirm("Replace your unsaved design with this run’s model snapshot?")
  )
    return;
  project.value = clone(currentRun.value.project);
  saved.value = "";
  clearSelection();
  validation.value = null;
  view.value = "design";
  notify("Run snapshot opened as an unsaved design");
}
function onConnect(c: Connection) {
  if (!project.value || !c.source || !c.target) return;
  if (project.value.edges.length >= 30) {
    notify("A project supports up to 30 connections");
    return;
  }
  if (c.source === c.target) {
    notify("A component cannot connect to itself");
    return;
  }
  if (
    project.value.edges.some(
      (e) => e.source === c.source && e.target === c.target,
    )
  ) {
    notify("This connection already exists");
    return;
  }
  project.value.edges.push({
    id: uid("link"),
    source: c.source,
    target: c.target,
    latencyMs: 10,
  });
  validation.value = null;
}
function onNodeClick(e: NodeMouseEvent) {
  selectedNodeId.value = e.node.id;
  selectedEdgeId.value = "";
  inspector.value = "properties";
}
function onEdgeClick(e: EdgeMouseEvent) {
  selectedEdgeId.value = e.edge.id;
  selectedNodeId.value = "";
  inspector.value = "properties";
}
function onDrag(e: NodeDragEvent) {
  for (const item of e.nodes) {
    const n = project.value?.nodes.find((n) => n.id === item.id);
    if (n)
      n.position = {
        x: Math.round(item.position.x),
        y: Math.round(item.position.y),
      };
  }
}
function addNode(type: NodeKind) {
  if (!project.value) return;
  if (project.value.nodes.length >= 30) {
    notify("A project supports up to 30 components");
    return;
  }
  const n: ComponentNode = {
    id: uid(type),
    type,
    name:
      type === "client"
        ? "Client"
        : type === "service"
          ? "New service"
          : "Database",
    position: {
      x: 150 + (project.value.nodes.length % 3) * 260,
      y: 80 + Math.floor(project.value.nodes.length / 3) * 180,
    },
    config: {
      processingMs: type === "client" ? 0 : 10,
      capacity: 1,
      deduplicate: type === "database",
    },
  };
  project.value.nodes.push(n);
  selectedNodeId.value = n.id;
  selectedEdgeId.value = "";
  inspector.value = "properties";
  view.value = "design";
  showMobilePalette.value = false;
  validation.value = null;
}
function removeSelected() {
  if (!project.value) return;
  if (selectedNode.value) {
    const id = selectedNodeId.value;
    project.value.nodes = project.value.nodes.filter((n) => n.id !== id);
    project.value.edges = project.value.edges.filter(
      (e) => e.source !== id && e.target !== id,
    );
    if (project.value.infrastructure)
      project.value.infrastructure.placements =
        project.value.infrastructure.placements.filter((p) => p.nodeId !== id);
  } else if (selectedEdge.value)
    project.value.edges = project.value.edges.filter(
      (e) => e.id !== selectedEdgeId.value,
    );
  clearSelection();
  validation.value = null;
}
function addRequirement() {
  if (!project.value || project.value.requirements.length >= 50) return;
  project.value.requirements.push({
    id: uid("req"),
    text: "New acceptance criterion",
    rule: "all_complete",
    targetMs: 300,
  });
}
function exportProject() {
  if (!project.value) return;
  const blob = new Blob([JSON.stringify(project.value, null, 2)], {
    type: "application/json",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${project.value.name.replace(/[^a-z0-9_-]+/gi, "-").toLowerCase() || "project"}.json`;
  a.click();
  URL.revokeObjectURL(url);
  notify("Project exported");
}
async function importProject(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  try {
    if (!file.name.toLowerCase().endsWith(".json"))
      throw new Error("Choose a project file with the .json extension.");
    if (file.size > 2 * 1024 * 1024)
      throw new Error("Project files must be smaller than 2 MiB.");
    const p: unknown = JSON.parse(await file.text());
    if (!p || typeof p !== "object")
      throw new Error("Choose a valid project JSON file.");
    const candidate = p as Project;
    if (
      ![1, 2].includes(candidate.schemaVersion) ||
      !Array.isArray(candidate.nodes) ||
      !Array.isArray(candidate.edges) ||
      !Array.isArray(candidate.requirements) ||
      !candidate.scenario ||
      typeof candidate.name !== "string"
    )
      throw new Error(
        "This file does not match a supported Software Studio project format (v1 or v2).",
      );
    const checked = await api<Validation>(
      "/validate?draft=true",
      "POST",
      candidate,
    );
    if (!checked.valid)
      throw new APIError(
        "Import rejected. Fix the reported invalid fields before importing.",
        checked.issues,
      );
    if (
      dirty.value &&
      !confirm("Discard unsaved changes and import this project?")
    )
      return;
    adopt(candidate, false);
    error.value = "";
    view.value = "design";
    await refreshRuns();
    notify("Project imported; save to keep it");
  } catch (e) {
    fail(e);
  } finally {
    input.value = "";
  }
}
function pause() {
  playing.value = false;
  if (replayTimer) clearInterval(replayTimer);
  replayTimer = undefined;
}
function step() {
  pause();
  const total = currentRun.value?.events?.length || 0;
  if (total) replayIndex.value = Math.min(total - 1, replayIndex.value + 1);
}
function toggleReplay() {
  if (playing.value) {
    pause();
    return;
  }
  const total = currentRun.value?.events?.length || 0;
  if (!total) return;
  if (replayIndex.value >= total - 1) replayIndex.value = -1;
  playing.value = true;
  replayTimer = setInterval(() => {
    replayIndex.value++;
    if (replayIndex.value >= total - 1) pause();
  }, replaySpeed.value);
}
function resetReplay() {
  pause();
  replayIndex.value = -1;
}
function seek(e: Event) {
  pause();
  replayIndex.value = Number((e.target as HTMLInputElement).value);
}
function beforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) {
    e.preventDefault();
    e.returnValue = "";
  }
}
watch(
  () => project.value,
  () => {
    validation.value = null;
  },
  { deep: true },
);
watch(replaySpeed, () => {
  if (playing.value) {
    pause();
    toggleReplay();
  }
});
onMounted(() => {
  connect();
  window.addEventListener("beforeunload", beforeUnload);
});
onUnmounted(() => {
  pause();
  clearTimeout(noticeTimer);
  window.removeEventListener("beforeunload", beforeUnload);
});
</script>

<template>
  <div class="studio-shell">
    <aside class="app-rail">
      <a
        class="brand"
        href="#"
        @click.prevent="view = 'design'"
        aria-label="Software Studio home"
        ><span class="brand-mark"><Box :size="24" /></span
        ><span
          >software<span class="brand-sub"
            >studio<span class="version"> / 0.2</span></span
          ></span
        ></a
      >
      <div class="rail-section">WORKSPACE</div>
      <nav class="main-nav" aria-label="Workspace">
        <button
          :class="{ active: view === 'plan' }"
          @click="view = 'plan'"
          aria-label="Plan workspace"
        >
          <FileCheck2 :size="18" /><span>Plan</span
          ><span class="nav-number">01</span>
        </button>
        <button
          :class="{ active: view === 'design' }"
          @click="view = 'design'"
          aria-label="Design workspace"
        >
          <GitBranch :size="18" /><span>Design</span
          ><span class="nav-number">02</span>
        </button>
        <button
          :class="{ active: view === 'deployment' }"
          @click="view = 'deployment'"
          aria-label="Deployment workspace"
        >
          <Server :size="18" /><span>Deployment</span
          ><span class="nav-number">03</span>
        </button>
        <button
          :class="{ active: view === 'results' }"
          @click="view = 'results'"
          aria-label="Experiments workspace"
        >
          <FlaskConical :size="18" /><span>Experiments</span
          ><span class="nav-number">04</span>
        </button>
      </nav>
      <div class="rail-divider" />
      <button class="rail-action" @click="showProjects = true">
        <FolderOpen :size="17" /> Projects <span>{{ projects.length }}</span>
      </button>
      <button class="rail-action" @click="showRunHistory = true">
        <Activity :size="17" /> Run history <span>{{ runs.length }}</span>
      </button>
      <div class="rail-bottom">
        <span class="small-logo"><Layers3 :size="17" /></span
        ><strong>Make the model work.</strong>
        <p>Design with intent.<br />Verify with evidence.</p>
        <div class="engine-status" :class="{ offline: !connected }">
          <i />{{ connected ? "Go engine connected" : "Engine offline" }}
        </div>
        <button v-if="!connected" class="text-link" @click="connect">
          Reconnect</button
        ><span class="local-label">LOCAL WORKSPACE · V0.2</span>
      </div>
    </aside>

    <main class="main-shell">
      <header class="topbar">
        <div class="project-heading">
          <span class="breadcrumb"
            >Projects <span>/</span> Engineering workspace</span
          >
          <div class="project-title">
            <input
              v-if="project"
              v-model="project.name"
              aria-label="Project name"
              maxlength="120"
            />
            <h1 v-else>Software Studio</h1>
            <span class="dirty-dot" v-if="dirty" title="Unsaved changes" /><span
              class="draft-tag"
              >{{ dirty ? "Draft" : "Saved" }}</span
            >
          </div>
        </div>
        <div class="topbar-actions">
          <button
            class="icon-button"
            @click="importInput?.click()"
            title="Import project JSON"
            aria-label="Import project JSON"
          >
            <ArrowUpFromLine :size="17" /></button
          ><button
            class="icon-button"
            @click="exportProject"
            :disabled="!project"
            title="Export project JSON"
            aria-label="Export project JSON"
          >
            <ArrowDownToLine :size="17" /></button
          ><span class="action-divider" /><button
            class="button secondary save-button"
            @click="saveProject"
            :disabled="!!busy || !project"
          >
            <LoaderCircle v-if="busy === 'save'" class="spin" :size="15" /><Save
              v-else
              :size="15"
            />
            Save project</button
          ><button
            class="button primary"
            @click="simulate"
            :disabled="!!busy || !project"
          >
            <LoaderCircle v-if="busy === 'run'" class="spin" :size="15" /><Play
              v-else
              :size="15"
            />{{ busy === "run" ? "Simulating…" : "Run simulation" }}
          </button>
        </div>
        <input
          ref="importInput"
          class="hidden"
          type="file"
          accept="application/json,.json"
          @change="importProject"
        />
      </header>

      <div v-if="error" class="alert error" role="alert">
        <CircleAlert :size="18" /><span>{{ error }}</span
        ><button @click="error = ''" aria-label="Dismiss error">
          <X :size="16" />
        </button>
      </div>
      <div v-if="validation && !validation.valid" class="validation-alert">
        <div>
          <CircleAlert :size="17" /><strong>Model needs attention</strong
          ><button @click="validation = null" aria-label="Dismiss validation">
            <X :size="15" />
          </button>
        </div>
        <ul>
          <li v-for="(issue, i) in validation.issues" :key="i">
            <code>{{ issue.path }}</code> {{ issue.message }}
          </li>
        </ul>
      </div>

      <div v-if="!project" class="welcome-state">
        <span class="welcome-icon"><GitBranch :size="36" /></span>
        <h2>
          {{
            busy
              ? "Opening your workspace…"
              : "Connect your engineering workspace"
          }}
        </h2>
        <p>
          The Vue editor connects to the Go engine for models, simulations, and
          saved projects.
        </p>
        <button class="button primary" :disabled="!!busy" @click="connect">
          <LoaderCircle v-if="busy" class="spin" :size="16" /> Reconnect to
          engine
        </button>
      </div>

      <template v-else>
        <div class="workspace-bar">
          <div class="workspace-label">
            <span class="section-eyebrow">{{
              view === "design"
                ? "ARCHITECTURE"
                : view === "plan"
                  ? "REQUIREMENTS"
                  : view === "deployment"
                    ? "COMPUTING CONTINUUM"
                    : "SIMULATION LAB"
            }}</span>
            <h2>
              {{
                view === "design"
                  ? "Design your system"
                  : view === "plan"
                    ? "Define what success means"
                    : view === "deployment"
                      ? "Place your system in the real world"
                      : "Test your assumptions"
              }}
            </h2>
          </div>
          <div class="workspace-tools">
            <span class="topology-label" v-if="view === 'design'"
              >{{ project.nodes.length }} components <span>·</span>
              {{ project.edges.length }} connections</span
            ><button
              class="button quiet"
              v-if="view === 'design' || view === 'deployment'"
              @click="validateModel"
              :disabled="!!busy"
            >
              <ShieldCheck :size="16" /> Validate model</button
            ><button
              class="button quiet"
              v-else-if="view === 'plan'"
              @click="addRequirement"
            >
              <Plus :size="16" /> Add requirement</button
            ><button class="button quiet" v-else @click="showRunHistory = true">
              <Activity :size="16" /> Run history
            </button>
          </div>
        </div>

        <DeploymentEditor
          v-if="view === 'deployment'"
          :project="project"
          :busy="!!busy"
          :active-host-id="!runStale ? currentEvent?.hostId : undefined"
          :active-link-id="!runStale ? currentEvent?.linkId : undefined"
          :replay-time-ms="!runStale ? currentEvent?.timeMs : undefined"
          @load-example="loadContinuumExample"
        />

        <section v-show="view === 'design'" class="design-workspace">
          <aside
            class="component-palette"
            :class="{ 'mobile-open': showMobilePalette }"
          >
            <div class="panel-heading">
              <span>COMPONENTS</span><Layers3 :size="14" />
            </div>
            <p class="panel-caption">
              Add a building block.<br />Connect its ports to define flow.
            </p>
            <button class="palette-card" @click="addNode('client')">
              <span class="palette-icon client"><Monitor :size="19" /></span
              ><span
                ><strong>Client</strong><small>Generate requests</small></span
              ><Plus :size="15" /></button
            ><button class="palette-card" @click="addNode('service')">
              <span class="palette-icon service"><Server :size="19" /></span
              ><span
                ><strong>Service</strong
                ><small>Process & queue work</small></span
              ><Plus :size="15" /></button
            ><button class="palette-card" @click="addNode('database')">
              <span class="palette-icon database"><Database :size="19" /></span
              ><span
                ><strong>Database</strong
                ><small>Persist & deduplicate</small></span
              ><Plus :size="15" />
            </button>
            <div class="palette-note">
              <GitBranch :size="17" /><strong>One connected pipeline</strong>
              <p>
                Client → services → database.<br />Connect from right to left
                ports.
              </p>
            </div>
            <div class="panel-heading model-heading">
              <span>MODEL TREE</span><span>{{ project.nodes.length }}</span>
            </div>
            <button
              v-for="n in project.nodes"
              :key="n.id"
              class="tree-row"
              :class="{ selected: selectedNodeId === n.id }"
              @click="
                selectedNodeId = n.id;
                selectedEdgeId = '';
                inspector = 'properties';
              "
            >
              <Monitor v-if="n.type === 'client'" :size="14" /><Database
                v-else-if="n.type === 'database'"
                :size="14"
              /><Server v-else :size="14" /><span>{{ n.name }}</span>
            </button>
          </aside>
          <div class="canvas-area">
            <div class="canvas-ribbon">
              <span><CircleDot :size="13" /> SYSTEM MODEL</span
              ><button
                @click="showMobilePalette = !showMobilePalette"
                class="mobile-palette-toggle"
              >
                <Plus :size="14" /> Components</button
              ><span class="canvas-hint"
                >Drag to arrange · Connect ports · Select to inspect</span
              >
            </div>
            <VueFlow
              id="studio-model"
              :nodes="flowNodes"
              :edges="flowEdges"
              :min-zoom="0.2"
              :max-zoom="2"
              :default-viewport="{ x: 10, y: 30, zoom: 0.8 }"
              :delete-key-code="null"
              fit-view-on-init
              @connect="onConnect"
              @node-click="onNodeClick"
              @edge-click="onEdgeClick"
              @node-drag-stop="onDrag"
              @pane-click="clearSelection"
              ><template #node-client="props"
                ><StudioNode v-bind="props" /></template
              ><template #node-service="props"
                ><StudioNode v-bind="props" /></template
              ><template #node-database="props"
                ><StudioNode v-bind="props" /></template
              ><Background
                pattern-color="#c7d3df"
                :gap="20"
                :size="1" /><Controls
                position="bottom-left"
                :show-interactive="false"
            /></VueFlow>
            <div class="canvas-legend">
              <span><i class="legend-client" />Client</span
              ><span><i class="legend-service" />Service</span
              ><span><i class="legend-db" />Database</span>
            </div>
            <div class="canvas-empty" v-if="!project.nodes.length">
              <Box :size="30" />
              <h3>Start with a component</h3>
              <p>
                Add a client, service, and database to build your first model.
              </p>
            </div>
          </div>
          <aside class="inspector">
            <div class="inspector-tabs">
              <button
                :class="{ active: inspector === 'properties' }"
                @click="inspector = 'properties'"
              >
                Properties</button
              ><button
                :class="{ active: inspector === 'scenario' }"
                @click="inspector = 'scenario'"
              >
                Scenario
              </button>
            </div>
            <div class="inspector-content" v-if="inspector === 'properties'">
              <div class="inspector-title">
                <Settings2 :size="16" /><strong>{{ selectedLabel }}</strong>
              </div>
              <template v-if="selectedNode"
                ><span class="type-chip">{{ selectedNode.type }}</span
                ><label class="field"
                  >Name<input
                    v-model="selectedNode.name"
                    maxlength="120" /></label
                ><template v-if="selectedNode.type !== 'client'"
                  ><label class="field"
                    >Processing time <span>ms</span
                    ><input
                      type="number"
                      v-model.number="selectedNode.config.processingMs"
                      min="0"
                      max="60000"
                      step="1" /></label
                  ><label class="field"
                    >Concurrent capacity<input
                      type="number"
                      v-model.number="selectedNode.config.capacity"
                      min="1"
                      max="100"
                      step="1"
                  /></label>
                  <p class="help-text">
                    Requests wait in a FIFO queue when all processing slots are
                    busy.
                  </p></template
                ><label
                  v-if="selectedNode.type === 'database'"
                  class="toggle-card"
                  ><input
                    type="checkbox"
                    v-model="selectedNode.config.deduplicate"
                  /><span
                    ><strong>Deduplicate writes</strong
                    ><small>One write per logical request ID.</small></span
                  ></label
                >
                <div class="inspector-footer">
                  <code>{{ selectedNode.id }}</code
                  ><button class="button danger" @click="removeSelected">
                    <Trash2 :size="14" /> Remove component
                  </button>
                </div></template
              ><template v-else-if="selectedEdge"
                ><label class="field"
                  >Link latency <span>ms</span
                  ><input
                    type="number"
                    v-model.number="selectedEdge.latencyMs"
                    min="0"
                    max="60000"
                    step="1"
                /></label>
                <div class="connection-detail">
                  <span>{{
                    project.nodes.find((n) => n.id === selectedEdge?.source)
                      ?.name
                  }}</span
                  ><ArrowRight :size="16" /><span>{{
                    project.nodes.find((n) => n.id === selectedEdge?.target)
                      ?.name
                  }}</span>
                </div>
                <p class="help-text">
                  Latency applies in each direction. Scenario jitter is added
                  independently to each traversal.
                </p>
                <button class="button danger" @click="removeSelected">
                  <Trash2 :size="14" /> Remove connection
                </button></template
              >
              <div class="inspector-empty" v-else>
                <Box :size="28" />
                <h3>Inspect your design</h3>
                <p>Select a component or connection to edit its properties.</p>
                <button class="button quiet" @click="inspector = 'scenario'">
                  Configure scenario <ArrowRight :size="14" />
                </button>
              </div>
            </div>
            <div class="inspector-content" v-else>
              <div class="inspector-title">
                <FlaskConical :size="16" /><strong>Experiment setup</strong>
              </div>
              <p class="help-text intro-help">
                A repeatable workload for this model. All times are virtual
                milliseconds.
              </p>
              <div class="field-pair">
                <label class="field"
                  >Requests<input
                    type="number"
                    v-model.number="project.scenario.requestCount"
                    min="1"
                    max="200"
                    step="1" /></label
                ><label class="field"
                  >Interval · ms<input
                    type="number"
                    v-model.number="project.scenario.intervalMs"
                    min="1"
                    max="60000"
                    step="1"
                /></label>
              </div>
              <div class="field-pair">
                <label class="field"
                  >Timeout · ms<input
                    type="number"
                    v-model.number="project.scenario.timeoutMs"
                    min="1"
                    max="60000"
                    step="1" /></label
                ><label class="field"
                  >Max retries<input
                    type="number"
                    v-model.number="project.scenario.maxRetries"
                    min="0"
                    max="5"
                    step="1"
                /></label>
              </div>
              <div class="field-pair">
                <label class="field"
                  >Jitter · ms<input
                    type="number"
                    v-model.number="project.scenario.jitterMs"
                    min="0"
                    max="1000"
                    step="1" /></label
                ><label class="field"
                  >Random seed<input
                    type="number"
                    v-model.number="project.scenario.seed"
                    min="0"
                    max="2147483647"
                    step="1"
                /></label>
              </div>
              <div class="inspector-separator" />
              <span class="section-eyebrow">FAULT INJECTION</span
              ><label class="toggle-card fault"
                ><input
                  type="checkbox"
                  v-model="project.scenario.dropFirstResponse"
                /><span
                  ><strong>Lose first response</strong
                  ><small
                    >Drop the first successful response for each request to
                    exercise retries.</small
                  ></span
                ></label
              >
              <div class="info-note">
                <ShieldCheck :size="16" />
                <p>
                  Same model, seed, and engine version reproduce the same event
                  trace.
                </p>
              </div>
              <button
                class="button primary full"
                @click="simulate"
                :disabled="!!busy"
              >
                <Play :size="15" /> Run this scenario
              </button>
            </div>
          </aside>
        </section>

        <section v-if="view === 'plan'" class="plan-workspace">
          <div class="plan-overview">
            <span class="section-eyebrow">PROJECT BRIEF</span>
            <h3>A clear plan. A testable design.</h3>
            <label class="field"
              >Purpose & assumptions<textarea
                v-model="project.description"
                rows="4"
                maxlength="4000"
                placeholder="Describe the system, its purpose, and assumptions…"
              />
            </label>
            <div class="info-note">
              <FileCheck2 :size="18" />
              <p>
                Every requirement below becomes an executable check against your
                simulation. Results remain linked by requirement ID.
              </p>
            </div>
          </div>
          <div class="requirements-panel">
            <div class="section-title">
              <h3>
                Acceptance criteria
                <span>{{ project.requirements.length }}</span>
              </h3>
              <button
                class="button quiet"
                @click="addRequirement"
                :disabled="project.requirements.length >= 50"
              >
                <Plus :size="15" /> Add
              </button>
            </div>
            <div
              class="requirement-card"
              v-for="(r, i) in project.requirements"
              :key="r.id"
            >
              <div class="requirement-header">
                <span class="req-number"
                  >REQ {{ String(i + 1).padStart(2, "0") }}</span
                ><code>{{ r.id }}</code
                ><button
                  class="icon-button"
                  @click="
                    project.requirements = project.requirements.filter(
                      (x) => x.id !== r.id,
                    )
                  "
                  aria-label="Remove requirement"
                >
                  <Trash2 :size="15" />
                </button>
              </div>
              <input
                class="requirement-text"
                v-model="r.text"
                aria-label="Requirement description"
                maxlength="500"
              />
              <div class="rule-row">
                <label
                  >Check<select v-model="r.rule">
                    <option value="no_duplicates">No duplicate writes</option>
                    <option value="all_complete">All requests complete</option>
                    <option value="latency_p95">P95 latency limit</option>
                  </select></label
                ><label v-if="r.rule === 'latency_p95'"
                  >Maximum · ms<input
                    type="number"
                    v-model.number="r.targetMs"
                    min="0"
                    max="3600000"
                    step="1"
                /></label>
              </div>
            </div>
            <div v-if="!project.requirements.length" class="empty-inline">
              <FileCheck2 :size="25" />
              <p>No acceptance criteria yet.</p>
              <button class="button secondary" @click="addRequirement">
                Add your first requirement
              </button>
            </div>
          </div>
        </section>

        <section v-if="view === 'results'" class="results-workspace">
          <template v-if="currentRun"
            ><div class="run-summary">
              <div>
                <span class="section-eyebrow">{{
                  runStale
                    ? "SAVED RUN · DESIGN HAS CHANGED"
                    : "LATEST EXPERIMENT"
                }}</span>
                <h3>
                  <span
                    class="run-state"
                    :class="{ failed: passCount < currentRun.verdicts.length }"
                    ><CheckCircle2
                      v-if="
                        passCount === currentRun.verdicts.length &&
                        currentRun.verdicts.length
                      "
                      :size="20" /><CircleAlert v-else :size="20" /></span
                  >{{ passCount }} of {{ currentRun.verdicts.length }} checks
                  passed
                </h3>
                <p>
                  {{ runTime(currentRun.createdAt) }} · Seed
                  {{ currentRun.seed }} · Engine {{ currentRun.engineVersion }}
                </p>
              </div>
              <div class="run-summary-actions">
                <button class="button secondary" @click="openRunModel">
                  <GitBranch :size="15" /> Open run model</button
                ><button class="button quiet" @click="view = 'design'">
                  Back to canvas <ArrowRight :size="15" />
                </button>
              </div>
            </div>
            <div class="metric-grid">
              <div class="metric-card">
                <span>COMPLETED</span
                ><strong
                  >{{ currentRun.metrics.completed
                  }}<small> / {{ currentRun.metrics.requests }}</small></strong
                >
                <p>Requests acknowledged</p>
              </div>
              <div
                class="metric-card"
                :class="{ bad: currentRun.metrics.duplicateWrites > 0 }"
              >
                <span>DUPLICATE WRITES</span
                ><strong>{{ currentRun.metrics.duplicateWrites }}</strong>
                <p>
                  {{ currentRun.metrics.deduplicated }} retries deduplicated
                </p>
              </div>
              <div class="metric-card">
                <span>P95 LATENCY</span
                ><strong
                  >{{ numberFormat(currentRun.metrics.p95LatencyMs)
                  }}<small> ms</small></strong
                >
                <p>Completed requests only</p>
              </div>
              <div class="metric-card">
                <span>VIRTUAL DURATION</span
                ><strong
                  >{{ numberFormat(currentRun.metrics.durationMs)
                  }}<small> ms</small></strong
                >
                <p>{{ currentRun.metrics.attempts }} total attempts</p>
              </div>
            </div>
            <InfrastructureResults :run="currentRun" :history="runs" />
            <div class="results-columns">
              <div class="verdict-panel">
                <div class="section-title">
                  <h3>Requirement checks</h3>
                  <FileCheck2 :size="18" />
                </div>
                <div
                  class="verdict-row"
                  v-for="v in currentRun.verdicts"
                  :key="v.requirementId"
                >
                  <CheckCircle2
                    v-if="v.passed"
                    class="text-green"
                    :size="19"
                  /><XCircle v-else class="text-red" :size="19" />
                  <div>
                    <strong>{{ v.text }}</strong>
                    <p>{{ v.message }}</p>
                    <code>{{ v.requirementId }}</code>
                  </div>
                  <span class="status-chip" :class="{ fail: !v.passed }">{{
                    v.passed ? "PASS" : "FAIL"
                  }}</span>
                </div>
                <p class="help-text" v-if="!currentRun.verdicts.length">
                  This run has no acceptance criteria. Add requirements in Plan.
                </p>
              </div>
              <div class="run-details">
                <div class="section-title">
                  <h3>Run evidence</h3>
                  <ShieldCheck :size="18" />
                </div>
                <dl>
                  <dt>Failed requests</dt>
                  <dd>{{ currentRun.metrics.failed }}</dd>
                  <dt>Total writes</dt>
                  <dd>{{ currentRun.metrics.writes }}</dd>
                  <dt>Mean latency</dt>
                  <dd>
                    {{ numberFormat(currentRun.metrics.averageLatencyMs) }} ms
                  </dd>
                  <dt>Recorded events</dt>
                  <dd>{{ currentRun.events.length }}</dd>
                  <dt>Model fingerprint</dt>
                  <dd>
                    <code :title="currentRun.modelHash"
                      >{{ currentRun.modelHash.slice(0, 16) }}…</code
                    >
                  </dd>
                </dl>
                <p class="help-text">
                  Results describe this model and scenario. They do not execute
                  or certify your application code.
                </p>
              </div>
            </div></template
          >
          <div v-else class="experiment-empty">
            <span class="welcome-icon"><FlaskConical :size="34" /></span
            ><span class="section-eyebrow">YOUR FIRST EXPERIMENT</span>
            <h3>Put your design to the test.</h3>
            <p>
              Run the model to inspect request flow, retries, latency, and
              whether your requirements hold.
            </p>
            <div class="experiment-steps">
              <span><GitBranch :size="17" /> Design</span
              ><ArrowRight :size="16" /><span
                ><FlaskConical :size="17" /> Simulate</span
              ><ArrowRight :size="16" /><span
                ><FileCheck2 :size="17" /> Verify</span
              >
            </div>
            <button class="button primary" @click="simulate" :disabled="!!busy">
              <Play :size="15" /> Run simulation</button
            ><button
              class="button quiet"
              @click="
                view = 'design';
                inspector = 'scenario';
              "
            >
              Configure scenario
            </button>
          </div>
        </section>

        <section class="trace-panel" v-if="currentRun && view !== 'plan'">
          <div class="trace-toolbar">
            <div class="trace-title">
              <Activity :size="17" /><strong>Event replay</strong
              ><span class="event-count"
                >{{ currentRun.events.length }} events</span
              ><span class="snapshot-warning" v-if="runStale"
                >Design changed · viewing saved trace</span
              >
            </div>
            <div class="replay-controls">
              <button
                class="icon-button"
                @click="resetReplay"
                aria-label="Reset replay"
                title="Reset replay"
              >
                <RotateCcw :size="15" /></button
              ><button
                class="icon-button play-control"
                @click="toggleReplay"
                :aria-label="playing ? 'Pause replay' : 'Play replay'"
              >
                <Pause v-if="playing" :size="15" /><Play
                  v-else
                  :size="15"
                /></button
              ><button
                class="icon-button"
                @click="step"
                aria-label="Step to next event"
                title="Step to next event"
              >
                <SkipForward :size="15" /></button
              ><select v-model.number="replaySpeed" aria-label="Replay speed">
                <option :value="400">Slow</option>
                <option :value="120">Normal</option>
                <option :value="25">Fast</option></select
              ><span class="virtual-clock"
                >{{ numberFormat(currentEvent?.timeMs)
                }}<small> ms</small></span
              >
            </div>
          </div>
          <input
            class="replay-range"
            type="range"
            min="-1"
            :max="Math.max(-1, currentRun.events.length - 1)"
            :value="replayIndex"
            @input="seek"
            aria-label="Seek event replay"
          />
          <div class="event-table-wrap">
            <table class="event-table">
              <thead>
                <tr>
                  <th>TIME</th>
                  <th>EVENT</th>
                  <th>COMPONENT</th>
                  <th>REQUEST</th>
                  <th>DETAIL</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="e in visibleEvents"
                  :key="e.sequence"
                  :class="{ highlight: e.sequence === currentEvent?.sequence }"
                  @click="
                    pause();
                    replayIndex = currentRun.events.findIndex(
                      (x) => x.sequence === e.sequence,
                    );
                  "
                >
                  <td class="mono">{{ e.timeMs }} <small>ms</small></td>
                  <td>
                    <span
                      class="event-kind"
                      :class="{
                        'event-fault': /drop|fail|timeout/.test(e.kind),
                      }"
                      >{{ e.kind.replaceAll("_", " ") }}</span
                    >
                  </td>
                  <td>
                    {{
                      currentRun.project.nodes.find((n) => n.id === e.nodeId)
                        ?.name ||
                      currentRun.project.infrastructure?.hosts.find(
                        (h) => h.id === e.hostId,
                      )?.name ||
                      e.linkId ||
                      e.nodeId ||
                      "—"
                    }}
                  </td>
                  <td class="mono">{{ e.requestId || "—" }}</td>
                  <td>{{ e.message }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <div
          v-if="!currentRun && view === 'design'"
          class="getting-started-strip"
        >
          <span class="strip-icon"><FlaskConical :size="20" /></span>
          <div>
            <strong>Your design is ready for an experiment.</strong>
            <p>
              Validate the pipeline, configure a scenario, and run your first
              simulation.
            </p>
          </div>
          <button class="button quiet" @click="view = 'plan'">
            Review requirements <ArrowRight :size="15" />
          </button>
        </div>
      </template>
      <footer class="statusbar">
        <span
          ><span class="status-indicator" />{{
            validation?.valid
              ? "Model validated"
              : dirty
                ? "Unsaved design changes"
                : "Workspace ready"
          }}</span
        ><span
          >Vue + Go <span class="footer-dot">·</span> Discrete-event simulation
          <span class="footer-dot">·</span> v0.2</span
        >
      </footer>
    </main>
    <div v-if="notice" class="toast" role="status">
      <CheckCircle2 :size="17" />{{ notice }}
    </div>

    <div
      v-if="showProjects || showRunHistory"
      class="modal-backdrop"
      @click.self="
        showProjects = false;
        showRunHistory = false;
      "
    >
      <section
        class="modal"
        role="dialog"
        aria-modal="true"
        :aria-label="showProjects ? 'Projects' : 'Run history'"
      >
        <div class="modal-heading">
          <div>
            <span class="section-eyebrow">WORKSPACE</span>
            <h2>{{ showProjects ? "Your projects" : "Experiment history" }}</h2>
          </div>
          <button
            class="icon-button"
            @click="
              showProjects = false;
              showRunHistory = false;
            "
            aria-label="Close dialog"
          >
            <X :size="20" />
          </button>
        </div>
        <template v-if="showProjects"
          ><div class="project-dialog-actions">
            <button
              class="button primary"
              @click="newProject"
              :disabled="!!busy"
            >
              <Plus :size="15" /> New project</button
            ><button
              class="button secondary"
              @click="saveCopy"
              :disabled="!!busy || !project"
            >
              <Save :size="15" /> Save a copy
            </button>
          </div>
          <div class="modal-list">
            <button
              class="project-item"
              v-for="p in projects"
              :key="p.id"
              @click="loadProject(p)"
            >
              <span class="item-icon"><FolderOpen :size="20" /></span
              ><span
                ><strong>{{ p.name }}</strong
                ><small
                  >{{ p.nodes.length }} components ·
                  {{ p.requirements.length }} requirements</small
                ></span
              ><ArrowRight :size="16" />
            </button>
            <div v-if="!projects.length" class="empty-inline">
              <FolderOpen :size="28" />
              <p>Your saved projects will appear here.</p>
            </div>
          </div>
          <button
            class="button secondary full"
            @click="resetExample"
            :disabled="!!busy"
          >
            <RotateCcw :size="16" /> Open example project
          </button>
          <button
            class="button secondary full continuum-example-button"
            @click="loadContinuumExample"
            :disabled="!!busy"
          >
            <Server :size="16" /> Open continuum example
          </button></template
        ><template v-else
          ><p class="help-text">
            Saved simulations for the current project. Each run includes its
            model snapshot.
          </p>
          <div class="modal-list">
            <button
              v-for="r in runs"
              :key="r.id"
              class="project-item"
              @click="openRun(r)"
            >
              <span class="item-icon"><Activity :size="20" /></span
              ><span
                ><strong>{{ new Date(r.createdAt).toLocaleString() }}</strong
                ><small
                  >{{ r.verdicts.filter((v) => v.passed).length }}/{{
                    r.verdicts.length
                  }}
                  passed · Seed {{ r.seed }} ·
                  {{ r.metrics.completed }} completed</small
                ></span
              ><ArrowRight :size="16" />
            </button>
            <div v-if="!runs.length" class="empty-inline">
              <FlaskConical :size="28" />
              <p>No runs yet. Run a simulation to create evidence.</p>
            </div>
          </div></template
        >
      </section>
    </div>
  </div>
</template>

<style scoped>
.continuum-example-button {
  margin-top: 10px;
}
</style>
