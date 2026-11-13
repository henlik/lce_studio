<script setup lang="ts">
import { feedbackEdges } from "../graphPresentation";
import { computed, ref } from "vue";
import type { WorkflowDefinition } from "../workflowTypes";
import CanvasControls from "./CanvasControls.vue";
import {
  useCanvasFocus,
  useCanvasViewport,
} from "../composables/useCanvasFocus";
const props = defineProps<{
  workflow: WorkflowDefinition;
  selected: string;
  disabled?: boolean;
  activeNode?: string;
  invalidIds?: string[];
}>();
const emit = defineEmits<{
  select: [id: string];
  move: [id: string, position: { x: number; y: number }];
}>();
const wrapper = ref<HTMLElement | null>(null),
  viewport = ref<HTMLElement | null>(null),
  svg = ref<SVGSVGElement | null>(null);
const { zoom, svgStyle, fit, zoomIn, zoomOut } = useCanvasViewport(viewport),
  { expanded, toggleExpand } = useCanvasFocus(wrapper, fit);
const safeNodes = computed(() =>
  props.workflow.nodes.map((n, i) => ({
    ...n,
    position: {
      x: Number.isFinite(n.position.x) ? n.position.x : i * 250,
      y: Number.isFinite(n.position.y) ? n.position.y : 0,
    },
  })),
);
function linksFor(n: WorkflowDefinition["nodes"][number]) {
  return [
    ...(n.next ? [{ target: n.next, label: "next" }] : []),
    ...(n.branch
      ? [
          { target: n.branch.then, label: `${n.branch.variable}: true` },
          { target: n.branch.else, label: `${n.branch.variable}: false` },
        ]
      : []),
    ...(n.command?.errorNext
      ? [{ target: n.command.errorNext, label: "command failure" }]
      : []),
  ];
}
const cyclic = computed(() =>
  feedbackEdges(
    safeNodes.value.flatMap((n) =>
      linksFor(n).map((l, i) => ({
        from: n.id,
        to: l.target,
        id: `${n.id}:${i}`,
      })),
    ),
  ),
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
const wires = computed(() => {
  const rows: {
    id: string;
    path: string;
    label: string;
    feedback: boolean;
    color: string;
    x: number;
    y: number;
  }[] = [];
  for (const n of safeNodes.value) {
    const links = linksFor(n);
    links.forEach((l, i) => {
      const t = safeNodes.value.find((x) => x.id === l.target);
      if (!t) return;
      const sx = n.position.x,
        sy = n.position.y,
        tx = t.position.x,
        ty = t.position.y;
      let path = "",
        x = 0,
        y = 0;
      if (t.id === n.id) {
        const top = sy - 60 - i * 25;
        path = `M ${sx + 155} ${sy} C ${sx + 230} ${top - 40}, ${sx - 20} ${top - 40}, ${sx + 55} ${sy}`;
        x = sx + 100;
        y = top - 20;
      } else if (Math.abs(tx - sx) > 220) {
        const right = tx > sx,
          a = right ? sx + 200 : sx,
          b = right ? tx : tx + 200,
          yy = sy + 46 + i * 8,
          z = ty + 46;
        path = `M ${a} ${yy} C ${(a + b) / 2} ${yy}, ${(a + b) / 2} ${z}, ${b} ${z}`;
        x = (a + b) / 2;
        y = (yy + z) / 2 - 10 - i * 12;
      } else {
        const down = ty > sy,
          a = down ? sy + 92 : sy,
          b = down ? ty : ty + 92,
          xx = sx + 100 + i * 14,
          z = tx + 100;
        path = `M ${xx} ${a} C ${xx} ${(a + b) / 2}, ${z} ${(a + b) / 2}, ${z} ${b}`;
        x = (xx + z) / 2 + 8;
        y = (a + b) / 2 - 6;
      }
      rows.push({
        id: `${n.id}:${i}`,
        path,
        label: l.label,
        feedback: cyclic.value.has(`${n.id}:${i}`),
        color: l.label.endsWith(": true")
          ? "#16846a"
          : l.label.endsWith(": false")
            ? "#b5771c"
            : l.label === "command failure"
              ? "#b54842"
              : "#829cac",
        x,
        y,
      });
    });
  }
  return rows;
});
const viewBox = computed(() => {
  const ns = safeNodes.value;
  const minX = Math.min(
      -30,
      ...ns.map((n) => n.position.x - 40),
      ...wires.value.map((w) => w.x - 100),
    ),
    maxX = Math.max(
      500,
      ...ns.map((n) => n.position.x + 240),
      ...wires.value.map((w) => w.x + 140),
    ),
    minY = Math.min(
      -30,
      ...ns.map((n) => n.position.y - 80),
      ...wires.value.map((w) => w.y - 60),
    ),
    maxY = Math.max(280, ...ns.map((n) => n.position.y + 135));
  return `${minX} ${minY} ${maxX - minX} ${maxY - minY}`;
});
const dragView = ref("");
let drag:
  | {
      id: string;
      origin: { x: number; y: number };
      start: DOMPoint;
      inverse: DOMMatrix;
    }
  | undefined;
function point(e: PointerEvent, m: DOMMatrix) {
  return new DOMPoint(e.clientX, e.clientY).matrixTransform(m);
}
function start(id: string, e: PointerEvent) {
  emit("select", id);
  if (props.disabled || e.button !== 0 || !svg.value) return;
  const n = props.workflow.nodes.find((n) => n.id === id);
  const m = svg.value.getScreenCTM()?.inverse();
  if (
    !n ||
    !m ||
    !Number.isFinite(n.position.x) ||
    !Number.isFinite(n.position.y)
  )
    return;
  drag = { id, origin: { ...n.position }, start: point(e, m), inverse: m };
  dragView.value = viewBox.value;
  svg.value.setPointerCapture(e.pointerId);
  e.preventDefault();
}
function move(e: PointerEvent) {
  if (!drag) return;
  const p = point(e, drag.inverse);
  emit("move", drag.id, {
    x: Math.round(drag.origin.x + p.x - drag.start.x),
    y: Math.round(drag.origin.y + p.y - drag.start.y),
  });
}
function stop() {
  drag = undefined;
  dragView.value = "";
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
        <strong>Workflow route</strong
        ><small
          >Drag steps · select to inspect · arrows execute automatically</small
        >
      </div>
      <CanvasControls
        label="workflow canvas"
        :zoom="zoom"
        :expanded="expanded"
        @zoom-in="zoomIn"
        @zoom-out="zoomOut"
        @fit="fit"
        @expand="toggleExpand"
      />
    </header>
    <div ref="viewport" class="canvas-viewport">
      <svg
        ref="svg"
        :style="svgStyle"
        :viewBox="dragView || viewBox"
        role="img"
        aria-label="Workflow nodes and automatic routes"
        @pointermove="move"
        @pointerup="stop"
        @pointercancel="stop"
      >
        <defs>
          <marker
            id="workflow-arrow"
            markerWidth="9"
            markerHeight="8"
            refX="8"
            refY="4"
            orient="auto"
          >
            <path d="M0,0 L8,4 L0,8" fill="#88a5b6" />
          </marker>
        </defs>
        <g v-for="wire in wires" :key="wire.id">
          <path
            :d="wire.path"
            fill="none"
            :stroke="wire.feedback ? '#9746b4' : wire.color"
            :stroke-dasharray="wire.feedback ? '7 4' : undefined"
            stroke-width="2"
            marker-end="url(#workflow-arrow)"
          />
          <text :x="wire.x" :y="wire.y" class="wire-label">
            {{ wire.feedback ? "↶ Feedback · " : "" }}{{ wire.label }}
          </text>
        </g>
        <g
          v-for="node in safeNodes"
          :key="node.id"
          :transform="`translate(${node.position.x},${node.position.y})`"
          class="wf-node"
          :class="{
            selected: selected === node.id,
            active: activeNode === node.id,
            invalid: invalidIds?.includes(node.id),
          }"
          :style="{ '--node-color': node.color || colors[node.kind] }"
          tabindex="0"
          role="button"
          :aria-label="`Inspect ${node.name}, ${node.kind}`"
          @pointerdown="start(node.id, $event)"
          @keydown.enter="emit('select', node.id)"
          @keydown.space.prevent="emit('select', node.id)"
        >
          <path
            v-if="node.kind === 'decision'"
            d="M16 0 H184 L200 16 V76 L184 92 H16 L0 76 V16 Z"
            class="wf-node-shape"
          />
          <rect
            v-else
            width="200"
            height="92"
            :rx="node.kind === 'end' ? 34 : node.kind === 'command' ? 2 : 10"
            class="wf-node-shape"
          />
          <text
            x="175"
            y="24"
            :fill="node.color || colors[node.kind]"
            font-size="18"
          >
            {{ icons[node.kind] }}
          </text>
          <circle cx="0" cy="46" r="4" fill="#2478ba" />
          <circle
            v-if="node.kind !== 'end'"
            cx="200"
            cy="46"
            r="4"
            fill="#c78116"
          />
          <text x="15" y="23" class="node-kind">
            {{ workflow.entry === node.id ? "ENTRY · " : ""
            }}{{ node.task?.kind || node.kind }}
          </text>
          <text x="15" y="47" class="node-title">
            {{ (node.name || node.id).slice(0, 25) }}
          </text>
          <text x="15" y="70" class="node-id">{{ node.id.slice(0, 28) }}</text>
        </g>
        <text
          v-if="!workflow.nodes.length"
          x="20"
          y="90"
          fill="#6b8797"
          font-size="16"
        >
          Add a task, decision, command or end to begin.
        </text>
      </svg>
    </div>
  </section>
</template>
<style scoped>
.wf-canvas {
  border: 1px solid #dbe6ec;
  background: #f8fbfc;
  border-radius: 9px;
  min-width: 0;
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
  color: #7b919e;
  font-size: 10px;
}
.wf-canvas .canvas-viewport {
  min-height: 330px;
}
.wf-canvas.canvas-focus--expanded .canvas-viewport {
  min-height: 0;
}
.wf-node {
  cursor: grab;
}
.wf-node rect {
  fill: white;
  stroke: #cfdee6;
  stroke-width: 1.5;
}
.wf-node.selected rect {
  stroke: #218c78;
  stroke-width: 3;
}
.wf-node.active rect {
  fill: #e6f5ef;
}
.node-kind {
  font-size: 10px;
  font-weight: 700;
  fill: #568d83;
  text-transform: uppercase;
}
.node-title {
  font-size: 13px;
  font-weight: 600;
  fill: #34556a;
}
.node-id {
  font-size: 10px;
  fill: #8598a3;
}
.wire-label {
  font-size: 10px;
  fill: #668697;
  paint-order: stroke;
  stroke: #f8fbfc;
  stroke-width: 5px;
  stroke-linejoin: round;
}
.wf-node:focus {
  outline: none;
}
.wf-node:focus rect {
  stroke: #246fbb;
  stroke-width: 3;
}
.wf-node .wf-node-shape {
  fill: white;
  stroke: var(--node-color);
  stroke-width: 1.7;
}
.wf-node.selected .wf-node-shape {
  stroke-width: 3;
}
.wf-node.invalid .wf-node-shape {
  stroke: #c4312d;
  fill: #fff0ee;
  stroke-width: 3;
}
.wf-node.active .wf-node-shape {
  fill: #ecfcf5;
  stroke-width: 3;
}
</style>
