<script setup lang="ts">
import { computed, ref } from "vue";
import { instanceMachine, type DeliveryModel } from "../deliveryTypes";
const props = defineProps<{
  model: DeliveryModel;
  selected: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  select: [id: string];
  move: [id: string, position: { x: number; y: number }];
}>();
const svg = ref<SVGSVGElement | null>(null),
  zoom = ref(1),
  frozen = ref<{ x: number; y: number; w: number; h: number } | null>(null);
let drag:
  | {
      id: string;
      x: number;
      y: number;
      ox: number;
      oy: number;
      transform: DOMMatrix;
    }
  | undefined;
const cards = computed(() =>
  props.model.instances.map((i, index) => ({
    ...i,
    x: Number.isFinite(i.position.x) ? i.position.x : 40 + (index % 3) * 350,
    y: Number.isFinite(i.position.y)
      ? i.position.y
      : 90 + Math.floor(index / 3) * 180,
    machineName:
      instanceMachine(props.model, i.id)?.name || "Unresolved machine",
  })),
);
const wires = computed(() =>
  props.model.routes.flatMap((r, index) => {
    const a = cards.value.find((i) => i.id === r.sourceInstanceId),
      b = cards.value.find((i) => i.id === r.targetInstanceId);
    if (!a || !b) return [];
    const forward = a.x <= b.x,
      sign = forward ? 1 : -1,
      x1 = a.x + (forward ? 270 : 0),
      x2 = b.x + (forward ? 0 : 270),
      y1 = a.y + 35,
      y2 = b.y + 35,
      top = Math.min(...cards.value.map((c) => c.y)) - 45 - index * 34;
    if (a.id === b.id)
      return [
        {
          id: r.id,
          name: r.name,
          label: `${r.effect} → ${r.command}`,
          path: `M ${a.x + 270} ${y1} C ${a.x + 350} ${top},${a.x - 80} ${top},${a.x} ${y1}`,
          x: a.x + 135,
          y: top + 8,
          minX: a.x - 90,
          maxX: a.x + 360,
          minY: top - 25,
          maxY: y1,
          index,
        },
      ];
    const d = Math.min(24, Math.max(10, Math.abs(x2 - x1) / 3)),
      out1 = x1 + sign * d,
      out2 = x2 - sign * d;
    return [
      {
        id: r.id,
        name: r.name,
        label: `${r.effect} → ${r.command}`,
        path: `M ${x1} ${y1} L ${out1} ${y1} L ${out1} ${top} L ${out2} ${top} L ${out2} ${y2} L ${x2} ${y2}`,
        x: (x1 + x2) / 2,
        y: top - 8,
        minX: Math.min(x1, x2) - 160,
        maxX: Math.max(x1, x2) + 160,
        minY: top - 35,
        maxY: Math.max(y1, y2),
        index,
      },
    ];
  }),
);
const bounds = computed(() => {
  const x =
      Math.min(
        0,
        ...cards.value.map((c) => c.x),
        ...wires.value.map((w) => w.minX),
      ) - 30,
    y =
      Math.min(
        0,
        ...cards.value.map((c) => c.y),
        ...wires.value.map((w) => w.minY),
      ) - 30;
  return {
    x,
    y,
    w:
      Math.max(
        700,
        ...cards.value.map((c) => c.x + 300),
        ...wires.value.map((w) => w.maxX),
      ) -
      x +
      40,
    h:
      Math.max(
        280,
        ...cards.value.map((c) => c.y + 120),
        ...wires.value.map((w) => w.maxY),
      ) -
      y +
      40,
  };
});
const visible = computed(() => frozen.value || bounds.value);
function point(e: PointerEvent, transform: DOMMatrix) {
  const p = svg.value!.createSVGPoint();
  p.x = e.clientX;
  p.y = e.clientY;
  return p.matrixTransform(transform);
}
function start(id: string, e: PointerEvent) {
  emit("select", id);
  if (props.disabled || e.button !== 0) return;
  const card = cards.value.find((c) => c.id === id)!,
    transform = svg.value!.getScreenCTM()!.inverse(),
    p = point(e, transform);
  frozen.value = { ...bounds.value };
  drag = { id, x: p.x, y: p.y, ox: card.x, oy: card.y, transform };
  svg.value!.setPointerCapture(e.pointerId);
  e.preventDefault();
}
function move(e: PointerEvent) {
  if (!drag || props.disabled) return;
  const p = point(e, drag.transform);
  emit("move", drag.id, {
    x: Math.max(
      -1000000,
      Math.min(1000000, Math.round(drag.ox + p.x - drag.x)),
    ),
    y: Math.max(
      -1000000,
      Math.min(1000000, Math.round(drag.oy + p.y - drag.y)),
    ),
  });
}
function end() {
  drag = undefined;
  frozen.value = null;
}
function short(value: string) {
  return value.length > 35 ? value.slice(0, 33) + "…" : value;
}
</script>
<template>
  <section class="delivery-diagram">
    <div class="delivery-diagram-tools">
      <strong>PINNED OWNERS & TYPED ROUTES</strong
      ><span
        >{{ model.instances.length }} instances ·
        {{ model.routes.length }} routes</span
      ><button
        class="button quiet"
        aria-label="Zoom message diagram out"
        @click="zoom = Math.max(1, zoom - 0.25)"
      >
        −</button
      ><button
        class="button quiet"
        aria-label="Zoom message diagram in"
        @click="zoom = Math.min(3, zoom + 0.25)"
      >
        +</button
      ><button class="button quiet" @click="zoom = 1">
        Fit message diagram
      </button>
    </div>
    <div class="delivery-diagram-scroll">
      <svg
        ref="svg"
        :viewBox="`${visible.x} ${visible.y} ${visible.w} ${visible.h}`"
        :style="{ width: `${zoom * 100}%` }"
        role="img"
        aria-label="Pinned state machine instances connected by typed effect to command routes"
        @pointermove="move"
        @pointerup="end"
        @pointercancel="end"
        @lostpointercapture="end"
      >
        <defs>
          <marker
            id="delivery-arrow"
            markerWidth="8"
            markerHeight="8"
            refX="7"
            refY="4"
            orient="auto"
          >
            <path d="M0,0 L8,4 L0,8" fill="#629b8f" />
          </marker>
        </defs>
        <g v-for="wire in wires" :key="wire.index">
          <path
            :d="wire.path"
            fill="none"
            stroke="#77a99d"
            stroke-width="2"
            marker-end="url(#delivery-arrow)"
          />
          <text
            :x="wire.x"
            :y="wire.y"
            text-anchor="middle"
            class="route-label"
          >
            {{ short(wire.name || wire.id) }}
            <title>{{ wire.id }} · {{ wire.label }}</title>
          </text>
        </g>
        <g
          v-for="card in cards"
          :key="card.id"
          :transform="`translate(${card.x},${card.y})`"
          role="button"
          tabindex="0"
          :aria-label="`Select message instance ${card.id}`"
          @pointerdown="start(card.id, $event)"
          @keydown.enter="emit('select', card.id)"
        >
          <rect
            width="270"
            height="97"
            rx="8"
            :class="{ selected: card.id === selected }"
          />
          <text x="12" y="25" class="instance-title">
            {{ short(card.name || card.id) }}
          </text>
          <text x="12" y="46">{{ short(card.id) }}</text>
          <text x="12" y="66">{{ short(card.machine.machineId) }}</text>
          <text x="12" y="84">
            @ {{ card.machine.version }} ·
            {{ card.machine.digest.slice(0, 10) }}
          </text>
          <title>{{ card.machineName }} · {{ card.machine.digest }}</title>
        </g>
      </svg>
    </div>
    <p class="state-help">
      Drag an instance to arrange the view. Routes declare compatible payload
      mappings; every dispatch and delivery still needs an explicit scenario
      step. Layout participates in the exact model lock.
    </p>
  </section>
</template>
<style scoped>
.delivery-diagram {
  background: #fff;
  border: 1px solid #dce6ed;
  border-radius: 9px;
  overflow: hidden;
  margin: 18px 0;
}
.delivery-diagram-tools {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  font-size: 11px;
  color: #627e90;
  border-bottom: 1px solid #e0e9ee;
}
.delivery-diagram-tools strong {
  margin-right: auto;
}
.delivery-diagram-scroll {
  overflow: auto;
  max-height: 520px;
  background: #f8fbfc;
}
.delivery-diagram svg {
  display: block;
  min-width: 700px;
  min-height: 320px;
  touch-action: none;
  user-select: none;
}
.delivery-diagram rect {
  fill: white;
  stroke: #a6c7c2;
  stroke-width: 1.5;
}
.delivery-diagram rect.selected {
  stroke: #168c78;
  stroke-width: 3;
}
.delivery-diagram g[role="button"] {
  cursor: pointer;
}
.delivery-diagram text {
  font-size: 11px;
  fill: #6a8695;
}
.delivery-diagram .instance-title {
  font-size: 15px;
  font-weight: 600;
  fill: #385c72;
}
.delivery-diagram .route-label {
  fill: #497d70;
  paint-order: stroke;
  stroke: #f8fbfc;
  stroke-width: 4px;
}
.delivery-diagram > p {
  margin: 0;
  padding: 12px 16px;
}
</style>
