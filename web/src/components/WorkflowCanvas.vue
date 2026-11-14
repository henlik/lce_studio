<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from "vue";
import type { WorkflowDefinition } from "../workflowTypes";
import {
  flowNodeHeight as H,
  flowNodeWidth as W,
  workflowFlowLayout,
  workflowRoutes,
  workflowTitleLines,
  type WorkflowRoute,
} from "../workflowPresentation";
import CanvasControls from "./CanvasControls.vue";
import { useCanvasFocus } from "../composables/useCanvasFocus";
const props = defineProps<{
  workflow: WorkflowDefinition;
  selected: string;
  disabled?: boolean;
  activeNode?: string;
  visitedIds?: string[];
  travelledRoutes?: WorkflowRoute[];
  activeRoute?: WorkflowRoute;
  invalidIds?: string[];
}>();
const emit = defineEmits<{
  select: [id: string];
  move: [id: string, position: { x: number; y: number }];
  arrange: [positions: Record<string, { x: number; y: number }>];
}>();
const wrapper = ref<HTMLElement | null>(null),
  viewport = ref<HTMLElement | null>(null),
  svg = ref<SVGSVGElement | null>(null);
const mode = ref<"flow" | "manual">("flow"),
  zoom = ref(1),
  overview = ref(false);
const layout = computed(() => workflowFlowLayout(props.workflow));
const safeNodes = computed(() =>
  props.workflow.nodes.map((n, i) => ({
    ...n,
    position:
      mode.value === "flow"
        ? layout.value.positions[n.id]
        : {
            x: Number.isFinite(n.position.x) ? n.position.x : i * 280,
            y: Number.isFinite(n.position.y) ? n.position.y : 0,
          },
  })),
);
const colors: Record<string, string> = {
  task: "#2478ba",
  decision: "#7957bb",
  command: "#15846a",
  end: "#526b82",
};
const icons: Record<string, string> = {
  task: "▤",
  decision: "◇",
  command: "▣",
  end: "●",
};
const edgeColors: Record<string, string> = {
  next: "#829cac",
  true: "#16846a",
  false: "#b5771c",
  error: "#b54842",
  signal: "#9746b4",
  return: "#9746b4",
  travelled: "#16846a",
  active: "#137fca",
};
const markerId = `workflow-${useId().replace(/:/g, "")}`;
const routes = computed(() => {
  const result = workflowRoutes(props.workflow);
  for (const route of [
    ...(props.travelledRoutes || []),
    ...(props.activeRoute ? [props.activeRoute] : []),
  ]) {
    if (route.kind === "signal" && !result.some((r) => r.id === route.id))
      result.push(route);
  }
  return result;
});
const travelled = computed(
  () => new Set(props.travelledRoutes?.map((r) => r.id)),
);
const wires = computed(() => {
  const rows = [];
  const left = Math.min(0, ...safeNodes.value.map((n) => n.position.x)),
    right = Math.max(W, ...safeNodes.value.map((n) => n.position.x + W));
  let outer = 0;
  for (const route of routes.value) {
    const source = safeNodes.value.find((n) => n.id === route.from),
      target = safeNodes.value.find((n) => n.id === route.to);
    if (!source || !target) continue;
    const sx = source.position.x,
      sy = source.position.y,
      tx = target.position.x,
      ty = target.position.y;
    const offset =
      route.kind === "true"
        ? -38
        : route.kind === "false"
          ? 38
          : route.kind === "error"
            ? 76
            : 0;
    const ax = sx + W / 2 + offset,
      ay = sy + H,
      bx = tx + W / 2,
      by = ty;
    const feedback = layout.value.returns.has(route.id);
    let path: string,
      x: number,
      y: number,
      minX = Math.min(ax, bx),
      maxX = Math.max(ax, bx);
    if (by > ay && by - ay < 150 && route.kind !== "signal") {
      const middle = ay + (by - ay) * (route.kind === "false" ? 0.68 : 0.36);
      path = `M ${ax} ${ay} V ${middle} H ${bx} V ${by}`;
      x = ax + 8;
      y = ay + (route.kind === "false" ? 62 : 26);
    } else {
      // Long jumps and loop returns travel in gutters outside the node lanes.
      const channel =
        route.kind === "error" || route.kind === "false"
          ? left - 44 - outer * 18
          : right + 44 + outer * 18;
      outer++;
      path = `M ${ax} ${ay} V ${ay + 28} H ${channel} V ${by - 28} H ${bx} V ${by}`;
      x = ax + 8;
      y = ay + (route.kind === "false" ? 62 : route.kind === "error" ? 86 : 20);
      minX = Math.min(minX, channel);
      maxX = Math.max(maxX, channel);
    }
    const active = props.activeRoute?.id === route.id,
      seen = travelled.value.has(route.id);
    const tone = active
      ? "active"
      : seen
        ? "travelled"
        : feedback
          ? "return"
          : route.kind;
    rows.push({
      ...route,
      path,
      x,
      y,
      feedback,
      jump: Math.abs(ty - sy) > 300 || feedback || route.kind === "signal",
      active,
      seen,
      tone,
      minX,
      maxX,
      labelWidth:
        Math.max(
          route.label.length,
          Math.abs(ty - sy) > 300 || feedback || route.kind === "signal"
            ? route.to.length + 2
            : 0,
        ) *
          6.6 +
        12,
      minY: Math.min(ay, by - 34),
      maxY: Math.max(ay + 86, by),
    });
  }
  // Keep the selected traversal above every alternative route.
  return rows.sort((a, b) => Number(a.active) - Number(b.active));
});
const bounds = computed(() => {
  const minX = Math.min(
    0,
    ...safeNodes.value.map((n) => n.position.x - 35),
    ...wires.value.map((w) => w.minX - 20),
  );
  const minY = Math.min(
    0,
    ...safeNodes.value.map((n) => n.position.y - 35),
    ...wires.value.map((w) => w.minY - 20),
  );
  const maxX = Math.max(
    360,
    ...safeNodes.value.map((n) => n.position.x + W + 35),
    ...wires.value.map((w) => Math.max(w.maxX + 25, w.x + w.labelWidth + 12)),
  );
  const maxY = Math.max(
    300,
    ...safeNodes.value.map((n) => n.position.y + H + 45),
    ...wires.value.map((w) => w.maxY + 35),
  );
  return { x: minX, y: minY, width: maxX - minX, height: maxY - minY };
});
const viewBox = computed(
  () =>
    `${bounds.value.x} ${bounds.value.y} ${bounds.value.width} ${bounds.value.height}`,
);
const dragBounds = ref<{ width: number; height: number } | null>(null);
const svgStyle = computed(() => ({
  width: `${(dragBounds.value || bounds.value).width * zoom.value}px`,
  height: `${(dragBounds.value || bounds.value).height * zoom.value}px`,
}));
async function focusNode(
  id = props.activeNode || props.selected || props.workflow.entry,
) {
  await nextTick();
  const n = safeNodes.value.find((n) => n.id === id),
    el = viewport.value;
  if (!n || !el) return;
  el.scrollTo({
    left:
      (n.position.x - bounds.value.x + W / 2) * zoom.value - el.clientWidth / 2,
    top:
      (n.position.y - bounds.value.y) * zoom.value -
      Math.min(70, el.clientHeight / 5),
  });
}
function readable() {
  overview.value = false;
  zoom.value = 1;
  void focusNode();
}
function fit() {
  overview.value = true;
  const el = viewport.value;
  if (!el?.clientWidth || !el.clientHeight) return;
  zoom.value = Math.max(
    0.005,
    Math.min(
      1,
      el.clientWidth / bounds.value.width,
      el.clientHeight / bounds.value.height,
    ),
  );
  void nextTick(() => el.scrollTo({ left: 0, top: 0 }));
}
function changeZoom(factor: number) {
  overview.value = false;
  zoom.value = Math.min(2, Math.max(0.005, zoom.value * factor));
  void focusNode();
}
const { expanded, toggleExpand } = useCanvasFocus(wrapper, () =>
  overview.value ? fit() : focusNode(),
);
function selectMode(value: "flow" | "manual") {
  mode.value = value;
  readable();
}
function saveArrangement() {
  if (props.disabled) return;
  emit("arrange", layout.value.positions);
  selectMode("manual");
}
watch(
  () => props.workflow,
  () => {
    mode.value = "flow";
    readable();
  },
);
watch(
  () => props.activeNode,
  () => {
    if (props.activeNode) readable();
  },
);
watch(
  () => props.selected,
  () => {
    if (props.selected && !drag) void focusNode(props.selected);
  },
);
watch(
  () => props.workflow.entry,
  () => void focusNode(),
);
let drag:
  | {
      id: string;
      origin: { x: number; y: number };
      start: DOMPoint;
      inverse: DOMMatrix;
    }
  | undefined;
const dragView = ref("");
function start(id: string, e: PointerEvent) {
  emit("select", id);
  if (mode.value !== "manual" || props.disabled || e.button !== 0 || !svg.value)
    return;
  const n = safeNodes.value.find((n) => n.id === id),
    inverse = svg.value.getScreenCTM()?.inverse();
  if (!n || !inverse) return;
  drag = {
    id,
    origin: { ...n.position },
    start: new DOMPoint(e.clientX, e.clientY).matrixTransform(inverse),
    inverse,
  };
  dragView.value = viewBox.value;
  dragBounds.value = { ...bounds.value };
  svg.value.setPointerCapture(e.pointerId);
  e.preventDefault();
}
function move(e: PointerEvent) {
  if (!drag) return;
  const p = new DOMPoint(e.clientX, e.clientY).matrixTransform(drag.inverse);
  emit("move", drag.id, {
    x: Math.round(drag.origin.x + p.x - drag.start.x),
    y: Math.round(drag.origin.y + p.y - drag.start.y),
  });
}
function stop() {
  drag = undefined;
  dragView.value = "";
  dragBounds.value = null;
}
function inspect(id: string) {
  emit("select", id);
  void focusNode(id);
}
</script>
<template>
  <section
    ref="wrapper"
    class="wf-canvas"
    :class="{ 'canvas-focus--expanded': expanded }"
  >
    <header>
      <div>
        <strong>Workflow flow</strong
        ><small>{{
          mode === "flow"
            ? "Read from top to bottom · branches split and rejoin"
            : "Saved arrangement · drag steps to position them"
        }}</small>
      </div>
      <CanvasControls
        label="workflow canvas"
        :zoom="zoom"
        :expanded="expanded"
        :min-zoom="0.005"
        :max-zoom="2"
        @zoom-in="changeZoom(1.25)"
        @zoom-out="changeZoom(0.8)"
        @fit="fit"
        @expand="toggleExpand"
      />
    </header>
    <div class="flow-toolbar">
      <div role="group" aria-label="Workflow arrangement">
        <button :aria-pressed="mode === 'flow'" @click="selectMode('flow')">
          Flow ↓
        </button>
        <button :aria-pressed="mode === 'manual'" @click="selectMode('manual')">
          Manual
        </button>
      </div>
      <button @click="readable">Readable size</button>
      <button @click="focusNode(workflow.entry)">Go to start</button>
      <button
        v-if="!disabled"
        :disabled="!workflow.nodes.length"
        @click="saveArrangement"
      >
        Save flow arrangement
      </button>
    </div>
    <div v-if="workflow.signals.length" class="flow-signals">
      <span>Interrupt routes</span
      ><button
        v-for="signal in workflow.signals"
        :key="signal.name"
        @click="inspect(signal.target)"
        :title="
          signal.when
            ? `Available when ${signal.when} is true`
            : 'Available while a task is waiting'
        "
      >
        ↗ {{ signal.name }} → {{ signal.target }}
      </button>
    </div>
    <div
      ref="viewport"
      class="canvas-viewport"
      tabindex="0"
      aria-label="Scrollable workflow flow"
    >
      <svg
        ref="svg"
        :style="svgStyle"
        :viewBox="dragView || viewBox"
        role="group"
        aria-label="Workflow nodes and automatic routes"
        @pointermove="move"
        @pointerup="stop"
        @pointercancel="stop"
      >
        <defs>
          <marker
            v-for="(color, kind) in edgeColors"
            :key="kind"
            :id="`${markerId}-${kind}`"
            markerWidth="8"
            markerHeight="8"
            refX="7"
            refY="4"
            orient="auto"
          >
            <path d="M0,0 L7,4 L0,8" :fill="color" />
          </marker>
        </defs>
        <g
          v-for="wire in wires"
          :key="wire.id"
          :class="{ 'wire-active': wire.active }"
        >
          <title>
            {{ wire.from }} → {{ wire.to }} · {{ wire.label
            }}{{ wire.feedback ? " · loop return" : "" }}
          </title>
          <path
            :d="wire.path"
            fill="none"
            :stroke="edgeColors[wire.tone]"
            :stroke-dasharray="
              wire.feedback || wire.kind === 'signal' ? '7 5' : undefined
            "
            :stroke-width="wire.active ? 4 : wire.seen ? 3 : 1.6"
            stroke-linejoin="round"
            :marker-end="`url(#${markerId}-${wire.tone})`"
            :opacity="activeNode && !wire.active && !wire.seen ? 0.4 : 1"
          />
          <g
            class="route-target"
            tabindex="0"
            role="button"
            :aria-label="`Follow ${wire.label} from ${wire.from} to ${wire.to}`"
            @click.stop="inspect(wire.to)"
            @keydown.enter="inspect(wire.to)"
            @keydown.space.prevent="inspect(wire.to)"
          >
            <rect
              :x="wire.x - 4"
              :y="wire.y - 12"
              :width="wire.labelWidth"
              :height="wire.jump ? 32 : 20"
              rx="3"
              fill="#f8fbfc"
              fill-opacity="0.95"
            />
            <text
              :x="wire.x"
              :y="wire.y"
              class="wire-label"
              :class="{ 'active-label': wire.active }"
            >
              {{ wire.feedback ? "↶ " : "" }}{{ wire.label }}
              <tspan v-if="wire.jump" :x="wire.x" dy="14">
                → {{ wire.to }}
              </tspan>
            </text>
          </g>
        </g>
        <g
          v-for="node in safeNodes"
          :key="node.id"
          :transform="`translate(${node.position.x},${node.position.y})`"
          class="wf-node"
          :class="{
            selected: selected === node.id,
            active: activeNode === node.id,
            visited: visitedIds?.includes(node.id),
            invalid: invalidIds?.includes(node.id),
          }"
          :style="{ '--node-color': node.color || colors[node.kind] }"
          tabindex="0"
          role="button"
          :aria-label="`Inspect ${node.name}, ${node.kind}${activeNode === node.id ? ', token here' : ''}`"
          @pointerdown="start(node.id, $event)"
          @keydown.enter="inspect(node.id)"
          @keydown.space.prevent="inspect(node.id)"
        >
          <title>{{ node.name || node.id }} · {{ node.id }}</title>
          <path
            v-if="node.kind === 'decision'"
            :d="`M18 0 H${W - 18} L${W} 18 V${H - 18} L${W - 18} ${H} H18 L0 ${H - 18} V18 Z`"
            class="wf-node-shape"
          />
          <rect
            v-else
            :width="W"
            :height="H"
            :rx="node.kind === 'end' ? 30 : node.kind === 'command' ? 3 : 10"
            class="wf-node-shape"
          />
          <text
            :x="W - 26"
            y="25"
            :fill="node.color || colors[node.kind]"
            font-size="18"
          >
            {{ icons[node.kind] }}
          </text>
          <circle :cx="W / 2" cy="0" r="4" fill="#2478ba" />
          <circle
            v-if="node.kind !== 'end'"
            :cx="W / 2"
            :cy="H"
            r="4"
            fill="#c78116"
          />
          <text x="15" y="23" class="node-kind">
            {{ workflow.entry === node.id ? "START · " : ""
            }}{{ node.task?.kind || node.kind }}
          </text>
          <text
            v-for="(line, i) in workflowTitleLines(node)"
            :key="i"
            x="15"
            :y="47 + i * 18"
            class="node-title"
          >
            {{ line }}
          </text>
          <text x="15" y="91" class="node-id">
            {{ node.id.length > 30 ? `${node.id.slice(0, 29)}…` : node.id }}
          </text>
          <g
            v-if="activeNode === node.id"
            :transform="`translate(${W - 13},${H - 16})`"
          >
            <circle r="12" fill="#137fca" stroke="white" stroke-width="2" />
            <text y="4" text-anchor="middle" fill="white" font-size="13">
              ●
            </text>
          </g>
        </g>
        <text
          v-if="!workflow.nodes.length"
          x="20"
          y="90"
          fill="#6b8797"
          font-size="14"
        >
          Add a step to begin.
        </text>
      </svg>
    </div>
    <footer>
      <span>↓ Sequence</span><span class="true-key">True</span
      ><span class="false-key">False</span><span class="error-key">Failure</span
      ><span class="loop-key">↶ Loop / interrupt</span
      ><span v-if="activeNode" class="token-key">● Token · blue route</span
      ><span v-if="visitedIds?.length" class="true-key">✓ Travelled</span>
    </footer>
  </section>
</template>
<style scoped>
.wf-canvas {
  border: 1px solid #dbe6ec;
  background: #f8fbfc;
  border-radius: 9px;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.wf-canvas header {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
  padding: 12px;
  border-bottom: 1px solid #e1e9ed;
  background: #fff;
  flex-wrap: wrap;
}
.wf-canvas header strong {
  display: block;
  font-size: 13px;
  color: #416173;
}
.wf-canvas header small {
  display: block;
  margin-top: 5px;
  color: #617b8a;
  font-size: 11px;
}
.flow-toolbar,
.flow-signals,
footer {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  padding: 9px 12px;
  border-bottom: 1px solid #e1e9ed;
  align-items: center;
  font-size: 11px;
}
.flow-toolbar > div {
  display: flex;
  gap: 2px;
}
.flow-toolbar button,
.flow-signals button {
  padding: 6px 9px;
  border: 1px solid #dbe5ea;
  border-radius: 5px;
  background: white;
  color: #466678;
  font-size: 11px;
  overflow-wrap: anywhere;
  text-align: left;
}
.flow-toolbar button[aria-pressed="true"] {
  background: #e2f3ed;
  border-color: #86bbaa;
  color: #176d59;
}
.flow-signals {
  color: #805096;
}
.flow-signals button {
  color: #805096;
}
.wf-canvas .canvas-viewport {
  min-height: 360px;
  height: clamp(430px, 62vh, 720px);
  background-image: radial-gradient(#dbe7ed 1px, transparent 1px);
  background-size: 18px 18px;
}
.wf-canvas .canvas-viewport > svg {
  margin: 0;
  flex-shrink: 0;
}
.wf-canvas.canvas-focus--expanded .canvas-viewport {
  min-height: 0;
  height: auto;
}
.wf-node {
  cursor: pointer;
}
.wf-node .wf-node-shape {
  fill: white;
  stroke: var(--node-color);
  stroke-width: 1.7;
}
.wf-node.selected .wf-node-shape,
.wf-node:focus .wf-node-shape {
  stroke-width: 3;
}
.wf-node.visited .wf-node-shape {
  fill: #f0faf5;
}
.wf-node.active .wf-node-shape {
  fill: #e9f5ff;
  stroke: #137fca;
  stroke-width: 3;
}
.wf-node.invalid .wf-node-shape {
  stroke: #c4312d;
  fill: #fff0ee;
  stroke-width: 3;
}
.wf-node:focus {
  outline: none;
}
.node-kind {
  font-size: 10px;
  font-weight: 700;
  fill: #547a8c;
  text-transform: uppercase;
}
.node-title {
  font-size: 13px;
  font-weight: 600;
  fill: #294e64;
}
.node-id {
  font-size: 10px;
  fill: #718a98;
}
.wire-label {
  font-size: 11px;
  fill: #526e80;
  paint-order: stroke;
  stroke: #f8fbfc;
  stroke-width: 5px;
  stroke-linejoin: round;
  cursor: pointer;
}
.route-target {
  cursor: pointer;
}
.route-target:focus {
  outline: 2px solid #137fca;
}
.active-label {
  fill: #086cae;
  font-weight: 700;
}
footer {
  border-top: 1px solid #e1e9ed;
  border-bottom: 0;
  color: #61798b;
  gap: 14px;
  background: white;
}
.true-key {
  color: #16846a;
}
.false-key {
  color: #a06913;
}
.error-key {
  color: #b54842;
}
.loop-key {
  color: #9746b4;
}
.token-key {
  color: #137fca;
}
@media (max-width: 600px) {
  .wf-canvas .canvas-viewport {
    height: 420px;
  }
}
</style>
