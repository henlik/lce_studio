<script setup lang="ts">
import type { DataContractLock } from "../dataTypes";
import { computed, ref } from "vue";
import CanvasControls from "./CanvasControls.vue";
import { useCanvasFocus, useCanvasViewport } from "../composables/useCanvasFocus";
import {
  nodeInputs,
  nodeOutputs,
  nestedPackage,
  valueText,
  type Behavior,
  type BehaviorBinding,
  type CompositeLock,
  type PrimitiveSpec,
} from "../behaviorTypes";
const props = defineProps<{
  behavior: Behavior;
  dependencies: CompositeLock[];
  primitives: PrimitiveSpec[];
  selectedId: string;
  readonly?: boolean;
  contracts?: DataContractLock[];
}>();
const emit = defineEmits<{
  select: [id: string];
  inspect: [id: string];
  move: [id: string, position: { x: number; y: number }];
}>();
const svg = ref<SVGSVGElement | null>(null);
const wrapper = ref<HTMLElement | null>(null);
const viewport = ref<HTMLElement | null>(null);
const { zoom, svgStyle, fit, zoomIn, zoomOut } = useCanvasViewport(viewport);
const { expanded, toggleExpand } = useCanvasFocus(wrapper, fit);
const dragView = ref("");
let drag:
  | { id: string; x: number; y: number; original: { x: number; y: number } }
  | undefined;
const nodes = computed(() =>
  props.behavior.nodes.map((node) => {
    const inputs = nodeInputs(
      node,
      props.dependencies,
      props.primitives,
      props.contracts,
      props.behavior,
    );
    const outputs = nodeOutputs(
      node,
      props.behavior,
      props.dependencies,
      props.primitives,
      props.contracts,
    );
    return {
      node,
      inputs,
      outputs,
      height: 62 + Math.max(inputs.length, outputs.length, 1) * 25,
      title: node.component
        ? nestedPackage(node, props.dependencies)?.name ||
          node.component.packageId
        : props.primitives.find((p) => p.op === node.op)?.name ||
          node.op ||
          "Unknown operation",
    };
  }),
);
const minX = computed(() =>
  Math.min(0, ...nodes.value.map((n) => n.node.position.x)),
);
const inputX = computed(() => minX.value - 215);
const outputX = computed(
  () => Math.max(250, ...nodes.value.map((n) => n.node.position.x + 250)) + 85,
);
const minY = computed(
  () => Math.min(0, ...nodes.value.map((n) => n.node.position.y)) - 35,
);
const graphHeight = computed(
  () =>
    Math.max(
      260,
      props.behavior.inputs.length * 34 + 80,
      props.behavior.outputs.length * 34 + 80,
      ...nodes.value.map((n) => n.node.position.y + n.height + 35),
    ) - minY.value,
);
const graphWidth = computed(() => outputX.value + 190 - inputX.value);
const viewBox = computed(
  () =>
    `${inputX.value - 15} ${minY.value} ${graphWidth.value + 30} ${graphHeight.value}`,
);
function sourcePoint(binding: BehaviorBinding | undefined) {
  if (binding?.input) {
    const index = props.behavior.inputs.findIndex(
      (p) => p.name === binding.input,
    );
    return index >= 0
      ? { x: inputX.value + 170, y: index * 34 + 54 }
      : undefined;
  }
  const n = nodes.value.find((n) => n.node.id === binding?.node);
  const index = n?.outputs.findIndex((p) => p.port === binding?.port) ?? -1;
  return n && index >= 0
    ? { x: n.node.position.x + 250, y: n.node.position.y + 59 + index * 25 }
    : undefined;
}
const wires = computed(() => {
  const result: { id: string; path: string }[] = [];
  function wire(
    binding: BehaviorBinding | undefined,
    x: number,
    y: number,
    id: string,
  ) {
    const from = sourcePoint(binding);
    if (!from) return;
    const curve = Math.max(40, Math.abs(x - from.x) / 2);
    result.push({
      id,
      path: `M ${from.x} ${from.y} C ${from.x + curve} ${from.y}, ${x - curve} ${y}, ${x} ${y}`,
    });
  }
  nodes.value.forEach((n) =>
    n.inputs.forEach((p, i) =>
      wire(
        n.node.inputs[p.name],
        n.node.position.x,
        n.node.position.y + 59 + i * 25,
        `${n.node.id}:${p.name}`,
      ),
    ),
  );
  props.behavior.outputs.forEach((p, i) =>
    wire(
      props.behavior.bindings[p.name],
      outputX.value,
      i * 34 + 54,
      `output:${p.name}`,
    ),
  );
  return result;
});
function point(event: PointerEvent) {
  const p = svg.value!.createSVGPoint();
  p.x = event.clientX;
  p.y = event.clientY;
  return p.matrixTransform(svg.value!.getScreenCTM()!.inverse());
}
function start(id: string, event: PointerEvent) {
  emit("select", id);
  if (props.readonly || event.button !== 0 || !svg.value) return;
  const p = point(event);
  const node = props.behavior.nodes.find((n) => n.id === id)!;
  drag = { id, x: p.x, y: p.y, original: { ...node.position } };
  dragView.value = viewBox.value;
  svg.value.setPointerCapture(event.pointerId);
  event.preventDefault();
}
function move(event: PointerEvent) {
  if (!drag || !svg.value) return;
  const p = point(event);
  emit("move", drag.id, {
    x: Math.round(drag.original.x + p.x - drag.x),
    y: Math.round(drag.original.y + p.y - drag.y),
  });
}
function stop() {
  drag = undefined;
  dragView.value = "";
}
const clipped = (text: string, max = 24) =>
  text.length > max ? `${text.slice(0, max - 1)}…` : text;
</script>
<template>
  <section ref="wrapper" class="behavior-graph-panel" :class="{ 'canvas-focus--expanded': expanded }">
    <div class="behavior-graph-toolbar">
      <span
        >BOUND DATAFLOW
        <small
          >{{ behavior.nodes.length }} nodes ·
          {{ wires.length }} connections</small
        ></span
      >
      <CanvasControls label="component graph" :zoom="zoom" :expanded="expanded" @zoom-in="zoomIn" @zoom-out="zoomOut" @fit="fit" @expand="toggleExpand" />
    </div>
    <div ref="viewport" class="behavior-graph-scroll canvas-viewport">
      <svg
        ref="svg"
        :viewBox="dragView || viewBox"
        :style="svgStyle"
        role="img"
        aria-label="Typed component input, operation and output bindings"
        @pointermove="move"
        @pointerup="stop"
        @pointercancel="stop"
        @lostpointercapture="stop"
      >
        <defs>
          <pattern
            id="behavior-dots"
            width="20"
            height="20"
            patternUnits="userSpaceOnUse"
          >
            <circle cx="1" cy="1" r=".7" fill="#d6e1e8" />
          </pattern>
        </defs>
        <rect
          :x="inputX - 15"
          :y="minY"
          :width="graphWidth + 30"
          :height="graphHeight"
          fill="url(#behavior-dots)"
        />
        <path
          v-for="wire in wires"
          :key="wire.id"
          :d="wire.path"
          class="behavior-wire"
        />
        <text :x="inputX" y="20" class="graph-section-label">
          PUBLIC INPUTS
        </text>
        <g
          v-for="(port, i) in behavior.inputs"
          :key="port.name + i"
          :transform="`translate(${inputX},${i * 34 + 39})`"
        >
          <rect width="170" height="29" rx="5" class="graph-boundary" />
          <text x="9" y="18" class="graph-port-name">
            {{ clipped(port.name, 19) }}
          </text>
          <text x="161" y="18" text-anchor="end" class="graph-port-type">
            {{ port.type.slice(0, 3) }}
          </text>
          <circle cx="170" cy="15" r="3" class="graph-port-dot" />
          <title>{{ port.name }}: {{ port.type }}</title>
        </g>
        <g
          v-for="n in nodes"
          :key="n.node.id"
          :transform="`translate(${n.node.position.x},${n.node.position.y})`"
          class="behavior-graph-node"
          :class="{
            selected: n.node.id === selectedId,
            nested: !!n.node.component,
          }"
          role="button"
          tabindex="0"
          :aria-label="`Select behavior node ${n.node.id}`"
          @pointerdown="start(n.node.id, $event)"
          @keydown.enter="emit('select', n.node.id)"
          @keydown.space.prevent="emit('select', n.node.id)"
          @dblclick="n.node.component && emit('inspect', n.node.id)"
        >
          <rect width="250" :height="n.height" rx="8" class="graph-node-box" />
          <text x="12" y="20" class="graph-node-title">
            {{ clipped(n.title || "", 30) }}
          </text>
          <text x="12" y="36" class="graph-node-subtitle">
            {{ n.node.id }} ·
            {{ n.node.component ? "nested component" : n.node.op }}
          </text>
          <line x1="0" y1="44" x2="250" y2="44" stroke="#edf1f4" />
          <g v-for="(port, i) in n.inputs" :key="port.name">
            <circle cx="0" :cy="59 + i * 25" r="3.5" class="graph-port-dot" />
            <text x="10" :y="62 + i * 25" class="graph-port-name">
              {{ clipped(port.name, 14) }}
              <tspan
                v-if="n.node.inputs[port.name]?.literal"
                class="graph-literal"
              >
                = {{ clipped(valueText(n.node.inputs[port.name].literal), 12) }}
              </tspan>
            </text>
            <title>{{ port.name }}: {{ port.type }}</title>
          </g>
          <g v-for="(port, i) in n.outputs" :key="port.port">
            <circle cx="250" :cy="59 + i * 25" r="3.5" class="graph-port-dot" />
            <text
              x="241"
              :y="62 + i * 25"
              text-anchor="end"
              class="graph-port-type"
            >
              {{ port.port }} · {{ port.type }}
            </text>
          </g>
          <title>
            {{ n.node.id }} — {{ n.title
            }}{{
              n.node.component
                ? ". Double-click to inspect its pinned definition."
                : ""
            }}
          </title>
        </g>
        <text :x="outputX" y="20" class="graph-section-label">
          PUBLIC OUTPUTS
        </text>
        <g
          v-for="(port, i) in behavior.outputs"
          :key="port.name + i"
          :transform="`translate(${outputX},${i * 34 + 39})`"
        >
          <rect width="170" height="29" rx="5" class="graph-boundary output" />
          <circle cx="0" cy="15" r="3" class="graph-port-dot" />
          <text x="9" y="18" class="graph-port-name">
            {{ clipped(port.name, 19) }}
          </text>
          <text x="161" y="18" text-anchor="end" class="graph-port-type">
            {{ port.type.slice(0, 3) }}
          </text>
          <text
            v-if="behavior.bindings[port.name]?.literal"
            x="9"
            y="40"
            class="graph-literal"
          >
            {{ clipped(valueText(behavior.bindings[port.name].literal), 24) }}
          </text>
          <title>{{ port.name }}: {{ port.type }}</title>
        </g>
      </svg>
    </div>
    <p>
      {{
        readonly
          ? "Pinned definition · select a node to inspect its bindings."
          : "Select and drag nodes to arrange the definition. Edit typed sources in the binding inspector."
      }}
      Nested calls can be opened with a double-click.
    </p>
  </section>
</template>
<style scoped>
.behavior-graph-panel {
  border: 1px solid #dfe8ee;
  border-radius: 9px;
  background: #fff;
  overflow: hidden;
}
.behavior-graph-panel.canvas-focus--expanded { display: flex; flex-direction: column; }
.behavior-graph-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  gap: 12px;
  flex-wrap: wrap;
  border-bottom: 1px solid #e7edf1;
}
.behavior-graph-toolbar > span {
  font-size: 12px;
  letter-spacing: 0.7px;
  color: #758d9e;
  font-weight: 600;
}
.behavior-graph-toolbar small {
  font-size: 12px;
  letter-spacing: 0;
  font-weight: 400;
  margin-left: 12px;
  color: #657e8f;
}
.behavior-graph-toolbar > div {
  display: flex;
  gap: 4px;
}
.behavior-graph-scroll {
  overflow: auto;
  background: #f9fbfc;
}
.behavior-graph-scroll svg {
  display: block;
  user-select: none;
  touch-action: none;
}
.behavior-wire {
  fill: none;
  stroke: #9ab5bf;
  stroke-width: 1.7;
}
.graph-section-label {
  font-size: 12px;
  letter-spacing: 1px;
  fill: #657e8f;
}
.graph-boundary {
  fill: #edf6f8;
  stroke: #cbdee5;
}
.graph-boundary.output {
  fill: #eef7f1;
  stroke: #d0e5d6;
}
.graph-port-name {
  font-size: 12px;
  fill: #5a7387;
}
.graph-port-type {
  font-size: 12px;
  fill: #657e8f;
}
.graph-port-dot {
  fill: #67a99c;
}
.behavior-graph-node {
  cursor: pointer;
  outline: none;
}
.graph-node-box {
  fill: #fff;
  stroke: #d3e0e8;
}
.behavior-graph-node.nested .graph-node-box {
  stroke: #b5d9ce;
  fill: #fbfefc;
}
.behavior-graph-node.selected .graph-node-box,
.behavior-graph-node:focus .graph-node-box {
  stroke: #1b9c86;
  stroke-width: 2;
}
.graph-node-title {
  font-size: 12px;
  font-weight: 600;
  fill: #3b596e;
}
.graph-node-subtitle {
  font-size: 12px;
  fill: #657e8f;
}
.graph-literal {
  font-size: 12px;
  fill: #b08d56;
}
.behavior-graph-panel > p {
  font-size: 12px;
  line-height: 1.6;
  padding: 10px 14px;
  margin: 0;
  border-top: 1px solid #e7edf1;
  color: #657e8f;
}
</style>
