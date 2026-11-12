<script setup lang="ts">
import { computed, ref } from "vue";
import type { DataContract } from "../dataTypes";
const props = defineProps<{
  contract: DataContract;
  selected: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  select: [name: string];
  move: [name: string, position: { x: number; y: number }];
}>();
const svg = ref<SVGSVGElement | null>(null),
  zoom = ref(1);
const frozenBounds = ref<{ x: number; y: number; w: number; h: number } | null>(
  null,
);
let drag:
  | {
      name: string;
      x: number;
      y: number;
      ox: number;
      oy: number;
      transform: DOMMatrix;
    }
  | undefined;
const cards = computed(() =>
  props.contract.entities.map((e) => ({
    ...e,
    fields:
      props.contract.records.find((r) => r.name === e.record)?.fields || [],
  })),
);
const wires = computed(() =>
  props.contract.relationships.flatMap((r, index) => {
    const a = cards.value.find((e) => e.name === r.fromEntity),
      b = cards.value.find((e) => e.name === r.toEntity);
    if (!a || !b) return [];
    const lane = props.contract.relationships
      .slice(0, index)
      .filter(
        (k) => k.fromEntity === r.fromEntity && k.toEntity === r.toEntity,
      ).length;
    const forward = a.position.x <= b.position.x,
      sign = forward ? 1 : -1;
    const x1 = a.position.x + (forward ? 280 : 0),
      y1 = a.position.y + 25,
      x2 = b.position.x + (forward ? 0 : 280),
      y2 = b.position.y + 25;
    const top =
      Math.min(...cards.value.map((c) => c.position.y)) - 40 - index * 30;
    if (a.name === b.name) {
      const left = a.position.x,
        right = left + 280,
        selfTop = top - lane * 25;
      return [
        {
          ...r,
          path: `M ${right} ${y1} C ${right + 90} ${selfTop},${left - 90} ${selfTop},${left} ${y2}`,
          x: left + 140,
          y: selfTop + 8,
          minX: left - 100,
          maxX: right + 100,
          minY: selfTop - 15,
          maxY: y1 + 30,
          index,
        },
      ];
    }
    const reach = Math.min(24, Math.max(10, Math.abs(x2 - x1) / 3)),
      out1 = x1 + sign * reach,
      out2 = x2 - sign * reach;
    const path = `M ${x1} ${y1} L ${out1} ${y1} L ${out1} ${top} L ${out2} ${top} L ${out2} ${y2} L ${x2} ${y2}`;
    return [
      {
        ...r,
        path,
        x: (x1 + x2) / 2,
        y: top - 8,
        minX: Math.min(x1, x2) - 140,
        maxX: Math.max(x1, x2) + 140,
        minY: top - 35,
        maxY: Math.max(y1, y2) + 30,
        index,
      },
    ];
  }),
);
const bounds = computed(() => {
  const minX =
      Math.min(
        0,
        ...cards.value.map((e) => e.position.x),
        ...wires.value.map((w) => w.minX),
      ) - 30,
    minY =
      Math.min(
        0,
        ...cards.value.map((e) => e.position.y),
        ...wires.value.map((w) => w.minY),
      ) - 30;
  return {
    x: minX,
    y: minY,
    w:
      Math.max(
        700,
        ...cards.value.map((e) => e.position.x + 310),
        ...wires.value.map((w) => w.maxX),
      ) -
      minX +
      40,
    h:
      Math.max(
        300,
        ...cards.value.map((e) => e.position.y + 90 + e.fields.length * 28),
        ...wires.value.map((w) => w.maxY),
      ) -
      minY +
      40,
  };
});
const visibleBounds = computed(() => frozenBounds.value || bounds.value);
function point(e: PointerEvent, transform: DOMMatrix) {
  const p = svg.value!.createSVGPoint();
  p.x = e.clientX;
  p.y = e.clientY;
  return p.matrixTransform(transform);
}
function start(name: string, e: PointerEvent) {
  emit("select", name);
  if (props.disabled || e.button !== 0) return;
  const card = cards.value.find((c) => c.name === name)!,
    transform = svg.value!.getScreenCTM()!.inverse(),
    p = point(e, transform);
  frozenBounds.value = { ...bounds.value };
  drag = {
    name,
    x: p.x,
    y: p.y,
    ox: card.position.x,
    oy: card.position.y,
    transform,
  };
  svg.value!.setPointerCapture(e.pointerId);
  e.preventDefault();
}
function move(e: PointerEvent) {
  if (!drag || props.disabled) return;
  const p = point(e, drag.transform);
  emit("move", drag.name, {
    x: Math.round(drag.ox + p.x - drag.x),
    y: Math.round(drag.oy + p.y - drag.y),
  });
}
function end() {
  drag = undefined;
  frozenBounds.value = null;
}
</script>
<template>
  <section class="data-graph">
    <div class="graph-tools">
      <strong>ENTITY RELATIONSHIPS</strong
      ><span
        >{{ contract.entities.length }} entities ·
        {{ contract.relationships.length }} foreign keys</span
      ><button
        class="button quiet"
        aria-label="Zoom data diagram out"
        @click="zoom = Math.max(1, zoom - 0.25)"
      >
        −</button
      ><button
        class="button quiet"
        aria-label="Zoom data diagram in"
        @click="zoom = Math.min(3, zoom + 0.25)"
      >
        +</button
      ><button class="button quiet" @click="zoom = 1">Fit diagram</button>
    </div>
    <div class="graph-scroll">
      <svg
        ref="svg"
        :viewBox="`${visibleBounds.x} ${visibleBounds.y} ${visibleBounds.w} ${visibleBounds.h}`"
        :style="{ width: `${zoom * 100}%` }"
        role="img"
        aria-label="Entity record fields, keys and required foreign key relationships"
        @pointermove="move"
        @pointerup="end"
        @pointercancel="end"
        @lostpointercapture="end"
      >
        <defs>
          <marker
            id="data-arrow"
            markerWidth="8"
            markerHeight="8"
            refX="7"
            refY="4"
            orient="auto"
          >
            <path d="M0,0 L8,4 L0,8" fill="#649d91" />
          </marker>
        </defs>
        <g v-for="r in wires" :key="r.index">
          <path
            :d="r.path"
            fill="none"
            stroke="#77a99d"
            stroke-width="2"
            marker-end="url(#data-arrow)"
          />
          <text :x="r.x" :y="r.y" text-anchor="middle" class="relation-label">
            {{ r.name }} · {{ r.minSources }}..{{
              r.maxSources === 0 ? "*" : "1"
            }}
            → 1
          </text>
          <title>
            {{ r.fromEntity }}({{ r.fromFields.join(", ") }}) →
            {{ r.toEntity }}({{ r.toFields.join(", ") }})
          </title>
        </g>
        <g
          v-for="e in cards"
          :key="e.name"
          :transform="`translate(${e.position.x},${e.position.y})`"
          class="entity-card"
          role="button"
          tabindex="0"
          :aria-label="`Select entity ${e.name}`"
          @pointerdown="start(e.name, $event)"
          @keydown.enter="emit('select', e.name)"
        >
          <rect
            width="280"
            :height="76 + e.fields.length * 28"
            rx="8"
            :class="{ selected: e.name === selected }"
          />
          <text x="12" y="24" class="entity-title">{{ e.name }}</text>
          <text x="12" y="44" class="entity-type">{{ e.record }}</text>
          <line x1="0" y1="55" x2="280" y2="55" stroke="#dce7eb" />
          <g v-for="(f, i) in e.fields" :key="f.name">
            <text x="12" :y="76 + i * 28" class="field-key">
              {{
                e.primaryKey.includes(f.name)
                  ? "PK"
                  : e.uniqueKeys.some((k) => k.includes(f.name))
                    ? "UK"
                    : ""
              }}
            </text>
            <text x="42" :y="76 + i * 28" class="field-name">{{ f.name }}</text>
            <text x="268" :y="76 + i * 28" text-anchor="end" class="field-type">
              {{ f.type }}
            </text>
          </g>
        </g>
      </svg>
    </div>
    <p>
      Each source row references exactly one target. The label gives source rows
      per target: 0..* permits many, 1..1 requires exactly one. Drag entity
      cards to arrange the view.
    </p>
  </section>
</template>
<style scoped>
.data-graph {
  border: 1px solid #dce6ed;
  border-radius: 9px;
  background: white;
  overflow: hidden;
  margin-bottom: 18px;
}
.graph-tools {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 10px 16px;
  border-bottom: 1px solid #e1e9ef;
  font-size: 12px;
  color: #627e90;
}
.graph-tools strong {
  margin-right: auto;
  letter-spacing: 0.6px;
}
.graph-tools .button {
  padding: 7px;
}
.graph-scroll {
  overflow: auto;
  max-height: 500px;
  background: #f8fbfc;
}
.graph-scroll svg {
  display: block;
  min-height: 320px;
  min-width: 680px;
  touch-action: none;
  user-select: none;
}
.entity-card {
  cursor: pointer;
}
.entity-card rect {
  fill: white;
  stroke: #a6c7c2;
  stroke-width: 1.5;
}
.entity-card rect.selected {
  stroke: #168c78;
  stroke-width: 3;
}
.entity-title {
  font-size: 16px;
  font-weight: 600;
  fill: #34566c;
}
.entity-type,
.field-type {
  font-size: 12px;
  fill: #698494;
}
.field-name {
  font-size: 13px;
  fill: #45677b;
}
.field-key {
  font-size: 10px;
  font-weight: 700;
  fill: #248e7a;
}
.relation-label {
  font-size: 11px;
  fill: #427969;
  paint-order: stroke;
  stroke: #f8fbfc;
  stroke-width: 4px;
}
.data-graph > p {
  padding: 12px 16px;
  font-size: 12px;
  color: #627e90;
  line-height: 1.7;
  margin: 0;
}
</style>
